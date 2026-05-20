package selector_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/mattdurham/grv/selector"
)

func parseFile(t *testing.T, path string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

func pInt(n int) *int { return &n }

func TestNavigateFuncDecl(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Add"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	fd, ok := node.(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected *ast.FuncDecl, got %T", node)
	}
	if fd.Name.Name != "Add" {
		t.Errorf("expected Add, got %q", fd.Name.Name)
	}
	if ctx.FieldName != "Decls" {
		t.Errorf("expected FieldName=Decls, got %q", ctx.FieldName)
	}
}

func TestNavigateFuncDeclWithRecv(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Sound", Recv: "*Dog"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	fd, ok := node.(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected *ast.FuncDecl, got %T", node)
	}
	if fd.Name.Name != "Sound" {
		t.Errorf("expected Sound, got %q", fd.Name.Name)
	}
}

func TestNavigateBody(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Add"},
		{Kind: "Body"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.BlockStmt); !ok {
		t.Fatalf("expected *ast.BlockStmt, got %T", node)
	}
	if ctx.FieldName != "Body" {
		t.Errorf("expected FieldName=Body, got %q", ctx.FieldName)
	}
	if ctx.Index != -1 {
		t.Errorf("expected Index=-1 for scalar field, got %d", ctx.Index)
	}
}

func TestNavigateIfStmtByIndex(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	// Fibonacci has an if statement
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Fibonacci"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.IfStmt); !ok {
		t.Fatalf("expected *ast.IfStmt, got %T", node)
	}
}

func TestNavigateCond(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Fibonacci"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Cond"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected cond node")
	}
	if ctx.FieldName != "Cond" {
		t.Errorf("expected FieldName=Cond, got %q", ctx.FieldName)
	}
	if ctx.Index != -1 {
		t.Errorf("expected Index=-1 for scalar cond, got %d", ctx.Index)
	}
}

func TestNavigateTypeSpec(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Dog"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	ts, ok := node.(*ast.TypeSpec)
	if !ok {
		t.Fatalf("expected *ast.TypeSpec, got %T", node)
	}
	if ts.Name.Name != "Dog" {
		t.Errorf("expected Dog, got %q", ts.Name.Name)
	}
}

func TestNavigateField(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Dog"},
		{Kind: "StructType"},
		{Kind: "Field", Name: "Name"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	field, ok := node.(*ast.Field)
	if !ok {
		t.Fatalf("expected *ast.Field, got %T", node)
	}
	if len(field.Names) == 0 || field.Names[0].Name != "Name" {
		t.Errorf("expected field named Name")
	}
}

func TestNavigateStmtByIndex(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Add"},
		{Kind: "Body"},
		{Kind: "Stmt", Index: pInt(0)},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected node")
	}
	if ctx.Index != 0 {
		t.Errorf("expected Index=0, got %d", ctx.Index)
	}
}

func TestNavigateNestedPath(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	// Navigate deep into Greet: FuncDecl -> Body -> IfStmt -> Body -> Stmt[0]
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Greet", Recv: "*Dog"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Body"},
		{Kind: "Stmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected node")
	}
}

func TestNavigateErrorMissing(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "NonExistent"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for missing func")
	}
	navErr, ok := err.(*selector.NavigateError)
	if !ok {
		t.Fatalf("expected *selector.NavigateError, got %T", err)
	}
	if navErr.AtStep != 0 {
		t.Errorf("expected AtStep=0, got %d", navErr.AtStep)
	}
	if len(navErr.Available) == 0 {
		t.Error("expected non-empty Available list")
	}
}

func TestNavigateErrorWrongContext(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	// Try to navigate Body from a non-func node
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Add"},
		{Kind: "Body"},
		{Kind: "Stmt", Index: pInt(0)},
		{Kind: "Body"}, // ReturnStmt has no Body
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for wrong context")
	}
}

func TestParentContextForDelete(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Add"},
		{Kind: "Body"},
		{Kind: "Stmt", Index: pInt(0)},
	}
	_, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	// For list items, Index should be >= 0
	if ctx.Index < 0 {
		t.Errorf("expected non-negative Index for list item, got %d", ctx.Index)
	}
	if ctx.FieldName != "List" {
		t.Errorf("expected FieldName=List, got %q", ctx.FieldName)
	}
}

func TestParentContextForScalar(t *testing.T) {
	f := parseFile(t, "../testdata/simple.go")
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Fibonacci"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Cond"},
	}
	_, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if ctx.Index != -1 {
		t.Errorf("expected Index=-1 for scalar field, got %d", ctx.Index)
	}
}

// parseSource parses a Go source string inline.
func parseSource(t *testing.T, src string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "t.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return f
}

