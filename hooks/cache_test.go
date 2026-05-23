package hooks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_GetMiss(t *testing.T) {
	c := NewCache()
	result, ok := c.Get("hook", "file")
	if ok || result != nil {
		t.Errorf("expected miss, got ok=%v result=%v", ok, result)
	}
}

func TestCache_SetThenGet(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.go")
	if err := os.WriteFile(f, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}

	c := NewCache()
	data := map[string]interface{}{"k": "v"}
	c.Set("hook", f, data, fi.ModTime())

	result, ok := c.Get("hook", f)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if result["k"] != "v" {
		t.Errorf("result: want 'v', got %v", result["k"])
	}
}

func TestCache_MtimeStaleness(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.go")
	if err := os.WriteFile(f, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCache()
	oldMtime := time.Now().Add(-time.Hour)
	c.Set("hook", f, map[string]interface{}{"k": "v"}, oldMtime)

	// Advance the file's mtime so it's newer than what's cached
	newMtime := time.Now()
	if err := os.Chtimes(f, newMtime, newMtime); err != nil {
		t.Fatal(err)
	}

	result, ok := c.Get("hook", f)
	if ok || result != nil {
		t.Errorf("expected stale miss, got ok=%v", ok)
	}
}

func TestCache_GetReturnsNilIfFileGone(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "gone.go")
	if err := os.WriteFile(f, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCache()
	c.Set("hook", f, map[string]interface{}{"x": 1}, time.Now())

	if err := os.Remove(f); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	result, ok := c.Get("hook", f)
	if ok || result != nil {
		t.Errorf("expected miss after file deleted, got ok=%v", ok)
	}
}

func TestCache_InvalidateClearsAllHooks(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.go")
	if err := os.WriteFile(f, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(f)

	c := NewCache()
	c.Set("hook1", f, map[string]interface{}{"a": 1}, fi.ModTime())
	c.Set("hook2", f, map[string]interface{}{"b": 2}, fi.ModTime())

	c.Invalidate(f)

	if _, ok := c.Get("hook1", f); ok {
		t.Error("hook1 should be evicted after Invalidate")
	}
	if _, ok := c.Get("hook2", f); ok {
		t.Error("hook2 should be evicted after Invalidate")
	}
}

func TestCache_InvalidateDoesNotAffectOtherFile(t *testing.T) {
	dir := t.TempDir()
	fA := filepath.Join(dir, "a.go")
	fB := filepath.Join(dir, "b.go")
	for _, f := range []string{fA, fB} {
		if err := os.WriteFile(f, []byte("package p\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	fiA, _ := os.Stat(fA)
	fiB, _ := os.Stat(fB)

	c := NewCache()
	c.Set("hook", fA, map[string]interface{}{"a": 1}, fiA.ModTime())
	c.Set("hook", fB, map[string]interface{}{"b": 2}, fiB.ModTime())

	c.Invalidate(fA)

	if _, ok := c.Get("hook", fA); ok {
		t.Error("fileA should be evicted")
	}
	if _, ok := c.Get("hook", fB); !ok {
		t.Error("fileB should still be cached")
	}
}
