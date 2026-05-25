package editor_test

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattdurham/grv/editor"
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

func TestParseFile_NonExistent(t *testing.T) {
	_, _, _, err := editor.ParseFile("/no/such/file/definitely_does_not_exist.go")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestParseFile_InvalidSyntax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(path, []byte("package p\nfunc badSyntax( {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := editor.ParseFile(path)
	if err == nil {
		t.Fatal("expected error for invalid syntax, got nil")
	}
}

func TestEdit_FnError(t *testing.T) {
	path := copyTestFile(t, "../testdata/simple.go")
	original, _ := os.ReadFile(path)

	sentinelErr := &testError{"fn failed"}
	_, err := editor.Edit(path, false, func(f *ast.File, fset *token.FileSet) error {
		return sentinelErr
	})
	if err == nil {
		t.Fatal("expected error from fn, got nil")
	}
	if err.Error() != sentinelErr.Error() {
		t.Errorf("error: got %v, want %v", err, sentinelErr)
	}

	after, _ := os.ReadFile(path)
	if string(original) != string(after) {
		t.Error("file should be unchanged when fn returns error")
	}
}

func TestWriteAtomic_ReadonlyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Skip("cannot make dir readonly:", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	path := filepath.Join(dir, "out.go")
	err := editor.WriteAtomic(path, []byte("package p\n"))
	if err == nil {
		t.Error("expected error writing to readonly dir, got nil")
	}
}

// TestEdit_CommentsPreservedOnUnmodifiedNodes verifies that doc and field
// comments on nodes untouched by the mutation survive formatting.
func TestEdit_CommentsPreservedOnUnmodifiedNodes(t *testing.T) {
	path := copyTestFile(t, "../testdata/commented.go")

	// Mutation: rename Validate → Check. Config, NewConfig, and their comments
	// are untouched and must survive.
	_, err := editor.Edit(path, false, func(f *ast.File, fset *token.FileSet) error {
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "Validate" {
				id.Name = "Check"
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)

	for _, want := range []string{
		"// Config holds application settings.",
		"// Host is the server hostname.",
		"// Port is the listening port.",
		"// Timeout in seconds.",
		"// NewConfig returns a default Config.",
		"// Validate checks that the config is valid.", // comment text unchanged, only ident renamed
	} {
		if !strings.Contains(src, want) {
			t.Errorf("comment missing after edit: %q", want)
		}
	}
}

// TestEdit_DocCommentSurvivesBodyReplacement verifies that a function's doc
// comment survives when its body is replaced entirely.
func TestEdit_DocCommentSurvivesBodyReplacement(t *testing.T) {
	path := copyTestFile(t, "../testdata/commented.go")

	_, err := editor.Edit(path, false, func(f *ast.File, fset *token.FileSet) error {
		ast.Inspect(f, func(n ast.Node) bool {
			fd, ok := n.(*ast.FuncDecl)
			if !ok || fd.Name.Name != "Validate" {
				return true
			}
			// Replace body with `return true` unconditionally.
			fd.Body.List = []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{
						&ast.Ident{Name: "true"},
					},
				},
			}
			return false
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)

	if !strings.Contains(src, "// Validate checks that the config is valid.") {
		t.Error("doc comment on Validate lost after body replacement")
	}
	if !strings.Contains(src, "// Config holds application settings.") {
		t.Error("Config doc comment lost after unrelated function body replacement")
	}
}

// TestEdit_FieldCommentsAfterNewField verifies that existing field comments
// survive when a new field (with no position) is inserted into a struct.
func TestEdit_FieldCommentsAfterNewField(t *testing.T) {
	path := copyTestFile(t, "../testdata/commented.go")

	_, err := editor.Edit(path, false, func(f *ast.File, fset *token.FileSet) error {
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Config" {
				return true
			}
			st := ts.Type.(*ast.StructType)
			st.Fields.List = append(st.Fields.List, &ast.Field{
				Names: []*ast.Ident{{Name: "Debug"}},
				Type:  &ast.Ident{Name: "bool"},
			})
			return false
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)

	for _, want := range []string{
		"// Host is the server hostname.",
		"// Port is the listening port.",
		"// Timeout in seconds.",
		"Debug",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("expected %q in output after field insert", want)
		}
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
