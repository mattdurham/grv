package kinds_test

import (
	"encoding/json"
	"testing"

	"github.com/lthiery/goast/kinds"
)

// roundTripJSON constructs a node struct directly, calls ToAST, calls FromAST on result,
// and verifies the JSON output matches original.
func roundTripJSON(t *testing.T, node kinds.Node, expectedKind string) {
	t.Helper()
	astNode, err := node.ToAST()
	if err != nil {
		t.Fatalf("ToAST: %v", err)
	}
	if astNode == nil {
		t.Fatal("ToAST returned nil")
	}

	// Marshal the original node to JSON
	orig, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal original: %v", err)
	}

	// Round-trip: get the kind string and marshal back
	var peek struct{ Kind string `json:"kind"` }
	if err := json.Unmarshal(orig, &peek); err != nil {
		t.Fatalf("unmarshal peek: %v", err)
	}
	if peek.Kind != expectedKind {
		t.Errorf("kind: got %q, want %q", peek.Kind, expectedKind)
	}

	// Re-marshal from AST
	roundTripped, err := kinds.MarshalNode(astNode)
	if err != nil {
		t.Fatalf("MarshalNode: %v", err)
	}

	// Compare JSON representations
	var origMap, rtMap interface{}
	if err := json.Unmarshal(orig, &origMap); err != nil {
		t.Fatalf("unmarshal orig: %v", err)
	}
	if err := json.Unmarshal(roundTripped, &rtMap); err != nil {
		t.Fatalf("unmarshal roundTripped: %v", err)
	}

	origJSON, _ := json.Marshal(origMap)
	rtJSON, _ := json.Marshal(rtMap)
	if string(origJSON) != string(rtJSON) {
		t.Errorf("round-trip mismatch:\n  orig: %s\n  rt:   %s", origJSON, rtJSON)
	}
}


func mustMarshal(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Expression tests

func TestRoundTrip_Ident(t *testing.T) {
	roundTripJSON(t, &kinds.Ident{KindField: "Ident", Name: "foo"}, "Ident")
}

func TestRoundTrip_BasicLit(t *testing.T) {
	roundTripJSON(t, &kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "42"}, "BasicLit")
	roundTripJSON(t, &kinds.BasicLit{KindField: "BasicLit", Tok: "STRING", Value: `"hello"`}, "BasicLit")
	roundTripJSON(t, &kinds.BasicLit{KindField: "BasicLit", Tok: "FLOAT", Value: "3.14"}, "BasicLit")
}

func TestRoundTrip_BinaryExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "a"})
	y := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "b"})
	roundTripJSON(t, &kinds.BinaryExpr{KindField: "BinaryExpr", X: x, Op: "+", Y: y}, "BinaryExpr")
}

func TestRoundTrip_UnaryExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "x"})
	roundTripJSON(t, &kinds.UnaryExpr{KindField: "UnaryExpr", Op: "-", X: x}, "UnaryExpr")
}

func TestRoundTrip_StarExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "T"})
	roundTripJSON(t, &kinds.StarExpr{KindField: "StarExpr", X: x}, "StarExpr")
}

func TestRoundTrip_ParenExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "x"})
	roundTripJSON(t, &kinds.ParenExpr{KindField: "ParenExpr", X: x}, "ParenExpr")
}

func TestRoundTrip_CallExpr(t *testing.T) {
	fun := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "println"})
	arg := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "x"})
	roundTripJSON(t, &kinds.CallExpr{
		KindField: "CallExpr",
		Fun:       fun,
		Args:      []json.RawMessage{arg},
		Ellipsis:  false,
	}, "CallExpr")
}

func TestRoundTrip_CallExprEllipsis(t *testing.T) {
	fun := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "append"})
	a := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "a"})
	b := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "b"})
	roundTripJSON(t, &kinds.CallExpr{
		KindField: "CallExpr",
		Fun:       fun,
		Args:      []json.RawMessage{a, b},
		Ellipsis:  true,
	}, "CallExpr")
}

