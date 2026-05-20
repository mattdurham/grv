// Namespace: goast/ops
package ops_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattdurham/grv/ops"
)

func fileText(t *testing.T, result json.RawMessage, err error) string {
	t.Helper()
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	return string(result)
}

func TestHandleFileRead_Basic(t *testing.T) {
	result, err := ops.HandleFileRead(ops.FileReadArgs{File: testdataSimple})
	if err != nil {
		t.Fatal(err)
	}
	text := fileText(t, result, err)
	var resp ops.FileReadResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(resp.Content, "package testdata") {
		t.Errorf("expected Go source content, got: %q...", resp.Content[:50])
	}
	if resp.Size <= 0 {
		t.Errorf("expected size > 0, got %d", resp.Size)
	}
	if resp.Readonly {
		t.Errorf("expected readonly=false for testdata file")
	}
}

func TestHandleFileRead_NotFound(t *testing.T) {
	_, err := ops.HandleFileRead(ops.FileReadArgs{File: "/no/such/file.txt"})
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestHandleFileRead_EmptyPath(t *testing.T) {
	_, err := ops.HandleFileRead(ops.FileReadArgs{File: ""})
	if err == nil {
		t.Error("expected error for empty file path")
	}
}

func TestHandleFileWrite_CreateNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	result, err := ops.HandleFileWrite(ops.FileWriteArgs{
		File:    path,
		Content: "hello world\n",
		DryRun:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := fileText(t, result, err)
	var resp ops.FileWriteResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Changed {
		t.Error("expected changed=true for new file")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello world\n" {
		t.Errorf("file content mismatch: %q", string(got))
	}
}

func TestHandleFileWrite_Overwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(path, []byte("key: old\n"), 0644)

	result, err := ops.HandleFileWrite(ops.FileWriteArgs{
		File:    path,
		Content: "key: new\n",
		DryRun:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := fileText(t, result, err)
	var resp ops.FileWriteResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Changed {
		t.Error("expected changed=true")
	}
	if !strings.Contains(resp.Diff, "new") {
		t.Errorf("diff should mention new content, got: %s", resp.Diff)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "key: new\n" {
		t.Errorf("file not updated: %q", string(got))
	}
}

func TestHandleFileWrite_DryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	original := `{"v":1}`
	os.WriteFile(path, []byte(original), 0644)

	result, err := ops.HandleFileWrite(ops.FileWriteArgs{
		File:    path,
		Content: `{"v":2}`,
		DryRun:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := fileText(t, result, err)
	var resp ops.FileWriteResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Changed {
		t.Error("expected changed=true (diff should exist)")
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("dry_run should not modify file; got %q", string(got))
	}
}

func TestHandleFileWrite_NoChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "same.txt")
	content := "unchanged\n"
	os.WriteFile(path, []byte(content), 0644)

	result, err := ops.HandleFileWrite(ops.FileWriteArgs{
		File:    path,
		Content: content,
		DryRun:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := fileText(t, result, err)
	var resp ops.FileWriteResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Changed {
		t.Error("expected changed=false for identical content")
	}
}

func TestHandleFileWrite_EmptyPath(t *testing.T) {
	_, err := ops.HandleFileWrite(ops.FileWriteArgs{File: "", Content: "x"})
	if err == nil {
		t.Error("expected error for empty file path")
	}
}
