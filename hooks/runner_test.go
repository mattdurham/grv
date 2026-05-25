package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func makeTestFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "test.go")
	if err := os.WriteFile(f, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestRunner_RunFile_BasicJSON(t *testing.T) {
	absFile := makeTestFile(t)
	cfg := HookConfig{
		Name:    "myhook",
		Command: []string{"sh", "-c", `echo '{"answer":42}'`},
		Scope:   "file",
		Cache:   false,
		Timeout: 5 * time.Second,
	}
	r := New([]HookConfig{cfg}, "")
	result := r.RunFile(absFile)
	if result["myhook.answer"] != float64(42) {
		t.Errorf("want myhook.answer=42, got %v (full result: %v)", result["myhook.answer"], result)
	}
}

func TestRunner_RunFile_CacheHit(t *testing.T) {
	absFile := makeTestFile(t)
	counterFile := filepath.Join(t.TempDir(), "counter.txt")

	cfg := HookConfig{
		Name:    "counted",
		Command: []string{"sh", "-c", `printf '{}' && echo x >> ` + counterFile},
		Scope:   "file",
		Cache:   true,
		Timeout: 5 * time.Second,
	}
	r := New([]HookConfig{cfg}, "")

	r.RunFile(absFile)
	r.RunFile(absFile)

	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("counter file not written: %v", err)
	}
	// Should have only 1 line (second call was a cache hit)
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("expected subprocess to run exactly once, counter has %d lines", lines)
	}
}

func TestRunner_RunFile_Timeout(t *testing.T) {
	absFile := makeTestFile(t)
	cfg := HookConfig{
		Name:    "slow",
		Command: []string{"sleep", "10"},
		Scope:   "file",
		Cache:   false,
		Timeout: 1 * time.Millisecond,
	}
	r := New([]HookConfig{cfg}, "")
	result := r.RunFile(absFile)
	if _, ok := result["slow.error"]; !ok {
		t.Errorf("expected slow.error key, got %v", result)
	}
}

func TestRunner_RunFile_NonZeroExit(t *testing.T) {
	absFile := makeTestFile(t)
	cfg := HookConfig{
		Name:    "failing",
		Command: []string{"sh", "-c", "exit 1"},
		Scope:   "file",
		Cache:   false,
		Timeout: 5 * time.Second,
	}
	r := New([]HookConfig{cfg}, "")
	result := r.RunFile(absFile)
	if _, ok := result["failing.error"]; !ok {
		t.Errorf("expected failing.error key, got %v", result)
	}
}

func TestRunner_RunFile_InvalidJSON(t *testing.T) {
	absFile := makeTestFile(t)
	cfg := HookConfig{
		Name:    "badjson",
		Command: []string{"echo", "not json"},
		Scope:   "file",
		Cache:   false,
		Timeout: 5 * time.Second,
	}
	r := New([]HookConfig{cfg}, "")
	result := r.RunFile(absFile)
	if _, ok := result["badjson.error"]; !ok {
		t.Errorf("expected badjson.error key, got %v", result)
	}
}

func TestRunner_RunFile_EmptyOutput(t *testing.T) {
	absFile := makeTestFile(t)
	cfg := HookConfig{
		Name:    "empty",
		Command: []string{"true"},
		Scope:   "file",
		Cache:   false,
		Timeout: 5 * time.Second,
	}
	r := New([]HookConfig{cfg}, "")
	result := r.RunFile(absFile)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestRunner_Invalidate(t *testing.T) {
	absFile := makeTestFile(t)
	counterFile := filepath.Join(t.TempDir(), "counter.txt")

	cfg := HookConfig{
		Name:    "inv",
		Command: []string{"sh", "-c", `printf '{}' && echo x >> ` + counterFile},
		Scope:   "file",
		Cache:   true,
		Timeout: 5 * time.Second,
	}
	r := New([]HookConfig{cfg}, "")

	r.RunFile(absFile)
	r.Invalidate(absFile)
	r.RunFile(absFile)

	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("counter file not written: %v", err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("expected subprocess to run twice (after invalidate), got %d lines", lines)
	}
}
func TestRunner_ImmutableHook_CacheHit_SameHash(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := t.TempDir()
	if err := exec.Command("git", "-C", repoDir, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()

	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Test").Run()

	if err := exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	absFile := filepath.Join(repoDir, "test.go")
	if err := os.WriteFile(absFile, []byte("package test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	counterFile := filepath.Join(t.TempDir(), "counter.txt")
	cfg := HookConfig{Name: "counted", Command: []string{"sh", "-c", `printf '{}' && echo x >> ` + counterFile}, Scope: "file", Immutable: true, Timeout: 5 * time.Second}
	r := New([]HookConfig{cfg}, repoDir)
	r.RunFile(absFile)
	r.RunFile(absFile)
	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("counter file not written: %v", err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("subprocess should run exactly once: want 1 lines, got %d", lines)
	}
}
func TestRunner_ImmutableHook_CacheMiss_NewHash(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := t.TempDir()
	if err := exec.Command("git", "-C", repoDir, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()

	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Test").Run()

	if err := exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	absFile := filepath.Join(repoDir, "test.go")
	if err := os.WriteFile(absFile, []byte("package test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	counterFile := filepath.Join(t.TempDir(), "counter.txt")
	cfg := HookConfig{Name: "counted", Command: []string{"sh", "-c", `printf '{}' && echo x >> ` + counterFile}, Scope: "file", Immutable: true, Timeout: 5 * time.Second}
	r := New([]HookConfig{cfg}, repoDir)
	r.RunFile(absFile)
	if err := exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "second").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	r.RunFile(absFile)
	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("counter file not written: %v", err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("subprocess should run twice (new hash forced miss): want 2 lines, got %d", lines)
	}
}
func TestRunner_ImmutableHook_Degrades_EmptyRepoRoot(t *testing.T) {
	absFile := makeTestFile(t)
	counterFile := filepath.Join(t.TempDir(), "counter.txt")
	cfg := HookConfig{Name: "counted", Command: []string{"sh", "-c", `printf '{}' && echo x >> ` + counterFile}, Scope: "file", Immutable: true, Timeout: 5 * time.Second}
	r := New([]HookConfig{cfg}, "")
	r.RunFile(absFile)
	r.RunFile(absFile)
	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("counter file not written: %v", err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("subprocess should run twice (no caching without repoRoot): want 2 lines, got %d", lines)
	}
}
func TestRunner_Invalidate_ClearsBothCaches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repoDir := t.TempDir()
	if err := exec.Command("git", "-C", repoDir, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()

	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Test").Run()

	if err := exec.Command("git", "-C", repoDir, "commit", "--allow-empty", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	absFile := filepath.Join(repoDir, "test.go")
	if err := os.WriteFile(absFile, []byte("package test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	counterFile := filepath.Join(t.TempDir(), "counter.txt")
	cfgMtime := HookConfig{Name: "mtime_hook", Command: []string{"sh", "-c", `printf '{}' && echo x >> ` + counterFile}, Scope: "file", Cache: true, Timeout: 5 * time.Second}
	cfgImm := HookConfig{Name: "imm_hook", Command: []string{"sh", "-c", `printf '{}' && echo x >> ` + counterFile}, Scope: "file", Immutable: true, Timeout: 5 * time.Second}
	r := New([]HookConfig{cfgMtime, cfgImm}, repoDir)
	r.RunFile(absFile)
	r.Invalidate(absFile)
	r.RunFile(absFile)
	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("counter file not written: %v", err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 4 {
		t.Errorf("both hooks should run twice after Invalidate: want 4 lines, got %d", lines)
	}
}