func TestRoundTrip_SelectorExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "fmt"})
	roundTripJSON(t, &kinds.SelectorExpr{KindField: "SelectorExpr", X: x, Sel: "Println"}, "SelectorExpr")
}

func TestRoundTrip_IndexExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "arr"})
	idx := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "i"})
	roundTripJSON(t, &kinds.IndexExpr{KindField: "IndexExpr", X: x, Index: idx}, "IndexExpr")
}

func TestRoundTrip_IndexListExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "Map"})
	i1 := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "string"})
	i2 := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	roundTripJSON(t, &kinds.IndexListExpr{
		KindField: "IndexListExpr",
		X:         x,
		Indices:   []json.RawMessage{i1, i2},
	}, "IndexListExpr")
}

func TestRoundTrip_SliceExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "s"})
	low := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "1"})
	high := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "3"})
	max := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "5"})
	// 2-index slice: s[1:3]
	roundTripJSON(t, &kinds.SliceExpr{KindField: "SliceExpr", X: x, Low: low, High: high}, "SliceExpr")
	// 3-index slice: s[1:3:5]
	roundTripJSON(t, &kinds.SliceExpr{KindField: "SliceExpr", X: x, Low: low, High: high, Max: max}, "SliceExpr")
}

func TestRoundTrip_TypeAssertExpr(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "v"})
	typ := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	roundTripJSON(t, &kinds.TypeAssertExpr{KindField: "TypeAssertExpr", X: x, Type: typ}, "TypeAssertExpr")
}

func TestRoundTrip_FuncLit(t *testing.T) {
	funcType := mustMarshal(&kinds.FuncType{
		KindField: "FuncType",
		Params:    []json.RawMessage{},
	})
	body := mustMarshal(&kinds.BlockStmt{
		KindField: "BlockStmt",
		List:      []json.RawMessage{},
	})
	roundTripJSON(t, &kinds.FuncLit{KindField: "FuncLit", Type: funcType, Body: body}, "FuncLit")
}

func TestRoundTrip_CompositeLit(t *testing.T) {
	typ := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "T"})
	elt := mustMarshal(&kinds.KeyValueExpr{
		KindField: "KeyValueExpr",
		Key:       mustMarshal(&kinds.Ident{KindField: "Ident", Name: "X"}),
		Value:     mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "1"}),
	})
	roundTripJSON(t, &kinds.CompositeLit{
		KindField: "CompositeLit",
		Type:      typ,
		Elts:      []json.RawMessage{elt},
	}, "CompositeLit")
}

func TestRoundTrip_KeyValueExpr(t *testing.T) {
	key := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "key"})
	val := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "1"})
	roundTripJSON(t, &kinds.KeyValueExpr{KindField: "KeyValueExpr", Key: key, Value: val}, "KeyValueExpr")
}

func TestRoundTrip_Ellipsis(t *testing.T) {
	elt := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	roundTripJSON(t, &kinds.Ellipsis{KindField: "Ellipsis", Elt: elt}, "Ellipsis")
}

// Type tests

func TestRoundTrip_ArrayType(t *testing.T) {
	// slice (len=null)
	elt := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	roundTripJSON(t, &kinds.ArrayType{KindField: "ArrayType", Elt: elt}, "ArrayType")
	// array (len=N)
	length := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "3"})
	roundTripJSON(t, &kinds.ArrayType{KindField: "ArrayType", Len: length, Elt: elt}, "ArrayType")
}

func TestRoundTrip_StructType(t *testing.T) {
	typ := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	field := mustMarshal(&kinds.Field{
		KindField: "Field",
		Names:     []string{"X"},
		Type:      typ,
	})
	roundTripJSON(t, &kinds.StructType{
		KindField: "StructType",
		Fields:    []json.RawMessage{field},
	}, "StructType")
}