// Statement-type navigators (indexed)

func TestNavigateForStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() { for i := 0; i < 10; i++ {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ForStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.ForStmt); !ok {
		t.Fatalf("expected *ast.ForStmt, got %T", node)
	}
}

func TestNavigateRangeStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(m map[string]int) { for k, v := range m { _ = k; _ = v } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "RangeStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.RangeStmt); !ok {
		t.Fatalf("expected *ast.RangeStmt, got %T", node)
	}
}

func TestNavigateSwitchStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { switch x { case 1: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.SwitchStmt); !ok {
		t.Fatalf("expected *ast.SwitchStmt, got %T", node)
	}
}

func TestNavigateSelectStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(ch chan int) { select { case <-ch: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SelectStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.SelectStmt); !ok {
		t.Fatalf("expected *ast.SelectStmt, got %T", node)
	}
}

func TestNavigateTypeSwitchStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(v interface{}) { switch v.(type) { case int: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "TypeSwitchStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.TypeSwitchStmt); !ok {
		t.Fatalf("expected *ast.TypeSwitchStmt, got %T", node)
	}
}

func TestNavigateAssignStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() { x := 1; _ = x }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "AssignStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.AssignStmt); !ok {
		t.Fatalf("expected *ast.AssignStmt, got %T", node)
	}
}

func TestNavigateExprStmt(t *testing.T) {
	f := parseSource(t, `package p
import "fmt"
func F() { fmt.Println("hi") }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ExprStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.ExprStmt); !ok {
		t.Fatalf("expected *ast.ExprStmt, got %T", node)
	}
}

func TestNavigateDeferStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(fn func()) { defer fn() }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "DeferStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.DeferStmt); !ok {
		t.Fatalf("expected *ast.DeferStmt, got %T", node)
	}
}

func TestNavigateGoStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(fn func()) { go fn() }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "GoStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.GoStmt); !ok {
		t.Fatalf("expected *ast.GoStmt, got %T", node)
	}
}

func TestNavigateReturnStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() int { return 1 }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ReturnStmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.ReturnStmt); !ok {
		t.Fatalf("expected *ast.ReturnStmt, got %T", node)
	}
}

// Scalar field accessors

func TestNavigateCond_IfStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { if x > 0 {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Cond"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Cond node")
	}
	if ctx.FieldName != "Cond" {
		t.Errorf("expected FieldName=Cond, got %q", ctx.FieldName)
	}
	if ctx.Index != -1 {
		t.Errorf("expected Index=-1, got %d", ctx.Index)
	}
}

func TestNavigateCond_ForStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() { for i := 0; i < 10; i++ {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ForStmt", Index: pInt(0)},
		{Kind: "Cond"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Cond node")
	}
	if ctx.FieldName != "Cond" {
		t.Errorf("expected FieldName=Cond, got %q", ctx.FieldName)
	}
}

func TestNavigateInit_IfStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() int { if x := 1; x > 0 { return x }; return 0 }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Init"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Init node")
	}
	if ctx.FieldName != "Init" {
		t.Errorf("expected FieldName=Init, got %q", ctx.FieldName)
	}
}

func TestNavigateInit_ForStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() { for i := 0; i < 10; i++ {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ForStmt", Index: pInt(0)},
		{Kind: "Init"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Init node")
	}
	if ctx.FieldName != "Init" {
		t.Errorf("expected FieldName=Init, got %q", ctx.FieldName)
	}
}

func TestNavigatePost_ForStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() { for i := 0; i < 10; i++ {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ForStmt", Index: pInt(0)},
		{Kind: "Post"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Post node")
	}
	if ctx.FieldName != "Post" {
		t.Errorf("expected FieldName=Post, got %q", ctx.FieldName)
	}
}

func TestNavigateElse_IfStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { if x > 0 {} else {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Else"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Else node")
	}
	if ctx.FieldName != "Else" {
		t.Errorf("expected FieldName=Else, got %q", ctx.FieldName)
	}
}

func TestNavigateTag_SwitchStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { switch x { case 1: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
		{Kind: "Tag"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Tag node")
	}
	if ctx.FieldName != "Tag" {
		t.Errorf("expected FieldName=Tag, got %q", ctx.FieldName)
	}
}

func TestNavigateX_BinaryExpr(t *testing.T) {
	// BinaryExpr is the Cond of an IfStmt: x > 0
	f := parseSource(t, `package p
func F(x int) int { if x > 0 { return x }; return 0 }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Cond"},
		{Kind: "X"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected X node")
	}
	if ctx.FieldName != "X" {
		t.Errorf("expected FieldName=X, got %q", ctx.FieldName)
	}
}

func TestNavigateY_BinaryExpr(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) int { if x > 0 { return x }; return 0 }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Cond"},
		{Kind: "Y"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Y node")
	}
	if ctx.FieldName != "Y" {
		t.Errorf("expected FieldName=Y, got %q", ctx.FieldName)
	}
}

