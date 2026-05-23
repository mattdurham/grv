package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpand_AllVars(t *testing.T) {
	vars := map[string]string{
		"file":      "/repo/pkg/foo.go",
		"dir":       "/repo/pkg",
		"repo_name": "myrepo",
		"repo_path": "/repo",
		"namespace": "github.com/user/myrepo/pkg",
		"pkg":       "pkg",
	}
	args := []string{"{file}", "{dir}", "{repo_name}", "{repo_path}", "{namespace}", "{pkg}"}
	result := Expand(args, vars)
	expected := []string{"/repo/pkg/foo.go", "/repo/pkg", "myrepo", "/repo", "github.com/user/myrepo/pkg", "pkg"}
	for i, got := range result {
		if got != expected[i] {
			t.Errorf("arg[%d]: want %q, got %q", i, expected[i], got)
		}
	}
}

func TestExpand_UnknownVar(t *testing.T) {
	result := Expand([]string{"{unknown}"}, map[string]string{})
	if result[0] != "{unknown}" {
		t.Errorf("unknown var should pass through, got %q", result[0])
	}
}

func TestExpand_TildeFirst(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	result := Expand([]string{"~/bin/lth"}, map[string]string{})
	if !strings.HasPrefix(result[0], home) {
		t.Errorf("tilde expansion: want prefix %q, got %q", home, result[0])
	}
	if !strings.HasSuffix(result[0], "/bin/lth") {
		t.Errorf("tilde expansion: want suffix '/bin/lth', got %q", result[0])
	}
}

func TestExpand_TildeInMiddle(t *testing.T) {
	result := Expand([]string{"foo~/bar"}, map[string]string{})
	if result[0] != "foo~/bar" {
		t.Errorf("tilde in middle should not expand, got %q", result[0])
	}
}

func TestCollectVars_Fields(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "foo.go")
	if err := os.WriteFile(goFile, []byte("package mypkg\n\nfunc Foo() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	vars := CollectVars(goFile, "/repo")
	if vars["file"] != goFile {
		t.Errorf("file: want %q, got %q", goFile, vars["file"])
	}
	if vars["dir"] != filepath.Dir(goFile) {
		t.Errorf("dir: want %q, got %q", filepath.Dir(goFile), vars["dir"])
	}
	if vars["repo_path"] != "/repo" {
		t.Errorf("repo_path: want '/repo', got %q", vars["repo_path"])
	}
	if vars["repo_name"] != "repo" {
		t.Errorf("repo_name: want 'repo', got %q", vars["repo_name"])
	}
	if vars["pkg"] != "mypkg" {
		t.Errorf("pkg: want 'mypkg', got %q", vars["pkg"])
	}
	// namespace may be empty (no go.mod in temp dir) — just verify key exists
	if _, ok := vars["namespace"]; !ok {
		t.Error("namespace key missing from vars")
	}
}

func TestCollectVars_EmptyRepoRoot(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "foo.go")
	if err := os.WriteFile(goFile, []byte("package p\n"), 0644); err != nil {
		t.Fatal(err)
	}
	vars := CollectVars(goFile, "")
	if vars["repo_path"] != "" {
		t.Errorf("repo_path: want '', got %q", vars["repo_path"])
	}
	if vars["repo_name"] != "" {
		t.Errorf("repo_name: want '', got %q", vars["repo_name"])
	}
}
