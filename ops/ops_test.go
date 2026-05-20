// Namespace: goast/ops
package ops_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lthiery/goast/ops"
	"github.com/mark3labs/mcp-go/mcp"
)

var ctx = context.Background()
var emptyReq = mcp.CallToolRequest{}

const testdataSimple = "../testdata/simple.go"

// copyToTemp copies a file to a temp dir and returns the new path.
func copyToTemp(t *testing.T, src string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, filepath.Base(src))
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return dst
}

// resultText extracts the text content from a CallToolResult.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		return ""
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func TestHandleASTList(t *testing.T) {
	result, err := ops.HandleASTList(ctx, emptyReq, ops.ASTListArgs{File: testdataSimple})
	if err != nil {
		t.Fatalf("HandleASTList: %v", err)
	}
	text := resultText(t, result)

	var items []ops.ASTListItem
	if err := json.Unmarshal([]byte(text), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected non-empty item list")
	}

	// Verify we find FuncDecl entries with expected names
	funcNames := map[string]bool{}
	typeNames := map[string]bool{}
	for _, item := range items {
		switch item.Kind {
		case "FuncDecl":
			funcNames[item.Name] = true
		case "TypeDecl":
			typeNames[item.Name] = true
		}
		if item.Line <= 0 {
			t.Errorf("item %s/%s has zero line", item.Kind, item.Name)
		}
	}

	for _, name := range []string{"Add", "Fibonacci", "SafeDivide"} {
		if !funcNames[name] {
			t.Errorf("expected FuncDecl %q in list", name)
		}
	}
	if !typeNames["Dog"] {
		t.Errorf("expected TypeDecl Dog in list")
	}

	// Check method receiver is populated
	for _, item := range items {
		if item.Name == "Sound" && item.Kind == "FuncDecl" {
			if item.Recv != "*Dog" {
				t.Errorf("Sound recv: got %q, want *Dog", item.Recv)
			}
		}
	}
}

func TestHandleASTQuery(t *testing.T) {
	path, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTQuery(ctx, emptyReq, ops.ASTQueryArgs{
		File: testdataSimple,
		Path: path,
	})
	if err != nil {
		t.Fatalf("HandleASTQuery: %v", err)
	}
	text := resultText(t, result)

	var resp ops.ASTQueryResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// Verify the node is a FuncDecl
	var peek struct{ Kind string `json:"kind"` }
	if err := json.Unmarshal(resp.Node, &peek); err != nil {
		t.Fatalf("unmarshal node: %v", err)
	}
	if peek.Kind != "FuncDecl" {
		t.Errorf("node kind: got %q, want FuncDecl", peek.Kind)
	}

	// Verify source is populated
	if !strings.Contains(resp.Source, "Add") {
		t.Errorf("source should contain Add, got: %q", resp.Source)
	}

	// Verify meta is populated
	if resp.Meta["line"] == nil {
		t.Error("meta.line should be present")
	}
}