func TestRoundTrip_InterfaceType(t *testing.T) {
	funcType := mustMarshal(&kinds.FuncType{KindField: "FuncType", Params: []json.RawMessage{}})
	method := mustMarshal(&kinds.Field{
		KindField: "Field",
		Names:     []string{"Do"},
		Type:      funcType,
	})
	roundTripJSON(t, &kinds.InterfaceType{
		KindField: "InterfaceType",
		Methods:   []json.RawMessage{method},
	}, "InterfaceType")
}

func TestRoundTrip_FuncType(t *testing.T) {
	typ := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	param := mustMarshal(&kinds.Field{KindField: "Field", Names: []string{"x"}, Type: typ})
	result := mustMarshal(&kinds.Field{KindField: "Field", Type: typ})
	// with type_params
	constraintType := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "any"})
	typeParam := mustMarshal(&kinds.Field{KindField: "Field", Names: []string{"T"}, Type: constraintType})
	roundTripJSON(t, &kinds.FuncType{
		KindField:  "FuncType",
		TypeParams: []json.RawMessage{typeParam},
		Params:     []json.RawMessage{param},
		Results:    []json.RawMessage{result},
	}, "FuncType")
}

func TestRoundTrip_MapType(t *testing.T) {
	key := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "string"})
	val := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	roundTripJSON(t, &kinds.MapType{KindField: "MapType", Key: key, Value: val}, "MapType")
}

func TestRoundTrip_ChanType(t *testing.T) {
	val := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	roundTripJSON(t, &kinds.ChanType{KindField: "ChanType", Dir: "SEND", Value: val}, "ChanType")
	roundTripJSON(t, &kinds.ChanType{KindField: "ChanType", Dir: "RECV", Value: val}, "ChanType")
	roundTripJSON(t, &kinds.ChanType{KindField: "ChanType", Dir: "BOTH", Value: val}, "ChanType")
}

// Field test

func TestRoundTrip_Field(t *testing.T) {
	typ := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	// named
	roundTripJSON(t, &kinds.Field{KindField: "Field", Names: []string{"X", "Y"}, Type: typ}, "Field")
	// unnamed (embedded)
	roundTripJSON(t, &kinds.Field{KindField: "Field", Type: mustMarshal(&kinds.Ident{KindField: "Ident", Name: "Base"})}, "Field")
	// with tag
	tagVal := "`json:\"x\"`"
	roundTripJSON(t, &kinds.Field{KindField: "Field", Names: []string{"X"}, Type: typ, Tag: &tagVal}, "Field")
}

// Statement tests

func TestRoundTrip_BlockStmt(t *testing.T) {
	ret := mustMarshal(&kinds.ReturnStmt{KindField: "ReturnStmt"})
	roundTripJSON(t, &kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{ret}}, "BlockStmt")
}

func TestRoundTrip_IfStmt(t *testing.T) {
	cond := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "true"})
	body := mustMarshal(&kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{}})
	// simple
	roundTripJSON(t, &kinds.IfStmt{KindField: "IfStmt", Cond: cond, Body: body}, "IfStmt")
	// with else
	elseBody := mustMarshal(&kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{}})
	roundTripJSON(t, &kinds.IfStmt{KindField: "IfStmt", Cond: cond, Body: body, Else: elseBody}, "IfStmt")
}

func TestRoundTrip_ForStmt(t *testing.T) {
	body := mustMarshal(&kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{}})
	roundTripJSON(t, &kinds.ForStmt{KindField: "ForStmt", Body: body}, "ForStmt")
}

func TestRoundTrip_RangeStmt(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "items"})
	key := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "i"})
	val := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "v"})
	body := mustMarshal(&kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{}})
	// := variant
	roundTripJSON(t, &kinds.RangeStmt{KindField: "RangeStmt", Key: key, Value: val, Tok: ":=", X: x, Body: body}, "RangeStmt")
	// = variant
	roundTripJSON(t, &kinds.RangeStmt{KindField: "RangeStmt", Key: key, Tok: "=", X: x, Body: body}, "RangeStmt")
}

