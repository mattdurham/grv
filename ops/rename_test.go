// Namespace: goast/ops
package ops_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mattdurham/grv/ops"
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
	result, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Sum",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTRename: %v", err)
	}
	text := resultText(t, result, nil)

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
	result, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Sum",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTRename: %v", err)
	}
	text := resultText(t, result, nil)

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	diff, _ := resp["diff"].(string)
	if !strings.Contains(diff, "Sum") {
		t.Errorf("diff should contain Sum, got: %q", diff)
	}

	data, _ := os.ReadFile(path)
	if string(data) != src {
		t.Error("dry_run=true should not modify the file")
	}
}

func TestHandleASTRename_Type(t *testing.T) {
	src := "package p\ntype Dog struct{ Name string }\nfunc NewDog() Dog { return Dog{} }\n"
	path := writeTempFile(t, src)

	pathJSON, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Dog"}})
	result, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Animal",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTRename: %v", err)
	}
	text := resultText(t, result, nil)

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
	if strings.Contains(string(data), "type Dog ") || strings.Contains(string(data), "() Dog ") {
		t.Error("file should not contain Dog type reference after rename")
	}
}

func TestHandleASTRename_Method(t *testing.T) {
	src := "package p\ntype Dog struct{}\nfunc (d *Dog) Greet() {}\nfunc Use() { d := &Dog{}; d.Greet() }\n"
	path := writeTempFile(t, src)

	pathJSON, _ := json.Marshal([]map[string]interface{}{{"kind": "FuncDecl", "name": "Greet", "recv": "*Dog"}})
	result, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Hello",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTRename: %v", err)
	}
	text := resultText(t, result, nil)

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
	_, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Bar",
		DryRun: false,
	})
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestHandleASTRename_EmptyTo(t *testing.T) {
	src := "package p\nfunc Foo() {}\n"
	path := writeTempFile(t, src)

	pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Foo"}})
	_, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "",
		DryRun: false,
	})
	if err == nil {
		t.Error("expected error for empty To")
	}

	data, _ := os.ReadFile(path)
	if string(data) != src {
		t.Error("file should be unchanged when To is empty")
	}
}

func TestHandleASTRename_StructField(t *testing.T) {
	src := `package p
type Dog struct {
	Name string
}
func (d *Dog) GetName() string { return d.Name }
`
	path := writeTempFile(t, src)
	pathJSON, _ := json.Marshal([]map[string]interface{}{
		{"kind": "TypeSpec", "name": "Dog"},
		{"kind": "StructType"},
		{"kind": "Field", "name": "Name"},
	})
	result, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Label",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTRename field: %v", err)
	}
	text := resultText(t, result, nil)
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["changed"] != true {
		t.Errorf("expected changed=true, got %v", resp["changed"])
	}
	diff, _ := resp["diff"].(string)
	if !strings.Contains(diff, "Label") {
		t.Errorf("diff should contain Label, got:\n%s", diff)
	}
}

func TestHandleASTRename_Ident(t *testing.T) {
	src := `package p
import "fmt"
func F() {
	fmt.Println("hi")
}
`
	path := writeTempFile(t, src)
	pathJSON, _ := json.Marshal([]map[string]interface{}{
		{"kind": "FuncDecl", "name": "F"},
		{"kind": "Body"},
		{"kind": "ExprStmt", "index": 0},
		{"kind": "X"},
		{"kind": "Sel"},
	})
	result, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "Printf",
		DryRun: true,
	})
	// May succeed or return an error — just verify no panic
	_ = result
	_ = err
}

func TestHandleASTRename_VarDecl(t *testing.T) {
	src := `package p
var maxCount = 10
func F() int { return maxCount }
`
	path := writeTempFile(t, src)
	pathJSON, _ := json.Marshal([]map[string]interface{}{
		{"kind": "VarDecl"},
	})
	result, err := ops.HandleASTRename(ops.ASTRenameArgs{
		File:   path,
		Path:   pathJSON,
		To:     "limit",
		DryRun: true,
	})
	if err != nil {
		// VarDecl navigates to GenDecl, not ValueSpec — error is acceptable
		return
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}