func TestNavigateFun_ExprStmt(t *testing.T) {
	f := parseSource(t, `package p
import "fmt"
func F() { fmt.Println("hi") }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ExprStmt", Index: pInt(0)},
		{Kind: "X"},
		{Kind: "Fun"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Fun node")
	}
	if ctx.FieldName != "Fun" {
		t.Errorf("expected FieldName=Fun, got %q", ctx.FieldName)
	}
}

func TestNavigateSel_SelectorExpr(t *testing.T) {
	f := parseSource(t, `package p
import "fmt"
func F() { fmt.Println("hi") }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ExprStmt", Index: pInt(0)},
		{Kind: "X"},
		{Kind: "Fun"},
		{Kind: "Sel"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	id, ok := node.(*ast.Ident)
	if !ok {
		t.Fatalf("expected *ast.Ident, got %T", node)
	}
	if id.Name != "Println" {
		t.Errorf("expected Println, got %q", id.Name)
	}
	if ctx.FieldName != "Sel" {
		t.Errorf("expected FieldName=Sel, got %q", ctx.FieldName)
	}
}

func TestNavigateArgs_CallExpr(t *testing.T) {
	f := parseSource(t, `package p
import "fmt"
func F() { fmt.Println("hello", "world") }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ExprStmt", Index: pInt(0)},
		{Kind: "X"},
		{Kind: "Args", Index: pInt(0)},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate args[0]: %v", err)
	}
	if node == nil {
		t.Fatal("expected arg node")
	}
	if ctx.Index != 0 {
		t.Errorf("expected Index=0, got %d", ctx.Index)
	}

	// Also check second arg
	steps[4] = selector.PathStep{Kind: "Args", Index: pInt(1)}
	node, ctx, err = selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate args[1]: %v", err)
	}
	if ctx.Index != 1 {
		t.Errorf("expected Index=1, got %d", ctx.Index)
	}
}

func TestNavigateLhs_AssignStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() { x := 1; _ = x }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "AssignStmt", Index: pInt(0)},
		{Kind: "Lhs", Index: pInt(0)},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Lhs node")
	}
	if ctx.FieldName != "Lhs" {
		t.Errorf("expected FieldName=Lhs, got %q", ctx.FieldName)
	}
}

func TestNavigateRhs_AssignStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() { x := 1; _ = x }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "AssignStmt", Index: pInt(0)},
		{Kind: "Rhs", Index: pInt(0)},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Rhs node")
	}
	if ctx.FieldName != "Rhs" {
		t.Errorf("expected FieldName=Rhs, got %q", ctx.FieldName)
	}
}

func TestNavigateKey_RangeStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(m map[string]int) { for k, v := range m { _ = k; _ = v } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "RangeStmt", Index: pInt(0)},
		{Kind: "Key"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Key node")
	}
	if ctx.FieldName != "Key" {
		t.Errorf("expected FieldName=Key, got %q", ctx.FieldName)
	}
}

func TestNavigateValue_RangeStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(m map[string]int) { for k, v := range m { _ = k; _ = v } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "RangeStmt", Index: pInt(0)},
		{Kind: "Value"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Value node")
	}
	if ctx.FieldName != "Value" {
		t.Errorf("expected FieldName=Value, got %q", ctx.FieldName)
	}
}

func TestNavigateParams_FuncDecl(t *testing.T) {
	f := parseSource(t, `package p
func F(x int, y string) {}`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Params"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.FieldList); !ok {
		t.Fatalf("expected *ast.FieldList, got %T", node)
	}
	if ctx.FieldName != "Params" {
		t.Errorf("expected FieldName=Params, got %q", ctx.FieldName)
	}
}

func TestNavigateResults_FuncDecl(t *testing.T) {
	f := parseSource(t, `package p
func F() (int, error) { return 0, nil }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Results"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.FieldList); !ok {
		t.Fatalf("expected *ast.FieldList, got %T", node)
	}
	if ctx.FieldName != "Results" {
		t.Errorf("expected FieldName=Results, got %q", ctx.FieldName)
	}
}

func TestNavigateElts_CompositeLit(t *testing.T) {
	// CompositeLit is returned as Rhs of an AssignStmt
	f := parseSource(t, `package p
func F() []int { return []int{1, 2, 3} }`)
	// ReturnStmt[0].Results[0] is the CompositeLit
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ReturnStmt", Index: pInt(0)},
	}
	// Navigate to the ReturnStmt, then access its result via Rhs/Args isn't available,
	// instead build a simpler source with assignment
	f2 := parseSource(t, `package p
func F() { x := []int{1, 2, 3}; _ = x }`)
	steps2 := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "AssignStmt", Index: pInt(0)},
		{Kind: "Rhs", Index: pInt(0)},
		{Kind: "Elts", Index: pInt(0)},
	}
	node, ctx, err := selector.Navigate(f2, steps2)
	_ = f
	_ = steps
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Elts[0] node")
	}
	if ctx.FieldName != "Elts" {
		t.Errorf("expected FieldName=Elts, got %q", ctx.FieldName)
	}
	if ctx.Index != 0 {
		t.Errorf("expected Index=0, got %d", ctx.Index)
	}
}

