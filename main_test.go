package main

import (
	"encoding/json"
	"testing"
)

func TestParseToolFlags_NoPositional(t *testing.T) {
	result := parseToolFlags("ast_query", []string{"--file", "foo.go"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["namespace"]; ok {
		t.Error("expected no 'namespace' key")
	}
	var file string
	if err := json.Unmarshal(m["file"], &file); err != nil || file != "foo.go" {
		t.Errorf("expected file=foo.go, got %s", file)
	}
}

func TestParseToolFlags_PositionalFirst(t *testing.T) {
	// Tree path: "FuncDecl name=foo" → [{"kind":"FuncDecl","name":"foo"}]
	result := parseToolFlags("ast_query", []string{"hooks", "--path", "FuncDecl name=foo"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil || ns != "hooks" {
		t.Errorf("expected namespace=hooks, got %s", ns)
	}
	var path []map[string]string
	if err := json.Unmarshal(m["path"], &path); err != nil || len(path) != 1 || path[0]["kind"] != "FuncDecl" {
		t.Errorf("expected path=[{kind:FuncDecl,name:foo}], got %s", m["path"])
	}
}

func TestParseToolFlags_PositionalOnly(t *testing.T) {
	result := parseToolFlags("ast_query", []string{"hooks#RunFile"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil || ns != "hooks#RunFile" {
		t.Errorf("expected namespace=hooks#RunFile, got %s", ns)
	}
}

func TestParseToolFlags_OnlyFirstPositionalCaptured(t *testing.T) {
	result := parseToolFlags("ast_query", []string{"hooks", "other"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil || ns != "hooks" {
		t.Errorf("expected namespace=hooks only, got %s", ns)
	}
}

func TestParseToolFlags_BoolFlagWithPositional(t *testing.T) {
	result := parseToolFlags("ast_delete", []string{"hooks", "--dry_run"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil || ns != "hooks" {
		t.Errorf("expected namespace=hooks, got %s", ns)
	}
	var dryRun bool
	if err := json.Unmarshal(m["dry_run"], &dryRun); err != nil || !dryRun {
		t.Error("expected dry_run=true")
	}
}

func TestParseToolFlags_Empty(t *testing.T) {
	result := parseToolFlags("ast_list", []string{})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestParseToolFlags_EmbeddedEquals(t *testing.T) {
	tests := []struct {
		args    []string
		key     string
		wantVal string
	}{
		{[]string{"--dry-run=somevalue"}, "dry_run", "somevalue"},
		{[]string{"--flag="}, "flag", ""},
	}
	for _, tc := range tests {
		result := parseToolFlags("ast_list", tc.args)
		var m map[string]json.RawMessage
		if err := json.Unmarshal(result, &m); err != nil {
			t.Fatalf("args %v: unmarshal: %v", tc.args, err)
		}
		raw, ok := m[tc.key]
		if !ok {
			t.Errorf("args %v: key %q missing from result %s", tc.args, tc.key, result)
			continue
		}
		var got string
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Errorf("args %v: unmarshal value: %v", tc.args, err)
			continue
		}
		if got != tc.wantVal {
			t.Errorf("args %v: key %q = %q, want %q", tc.args, tc.key, got, tc.wantVal)
		}
	}
}

func TestParseToolFlags_TreePath(t *testing.T) {
	result := parseToolFlags("ast_query", []string{"--path", "FuncDecl name=foo / BlockStmt"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var path []map[string]string
	if err := json.Unmarshal(m["path"], &path); err != nil || len(path) != 2 {
		t.Fatalf("expected 2 steps, got %s", m["path"])
	}
	if path[0]["kind"] != "FuncDecl" || path[0]["name"] != "foo" {
		t.Errorf("step 0: want FuncDecl name=foo, got %v", path[0])
	}
	if path[1]["kind"] != "BlockStmt" {
		t.Errorf("step 1: want BlockStmt, got %v", path[1])
	}
}

func TestParseTreePaths_MultiPath(t *testing.T) {
	raw := parseTreePaths("FuncDecl name=foo\n---\nFuncDecl name=bar / BlockStmt")
	if raw == nil {
		t.Fatal("expected non-nil result")
	}
	var paths [][]map[string]string
	if err := json.Unmarshal(raw, &paths); err != nil || len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %s", raw)
	}
}

func TestParsePatchOps_SetAndDelete(t *testing.T) {
	raw, err := parsePatchOps("set name \"newName\"\ndelete recv")
	if err != nil {
		t.Fatal(err)
	}
	var ops []map[string]any
	if err := json.Unmarshal(raw, &ops); err != nil || len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %s", raw)
	}
	if ops[0]["op"] != "set" || ops[0]["field"] != "name" {
		t.Errorf("op 0: %v", ops[0])
	}
	if ops[1]["op"] != "delete" || ops[1]["field"] != "recv" {
		t.Errorf("op 1: %v", ops[1])
	}
}

func TestParseBatchOps_DeleteMany(t *testing.T) {
	raw, err := parseBatchOps("ast_delete_many", "path FuncDecl name=foo\n---\npath FuncDecl name=bar")
	if err != nil {
		t.Fatal(err)
	}
	var ops []map[string]any
	if err := json.Unmarshal(raw, &ops); err != nil || len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %s", raw)
	}
}