func TestRoundTrip_SwitchStmt(t *testing.T) {
	tag := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "x"})
	body := mustMarshal(&kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{}})
	roundTripJSON(t, &kinds.SwitchStmt{KindField: "SwitchStmt", Tag: tag, Body: body}, "SwitchStmt")
}

func TestRoundTrip_TypeSwitchStmt(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "v"})
	typeAssert := mustMarshal(&kinds.TypeAssertExpr{KindField: "TypeAssertExpr", X: x})
	assign := mustMarshal(&kinds.ExprStmt{KindField: "ExprStmt", X: typeAssert})
	body := mustMarshal(&kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{}})
	roundTripJSON(t, &kinds.TypeSwitchStmt{KindField: "TypeSwitchStmt", Assign: assign, Body: body}, "TypeSwitchStmt")
}

func TestRoundTrip_SelectStmt(t *testing.T) {
	body := mustMarshal(&kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{}})
	roundTripJSON(t, &kinds.SelectStmt{KindField: "SelectStmt", Body: body}, "SelectStmt")
}

func TestRoundTrip_CaseClause(t *testing.T) {
	expr := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "1"})
	ret := mustMarshal(&kinds.ReturnStmt{KindField: "ReturnStmt"})
	// regular case
	roundTripJSON(t, &kinds.CaseClause{
		KindField: "CaseClause",
		List:      []json.RawMessage{expr},
		Body:      []json.RawMessage{ret},
	}, "CaseClause")
	// default case (list=nil)
	roundTripJSON(t, &kinds.CaseClause{
		KindField: "CaseClause",
		List:      nil,
		Body:      []json.RawMessage{ret},
	}, "CaseClause")
}

func TestRoundTrip_CommClause(t *testing.T) {
	body := []json.RawMessage{}
	roundTripJSON(t, &kinds.CommClause{KindField: "CommClause", Body: body}, "CommClause")
}

func TestRoundTrip_AssignStmt(t *testing.T) {
	lhs := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "x"})
	rhs := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "1"})
	roundTripJSON(t, &kinds.AssignStmt{
		KindField: "AssignStmt",
		Lhs:       []json.RawMessage{lhs},
		Tok:       ":=",
		Rhs:       []json.RawMessage{rhs},
	}, "AssignStmt")
	roundTripJSON(t, &kinds.AssignStmt{
		KindField: "AssignStmt",
		Lhs:       []json.RawMessage{lhs},
		Tok:       "+=",
		Rhs:       []json.RawMessage{rhs},
	}, "AssignStmt")
}

func TestRoundTrip_ReturnStmt(t *testing.T) {
	val := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "x"})
	roundTripJSON(t, &kinds.ReturnStmt{KindField: "ReturnStmt", Results: []json.RawMessage{val}}, "ReturnStmt")
}

func TestRoundTrip_ExprStmt(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "f"})
	call := mustMarshal(&kinds.CallExpr{KindField: "CallExpr", Fun: x, Args: []json.RawMessage{}})
	roundTripJSON(t, &kinds.ExprStmt{KindField: "ExprStmt", X: call}, "ExprStmt")
}

func TestRoundTrip_SendStmt(t *testing.T) {
	ch := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "ch"})
	val := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "1"})
	roundTripJSON(t, &kinds.SendStmt{KindField: "SendStmt", Chan: ch, Value: val}, "SendStmt")
}

func TestRoundTrip_GoStmt(t *testing.T) {
	fun := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "f"})
	call := mustMarshal(&kinds.CallExpr{KindField: "CallExpr", Fun: fun, Args: []json.RawMessage{}})
	roundTripJSON(t, &kinds.GoStmt{KindField: "GoStmt", Call: call}, "GoStmt")
}

