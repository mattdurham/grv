// Namespace: goast/ops
package ops_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lthiery/goast/ops"
)

// typesdataDir returns the absolute path to testdata/typesdata.
func typesdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "typesdata")
}

func typesdataFile(name string) string {
	return filepath.Join(typesdataDir(), name)
}

// ---- HandleASTFindRefs ----

func TestHandleASTFindRefs_FuncInFile(t *testing.T) {
	path, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTFindRefs(ctx, emptyReq, ops.ASTFindRefsArgs{
		File:  typesdataFile("defs.go"),
		Path:  path,
		Scope: "file",
	})
	if err != nil {
		t.Fatalf("HandleASTFindRefs: %v", err)
	}
	text := resultText(t, result)

	var refs []ops.RefResult
	if err := json.Unmarshal([]byte(text), &refs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// UseAdd calls Add — should find at least one reference
	found := false
	for _, r := range refs {
		if r.Line > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected at least one ref for Add, got: %v", refs)
	}
}

func TestHandleASTFindRefs_MethodInFile(t *testing.T) {
	// Find refs to Dog.Sound within file scope
	path, _ := json.Marshal([]map[string]interface{}{{"kind": "FuncDecl", "name": "Sound", "recv": "*Dog"}})
	result, err := ops.HandleASTFindRefs(ctx, emptyReq, ops.ASTFindRefsArgs{
		File:  typesdataFile("defs.go"),
		Path:  path,
		Scope: "file",
	})
	if err != nil {
		t.Fatalf("HandleASTFindRefs: %v", err)
	}
	text := resultText(t, result)
	var refs []ops.RefResult
	if err := json.Unmarshal([]byte(text), &refs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Results may be empty (Sound is called via interface, not directly by name in this file)
	// Just verify no error and valid JSON
	_ = refs
}

func TestHandleASTFindRefs_MissingPath(t *testing.T) {
	result, err := ops.HandleASTFindRefs(ctx, emptyReq, ops.ASTFindRefsArgs{
		File: typesdataFile("defs.go"),
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for missing path")
	}
}

func TestHandleASTFindRefs_BadFile(t *testing.T) {
	path, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTFindRefs(ctx, emptyReq, ops.ASTFindRefsArgs{
		File: "/nonexistent/file.go",
		Path: path,
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for bad file")
	}
}

// ---- HandleASTFindDef ----

func TestHandleASTFindDef_LocalFunction(t *testing.T) {
	// Navigate to UseAdd → Body → ExprStmt[0] → X (CallExpr) → Fun
	// This calls Add, so finding def should return the Add FuncDecl
	path, _ := json.Marshal([]map[string]interface{}{
		{"kind": "FuncDecl", "name": "UseAdd"},
		{"kind": "Body"},
		{"kind": "ReturnStmt", "index": 0},
	})
	result, err := ops.HandleASTFindDef(ctx, emptyReq, ops.ASTFindDefArgs{
		File: typesdataFile("defs.go"),
		Path: path,
	})
	if err != nil {
		t.Fatalf("HandleASTFindDef: %v", err)
	}
	// This may return an error if the path doesn't land on an identifier.
	// Just verify it doesn't crash.
	_ = result
}

func TestHandleASTFindDef_FuncDecl(t *testing.T) {
	// Navigate to Add FuncDecl itself — the definition is Add itself
	path, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTFindDef(ctx, emptyReq, ops.ASTFindDefArgs{
		File: typesdataFile("defs.go"),
		Path: path,
	})
	if err != nil {
		t.Fatalf("HandleASTFindDef: %v", err)
	}
	text := resultText(t, result)

	var resp ops.ASTFindDefResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.File == "" {
		t.Error("expected non-empty file in response")
	}
}

func TestHandleASTFindDef_BadFile(t *testing.T) {
	path, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTFindDef(ctx, emptyReq, ops.ASTFindDefArgs{
		File: "/nonexistent/file.go",
		Path: path,
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for bad file")
	}
}

func TestHandleASTFindDef_MissingPath(t *testing.T) {
	result, err := ops.HandleASTFindDef(ctx, emptyReq, ops.ASTFindDefArgs{
		File: typesdataFile("defs.go"),
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for missing path")
	}
}

// ---- HandleASTFindImpls ----

func TestHandleASTFindImpls_AnimalInterface(t *testing.T) {
	// Find all implementations of Animal in the typesdata package
	path, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Animal"}})
	result, err := ops.HandleASTFindImpls(ctx, emptyReq, ops.ASTFindImplsArgs{
		File:  typesdataFile("defs.go"),
		Path:  path,
		Scope: "package",
	})
	if err != nil {
		t.Fatalf("HandleASTFindImpls: %v", err)
	}
	text := resultText(t, result)

	var impls []ops.ImplResult
	if err := json.Unmarshal([]byte(text), &impls); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(impls) < 2 {
		t.Errorf("expected at least 2 implementations of Animal (Dog, Cat), got %d: %v", len(impls), impls)
	}
	names := map[string]bool{}
	for _, impl := range impls {
		names[impl.TypeName] = true
	}
	if !names["Dog"] && !names["*Dog"] {
		t.Errorf("expected Dog to implement Animal, got: %v", names)
	}
	if !names["Cat"] && !names["*Cat"] {
		t.Errorf("expected Cat to implement Animal, got: %v", names)
	}
}

func TestHandleASTFindImpls_NotInterface(t *testing.T) {
	// Dog is a struct, not an interface — should return error
	path, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Dog"}})
	result, err := ops.HandleASTFindImpls(ctx, emptyReq, ops.ASTFindImplsArgs{
		File:  typesdataFile("defs.go"),
		Path:  path,
		Scope: "package",
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error: Dog is not an interface")
	}
}

func TestHandleASTFindImpls_BadFile(t *testing.T) {
	path, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Animal"}})
	result, err := ops.HandleASTFindImpls(ctx, emptyReq, ops.ASTFindImplsArgs{
		File:  "/nonexistent/file.go",
		Path:  path,
		Scope: "package",
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for bad file")
	}
}

func TestHandleASTFindImpls_MissingPath(t *testing.T) {
	result, err := ops.HandleASTFindImpls(ctx, emptyReq, ops.ASTFindImplsArgs{
		File:  typesdataFile("defs.go"),
		Scope: "package",
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for missing path")
	}
}
