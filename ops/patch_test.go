package ops_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mattdurham/grv/hooks"
	"github.com/mattdurham/grv/ops"
)

// patchSrc is a minimal Go file used across patch tests.
const patchSrc = `package x

func Greet(name string) string {
	return name
}
`

// buildPatchPath constructs the JSON path argument for ast_patch.
func buildPatchPath(t *testing.T, steps []map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(steps)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// decodePatchResp unmarshals a {changed, diff} response.
func decodePatchResp(t *testing.T, raw json.RawMessage) (changed bool, diff string) {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if v, ok := m["changed"]; ok {
		_ = json.Unmarshal(v, &changed)
	}
	if v, ok := m["diff"]; ok {
		_ = json.Unmarshal(v, &diff)
	}
	return
}

// TestPatch_SetOp renames a function via set on the "name" field.
func TestPatch_SetOp(t *testing.T) {
	path := writeTemp(t, patchSrc)

	raw, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File: path,
		Path: buildPatchPath(t, []map[string]any{
			{"kind": "FuncDecl", "name": "Greet"},
		}),
		Ops: []ops.PatchOp{
			{Op: "set", Field: "name", Value: json.RawMessage(`"Hello"`)},
		},
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTPatch: %v", err)
	}
	changed, diff := decodePatchResp(t, raw)
	if !changed {
		t.Error("expected changed=true")
	}
	if !strings.Contains(diff, "Hello") {
		t.Errorf("diff should mention Hello, got:\n%s", diff)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Hello") {
		t.Error("file should contain Hello after set")
	}
	if strings.Contains(string(data), "Greet") {
		t.Error("file should not contain Greet after rename")
	}
}

// TestPatch_AppendOp appends a return statement to a BlockStmt.
func TestPatch_AppendOp(t *testing.T) {
	src := `package x

func Items() []string {
	return []string{"a"}
}
`
	path := writeTemp(t, src)

	newStmt := json.RawMessage(`{
		"kind": "ExprStmt",
		"x": {"kind": "CallExpr", "fun": {"kind": "Ident", "name": "println"}, "args": [], "ellipsis": false}
	}`)

	raw, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File: path,
		Path: buildPatchPath(t, []map[string]any{
			{"kind": "FuncDecl", "name": "Items"},
			{"kind": "Body"},
		}),
		Ops: []ops.PatchOp{
			{Op: "append", Field: "list", Value: newStmt},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTPatch append: %v", err)
	}
	changed, diff := decodePatchResp(t, raw)
	if !changed {
		t.Error("expected changed=true for append")
	}
	if !strings.Contains(diff, "println") {
		t.Errorf("diff should mention appended call, got:\n%s", diff)
	}
	// dry_run=true: file must be untouched.
	data, _ := os.ReadFile(path)
	if string(data) != src {
		t.Error("dry_run=true should not modify the file")
	}
}

// TestPatch_PrependOp prepends a statement to the front of a BlockStmt.
func TestPatch_PrependOp(t *testing.T) {
	src := `package x

func Run() {
	println("second")
}
`
	path := writeTemp(t, src)

	firstStmt := json.RawMessage(`{
		"kind": "ExprStmt",
		"x": {"kind": "CallExpr", "fun": {"kind": "Ident", "name": "println"}, "args": [{"kind": "BasicLit", "tok": "STRING", "value": "\"first\""}], "ellipsis": false}
	}`)

	raw, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File: path,
		Path: buildPatchPath(t, []map[string]any{
			{"kind": "FuncDecl", "name": "Run"},
			{"kind": "Body"},
		}),
		Ops: []ops.PatchOp{
			{Op: "prepend", Field: "list", Value: firstStmt},
		},
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTPatch prepend: %v", err)
	}
	changed, diff := decodePatchResp(t, raw)
	if !changed {
		t.Error("expected changed=true for prepend")
	}
	if !strings.Contains(diff, "first") {
		t.Errorf("diff should mention prepended stmt, got:\n%s", diff)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	firstIdx := strings.Index(content, `"first"`)
	secondIdx := strings.Index(content, `"second"`)
	if firstIdx < 0 || secondIdx < 0 {
		t.Fatalf("both strings should appear in file, got:\n%s", content)
	}
	if firstIdx > secondIdx {
		t.Error("prepended statement should appear before original statement")
	}
}

// TestPatch_DeleteFieldOp removes a scalar field from a node.
func TestPatch_DeleteFieldOp(t *testing.T) {
	// A FuncDecl with a doc comment that we can strip by deleting the "doc" field.
	// We navigate to the FuncDecl and delete its "recv" (nil for a plain func, but
	// we target "doc" which is also nil — so instead we use a method to have a real recv).
	src := `package x

type T struct{}

func (t *T) Act() {}
`
	path := writeTemp(t, src)

	raw, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File: path,
		Path: buildPatchPath(t, []map[string]any{
			{"kind": "FuncDecl", "name": "Act"},
		}),
		Ops: []ops.PatchOp{
			{Op: "delete", Field: "recv"},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("HandleASTPatch delete field: %v", err)
	}
	changed, _ := decodePatchResp(t, raw)
	if !changed {
		t.Error("expected changed=true when deleting recv field")
	}
	// dry_run=true: file must be untouched.
	data, _ := os.ReadFile(path)
	if string(data) != src {
		t.Error("dry_run=true should not modify the file")
	}
}

