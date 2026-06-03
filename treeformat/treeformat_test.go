package treeformat_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mattdurham/grv/treeformat"
)

func mustMarshal(t *testing.T, input string) string {
	t.Helper()
	out, err := treeformat.Marshal(json.RawMessage(input))
	if err != nil {
		t.Fatalf("Marshal(%s): %v", input, err)
	}
	return string(out)
}

func mustUnmarshal(t *testing.T, input string) string {
	t.Helper()
	out, err := treeformat.Unmarshal([]byte(input))
	if err != nil {
		t.Fatalf("Unmarshal(%q): %v", input, err)
	}
	return string(out)
}

func roundTrip(t *testing.T, jsonInput string) {
	t.Helper()
	tree := mustMarshal(t, jsonInput)
	back := mustUnmarshal(t, tree)
	tree2 := mustMarshal(t, back)
	if tree != tree2 {
		t.Errorf("round-trip not stable:\n  first:  %s\n  second: %s", tree, tree2)
	}
}

// 1. Simple Ident
func TestMarshal_SimpleIdent(t *testing.T) {
	out := mustMarshal(t, `{"kind":"Ident","name":"foo"}`)
	if out != "Ident name=foo" {
		t.Errorf("got %q", out)
	}
}

func TestUnmarshal_SimpleIdent(t *testing.T) {
	out := mustUnmarshal(t, "Ident name=foo")
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["kind"] != "Ident" || m["name"] != "foo" {
		t.Errorf("got %s", out)
	}
}

// 2. Scalar array inline
func TestMarshal_ScalarArrayInline(t *testing.T) {
	out := mustMarshal(t, `{"kind":"Field","names":["a","b"]}`)
	if !strings.Contains(out, "names=[a,b]") {
		t.Errorf("expected names=[a,b] in %q", out)
	}
}

func TestRoundTrip_ScalarArray(t *testing.T) {
	roundTrip(t, `{"kind":"Field","names":["a","b"]}`)
}

// 3. Empty array inline
func TestMarshal_EmptyArray(t *testing.T) {
	out := mustMarshal(t, `{"kind":"BlockStmt","list":[]}`)
	if !strings.Contains(out, "list=[]") {
		t.Errorf("expected list=[] in %q", out)
	}
}

func TestRoundTrip_EmptyArray(t *testing.T) {
	roundTrip(t, `{"kind":"BlockStmt","list":[]}`)
}

// 4. Object child (no [] suffix — single object)
func TestMarshal_ObjectChild(t *testing.T) {
	out := mustMarshal(t, `{"kind":"FuncDecl","name":"Run","body":{"kind":"BlockStmt","list":[]}}`)
	if !strings.Contains(out, "body\n") {
		t.Errorf("expected 'body' child block (no []) in:\n%s", out)
	}
	if strings.Contains(out, "body[]") {
		t.Errorf("single-object child must NOT have [] suffix:\n%s", out)
	}
}

func TestRoundTrip_ObjectChild(t *testing.T) {
	roundTrip(t, `{"kind":"FuncDecl","name":"Run","body":{"kind":"BlockStmt","list":[]}}`)
}

// 5. Object array — [] suffix, single-item preserves as array
func TestMarshal_ObjectArraySuffix(t *testing.T) {
	out := mustMarshal(t, `{"kind":"AssignStmt","tok":":=","lhs":[{"kind":"Ident","name":"x"}]}`)
	if !strings.Contains(out, "lhs[]") {
		t.Errorf("expected 'lhs[]' in:\n%s", out)
	}
}

func TestUnmarshal_SingleItemArrayPreserved(t *testing.T) {
	// lhs[] with one item must round-trip as an array, not a bare object.
	back := mustUnmarshal(t, "AssignStmt tok=\":=\"\n  lhs[]\n    Ident name=x")
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(back), &m); err != nil {
		t.Fatal(err)
	}
	lhs, ok := m["lhs"].([]interface{})
	if !ok {
		t.Fatalf("lhs must be []interface{}, got %T: %s", m["lhs"], back)
	}
	if len(lhs) != 1 {
		t.Errorf("lhs should have 1 element, got %d", len(lhs))
	}
}

func TestRoundTrip_ObjectArray(t *testing.T) {
	roundTrip(t, `{"kind":"AssignStmt","tok":":=","lhs":[{"kind":"Ident","name":"x"}]}`)
}

// 6. tok field round-trips as tok (not kind_)
func TestRoundTrip_TokField(t *testing.T) {
	input := `{"kind":"AssignStmt","tok":":=","lhs":[{"kind":"Ident","name":"x"}]}`
	tree := mustMarshal(t, input)
	back := mustUnmarshal(t, tree)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(back), &m); err != nil {
		t.Fatal(err)
	}
	if m["tok"] != ":=" {
		t.Errorf("tok field not preserved: %s", back)
	}
	if _, hasKindUnderscore := m["kind_"]; hasKindUnderscore {
		t.Errorf("tok must not be remapped to kind_: %s", back)
	}
}

// 7. Null field
func TestMarshal_NullField(t *testing.T) {
	out := mustMarshal(t, `{"kind":"IfStmt","cond":{"kind":"Ident","name":"ok"},"init":null}`)
	if !strings.Contains(out, "init=null") {
		t.Errorf("expected init=null inline in:\n%s", out)
	}
}

func TestRoundTrip_NullField(t *testing.T) {
	roundTrip(t, `{"kind":"IfStmt","cond":{"kind":"Ident","name":"ok"},"init":null}`)
}

// 8. Full round-trip for complex nested node
func TestRoundTrip_Complex(t *testing.T) {
	roundTrip(t, `{"kind":"FuncDecl","name":"Run","type":{"kind":"FuncType","params":{"kind":"FieldList","list":[]}},"body":{"kind":"BlockStmt","list":[]}}`)
}
