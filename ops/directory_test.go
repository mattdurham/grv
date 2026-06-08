// Namespace: goast/ops
package ops_test

import (
	"encoding/json"
	"testing"

	"github.com/mattdurham/grv/ops"
)

func dirText(t *testing.T, result json.RawMessage, err error) string {
	t.Helper()
	if err != nil {
		t.Fatalf("tool returned error: %v", err)
	}
	return string(result)
}

func TestHandleASTDirectory_NonGoFiles(t *testing.T) {
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata"})
	if err != nil {
		t.Fatal(err)
	}
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(dirText(t, result, err)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	foundMod := false
	for _, f := range resp.NonGoFiles {
		if f.File == "test.mod" {
			foundMod = true
			if f.Size <= 0 {
				t.Errorf("expected size > 0 for test.mod")
			}
			break
		}
	}
	if !foundMod {
		t.Errorf("expected test.mod in non_go_files, got: %v", resp.NonGoFiles)
	}
}

func TestHandleASTDirectory_NoGoFiles(t *testing.T) {
	// ast_directory must never return Go files — those belong to AST tools.
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata"})
	if err != nil {
		t.Fatal(err)
	}
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(dirText(t, result, err)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, f := range resp.NonGoFiles {
		if len(f.File) > 3 && f.File[len(f.File)-3:] == ".go" {
			t.Errorf("ast_directory must not return .go files, got: %s", f.File)
		}
	}
}

func TestHandleASTDirectory_Subdirs(t *testing.T) {
	// With recursive=false, Subdirs should list immediate subdirectories.
	f := false
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata", Recursive: &f})
	if err != nil {
		t.Fatal(err)
	}
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(dirText(t, result, err)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	foundTypesdata := false
	for _, d := range resp.Subdirs {
		if d == "typesdata" {
			foundTypesdata = true
			break
		}
	}
	if !foundTypesdata {
		t.Errorf("expected typesdata subdir, got: %v", resp.Subdirs)
	}
}

func TestHandleASTDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(dirText(t, result, err)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.NonGoFiles) != 0 {
		t.Error("expected empty non_go_files for empty dir")
	}
}

func TestHandleASTDirectory_InvalidDir(t *testing.T) {
	_, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "/no/such/dir"})
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestHandleASTDirectory_EmptyArg(t *testing.T) {
	_, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: ""})
	if err == nil {
		t.Error("expected error for empty dir")
	}
}