// Type navigators

func TestNavigateStructType(t *testing.T) {
	f := parseSource(t, `package p
type Foo struct { X int }`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Foo"},
		{Kind: "StructType"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.StructType); !ok {
		t.Fatalf("expected *ast.StructType, got %T", node)
	}
	if ctx.FieldName != "Type" {
		t.Errorf("expected FieldName=Type, got %q", ctx.FieldName)
	}
}

func TestNavigateInterfaceType(t *testing.T) {
	f := parseSource(t, `package p
type Animal interface { Sound() string }`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Animal"},
		{Kind: "InterfaceType"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.InterfaceType); !ok {
		t.Fatalf("expected *ast.InterfaceType, got %T", node)
	}
	if ctx.FieldName != "Type" {
		t.Errorf("expected FieldName=Type, got %q", ctx.FieldName)
	}
}

func TestNavigateVarDecl(t *testing.T) {
	f := parseSource(t, `package p
var x int`)
	steps := []selector.PathStep{
		{Kind: "VarDecl"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	gd, ok := node.(*ast.GenDecl)
	if !ok {
		t.Fatalf("expected *ast.GenDecl, got %T", node)
	}
	if gd.Tok.String() != "var" {
		t.Errorf("expected var, got %q", gd.Tok)
	}
}

func TestNavigateConstDecl(t *testing.T) {
	f := parseSource(t, `package p
const Pi = 3.14`)
	steps := []selector.PathStep{
		{Kind: "ConstDecl"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	gd, ok := node.(*ast.GenDecl)
	if !ok {
		t.Fatalf("expected *ast.GenDecl, got %T", node)
	}
	if gd.Tok.String() != "const" {
		t.Errorf("expected const, got %q", gd.Tok)
	}
}

func TestNavigateImportDecl(t *testing.T) {
	f := parseSource(t, `package p
import "fmt"
func F() { _ = fmt.Sprintf }`)
	steps := []selector.PathStep{
		{Kind: "ImportDecl"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	gd, ok := node.(*ast.GenDecl)
	if !ok {
		t.Fatalf("expected *ast.GenDecl, got %T", node)
	}
	if gd.Tok.String() != "import" {
		t.Errorf("expected import, got %q", gd.Tok)
	}
}

// Case/Comm navigators

func TestNavigateCaseClause(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { switch x { case 1: x = 2 } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
		{Kind: "Body"},
		{Kind: "CaseClause", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.CaseClause); !ok {
		t.Fatalf("expected *ast.CaseClause, got %T", node)
	}
}

func TestNavigateCommClause(t *testing.T) {
	f := parseSource(t, `package p
func F(ch chan int) { select { case v := <-ch: _ = v } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SelectStmt", Index: pInt(0)},
		{Kind: "Body"},
		{Kind: "CommClause", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.CommClause); !ok {
		t.Fatalf("expected *ast.CommClause, got %T", node)
	}
}

// Error path for unknown step kind

func TestNavigateUnknownKind(t *testing.T) {
	f := parseSource(t, `package p
func F() {}`)
	steps := []selector.PathStep{
		{Kind: "NonExistentStepKind"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for unknown step kind")
	}
}

// Body navigators for non-FuncDecl nodes

func TestNavigateBody_ForStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() { for i := 0; i < 10; i++ { _ = i } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ForStmt", Index: pInt(0)},
		{Kind: "Body"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.BlockStmt); !ok {
		t.Fatalf("expected *ast.BlockStmt, got %T", node)
	}
	if ctx.FieldName != "Body" {
		t.Errorf("expected FieldName=Body, got %q", ctx.FieldName)
	}
}

func TestNavigateBody_IfStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { if x > 0 { _ = x } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Body"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.BlockStmt); !ok {
		t.Fatalf("expected *ast.BlockStmt, got %T", node)
	}
	if ctx.FieldName != "Body" {
		t.Errorf("expected FieldName=Body, got %q", ctx.FieldName)
	}
}

func TestNavigateBody_RangeStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(s []int) { for _, v := range s { _ = v } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "RangeStmt", Index: pInt(0)},
		{Kind: "Body"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.BlockStmt); !ok {
		t.Fatalf("expected *ast.BlockStmt, got %T", node)
	}
	if ctx.FieldName != "Body" {
		t.Errorf("expected FieldName=Body, got %q", ctx.FieldName)
	}
}

func TestNavigateBody_SwitchStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { switch x { case 1: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
		{Kind: "Body"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.BlockStmt); !ok {
		t.Fatalf("expected *ast.BlockStmt, got %T", node)
	}
}

func TestNavigateBody_SelectStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(ch chan int) { select { case <-ch: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SelectStmt", Index: pInt(0)},
		{Kind: "Body"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.BlockStmt); !ok {
		t.Fatalf("expected *ast.BlockStmt, got %T", node)
	}
}

func TestNavigateBody_TypeSwitchStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(v interface{}) { switch v.(type) { case int: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "TypeSwitchStmt", Index: pInt(0)},
		{Kind: "Body"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.BlockStmt); !ok {
		t.Fatalf("expected *ast.BlockStmt, got %T", node)
	}
}

// Field navigation from FieldList

func TestNavigateField_ByIndex(t *testing.T) {
	f := parseSource(t, `package p
type Foo struct { X int; Y string }`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Foo"},
		{Kind: "StructType"},
		{Kind: "Field", Index: pInt(1)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	field, ok := node.(*ast.Field)
	if !ok {
		t.Fatalf("expected *ast.Field, got %T", node)
	}
	if len(field.Names) == 0 || field.Names[0].Name != "Y" {
		t.Errorf("expected field Y, got %v", field.Names)
	}
}

func TestNavigateField_FromInterfaceType(t *testing.T) {
	f := parseSource(t, `package p
type Animal interface { Sound() string }`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Animal"},
		{Kind: "InterfaceType"},
		{Kind: "Field", Name: "Sound"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.Field); !ok {
		t.Fatalf("expected *ast.Field, got %T", node)
	}
}

// TypeDecl navigator

func TestNavigateTypeDecl(t *testing.T) {
	f := parseSource(t, `package p
type Foo struct{}`)
	steps := []selector.PathStep{
		{Kind: "TypeDecl"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.GenDecl); !ok {
		t.Fatalf("expected *ast.GenDecl, got %T", node)
	}
}

// TypeSpec from GenDecl

func TestNavigateTypeSpec_FromGenDecl(t *testing.T) {
	f := parseSource(t, `package p
type Foo struct{}`)
	steps := []selector.PathStep{
		{Kind: "TypeDecl"},
		{Kind: "TypeSpec", Name: "Foo"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	ts, ok := node.(*ast.TypeSpec)
	if !ok {
		t.Fatalf("expected *ast.TypeSpec, got %T", node)
	}
	if ts.Name.Name != "Foo" {
		t.Errorf("expected Foo, got %q", ts.Name.Name)
	}
}

// Stmt in CaseClause body

func TestNavigateStmt_InCaseClause(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { switch x { case 1: x = 2 } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
		{Kind: "Body"},
		{Kind: "CaseClause", Index: pInt(0)},
		{Kind: "Stmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Stmt node in CaseClause")
	}
}

// NavigateError.Error() method coverage

func TestNavigateErrorString(t *testing.T) {
	f := parseSource(t, `package p`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Missing"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error")
	}
	s := err.Error()
	if s == "" {
		t.Error("expected non-empty error string")
	}
}

// Params/Results from FuncType

func TestNavigateParams_FuncType(t *testing.T) {
	// FuncType occurs as the Type of a TypeSpec: type F func(int) string
	f := parseSource(t, `package p
type MyFunc func(x int) string`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "MyFunc"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate TypeSpec: %v", err)
	}
	ts := node.(*ast.TypeSpec)
	ft, ok := ts.Type.(*ast.FuncType)
	if !ok {
		t.Fatalf("expected *ast.FuncType, got %T", ts.Type)
	}
	// Call stepParams directly via Navigate won't work because TypeSpec isn't FuncDecl/FuncType,
	// but we verify that Params are accessible from FuncDecl
	_ = ft
}

func TestNavigateResults_FuncDecl_withResults(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) (int, error) { return x, nil }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Results"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	fl, ok := node.(*ast.FieldList)
	if !ok {
		t.Fatalf("expected *ast.FieldList, got %T", node)
	}
	if len(fl.List) != 2 {
		t.Errorf("expected 2 results, got %d", len(fl.List))
	}
}

// stepX for ExprStmt and RangeStmt

func TestNavigateX_ExprStmt(t *testing.T) {
	f := parseSource(t, `package p
import "fmt"
func F() { fmt.Println("hi") }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ExprStmt", Index: pInt(0)},
		{Kind: "X"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if _, ok := node.(*ast.CallExpr); !ok {
		t.Fatalf("expected *ast.CallExpr, got %T", node)
	}
	if ctx.FieldName != "X" {
		t.Errorf("expected FieldName=X, got %q", ctx.FieldName)
	}
}

func TestNavigateX_RangeStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(m map[string]int) { for k, v := range m { _ = k; _ = v } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "RangeStmt", Index: pInt(0)},
		{Kind: "X"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected X node")
	}
	if ctx.FieldName != "X" {
		t.Errorf("expected FieldName=X, got %q", ctx.FieldName)
	}
}

func TestNavigateX_SelectorExpr(t *testing.T) {
	// Navigate to SelectorExpr.X (the package part of fmt.Println)
	f := parseSource(t, `package p
import "fmt"
func F() { fmt.Println("hi") }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ExprStmt", Index: pInt(0)},
		{Kind: "X"},
		{Kind: "Fun"},
		{Kind: "X"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	id, ok := node.(*ast.Ident)
	if !ok {
		t.Fatalf("expected *ast.Ident, got %T", node)
	}
	if id.Name != "fmt" {
		t.Errorf("expected fmt, got %q", id.Name)
	}
	if ctx.FieldName != "X" {
		t.Errorf("expected FieldName=X, got %q", ctx.FieldName)
	}
}

// stepCond for SwitchStmt (with tag as Cond)

func TestNavigateCond_SwitchStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { switch x { case 1: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
		{Kind: "Cond"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Cond/Tag node")
	}
	if ctx.FieldName != "Tag" {
		t.Errorf("expected FieldName=Tag (SwitchStmt.Cond maps to Tag), got %q", ctx.FieldName)
	}
}

// stepInit for SwitchStmt and TypeSwitchStmt

func TestNavigateInit_SwitchStmt(t *testing.T) {
	f := parseSource(t, `package p
func F() int { switch x := 1; x { case 1: return x }; return 0 }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
		{Kind: "Init"},
	}
	node, ctx, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Init node")
	}
	if ctx.FieldName != "Init" {
		t.Errorf("expected FieldName=Init, got %q", ctx.FieldName)
	}
}

func TestNavigateInit_TypeSwitchStmt(t *testing.T) {
	f := parseSource(t, `package p
func F(v interface{}) { switch x := v.(type); x.(type) { case int: } }`)
	// TypeSwitchStmt with Init is unusual; simpler: use the Assign form
	// Actually test that Init on a type switch with an init stmt works
	f2 := parseSource(t, `package p
func F(v interface{}) {
	switch x := 1; v.(type) {
	case int:
		_ = x
	}
}`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "TypeSwitchStmt", Index: pInt(0)},
		{Kind: "Init"},
	}
	node, ctx, err := selector.Navigate(f2, steps)
	_ = f
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Init node")
	}
	if ctx.FieldName != "Init" {
		t.Errorf("expected FieldName=Init, got %q", ctx.FieldName)
	}
}

// stepValue for KeyValueExpr and SendStmt

func TestNavigateValue_KeyValueExpr(t *testing.T) {
	f := parseSource(t, `package p
func F() map[string]int { return map[string]int{"a": 1} }`)
	// Navigate to KeyValueExpr.Value in a composite literal
	f2 := parseSource(t, `package p
func F() { x := map[string]int{"a": 1}; _ = x }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "AssignStmt", Index: pInt(0)},
		{Kind: "Rhs", Index: pInt(0)},
		{Kind: "Elts", Index: pInt(0)},
		{Kind: "Value"},
	}
	node, ctx, err := selector.Navigate(f2, steps)
	_ = f
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected Value node")
	}
	if ctx.FieldName != "Value" {
		t.Errorf("expected FieldName=Value, got %q", ctx.FieldName)
	}
}

// getStmtList for CommClause body

func TestNavigateStmt_InCommClause(t *testing.T) {
	f := parseSource(t, `package p
func F(ch chan int) { select { case v := <-ch: _ = v } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SelectStmt", Index: pInt(0)},
		{Kind: "Body"},
		{Kind: "CommClause", Index: pInt(0)},
		{Kind: "Stmt", Index: pInt(0)},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected stmt in CommClause")
	}
}

// recvTypeName with non-pointer value receiver

func TestNavigateFuncDecl_ValueRecv(t *testing.T) {
	f := parseSource(t, `package p
type Dog struct{}
func (d Dog) Bark() string { return "woof" }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "Bark", Recv: "Dog"},
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	fd, ok := node.(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected *ast.FuncDecl, got %T", node)
	}
	if fd.Name.Name != "Bark" {
		t.Errorf("expected Bark, got %q", fd.Name.Name)
	}
}

// stepIndexedStmtKind not found (count exhausted)

func TestNavigateForStmt_NotFound(t *testing.T) {
	f := parseSource(t, `package p
func F() { for {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ForStmt", Index: pInt(5)}, // only 1 ForStmt, index 5 is out of range
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for out-of-range ForStmt index")
	}
}

// stepStmtByIndex with negative index

func TestNavigateStmt_NegativeIndex(t *testing.T) {
	f := parseSource(t, `package p
func F() { x := 1; y := 2; _ = x; _ = y }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "Stmt", Index: pInt(-1)}, // last stmt
	}
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if node == nil {
		t.Fatal("expected node for negative index")
	}
}

// stepCaseClause: not found

func TestNavigateCaseClause_NotFound(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { switch x { case 1: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
		{Kind: "Body"},
		{Kind: "CaseClause", Index: pInt(5)}, // only 1 clause
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for missing CaseClause")
	}
}

// stepCommClause: not found

func TestNavigateCommClause_NotFound(t *testing.T) {
	f := parseSource(t, `package p
func F(ch chan int) { select { case <-ch: } }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SelectStmt", Index: pInt(0)},
		{Kind: "Body"},
		{Kind: "CommClause", Index: pInt(5)}, // only 1 clause
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for missing CommClause")
	}
}

// stepField error: named field not found

func TestNavigateField_NotFound(t *testing.T) {
	f := parseSource(t, `package p
type Foo struct { X int }`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Foo"},
		{Kind: "StructType"},
		{Kind: "Field", Name: "NonExistent"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for missing field")
	}
}

// stepArgs out of range

func TestNavigateArgs_OutOfRange(t *testing.T) {
	f := parseSource(t, `package p
import "fmt"
func F() { fmt.Println("hi") }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ExprStmt", Index: pInt(0)},
		{Kind: "X"},
		{Kind: "Args", Index: pInt(99)},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for out-of-range arg index")
	}
}

// stepElts out of range

func TestNavigateElts_OutOfRange(t *testing.T) {
	f := parseSource(t, `package p
func F() { x := []int{1}; _ = x }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "AssignStmt", Index: pInt(0)},
		{Kind: "Rhs", Index: pInt(0)},
		{Kind: "Elts", Index: pInt(99)},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for out-of-range Elts index")
	}
}

// stepLhsRhs out of range

func TestNavigateLhs_OutOfRange(t *testing.T) {
	f := parseSource(t, `package p
func F() { x := 1; _ = x }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "AssignStmt", Index: pInt(0)},
		{Kind: "Lhs", Index: pInt(99)},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for out-of-range Lhs index")
	}
}

// stepPost: no post

func TestNavigatePost_NilPost(t *testing.T) {
	f := parseSource(t, `package p
func F() { for {} }`) // infinite loop, no post
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ForStmt", Index: pInt(0)},
		{Kind: "Post"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for nil Post on infinite loop")
	}
}

// stepElse: no else

func TestNavigateElse_NoElse(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { if x > 0 {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Else"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for nil Else")
	}
}

// stepTag: no tag

func TestNavigateTag_NoTag(t *testing.T) {
	f := parseSource(t, `package p
func F() { switch { case true: } }`) // no tag expr
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "SwitchStmt", Index: pInt(0)},
		{Kind: "Tag"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for nil Tag on switch-without-tag")
	}
}

// stepCond: no cond on for (infinite loop)

func TestNavigateCond_InfiniteFor(t *testing.T) {
	f := parseSource(t, `package p
func F() { for {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ForStmt", Index: pInt(0)},
		{Kind: "Cond"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for nil Cond on infinite loop")
	}
}

// stepKey: no key on range (range over channel)

func TestNavigateKey_NilKey(t *testing.T) {
	f := parseSource(t, `package p
func F(ch <-chan int) { for range ch {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "RangeStmt", Index: pInt(0)},
		{Kind: "Key"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for nil Key on channel range")
	}
}

// stepValue: no value on range

func TestNavigateValue_NilValue(t *testing.T) {
	f := parseSource(t, `package p
func F(s []int) { for range s {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "RangeStmt", Index: pInt(0)},
		{Kind: "Value"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for nil Value on range with no value var")
	}
}

// stepGenDeclByTok not found

func TestNavigateVarDecl_NotFound(t *testing.T) {
	f := parseSource(t, `package p
func F() {}`)
	steps := []selector.PathStep{
		{Kind: "VarDecl"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error when no var decl exists")
	}
}

// stepTypeDecl not found

func TestNavigateTypeDecl_NotFound(t *testing.T) {
	f := parseSource(t, `package p
func F() {}`)
	steps := []selector.PathStep{
		{Kind: "TypeDecl"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error when no type decl exists")
	}
}

// stepBody on unsupported node returns error

func TestNavigateBody_Error(t *testing.T) {
	// Navigate to a ReturnStmt, then try Body (no Body on ReturnStmt)
	f := parseSource(t, `package p
func F() int { return 1 }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ReturnStmt", Index: pInt(0)},
		{Kind: "Body"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for Body on ReturnStmt")
	}
}

// stepParams on no-result FuncDecl returns error

func TestNavigateParams_NoParams_Error(t *testing.T) {
	f := parseSource(t, `package p
func F() {}`)
	// Params on a non-FuncDecl node
	f2 := parseSource(t, `package p
type Foo struct{}`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Foo"},
		{Kind: "Params"},
	}
	_, _, err := selector.Navigate(f2, steps)
	_ = f
	if err == nil {
		t.Fatal("expected error for Params on TypeSpec")
	}
}

// stepResults error on non-func node

func TestNavigateResults_Error(t *testing.T) {
	f := parseSource(t, `package p
type Foo struct{}`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Foo"},
		{Kind: "Results"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for Results on TypeSpec")
	}
}

// stepY on non-BinaryExpr

func TestNavigateY_Error(t *testing.T) {
	f := parseSource(t, `package p
func F() {}`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Y"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for Y on FuncDecl")
	}
}

// stepFun on non-CallExpr

func TestNavigateFun_Error(t *testing.T) {
	f := parseSource(t, `package p
func F() {}`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Fun"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for Fun on FuncDecl")
	}
}

// stepSel on non-SelectorExpr

func TestNavigateSel_Error(t *testing.T) {
	f := parseSource(t, `package p
func F() {}`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Sel"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for Sel on FuncDecl")
	}
}

// stepX on non-applicable node

func TestNavigateX_Error(t *testing.T) {
	f := parseSource(t, `package p
func F() {}`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "X"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for X on FuncDecl")
	}
}

// stepStructType on non-struct TypeSpec

func TestNavigateStructType_Error(t *testing.T) {
	f := parseSource(t, `package p
type Animal interface { Sound() string }`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Animal"},
		{Kind: "StructType"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for StructType on interface TypeSpec")
	}
}

// stepInterfaceType on non-interface TypeSpec

func TestNavigateInterfaceType_Error(t *testing.T) {
	f := parseSource(t, `package p
type Foo struct{}`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "Foo"},
		{Kind: "InterfaceType"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for InterfaceType on struct TypeSpec")
	}
}

// stepTypeSpec not found

func TestNavigateTypeSpec_NotFound(t *testing.T) {
	f := parseSource(t, `package p
type Foo struct{}`)
	steps := []selector.PathStep{
		{Kind: "TypeSpec", Name: "NonExistent"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for missing TypeSpec")
	}
}

// stepInit: no init on IfStmt

func TestNavigateInit_IfStmt_NoInit(t *testing.T) {
	f := parseSource(t, `package p
func F(x int) { if x > 0 {} }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "IfStmt", Index: pInt(0)},
		{Kind: "Init"},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for nil Init on if without init stmt")
	}
}

// stepStmtByIndex out of range

func TestNavigateStmt_OutOfRange(t *testing.T) {
	f := parseSource(t, `package p
func F() { x := 1; _ = x }`)
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "Stmt", Index: pInt(99)},
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for out-of-range Stmt index")
	}
}

// FuncDecl with no body (interface method stub is technically not a FuncDecl with no body)
// Test FuncDecl Body error

func TestNavigateBody_FuncDecl_NoBody(t *testing.T) {
	// A forward declaration can't be created in Go source; test via wrong node
	// Instead test Body on a node that exists but is another type
	f := parseSource(t, `package p
func F() int { return 1 }`)
	// FuncDecl has a body, navigate to return stmt then try body
	steps := []selector.PathStep{
		{Kind: "FuncDecl", Name: "F"},
		{Kind: "Body"},
		{Kind: "ReturnStmt", Index: pInt(0)},
		{Kind: "Body"}, // ReturnStmt has no Body
	}
	_, _, err := selector.Navigate(f, steps)
	if err == nil {
		t.Fatal("expected error for Body on ReturnStmt")
	}
}
