package meta_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/lthiery/goast/meta"
)

func TestFileInfo(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}

func init() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	m := meta.FileInfo(fset, []byte(src), f)
	if m["package"] != "main" {
		t.Errorf("package: got %v", m["package"])
	}
	if m["func_count"] != 2 {
		t.Errorf("func_count: got %v, want 2", m["func_count"])
	}
	if m["import_count"] != 1 {
		t.Errorf("import_count: got %v, want 1", m["import_count"])
	}
	if m["has_init"] != true {
		t.Errorf("has_init: got %v, want true", m["has_init"])
	}
}

func TestComputeFuncDecl(t *testing.T) {
	src := `package p
func (r *R) Handle(w int, req string) (int, error) {
	if w > 0 {
		return w, nil
	}
	return 0, nil
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0]
	m := meta.Compute(fset, []byte(src), fd, f, 1)
	if m["exported"] != true {
		t.Errorf("exported: got %v", m["exported"])
	}
	if m["is_method"] != true {
		t.Errorf("is_method: got %v", m["is_method"])
	}
	if m["has_error_return"] != true {
		t.Errorf("has_error_return: got %v", m["has_error_return"])
	}
	if m["param_count"] != 2 {
		t.Errorf("param_count: got %v, want 2", m["param_count"])
	}
	if m["result_count"] != 2 {
		t.Errorf("result_count: got %v, want 2", m["result_count"])
	}
	// cyclomatic: 1 base + 1 for if = 2
	if m["cyclomatic_complexity"] != 2 {
		t.Errorf("cyclomatic_complexity: got %v, want 2", m["cyclomatic_complexity"])
	}
}

func TestComputeUniversalFields(t *testing.T) {
	src := `package p
func Foo() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0]
	m := meta.Compute(fset, []byte(src), fd, f, 1)
	for _, field := range []string{"line", "end_line", "col", "byte_offset", "byte_end", "depth", "parent_kind"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing universal field %q", field)
		}
	}
	if m["depth"] != 1 {
		t.Errorf("depth: got %v, want 1", m["depth"])
	}
	if m["parent_kind"] != "File" {
		t.Errorf("parent_kind: got %v, want File", m["parent_kind"])
	}
}

func TestComputeIfStmt(t *testing.T) {
	src := `package p
func F(x int) int {
	if x > 0 {
		return x
	} else if x < 0 {
		return -x
	}
	return 0
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	ifStmt := fd.Body.List[0].(*ast.IfStmt)
	m := meta.Compute(fset, []byte(src), ifStmt, fd.Body, 2)
	if m["has_else"] != true {
		t.Errorf("has_else: got %v, want true", m["has_else"])
	}
	if m["else_is_if"] != true {
		t.Errorf("else_is_if: got %v, want true", m["else_is_if"])
	}
	if m["body_stmt_count"] != 1 {
		t.Errorf("body_stmt_count: got %v, want 1", m["body_stmt_count"])
	}
}

func TestComputeCallExpr(t *testing.T) {
	src := `package p
import "fmt"
func F() {
	fmt.Println("hello", "world")
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[1].(*ast.FuncDecl)
	exprStmt := fd.Body.List[0].(*ast.ExprStmt)
	call := exprStmt.X.(*ast.CallExpr)
	m := meta.Compute(fset, []byte(src), call, exprStmt, 3)
	if m["arg_count"] != 2 {
		t.Errorf("arg_count: got %v, want 2", m["arg_count"])
	}
	if m["callee"] != "fmt.Println" {
		t.Errorf("callee: got %v, want fmt.Println", m["callee"])
	}
}

func TestComputeStructType(t *testing.T) {
	src := `package p
type Dog struct {
	Name string
	age  int
	Base
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Navigate to StructType
	var structNode *ast.StructType
	for _, decl := range f.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					if st, ok := ts.Type.(*ast.StructType); ok {
						structNode = st
					}
				}
			}
		}
	}
	if structNode == nil {
		t.Fatal("struct not found")
	}
	m := meta.Compute(fset, []byte(src), structNode, nil, 2)
	if m["field_count"] != 3 {
		t.Errorf("field_count: got %v, want 3", m["field_count"])
	}
	if m["has_embedded"] != true {
		t.Errorf("has_embedded: got %v", m["has_embedded"])
	}
	if m["exported_field_count"] != 1 {
		t.Errorf("exported_field_count: got %v, want 1", m["exported_field_count"])
	}
}