// TestPatch_DeleteByIndexOp removes a specific element from a list field.
func TestPatch_DeleteByIndexOp(t *testing.T) {
	src := `package x

func Multi() {
	_ = 1
	_ = 2
	_ = 3
}
`
	path := writeTemp(t, src)

	// Navigate to the BlockStmt and delete the middle statement (index 1) from "list".
	raw, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File: path,
		Path: buildPatchPath(t, []map[string]any{
			{"kind": "FuncDecl", "name": "Multi"},
			{"kind": "Body"},
		}),
		Ops: []ops.PatchOp{
			{Op: "delete", Field: "list", Index: 1},
		},
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("HandleASTPatch delete by index: %v", err)
	}
	changed, _ := decodePatchResp(t, raw)
	if !changed {
		t.Error("expected changed=true after deleting element")
	}
	data, _ := os.ReadFile(path)
	// After deleting the middle statement the body should have 2 statements.
	// Count occurrences of "_ =" as a proxy.
	count := strings.Count(string(data), "_ =")
	if count != 2 {
		t.Errorf("expected 2 statements after delete, file:\n%s", string(data))
	}
}

// TestPatch_BadPath returns an error and leaves the file unchanged.
func TestPatch_BadPath(t *testing.T) {
	path := writeTemp(t, patchSrc)
	before, _ := os.ReadFile(path)

	_, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File: path,
		Path: buildPatchPath(t, []map[string]any{
			{"kind": "FuncDecl", "name": "DoesNotExist"},
		}),
		Ops: []ops.PatchOp{
			{Op: "set", Field: "name", Value: json.RawMessage(`"X"`)},
		},
		DryRun: false,
	})
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("file should be unchanged after bad-path error")
	}
}

// TestPatch_UnknownOp returns an error before touching the file.
func TestPatch_UnknownOp(t *testing.T) {
	path := writeTemp(t, patchSrc)
	before, _ := os.ReadFile(path)

	_, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File: path,
		Path: buildPatchPath(t, []map[string]any{
			{"kind": "FuncDecl", "name": "Greet"},
		}),
		Ops: []ops.PatchOp{
			{Op: "frobnicate", Field: "name", Value: json.RawMessage(`"X"`)},
		},
		DryRun: false,
	})
	if err == nil {
		t.Fatal("expected error for unknown op")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("file should be unchanged after unknown-op error")
	}
}

// TestPatch_EmptyOps returns {changed:false} without touching the file.
func TestPatch_EmptyOps(t *testing.T) {
	path := writeTemp(t, patchSrc)
	before, _ := os.ReadFile(path)

	_, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File:   path,
		Path:   buildPatchPath(t, []map[string]any{{"kind": "FuncDecl", "name": "Greet"}}),
		Ops:    []ops.PatchOp{},
		DryRun: false,
	})
	// HandleASTPatch treats empty ops as an error (guards against no-op writes).
	if err == nil {
		t.Fatal("expected error for empty ops slice")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("file should be unchanged for empty ops")
	}
}

// TestPatch_CheckViolationRestoresFile verifies that when a patch produces a
// check violation the file is rolled back to its original state.
func TestPatch_CheckViolationRestoresFile(t *testing.T) {
	// Start from a clean file that passes error_handled.
	src := `package x

import "fmt"

func Safe() {
	fmt.Println("ok")
}
`
	path := writeTemp(t, src)
	before, _ := os.ReadFile(path)

	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{"error_handled"}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })

	// Replace the BlockStmt list with a statement that assigns only one result
	// of fmt.Println (discarding the error), which triggers error_handled.
	// Pattern: n, _ := fmt.Println("hi") followed by _ = n.
	badList := json.RawMessage(`[
		{"kind":"AssignStmt","lhs":[{"kind":"Ident","name":"n"},{"kind":"Ident","name":"_"}],"tok":":=","rhs":[{"kind":"CallExpr","fun":{"kind":"SelectorExpr","x":{"kind":"Ident","name":"fmt"},"sel":"Println"},"args":[{"kind":"BasicLit","tok":"STRING","value":"\"hi\""}],"ellipsis":false}]},
		{"kind":"AssignStmt","lhs":[{"kind":"Ident","name":"_"}],"tok":"=","rhs":[{"kind":"Ident","name":"n"}]}
	]`)

	_, err := ops.HandleASTPatch(ops.ASTPatchArgs{
		File: path,
		Path: buildPatchPath(t, []map[string]any{
			{"kind": "FuncDecl", "name": "Safe"},
			{"kind": "Body"},
		}),
		Ops: []ops.PatchOp{
			{Op: "set", Field: "list", Value: badList},
		},
		DryRun: false,
	})
	if err == nil {
		t.Fatal("expected enforcePostWrite to return an error for check violation")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Errorf("file should be restored to original after check violation\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
