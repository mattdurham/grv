package ops_test

import (
	"encoding/json"
	"testing"

	"github.com/mattdurham/grv/ops"
)

type mockRunner struct {
	result map[string]interface{}
}

func (m mockRunner) RunFile(absFile string) map[string]interface{} {
	return m.result
}

func (m mockRunner) Invalidate(absFile string) {}

func withMockRunner(t *testing.T, r ops.RunnerInterface) {
	t.Helper()
	orig := ops.DefaultHookRunner
	t.Cleanup(func() { ops.DefaultHookRunner = orig })
	ops.SetDefaultHookRunner(r)
}

func TestHandleASTList_HookMeta(t *testing.T) {
	withMockRunner(t, mockRunner{result: map[string]interface{}{"lth.memory": "hello"}})

	result, err := ops.HandleASTList(ops.ASTListArgs{File: testdataSimple})
	if err != nil {
		t.Fatalf("HandleASTList: %v", err)
	}

	var items []ops.ASTListItem
	if err := json.Unmarshal(result, &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected items")
	}
	if items[0].Meta == nil {
		t.Fatal("expected Meta to be populated")
	}
	if items[0].Meta["lth.memory"] != "hello" {
		t.Errorf("expected lth.memory=hello in item meta, got %v", items[0].Meta)
	}
}

func TestHandleASTQuery_HookMeta(t *testing.T) {
	withMockRunner(t, mockRunner{result: map[string]interface{}{"lth.memory": "hello"}})

	path, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTQuery(ops.ASTQueryArgs{
		File: testdataSimple,
		Path: path,
	})
	if err != nil {
		t.Fatalf("HandleASTQuery: %v", err)
	}

	var resp ops.ASTQueryResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Meta["lth.memory"] != "hello" {
		t.Errorf("expected lth.memory=hello in resp.Meta, got %v", resp.Meta)
	}
}

func TestHandleASTMeta_HookAllowlist(t *testing.T) {
	withMockRunner(t, mockRunner{result: map[string]interface{}{
		"lth.memory": "hello",
		"other.data": "world",
	}})

	path, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTMeta(ops.ASTMetaArgs{
		File:  testdataSimple,
		Path:  path,
		Hooks: []string{"lth"},
	})
	if err != nil {
		t.Fatalf("HandleASTMeta: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["lth.memory"] != "hello" {
		t.Errorf("expected lth.memory=hello in meta, got %v", m)
	}
	if _, ok := m["other.data"]; ok {
		t.Error("other.data should be filtered out by allowlist")
	}
}

func TestHookRunner_Nil_NoPanic(t *testing.T) {
	withMockRunner(t, nil)

	path, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})
	result, err := ops.HandleASTQuery(ops.ASTQueryArgs{
		File: testdataSimple,
		Path: path,
	})
	if err != nil {
		t.Fatalf("unexpected error with nil runner: %v", err)
	}

	var resp ops.ASTQueryResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Normal meta fields should still be present
	if resp.Meta["line"] == nil {
		t.Error("expected normal meta with nil runner")
	}
}
