package hooks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_TwoHooks(t *testing.T) {
	dir := t.TempDir()
	toml := `
[[hooks]]
name    = "lth"
command = ["~/bin/lth", "search", "{namespace}"]
scope   = "file"
cache   = true
timeout = "5s"

[[hooks]]
name    = "echo"
command = ["echo", "hi"]
scope   = "file"
cache   = false
timeout = "2s"
`
	if err := os.WriteFile(filepath.Join(dir, "goast.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(configs))
	}

	h := configs[0]
	if h.Name != "lth" {
		t.Errorf("Name: want 'lth', got %q", h.Name)
	}
	if len(h.Command) != 3 || h.Command[0] != "~/bin/lth" || h.Command[1] != "search" || h.Command[2] != "{namespace}" {
		t.Errorf("Command: unexpected %v", h.Command)
	}
	if h.Scope != "file" {
		t.Errorf("Scope: want 'file', got %q", h.Scope)
	}
	if !h.Cache {
		t.Error("Cache: want true")
	}
	if h.Timeout != 5*time.Second {
		t.Errorf("Timeout: want 5s, got %v", h.Timeout)
	}

	h2 := configs[1]
	if h2.Name != "echo" {
		t.Errorf("Name: want 'echo', got %q", h2.Name)
	}
	if len(h2.Command) != 2 || h2.Command[0] != "echo" || h2.Command[1] != "hi" {
		t.Errorf("Command: unexpected %v", h2.Command)
	}
	if h2.Cache {
		t.Error("Cache: want false")
	}
	if h2.Timeout != 2*time.Second {
		t.Errorf("Timeout: want 2s, got %v", h2.Timeout)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	configs, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if configs != nil {
		t.Errorf("expected nil, got %v", configs)
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "goast.toml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	configs, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected empty slice, got %v", configs)
	}
}

func TestLoadConfig_DefaultTimeout(t *testing.T) {
	dir := t.TempDir()
	toml := `
[[hooks]]
name    = "noto"
command = ["echo", "hi"]
scope   = "file"
`
	if err := os.WriteFile(filepath.Join(dir, "goast.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	configs, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(configs))
	}
	if configs[0].Timeout != 5*time.Second {
		t.Errorf("default timeout: want 5s, got %v", configs[0].Timeout)
	}
}

func TestLoadConfig_KindsField(t *testing.T) {
	dir := t.TempDir()
	toml := `
[[hooks]]
name    = "kindshook"
command = ["echo", "hi"]
scope   = "file"
kinds   = ["FuncDecl", "TypeDecl"]
`
	if err := os.WriteFile(filepath.Join(dir, "goast.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	configs, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(configs))
	}
	if len(configs[0].Kinds) != 2 || configs[0].Kinds[0] != "FuncDecl" || configs[0].Kinds[1] != "TypeDecl" {
		t.Errorf("Kinds: unexpected %v", configs[0].Kinds)
	}
}

func TestLoadConfig_WalkUp(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subpkg")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	toml := "[[hooks]]\nname = \"walkup\"\ncommand = [\"echo\", \"hi\"]\nscope = \"file\"\n"
	if err := os.WriteFile(filepath.Join(root, "goast.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	configs, err := LoadConfig(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 || configs[0].Name != "walkup" {
		t.Errorf("expected walkup hook from parent dir, got %v", configs)
	}
}

func TestLoadConfig_SkipsHookWithNoName(t *testing.T) {
	dir := t.TempDir()
	toml := `
[[hooks]]
command = ["echo", "invalid"]
scope   = "file"

[[hooks]]
name    = "valid"
command = ["echo", "ok"]
scope   = "file"
`
	if err := os.WriteFile(filepath.Join(dir, "goast.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	configs, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 hook (unnamed skipped), got %d", len(configs))
	}
	if configs[0].Name != "valid" {
		t.Errorf("expected 'valid', got %q", configs[0].Name)
	}
}
func TestLoadConfig_ImmutableField(t *testing.T) {
	dir := t.TempDir()
	toml := `
[[hooks]]
name      = "lth"
command   = ["echo", "hi"]
scope     = "file"
immutable = true

[[hooks]]
name    = "echo"
command = ["echo", "bye"]
scope   = "file"
`
	if err := os.WriteFile(filepath.Join(dir, "goast.toml"), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}
	configs, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(configs))
	}
	if !configs[0].Immutable {
		t.Error("Immutable: want true for first hook")
	}
	if configs[1].Immutable {
		t.Error("Immutable: want false for second hook")
	}
}
