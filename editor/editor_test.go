package editor_test

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/lthiery/goast/editor"
)

func copyTestFile(t *testing.T, src string) string {
	t.Helper()
	content, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, filepath.Base(src))
	if err := os.WriteFile(dst, content, 0644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return dst
}

func TestEditNoChange(t *testing.T) {
	path := copyTestFile(t, "../testdata/simple.go")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	result, err := editor.Edit(path, false, func(f *ast.File, fset *token.FileSet) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if result.Changed {
		t.Error("expected no change")
	}
	if result.Diff != "" {
		t.Errorf("expected empty diff, got: %q", result.Diff)
	}

	after, _ := os.ReadFile(path)
	if string(original) != string(after) {
		t.Error("file should not have changed")
	}
}

func TestEditWithChange(t *testing.T) {
	path := copyTestFile(t, "../testdata/simple.go")

	result, err := editor.Edit(path, false, func(f *ast.File, fset *token.FileSet) error {
		// rename package name (no-op functionally but changes AST)
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "Add" {
				id.Name = "Sum"
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !result.Changed {
		t.Error("expected change")
	}
	if result.Diff == "" {
		t.Error("expected non-empty diff")
	}

	content, _ := os.ReadFile(path)
	if string(content) == "" {
		t.Error("file should have content")
	}
}

func TestEditDryRun(t *testing.T) {
	path := copyTestFile(t, "../testdata/simple.go")
	original, _ := os.ReadFile(path)

	result, err := editor.Edit(path, true, func(f *ast.File, fset *token.FileSet) error {
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "Add" {
				id.Name = "Sum"
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !result.Changed {
		t.Error("expected change reported")
	}
	if result.Diff == "" {
		t.Error("expected diff in dry run")
	}

	after, _ := os.ReadFile(path)
	if string(original) != string(after) {
		t.Error("dry run must not write file")
	}
}

func TestEditAtomicWrite(t *testing.T) {
	path := copyTestFile(t, "../testdata/simple.go")
	original, _ := os.ReadFile(path)

	_, err := editor.Edit(path, false, func(f *ast.File, fset *token.FileSet) error {
		return &testError{"intentional error"}
	})
	if err == nil {
		t.Fatal("expected error")
	}

	after, _ := os.ReadFile(path)
	if string(original) != string(after) {
		t.Error("original file should be unchanged after error")
	}
}

func TestParseFile(t *testing.T) {
	path := copyTestFile(t, "../testdata/simple.go")
	f, fset, src, err := editor.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if f == nil {
		t.Fatal("expected ast.File")
	}
	if fset == nil {
		t.Fatal("expected token.FileSet")
	}
	if len(src) == 0 {
		t.Fatal("expected source bytes")
	}
	if f.Name.Name != "testdata" {
		t.Errorf("expected package testdata, got %q", f.Name.Name)
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