func TestHandleASTQueryEmptyPath(t *testing.T) {
	result, err := ops.HandleASTQuery(ctx, emptyReq, ops.ASTQueryArgs{
		File: testdataSimple,
		Path: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("HandleASTQuery empty path: %v", err)
	}
	text := resultText(t, result)

	// Empty path → file-level meta in ASTQueryResponse
	var resp ops.ASTQueryResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Meta["package"] == nil {
		t.Error("expected package in file-level meta")
	}
}

func TestHandleASTInsert(t *testing.T) {
	tmpFile := copyToTemp(t, testdataSimple)

	// Insert a ReturnStmt at index 0 in Add's body
	path, _ := json.Marshal([]map[string]string{
		{"kind": "FuncDecl", "name": "Add"},
		{"kind": "Body"},
	})
	// Insert: return 0
	retNode, _ := json.Marshal(map[string]interface{}{
		"kind": "ReturnStmt",
		"results": []interface{}{
			map[string]interface{}{"kind": "BasicLit", "tok": "INT", "value": "0"},
		},
	})

	result, err := ops.HandleASTInsert(ctx, emptyReq, ops.ASTInsertArgs{
		File:   tmpFile,
		Path:   path,
		Index:  0,
		Node:   retNode,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTInsert: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["changed"] != true {
		t.Error("expected changed=true")
	}
	diff, _ := resp["diff"].(string)
	if !strings.Contains(diff, "return 0") {
		t.Errorf("diff should contain 'return 0', got:\n%s", diff)
	}
}

func TestHandleASTInsertDryRunNoWrite(t *testing.T) {
	tmpFile := copyToTemp(t, testdataSimple)
	originalData, _ := os.ReadFile(tmpFile)

	path, _ := json.Marshal([]map[string]string{
		{"kind": "FuncDecl", "name": "Add"},
		{"kind": "Body"},
	})
	retNode, _ := json.Marshal(map[string]interface{}{
		"kind":    "ReturnStmt",
		"results": []interface{}{map[string]interface{}{"kind": "Ident", "name": "x"}},
	})

	_, err := ops.HandleASTInsert(ctx, emptyReq, ops.ASTInsertArgs{
		File:   tmpFile,
		Path:   path,
		Index:  0,
		Node:   retNode,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTInsert: %v", err)
	}

	// File should be unchanged when dry_run=true
	afterData, _ := os.ReadFile(tmpFile)
	if string(originalData) != string(afterData) {
		t.Error("dry_run=true should not modify the file")
	}
}

func TestHandleASTReplace(t *testing.T) {
	tmpFile := copyToTemp(t, testdataSimple)

	// Replace the return statement in Add with "return 99"
	// Add body: [return a + b]  → replace Stmt[0] with return 99
	path, _ := json.Marshal([]map[string]interface{}{
		{"kind": "FuncDecl", "name": "Add"},
		{"kind": "Body"},
		{"kind": "Stmt", "index": 0},
	})
	newNode, _ := json.Marshal(map[string]interface{}{
		"kind": "ReturnStmt",
		"results": []interface{}{
			map[string]interface{}{"kind": "BasicLit", "tok": "INT", "value": "99"},
		},
	})

	result, err := ops.HandleASTReplace(ctx, emptyReq, ops.ASTReplaceArgs{
		File:   tmpFile,
		Path:   path,
		Node:   newNode,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTReplace: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["changed"] != true {
		t.Error("expected changed=true")
	}
	diff, _ := resp["diff"].(string)
	if !strings.Contains(diff, "99") {
		t.Errorf("diff should contain 99, got:\n%s", diff)
	}
}

func TestHandleASTDelete(t *testing.T) {
	tmpFile := copyToTemp(t, testdataSimple)

	// Delete Stmt[0] from Add's body (the only return statement)
	path, _ := json.Marshal([]map[string]interface{}{
		{"kind": "FuncDecl", "name": "Add"},
		{"kind": "Body"},
		{"kind": "Stmt", "index": 0},
	})

	result, err := ops.HandleASTDelete(ctx, emptyReq, ops.ASTDeleteArgs{
		File:   tmpFile,
		Path:   path,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTDelete: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["changed"] != true {
		t.Error("expected changed=true")
	}
	diff, _ := resp["diff"].(string)
	if !strings.Contains(diff, "-") {
		t.Errorf("diff should contain removal lines, got:\n%s", diff)
	}
}

func TestHandleAddImport(t *testing.T) {
	tmpFile := copyToTemp(t, testdataSimple)

	result, err := ops.HandleAddImport(ctx, emptyReq, ops.AddImportArgs{
		File: tmpFile,
		Path: "strings",
	})
	if err != nil {
		t.Fatalf("HandleAddImport: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["changed"] != true {
		t.Errorf("expected changed=true, got %v", resp["changed"])
	}
	diff, _ := resp["diff"].(string)
	if !strings.Contains(diff, "strings") {
		t.Errorf("diff should mention strings import, got:\n%s", diff)
	}

	// File should now contain strings import
	data, _ := os.ReadFile(tmpFile)
	if !strings.Contains(string(data), `"strings"`) {
		t.Error("file should contain strings import after add")
	}
}

func TestHandleAddImportAlreadyPresent(t *testing.T) {
	tmpFile := copyToTemp(t, testdataSimple)

	// fmt is already imported in simple.go
	result, err := ops.HandleAddImport(ctx, emptyReq, ops.AddImportArgs{
		File: tmpFile,
		Path: "fmt",
	})
	if err != nil {
		t.Fatalf("HandleAddImport: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Should be no-op since fmt is already present
	if resp["changed"] == true {
		t.Error("expected changed=false for already-present import")
	}
}

func TestHandleDeleteImport(t *testing.T) {
	tmpFile := copyToTemp(t, testdataSimple)

	result, err := ops.HandleDeleteImport(ctx, emptyReq, ops.DeleteImportArgs{
		File: tmpFile,
		Path: "fmt",
	})
	if err != nil {
		t.Fatalf("HandleDeleteImport: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	diff, _ := resp["diff"].(string)
	if !strings.Contains(diff, "fmt") {
		t.Errorf("diff should mention fmt, got:\n%s", diff)
	}
}

func TestHandleListImports(t *testing.T) {
	result, err := ops.HandleListImports(ctx, emptyReq, ops.ListImportsArgs{File: testdataSimple})
	if err != nil {
		t.Fatalf("HandleListImports: %v", err)
	}
	text := resultText(t, result)

	var imports []ops.ImportInfo
	if err := json.Unmarshal([]byte(text), &imports); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(imports) == 0 {
		t.Fatal("expected non-empty imports list")
	}

	// simple.go imports "fmt"
	found := false
	for _, imp := range imports {
		if imp.Path == "fmt" {
			found = true
			if !imp.Used {
				t.Error("fmt should be marked as used")
			}
		}
	}
	if !found {
		t.Error("expected fmt in imports list")
	}
}
