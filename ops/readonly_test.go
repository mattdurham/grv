// Namespace: goast/ops
package ops_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattdurham/grv/ops"
)

func TestIsReadonly_NormalFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	result, err := ops.HandleFileRead(ops.FileReadArgs{File: f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	var resp ops.FileReadResult
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Readonly {
		t.Errorf("expected readonly=false for temp file, got true")
	}
}

func TestIsReadonly_VendorPath(t *testing.T) {
	vendorPath := filepath.Join(t.TempDir(), "vendor", "pkg", "data.txt")
	if err := os.MkdirAll(filepath.Dir(vendorPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vendorPath, []byte("vendor data\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, writeErr := ops.HandleFileWrite(ops.FileWriteArgs{
		File:    vendorPath,
		Content: "modified\n",
		DryRun:  false,
	})
	if writeErr == nil {
		t.Error("expected error for vendor path write, got success")
	}

	result, err := ops.HandleFileRead(ops.FileReadArgs{File: vendorPath})
	if err != nil {
		t.Fatal(err)
	}
	var resp ops.FileReadResult
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Readonly {
		t.Error("expected readonly=true for vendor path")
	}
}

func TestIsReadonly_GOROOTFile(t *testing.T) {
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		t.Skip("GOROOT not set")
	}
	stdlibFile := filepath.Join(goroot, "src", "fmt", "print.go")
	if _, err := os.Stat(stdlibFile); os.IsNotExist(err) {
		t.Skip("stdlib file not found at", stdlibFile)
	}
	// .go files are blocked by file_read regardless of readonly status.
	_, err := ops.HandleFileRead(ops.FileReadArgs{File: stdlibFile})
	if err == nil {
		t.Error("expected file_read to reject a .go file")
	}
}

func TestIsReadonly_WriteToolsEnforced(t *testing.T) {
	vendorFile := filepath.Join(t.TempDir(), "vendor", "pkg", "code.go")
	if err := os.MkdirAll(filepath.Dir(vendorFile), 0755); err != nil {
		t.Fatal(err)
	}
	src := "package pkg\nfunc Foo() {}\n"
	if err := os.WriteFile(vendorFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	path, _ := json.Marshal([]map[string]interface{}{
		{"kind": "FuncDecl", "name": "Foo"},
		{"kind": "Body"},
	})
	node, _ := json.Marshal(map[string]interface{}{
		"kind": "ReturnStmt",
	})
	_, err := ops.HandleASTInsert(ops.ASTInsertArgs{
		File:   vendorFile,
		Path:   path,
		Index:  0,
		Node:   node,
		DryRun: false,
	})
	if err == nil {
		t.Error("expected error for ast_insert on vendor file")
	}
}
