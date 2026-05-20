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

func TestComputeForStmt(t *testing.T) {
	src := `package p
func F() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	forStmt := fd.Body.List[0].(*ast.ForStmt)
	m := meta.Compute(fset, []byte(src), forStmt, fd.Body, 2)
	if m["has_init"] != true {
		t.Errorf("has_init: got %v, want true", m["has_init"])
	}
	if m["body_stmt_count"] != 1 {
		t.Errorf("body_stmt_count: got %v, want 1", m["body_stmt_count"])
	}
}

func TestComputeRangeStmt(t *testing.T) {
	src := `package p
func F(items []int) {
	for i, v := range items {
		_ = i
		_ = v
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	rangeStmt := fd.Body.List[0].(*ast.RangeStmt)
	m := meta.Compute(fset, []byte(src), rangeStmt, fd.Body, 2)
	if m["body_stmt_count"] != 2 {
		t.Errorf("body_stmt_count: got %v, want 2", m["body_stmt_count"])
	}
}

func TestComputeSwitchStmt(t *testing.T) {
	src := `package p
func F(x int) {
	switch x {
	case 1:
		_ = x
	case 2:
		_ = x
	default:
		_ = x
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	switchStmt := fd.Body.List[0].(*ast.SwitchStmt)
	m := meta.Compute(fset, []byte(src), switchStmt, fd.Body, 2)
	if m["case_count"] != 3 {
		t.Errorf("case_count: got %v, want 3", m["case_count"])
	}
	if m["has_default"] != true {
		t.Errorf("has_default: got %v, want true", m["has_default"])
	}
}

func TestComputeTypeSwitchStmt(t *testing.T) {
	src := `package p
func F(v interface{}) {
	switch v.(type) {
	case int:
		_ = v
	case string:
		_ = v
	default:
		_ = v
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	tsStmt := fd.Body.List[0].(*ast.TypeSwitchStmt)
	m := meta.Compute(fset, []byte(src), tsStmt, fd.Body, 2)
	if m["case_count"] != 3 {
		t.Errorf("case_count: got %v, want 3", m["case_count"])
	}
	if m["has_default"] != true {
		t.Errorf("has_default: got %v, want true", m["has_default"])
	}
}

func TestComputeSelectStmt(t *testing.T) {
	src := `package p
func F(ch <-chan int) {
	select {
	case <-ch:
		_ = ch
	default:
		_ = ch
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	selStmt := fd.Body.List[0].(*ast.SelectStmt)
	m := meta.Compute(fset, []byte(src), selStmt, fd.Body, 2)
	if m["case_count"] != 2 {
		t.Errorf("case_count: got %v, want 2", m["case_count"])
	}
	if m["has_default"] != true {
		t.Errorf("has_default: got %v, want true", m["has_default"])
	}
}

func TestComputeInterfaceType(t *testing.T) {
	src := `package p
type Reader interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	gd := f.Decls[0].(*ast.GenDecl)
	ts := gd.Specs[0].(*ast.TypeSpec)
	iface := ts.Type.(*ast.InterfaceType)
	m := meta.Compute(fset, []byte(src), iface, gd, 2)
	if m["method_count"] != 2 {
		t.Errorf("method_count: got %v, want 2", m["method_count"])
	}
	if m["is_empty"] != false {
		t.Errorf("is_empty: got %v, want false", m["is_empty"])
	}
}

func TestComputeEmptyInterface(t *testing.T) {
	src := `package p
type Any interface{}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	gd := f.Decls[0].(*ast.GenDecl)
	ts := gd.Specs[0].(*ast.TypeSpec)
	iface := ts.Type.(*ast.InterfaceType)
	m := meta.Compute(fset, []byte(src), iface, gd, 2)
	if m["method_count"] != 0 {
		t.Errorf("method_count: got %v, want 0", m["method_count"])
	}
	if m["is_empty"] != true {
		t.Errorf("is_empty: got %v, want true", m["is_empty"])
	}
}

func TestComputeField(t *testing.T) {
	src := `package p
type S struct {
	X, Y int
	z    string ` + "`" + `json:"z"` + "`" + `
	Base
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	gd := f.Decls[0].(*ast.GenDecl)
	ts := gd.Specs[0].(*ast.TypeSpec)
	st := ts.Type.(*ast.StructType)

	// multi-name exported field X, Y
	fieldXY := st.Fields.List[0]
	m := meta.Compute(fset, []byte(src), fieldXY, st, 3)
	if m["name_count"] != 2 {
		t.Errorf("name_count: got %v, want 2", m["name_count"])
	}
	if m["exported"] != true {
		t.Errorf("exported: got %v, want true", m["exported"])
	}
	if m["is_embedded"] != false {
		t.Errorf("is_embedded: got %v, want false", m["is_embedded"])
	}
	if m["has_tag"] != false {
		t.Errorf("has_tag: got %v, want false", m["has_tag"])
	}

	// unexported field with tag
	fieldZ := st.Fields.List[1]
	m2 := meta.Compute(fset, []byte(src), fieldZ, st, 3)
	if m2["has_tag"] != true {
		t.Errorf("has_tag: got %v, want true", m2["has_tag"])
	}
	if m2["exported"] != false {
		t.Errorf("exported: got %v, want false", m2["exported"])
	}

	// embedded field
	fieldBase := st.Fields.List[2]
	m3 := meta.Compute(fset, []byte(src), fieldBase, st, 3)
	if m3["is_embedded"] != true {
		t.Errorf("is_embedded: got %v, want true", m3["is_embedded"])
	}
}

func TestComputeTypeSpec(t *testing.T) {
	// alias
	src := `package p
type MyInt = int
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	gd := f.Decls[0].(*ast.GenDecl)
	ts := gd.Specs[0].(*ast.TypeSpec)
	m := meta.Compute(fset, []byte(src), ts, gd, 2)
	if m["is_alias"] != true {
		t.Errorf("is_alias: got %v, want true", m["is_alias"])
	}
	if m["underlying_kind"] != "ident" {
		t.Errorf("underlying_kind: got %v, want ident", m["underlying_kind"])
	}
}

func TestComputeTypeSpec_Generics(t *testing.T) {
	src := `package p
type Container[T any] struct{}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	gd := f.Decls[0].(*ast.GenDecl)
	ts := gd.Specs[0].(*ast.TypeSpec)
	m := meta.Compute(fset, []byte(src), ts, gd, 2)
	if m["has_type_params"] != true {
		t.Errorf("has_type_params: got %v, want true", m["has_type_params"])
	}
	if m["underlying_kind"] != "struct" {
		t.Errorf("underlying_kind: got %v, want struct", m["underlying_kind"])
	}
}

func TestComputeFuncDecl_Unexported(t *testing.T) {
	src := `package p
func privateFunc() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0]
	m := meta.Compute(fset, []byte(src), fd, f, 1)
	if m["exported"] != false {
		t.Errorf("exported: got %v, want false", m["exported"])
	}
	if m["is_method"] != false {
		t.Errorf("is_method: got %v, want false", m["is_method"])
	}
}

func TestComputeFuncDecl_HighCyclomatic(t *testing.T) {
	src := `package p
func Complex(x int) int {
	if x > 0 {
		for i := 0; i < x; i++ {
			switch i {
			case 1:
				x++
			case 2:
				x--
			}
		}
	}
	return x
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0]
	m := meta.Compute(fset, []byte(src), fd, f, 1)
	cc, ok := m["cyclomatic_complexity"].(int)
	if !ok {
		t.Fatalf("cyclomatic_complexity not int: %T", m["cyclomatic_complexity"])
	}
	if cc < 4 {
		t.Errorf("cyclomatic_complexity: got %d, want >= 4", cc)
	}
}

func TestUnderlyingKind_Variants(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`package p; type T *int`, "pointer"},
		{`package p; type T interface{}`, "interface"},
		{`package p; type T [3]int`, "array"},
		{`package p; type T []int`, "slice"},
		{`package p; type T map[string]int`, "map"},
		{`package p; type T chan int`, "chan"},
		{`package p; type T func()`, "func"},
	}
	for _, tc := range cases {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "test.go", tc.src, 0)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.src, err)
		}
		gd := f.Decls[0].(*ast.GenDecl)
		ts := gd.Specs[0].(*ast.TypeSpec)
		m := meta.Compute(fset, []byte(tc.src), ts, gd, 2)
		if m["underlying_kind"] != tc.want {
			t.Errorf("src=%q: underlying_kind: got %v, want %v", tc.src, m["underlying_kind"], tc.want)
		}
	}
}