func TestRoundTrip_DeferStmt(t *testing.T) {
	fun := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "cleanup"})
	call := mustMarshal(&kinds.CallExpr{KindField: "CallExpr", Fun: fun, Args: []json.RawMessage{}})
	roundTripJSON(t, &kinds.DeferStmt{KindField: "DeferStmt", Call: call}, "DeferStmt")
}

func TestRoundTrip_IncDecStmt(t *testing.T) {
	x := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "i"})
	roundTripJSON(t, &kinds.IncDecStmt{KindField: "IncDecStmt", X: x, Tok: "++"}, "IncDecStmt")
	roundTripJSON(t, &kinds.IncDecStmt{KindField: "IncDecStmt", X: x, Tok: "--"}, "IncDecStmt")
}

func TestRoundTrip_LabeledStmt(t *testing.T) {
	inner := mustMarshal(&kinds.BranchStmt{KindField: "BranchStmt", Tok: "break"})
	roundTripJSON(t, &kinds.LabeledStmt{KindField: "LabeledStmt", Label: "loop", Stmt: inner}, "LabeledStmt")
}

func TestRoundTrip_BranchStmt(t *testing.T) {
	roundTripJSON(t, &kinds.BranchStmt{KindField: "BranchStmt", Tok: "break"}, "BranchStmt")
	roundTripJSON(t, &kinds.BranchStmt{KindField: "BranchStmt", Tok: "continue"}, "BranchStmt")
	roundTripJSON(t, &kinds.BranchStmt{KindField: "BranchStmt", Tok: "fallthrough"}, "BranchStmt")
	label := "outer"
	roundTripJSON(t, &kinds.BranchStmt{KindField: "BranchStmt", Tok: "goto", Label: &label}, "BranchStmt")
}

func TestRoundTrip_DeclStmt(t *testing.T) {
	nameType := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	spec := mustMarshal(&kinds.ValueSpec{KindField: "ValueSpec", Names: []string{"x"}, Type: nameType})
	decl := mustMarshal(&kinds.VarDecl{KindField: "VarDecl", Specs: []json.RawMessage{spec}})
	roundTripJSON(t, &kinds.DeclStmt{KindField: "DeclStmt", Decl: decl}, "DeclStmt")
}

// Declaration tests

func TestRoundTrip_FuncDecl(t *testing.T) {
	// function without receiver
	funcType := mustMarshal(&kinds.FuncType{KindField: "FuncType", Params: []json.RawMessage{}})
	body := mustMarshal(&kinds.BlockStmt{KindField: "BlockStmt", List: []json.RawMessage{}})
	roundTripJSON(t, &kinds.FuncDecl{
		KindField: "FuncDecl",
		Name:      "Foo",
		Type:      funcType,
		Body:      body,
	}, "FuncDecl")

	// method with receiver
	recvType := mustMarshal(&kinds.StarExpr{
		KindField: "StarExpr",
		X:         mustMarshal(&kinds.Ident{KindField: "Ident", Name: "T"}),
	})
	recv := mustMarshal(&kinds.Field{KindField: "Field", Names: []string{"t"}, Type: recvType})
	roundTripJSON(t, &kinds.FuncDecl{
		KindField: "FuncDecl",
		Recv:      recv,
		Name:      "Bar",
		Type:      funcType,
		Body:      body,
	}, "FuncDecl")
}

func TestRoundTrip_ImportDecl(t *testing.T) {
	spec := mustMarshal(&kinds.ImportSpec{KindField: "ImportSpec", Path: "fmt"})
	roundTripJSON(t, &kinds.ImportDecl{KindField: "ImportDecl", Specs: []json.RawMessage{spec}}, "ImportDecl")
	// multi-spec
	spec2 := mustMarshal(&kinds.ImportSpec{KindField: "ImportSpec", Path: "os"})
	roundTripJSON(t, &kinds.ImportDecl{KindField: "ImportDecl", Specs: []json.RawMessage{spec, spec2}}, "ImportDecl")
}

