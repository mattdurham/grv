// Namespace: goast/ops
package ops_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/lthiery/goast/ops"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestHandleASTRename_Function(t *testing.T) {
	path := writeTempFile(t, "package p\nfunc Add(x, y int) int { return Add(x, y) }\n")

	pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTRename(ctx, emptyReq, ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Sum",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTRename: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool error: %v", result.Content)
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
	if diff == "" {
		t.Error("expected non-empty diff")
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Sum") {
		t.Error("file should contain Sum after rename")
	}
	if strings.Contains(string(data), "Add") {
		t.Error("file should not contain Add after rename")
	}
}

func TestHandleASTRename_DryRun(t *testing.T) {
	src := "package p\nfunc Add(x, y int) int { return Add(x, y) }\n"
	path := writeTempFile(t, src)

	pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTRename(ctx, emptyReq, ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Sum",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTRename: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	diff, _ := resp["diff"].(string)
	if !strings.Contains(diff, "Sum") {
		t.Errorf("diff should contain Sum, got: %q", diff)
	}

	// File should be unchanged
	data, _ := os.ReadFile(path)
	if string(data) != src {
		t.Error("dry_run=true should not modify the file")
	}
}

func TestHandleASTRename_Type(t *testing.T) {
	src := "package p\ntype Dog struct{ Name string }\nfunc NewDog() Dog { return Dog{} }\n"
	path := writeTempFile(t, src)

	pathJSON, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Dog"}})
	result, err := ops.HandleASTRename(ctx, emptyReq, ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Animal",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTRename: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["changed"] != true {
		t.Error("expected changed=true")
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Animal") {
		t.Error("file should contain Animal after rename")
	}
	// "Dog" as a standalone identifier should be gone; "NewDog" (different ident) is unchanged
	if strings.Contains(string(data), "type Dog ") || strings.Contains(string(data), "() Dog ") {
		t.Error("file should not contain Dog type reference after rename")
	}
}

func TestHandleASTRename_Method(t *testing.T) {
	src := "package p\ntype Dog struct{}\nfunc (d *Dog) Greet() {}\nfunc Use() { d := &Dog{}; d.Greet() }\n"
	path := writeTempFile(t, src)

	pathJSON, _ := json.Marshal([]map[string]interface{}{{"kind": "FuncDecl", "name": "Greet", "recv": "*Dog"}})
	result, err := ops.HandleASTRename(ctx, emptyReq, ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Hello",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTRename: %v", err)
	}
	text := resultText(t, result)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["changed"] != true {
		t.Error("expected changed=true")
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Hello") {
		t.Error("file should contain Hello after rename")
	}
	if strings.Contains(string(data), "Greet") {
		t.Error("file should not contain Greet after rename")
	}
}

func TestHandleASTRename_BadPath(t *testing.T) {
	path := writeTempFile(t, "package p\nfunc Foo() {}\n")

	pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "NonExistent"}})
	result, err := ops.HandleASTRename(ctx, emptyReq, ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Bar",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for non-existent path")
	}
}

func TestHandleASTRename_EmptyTo(t *testing.T) {
	src := "package p\nfunc Foo() {}\n"
	path := writeTempFile(t, src)

	pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Foo"}})
	result, err := ops.HandleASTRename(ctx, emptyReq, ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for empty To")
	}

	// File should be unchanged
	data, _ := os.ReadFile(path)
	if string(data) != src {
		t.Error("file should be unchanged when To is empty")
	}
}