func TestParentKindName_Variants(t *testing.T) {
	// Test parentKindName via Compute with various parent types by checking parent_kind field
	src := `package p
func F() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	forStmt := fd.Body.List[0].(*ast.ForStmt)

	// ForStmt as parent
	m := meta.Compute(fset, []byte(src), forStmt.Body.List[0], forStmt, 3)
	if m["parent_kind"] != "ForStmt" {
		t.Errorf("parent_kind: got %v, want ForStmt", m["parent_kind"])
	}
}

func TestExprString_IndexExpr(t *testing.T) {
	// Test exprString via recvTypeName with an IndexExpr receiver (generic method)
	src := `package p
func (r Container[T]) Method() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	m := meta.Compute(fset, []byte(src), fd, f, 1)
	// recv_type should be set (may be Container[...])
	if _, ok := m["recv_type"]; !ok {
		t.Error("recv_type not set for generic receiver")
	}
}

func TestParentKindName_AllVariants(t *testing.T) {
	// Covers parentKindName branches by computing meta with each node type as parent
	src := `package p
import "fmt"
func F(x int) {
	if x > 0 {
		for range []int{} {
			switch x {
			case 1:
				select {
				default:
				}
			}
		}
	}
	fmt.Println(x)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[1].(*ast.FuncDecl)

	// parent: IfStmt
	ifStmt := fd.Body.List[0].(*ast.IfStmt)
	rangeStmt := ifStmt.Body.List[0].(*ast.RangeStmt)
	m := meta.Compute(fset, []byte(src), rangeStmt, ifStmt, 3)
	if m["parent_kind"] != "IfStmt" {
		t.Errorf("parent_kind: got %v, want IfStmt", m["parent_kind"])
	}

	// parent: RangeStmt
	switchStmt := rangeStmt.Body.List[0].(*ast.SwitchStmt)
	m = meta.Compute(fset, []byte(src), switchStmt, rangeStmt, 4)
	if m["parent_kind"] != "RangeStmt" {
		t.Errorf("parent_kind: got %v, want RangeStmt", m["parent_kind"])
	}

	// parent: SwitchStmt
	caseClause := switchStmt.Body.List[0].(*ast.CaseClause)
	m = meta.Compute(fset, []byte(src), caseClause, switchStmt, 5)
	if m["parent_kind"] != "SwitchStmt" {
		t.Errorf("parent_kind: got %v, want SwitchStmt", m["parent_kind"])
	}

	// parent: CaseClause
	selStmt := caseClause.Body[0].(*ast.SelectStmt)
	m = meta.Compute(fset, []byte(src), selStmt, caseClause, 6)
	if m["parent_kind"] != "CaseClause" {
		t.Errorf("parent_kind: got %v, want CaseClause", m["parent_kind"])
	}

	// parent: SelectStmt
	commClause := selStmt.Body.List[0].(*ast.CommClause)
	m = meta.Compute(fset, []byte(src), commClause, selStmt, 7)
	if m["parent_kind"] != "SelectStmt" {
		t.Errorf("parent_kind: got %v, want SelectStmt", m["parent_kind"])
	}

	// parent: FuncDecl
	m = meta.Compute(fset, []byte(src), fd.Body, fd, 2)
	if m["parent_kind"] != "FuncDecl" {
		t.Errorf("parent_kind: got %v, want FuncDecl", m["parent_kind"])
	}
}

func TestParentKindName_ExprParents(t *testing.T) {
	src := `package p
import "fmt"
func F() {
	fmt.Println(func() {
		_ = []int{1, 2}
	})
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
	funcLit := call.Args[0].(*ast.FuncLit)
	compLit := funcLit.Body.List[0].(*ast.AssignStmt).Rhs[0].(*ast.CompositeLit)

	// parent: FuncLit
	m := meta.Compute(fset, []byte(src), funcLit.Body, funcLit, 4)
	if m["parent_kind"] != "FuncLit" {
		t.Errorf("parent_kind: got %v, want FuncLit", m["parent_kind"])
	}

	// parent: CallExpr
	m = meta.Compute(fset, []byte(src), funcLit, call, 5)
	if m["parent_kind"] != "CallExpr" {
		t.Errorf("parent_kind: got %v, want CallExpr", m["parent_kind"])
	}

	// parent: CompositeLit
	elt := compLit.Elts[0]
	m = meta.Compute(fset, []byte(src), elt, compLit, 6)
	if m["parent_kind"] != "CompositeLit" {
		t.Errorf("parent_kind: got %v, want CompositeLit", m["parent_kind"])
	}
}

func TestParentKindName_TypeParents(t *testing.T) {
	src := `package p
type R interface {
	Do() error
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	gd := f.Decls[0].(*ast.GenDecl)
	ts := gd.Specs[0].(*ast.TypeSpec)
	iface := ts.Type.(*ast.InterfaceType)
	fieldList := iface.Methods
	field := fieldList.List[0]

	// parent: InterfaceType
	m := meta.Compute(fset, []byte(src), fieldList, iface, 3)
	if m["parent_kind"] != "InterfaceType" {
		t.Errorf("parent_kind: got %v, want InterfaceType", m["parent_kind"])
	}

	// parent: FieldList
	m = meta.Compute(fset, []byte(src), field, fieldList, 4)
	if m["parent_kind"] != "FieldList" {
		t.Errorf("parent_kind: got %v, want FieldList", m["parent_kind"])
	}

	// parent: Field (functype)
	funcType := field.Type.(*ast.FuncType)
	m = meta.Compute(fset, []byte(src), funcType, field, 5)
	if m["parent_kind"] != "Field" {
		t.Errorf("parent_kind: got %v, want Field", m["parent_kind"])
	}

	// parent: FuncType
	results := funcType.Results
	m = meta.Compute(fset, []byte(src), results, funcType, 6)
	if m["parent_kind"] != "FuncType" {
		t.Errorf("parent_kind: got %v, want FuncType", m["parent_kind"])
	}

	// parent: GenDecl
	m = meta.Compute(fset, []byte(src), ts, gd, 2)
	if m["parent_kind"] != "GenDecl" {
		t.Errorf("parent_kind: got %v, want GenDecl", m["parent_kind"])
	}
}

func TestParentKindName_TypeSwitchParent(t *testing.T) {
	src := `package p
func F(v interface{}) {
	switch v.(type) {
	case int:
	}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	tsStmt := fd.Body.List[0].(*ast.TypeSwitchStmt)
	caseClause := tsStmt.Body.List[0].(*ast.CaseClause)

	// parent: TypeSwitchStmt
	m := meta.Compute(fset, []byte(src), caseClause, tsStmt, 3)
	if m["parent_kind"] != "TypeSwitchStmt" {
		t.Errorf("parent_kind: got %v, want TypeSwitchStmt", m["parent_kind"])
	}
}

func TestComputeCallExpr_SimpleIdent(t *testing.T) {
	src := `package p
func F() {
	println("hello")
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0].(*ast.FuncDecl)
	exprStmt := fd.Body.List[0].(*ast.ExprStmt)
	call := exprStmt.X.(*ast.CallExpr)
	m := meta.Compute(fset, []byte(src), call, exprStmt, 3)
	if m["callee"] != "println" {
		t.Errorf("callee: got %v, want println", m["callee"])
	}
}

func TestComputeIfStmt_NoElse(t *testing.T) {
	src := `package p
func F(x int) {
	if x > 0 {
		_ = x
	}
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
	if m["has_else"] != false {
		t.Errorf("has_else: got %v, want false", m["has_else"])
	}
	if m["else_is_if"] != false {
		t.Errorf("else_is_if: got %v, want false", m["else_is_if"])
	}
}

func TestComputeFuncDecl_NoParams(t *testing.T) {
	// FuncDecl with no params and no results — covers else branches in computeFuncDecl
	src := `package p
func Noop() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := f.Decls[0]
	m := meta.Compute(fset, []byte(src), fd, f, 1)
	if m["param_count"] != 0 {
		t.Errorf("param_count: got %v, want 0", m["param_count"])
	}
	if m["result_count"] != 0 {
		t.Errorf("result_count: got %v, want 0", m["result_count"])
	}
	if m["has_error_return"] != false {
		t.Errorf("has_error_return: got %v, want false", m["has_error_return"])
	}
}

func TestFileInfo_TypeCount(t *testing.T) {
	src := `package p
type A int
type B string
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	m := meta.FileInfo(fset, []byte(src), f)
	if m["type_count"] != 2 {
		t.Errorf("type_count: got %v, want 2", m["type_count"])
	}
}

func TestComputeInterfaceType_WithEmbed(t *testing.T) {
	src := `package p
type ReadWriter interface {
	Reader
	Write(p []byte) (int, error)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	gd := f.Decls[0].(*ast.GenDecl)
	ts := gd.Specs[0].(*ast.TypeSpec)
	iface := ts.Type.(*ast.InterfaceType)
	m := meta.Compute(fset, []byte(src), iface, gd, 2)
	if m["embed_count"] != 1 {
		t.Errorf("embed_count: got %v, want 1", m["embed_count"])
	}
	if m["method_count"] != 1 {
		t.Errorf("method_count: got %v, want 1", m["method_count"])
	}
}
