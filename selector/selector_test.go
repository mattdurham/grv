package selector_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/lthiery/goast/selector"
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
