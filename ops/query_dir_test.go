package ops_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattdurham/grv/ops"
)

func TestHandleASTList_Dir_Basic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package pkg\n\nfunc Alpha() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package pkg\n\nfunc Beta() {}\n"), 0644)
	result, err := ops.HandleASTList(ops.ASTListArgs{Dir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(result, &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	names := make(map[string]bool)
	for _, item := range items {
		if n, ok := item["name"].(string); ok {
			names[n] = true
		}
	}
	if !names["Alpha"] || !names["Beta"] {
		t.Fatalf("expected Alpha and Beta in results, got %v", names)
	}
}

func TestHandleASTList_Dir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := ops.HandleASTList(ops.ASTListArgs{Dir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(result, &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(items))
	}
}

func TestHandleASTList_Dir_FilePreference(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.go")
	os.WriteFile(filePath, []byte("package pkg\n\nfunc Alpha() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package pkg\n\nfunc Beta() {}\n"), 0644)
	// when both File and Dir are set, File wins
	result, err := ops.HandleASTList(ops.ASTListArgs{File: filePath, Dir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []map[string]interface{}
	json.Unmarshal(result, &items)
	for _, item := range items {
		if n, ok := item["name"].(string); ok && n == "Beta" {
			t.Fatal("Beta should not appear when File takes precedence over Dir")
		}
	}
}

func TestHandleASTList_Dir_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "prod.go"), []byte("package pkg\n\nfunc ProdFunc() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "prod_test.go"), []byte("package pkg\n\nfunc TestHelper() {}\n"), 0644)
	result, err := ops.HandleASTList(ops.ASTListArgs{Dir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(result, &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	names := make(map[string]bool)
	for _, item := range items {
		if n, ok := item["name"].(string); ok {
			names[n] = true
		}
	}
	if names["TestHelper"] {
		t.Error("TestHelper from _test.go should not appear in Dir results")
	}
	if !names["ProdFunc"] {
		t.Error("ProdFunc from prod.go should appear in Dir results")
	}
}

func TestHandleASTList_File_Unchanged(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "only.go")
	os.WriteFile(filePath, []byte("package pkg\n\nfunc OnlyFunc() {}\n"), 0644)
	result, err := ops.HandleASTList(ops.ASTListArgs{File: filePath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []map[string]interface{}
	json.Unmarshal(result, &items)
	found := false
	for _, item := range items {
		if n, ok := item["name"].(string); ok && n == "OnlyFunc" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected OnlyFunc in File-based result")
	}
}
