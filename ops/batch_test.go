package ops_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattdurham/grv/ops"
)

func writeTempBatch(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func decodeChangedDiff(t *testing.T, raw json.RawMessage) (changed bool, diff string) {
	t.Helper()
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if err := json.Unmarshal(resp["changed"], &changed); err != nil {
		t.Fatalf("decode changed: %v", err)
	}
	if d, ok := resp["diff"]; ok {
		_ = json.Unmarshal(d, &diff)
	}
	return
}

const batchSrc = `package x

func Foo() int {
	return 1
}

func Bar() int {
	return 2
}
`

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHandleASTReplaceMany_TwoOpsSucceed(t *testing.T) {
	path := writeTempBatch(t, batchSrc)

	fooPath := mustMarshal(t, []map[string]any{
		{"kind": "FuncDecl", "name": "Foo"},
		{"kind": "Body"},
		{"kind": "Stmt", "index": 0},
	})
	barPath := mustMarshal(t, []map[string]any{
		{"kind": "FuncDecl", "name": "Bar"},
		{"kind": "Body"},
		{"kind": "Stmt", "index": 0},
	})
	newReturn := mustMarshal(t, map[string]any{
		"kind": "ReturnStmt",
		"results": []map[string]any{
			{"kind": "BasicLit", "tok": "INT", "value": "99"},
		},
	})

	raw, err := ops.HandleASTReplaceMany(ops.ASTReplaceManyArgs{
		File: path,
		Ops: []ops.ReplaceOp{
			{Path: fooPath, Node: newReturn},
			{Path: barPath, Node: newReturn},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	changed, _ := decodeChangedDiff(t, raw)
	if !changed {
		t.Error("expected changed=true")
	}
}

func TestHandleASTReplaceMany_BadPathAbortsWholeFile(t *testing.T) {
	path := writeTempBatch(t, batchSrc)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	goodPath := mustMarshal(t, []map[string]any{
		{"kind": "FuncDecl", "name": "Foo"},
		{"kind": "Body"},
		{"kind": "Stmt", "index": 0},
	})
	badPath := mustMarshal(t, []map[string]any{
		{"kind": "FuncDecl", "name": "DoesNotExist"},
	})
	newReturn := mustMarshal(t, map[string]any{
		"kind": "ReturnStmt",
		"results": []map[string]any{
			{"kind": "BasicLit", "tok": "INT", "value": "99"},
		},
	})

	_, callErr := ops.HandleASTReplaceMany(ops.ASTReplaceManyArgs{
		File: path,
		Ops: []ops.ReplaceOp{
			{Path: goodPath, Node: newReturn},
			{Path: badPath, Node: newReturn},
		},
		DryRun: false,
	})
	if callErr == nil {
		t.Fatal("expected error for bad path, got nil")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("file was modified despite batch failure")
	}
}

func TestHandleASTReplaceMany_EmptyOps(t *testing.T) {
	path := writeTempBatch(t, batchSrc)

	raw, err := ops.HandleASTReplaceMany(ops.ASTReplaceManyArgs{
		File:   path,
		Ops:    []ops.ReplaceOp{},
		DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	changed, diff := decodeChangedDiff(t, raw)
	if changed {
		t.Error("expected changed=false for empty ops")
	}
	if diff != "" {
		t.Errorf("expected empty diff, got %q", diff)
	}
}

func TestHandleASTInsertMany_TwoInsertsSucceed(t *testing.T) {
	path := writeTempBatch(t, batchSrc)

	fooBody := mustMarshal(t, []map[string]any{
		{"kind": "FuncDecl", "name": "Foo"},
		{"kind": "Body"},
	})
	barBody := mustMarshal(t, []map[string]any{
		{"kind": "FuncDecl", "name": "Bar"},
		{"kind": "Body"},
	})
	newStmt := mustMarshal(t, map[string]any{
		"kind": "ExprStmt",
		"x": map[string]any{
			"kind":      "BasicLit",
			"tok":       "INT",
			"value":     "0",
		},
	})

	raw, err := ops.HandleASTInsertMany(ops.ASTInsertManyArgs{
		File: path,
		Ops: []ops.InsertOp{
			{Path: fooBody, Index: 0, Node: newStmt},
			{Path: barBody, Index: 0, Node: newStmt},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	changed, _ := decodeChangedDiff(t, raw)
	if !changed {
		t.Error("expected changed=true")
	}
}

func TestHandleASTDeleteMany_TwoDeletesSucceed(t *testing.T) {
	src := `package x

func Foo() {
	_ = 1
	_ = 2
}
`
	path := writeTempBatch(t, src)

	stmt0 := mustMarshal(t, []map[string]any{
		{"kind": "FuncDecl", "name": "Foo"},
		{"kind": "Body"},
		{"kind": "Stmt", "index": 1},
	})
	stmt1 := mustMarshal(t, []map[string]any{
		{"kind": "FuncDecl", "name": "Foo"},
		{"kind": "Body"},
		{"kind": "Stmt", "index": 0},
	})

	raw, err := ops.HandleASTDeleteMany(ops.ASTDeleteManyArgs{
		File: path,
		Ops: []ops.DeleteOp{
			{Path: stmt0},
			{Path: stmt1},
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	changed, _ := decodeChangedDiff(t, raw)
	if !changed {
		t.Error("expected changed=true")
	}
}