func TestRoundTrip_ConstDecl(t *testing.T) {
	val := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "1"})
	spec := mustMarshal(&kinds.ValueSpec{
		KindField: "ValueSpec",
		Names:     []string{"X"},
		Values:    []json.RawMessage{val},
	})
	roundTripJSON(t, &kinds.ConstDecl{KindField: "ConstDecl", Specs: []json.RawMessage{spec}}, "ConstDecl")
}

func TestRoundTrip_TypeDecl(t *testing.T) {
	underlying := mustMarshal(&kinds.StructType{KindField: "StructType", Fields: []json.RawMessage{}})
	spec := mustMarshal(&kinds.TypeSpec{KindField: "TypeSpec", Name: "MyType", Type: underlying})
	roundTripJSON(t, &kinds.TypeDecl{KindField: "TypeDecl", Specs: []json.RawMessage{spec}}, "TypeDecl")
}

func TestRoundTrip_VarDecl(t *testing.T) {
	typ := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	spec := mustMarshal(&kinds.ValueSpec{KindField: "ValueSpec", Names: []string{"x"}, Type: typ})
	roundTripJSON(t, &kinds.VarDecl{KindField: "VarDecl", Specs: []json.RawMessage{spec}}, "VarDecl")
}

// Spec tests

func TestRoundTrip_ImportSpec(t *testing.T) {
	roundTripJSON(t, &kinds.ImportSpec{KindField: "ImportSpec", Path: "fmt"}, "ImportSpec")
	// with alias
	name := "f"
	roundTripJSON(t, &kinds.ImportSpec{KindField: "ImportSpec", Name: &name, Path: "fmt"}, "ImportSpec")
}

func TestRoundTrip_ValueSpec(t *testing.T) {
	typ := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "int"})
	val := mustMarshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: "42"})
	roundTripJSON(t, &kinds.ValueSpec{
		KindField: "ValueSpec",
		Names:     []string{"x", "y"},
		Type:      typ,
		Values:    []json.RawMessage{val, val},
	}, "ValueSpec")
}

func TestRoundTrip_TypeSpec(t *testing.T) {
	underlying := mustMarshal(&kinds.StructType{KindField: "StructType", Fields: []json.RawMessage{}})
	roundTripJSON(t, &kinds.TypeSpec{KindField: "TypeSpec", Name: "MyType", Type: underlying}, "TypeSpec")
	// with type_params
	constraintType := mustMarshal(&kinds.Ident{KindField: "Ident", Name: "any"})
	typeParam := mustMarshal(&kinds.Field{KindField: "Field", Names: []string{"T"}, Type: constraintType})
	roundTripJSON(t, &kinds.TypeSpec{
		KindField:  "TypeSpec",
		Name:       "Container",
		TypeParams: []json.RawMessage{typeParam},
		Type:       underlying,
	}, "TypeSpec")
}

func TestUnmarshalNode_NullInput(t *testing.T) {
	// null JSON should return nil, nil
	node, err := kinds.UnmarshalNode(json.RawMessage("null"))
	if err != nil {
		t.Fatalf("expected nil error for null input, got %v", err)
	}
	if node != nil {
		t.Fatalf("expected nil node for null input, got %T", node)
	}
}

func TestUnmarshalNode_EmptyInput(t *testing.T) {
	// empty bytes should return nil, nil
	node, err := kinds.UnmarshalNode(json.RawMessage(""))
	if err != nil {
		t.Fatalf("expected nil error for empty input, got %v", err)
	}
	if node != nil {
		t.Fatalf("expected nil node for empty input, got %T", node)
	}
}

func TestUnmarshalNode_UnknownKind(t *testing.T) {
	// unknown kind should return an error
	_, err := kinds.UnmarshalNode(json.RawMessage(`{"kind":"NoSuchKind"}`))
	if err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
}
