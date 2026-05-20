// Namespace: goast/ops
package ops_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lthiery/goast/ops"
	"github.com/mark3labs/mcp-go/mcp"
)

var (
	readonlyCtx     = context.Background()
	readonlyEmptyReq = mcp.CallToolRequest{}
)

func TestIsReadonly_NormalFile(t *testing.T) {
	// A regular temp file is not readonly
	f, err := os.CreateTemp(t.TempDir(), "test*.go")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	result, err := ops.HandleFileRead(readonlyCtx, readonlyEmptyReq, ops.FileReadArgs{File: f.Name()})
	if err != nil {
		t.Fatal(err)
	}
	var resp ops.FileReadResult
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Readonly {
		t.Errorf("expected readonly=false for temp file, got true")
	}
}

func TestIsReadonly_VendorPath(t *testing.T) {
	// A path containing /vendor/ should be readonly
	vendorPath := filepath.Join(t.TempDir(), "vendor", "pkg", "file.go")
	if err := os.MkdirAll(filepath.Dir(vendorPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vendorPath, []byte("package pkg\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// file_write should refuse it
	result, err := ops.HandleFileWrite(readonlyCtx, readonlyEmptyReq, ops.FileWriteArgs{
		File:    vendorPath,
		Content: "package pkg // modified\n",
		DryRun:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected tool error for vendor path write, got success")
	}

	// file_read should succeed but show readonly=true
	readResult, err := ops.HandleFileRead(readonlyCtx, readonlyEmptyReq, ops.FileReadArgs{File: vendorPath})
	if err != nil {
		t.Fatal(err)
	}
	tc, ok := readResult.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	var resp ops.FileReadResult
	if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Readonly {
		t.Error("expected readonly=true for vendor path")
	}
}

func TestIsReadonly_GOROOTFile(t *testing.T) {
	goroot := runtime.GOROOT()
	if goroot == "" {
		t.Skip("GOROOT not set")
	}
	// Pick a known stdlib file
	stdlibFile := filepath.Join(goroot, "src", "fmt", "print.go")
	if _, err := os.Stat(stdlibFile); os.IsNotExist(err) {
		t.Skip("stdlib file not found at", stdlibFile)
	}

	readResult, err := ops.HandleFileRead(readonlyCtx, readonlyEmptyReq, ops.FileReadArgs{File: stdlibFile})
	if err != nil {
		t.Fatal(err)
	}
	tc, ok := readResult.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	var resp ops.FileReadResult
	if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Readonly {
		t.Errorf("expected readonly=true for GOROOT file %s", stdlibFile)
	}
}

func TestIsReadonly_WriteToolsEnforced(t *testing.T) {
	// Verify that ast_insert refuses vendor paths
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
	result, err := ops.HandleASTInsert(readonlyCtx, readonlyEmptyReq, ops.ASTInsertArgs{
		File:   vendorFile,
		Path:   path,
		Index:  0,
		Node:   node,
		DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected tool error for ast_insert on vendor file")
	}
}
