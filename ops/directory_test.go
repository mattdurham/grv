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

func TestHandleASTDirectory_Basic(t *testing.T) {
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata"})
	if err != nil {
		t.Fatal(err)
	}
	text := dirText(t, result, err)
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.GoFiles) == 0 {
		t.Error("expected Go files in testdata/")
	}
}

func TestHandleASTDirectory_GoSymbols(t *testing.T) {
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata"})
	if err != nil {
		t.Fatal(err)
	}
	text := dirText(t, result, err)
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Find simple.go entry
	var simpleEntry *ops.GoFileEntry
	for i := range resp.GoFiles {
		if resp.GoFiles[i].File == "simple.go" {
			simpleEntry = &resp.GoFiles[i]
			break
		}
	}
	if simpleEntry == nil {
		t.Fatal("simple.go not found in directory listing")
	}

	// Verify Dog struct is present
	foundDog := false
	for _, s := range simpleEntry.Structs {
		if s.Name == "Dog" {
			foundDog = true
			break
		}
	}
	if !foundDog {
		t.Errorf("expected Dog struct in simple.go, structs: %v", simpleEntry.Structs)
	}

	// Verify Add function is present
	foundAdd := false
	for _, f := range simpleEntry.Functions {
		if f.Name == "Add" {
			foundAdd = true
			break
		}
	}
	if !foundAdd {
		t.Errorf("expected Add function in simple.go, functions: %v", simpleEntry.Functions)
	}

	// Verify interface is present
	if len(simpleEntry.Interfaces) == 0 {
		t.Error("expected at least one interface in simple.go")
	}
}

func TestHandleASTDirectory_NonGoFiles(t *testing.T) {
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata"})
	if err != nil {
		t.Fatal(err)
	}
	text := dirText(t, result, err)
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
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

func TestHandleASTDirectory_Subdirs(t *testing.T) {
	// With recursive=false, Subdirs should list immediate subdirectories.
	f := false
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata", Recursive: &f})
	if err != nil {
		t.Fatal(err)
	}
	text := dirText(t, result, err)
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
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

func TestHandleASTDirectory_ReadonlyField(t *testing.T) {
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata"})
	if err != nil {
		t.Fatal(err)
	}
	text := dirText(t, result, err)
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, f := range resp.GoFiles {
		if f.Readonly {
			t.Errorf("testdata/%s should not be readonly", f.File)
		}
	}
}

func TestHandleASTDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	text := dirText(t, result, err)
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.GoFiles) != 0 || len(resp.NonGoFiles) != 0 {
		t.Error("expected empty results for empty dir")
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

func TestHandleASTDirectory_GoSymbolsDetailed(t *testing.T) {
	// Exercises parseGoFile more deeply — methods, globals
	result, err := ops.HandleASTDirectory(ops.ASTDirectoryArgs{Dir: "../testdata"})
	if err != nil {
		t.Fatal(err)
	}
	text := dirText(t, result, err)
	var resp ops.ASTDirectoryResult
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, f := range resp.GoFiles {
		if f.File != "simple.go" {
			continue
		}
		// Should have methods (Sound, Greet on *Dog)
		hasMethod := false
		for _, fn := range f.Functions {
			if fn.Recv != "" {
				hasMethod = true
				break
			}
		}
		if !hasMethod {
			t.Error("expected at least one method (with receiver) in simple.go")
		}
		// Package name must be set
		if f.Package == "" {
			t.Error("expected non-empty package name")
		}
	}
}
