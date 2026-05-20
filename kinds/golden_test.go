package kinds_test

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattdurham/grv/kinds"
)

// runProgram builds an ast.File from decls, formats it, writes to a temp dir,
// runs with go run, and returns stdout. Fails the test on any error.
func runProgram(t *testing.T, decls []ast.Decl) string {
	t.Helper()
	fset := token.NewFileSet()
	file := &ast.File{Name: &ast.Ident{Name: "main"}, Decls: decls}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		t.Fatalf("go/format failed: %v\nsource so far: %s", err, buf.String())
	}
	src := buf.String()
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(goFile, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	out, err := exec.Command("go", "run", goFile).Output()
	if err != nil {
		t.Fatalf("go run failed; source:\n%s\nerror: %v", src, err)
	}
	return string(out)
}

// buildDecl unmarshals a JSON node and calls ToAST, failing on error.
func buildDecl(t *testing.T, data json.RawMessage) ast.Decl {
	t.Helper()
	node, err := kinds.UnmarshalNode(data)
	if err != nil {
		t.Fatalf("UnmarshalNode: %v", err)
	}
	astNode, err := node.ToAST()
	if err != nil {
		t.Fatalf("ToAST: %v", err)
	}
	return astNode.(ast.Decl)
}

func importDecl(paths ...string) json.RawMessage {
	specs := make([]json.RawMessage, len(paths))
	for i, p := range paths {
		specs[i], _ = json.Marshal(&kinds.ImportSpec{KindField: "ImportSpec", Path: p})
	}
	b, _ := json.Marshal(&kinds.ImportDecl{KindField: "ImportDecl", Specs: specs})
	return b
}

func ident(name string) json.RawMessage {
	b, _ := json.Marshal(&kinds.Ident{KindField: "Ident", Name: name})
	return b
}

func intLit(v string) json.RawMessage {
	b, _ := json.Marshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "INT", Value: v})
	return b
}

func strLit(v string) json.RawMessage {
	b, _ := json.Marshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "STRING", Value: v})
	return b
}

func sel(x json.RawMessage, field string) json.RawMessage {
	b, _ := json.Marshal(&kinds.SelectorExpr{KindField: "SelectorExpr", X: x, Sel: field})
	return b
}

func call(fun json.RawMessage, args ...json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(&kinds.CallExpr{KindField: "CallExpr", Fun: fun, Args: args})
	return b
}

func exprStmt(x json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(&kinds.ExprStmt{KindField: "ExprStmt", X: x})
	return b
}

func block(stmts ...json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(&kinds.BlockStmt{KindField: "BlockStmt", List: stmts})
	return b
}

func returnStmt(results ...json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(&kinds.ReturnStmt{KindField: "ReturnStmt", Results: results})
	return b
}

func funcDecl(name string, params, results []json.RawMessage, body json.RawMessage) json.RawMessage {
	ft, _ := json.Marshal(&kinds.FuncType{KindField: "FuncType", Params: params, Results: results})
	b, _ := json.Marshal(&kinds.FuncDecl{KindField: "FuncDecl", Name: name, Type: ft, Body: body})
	return b
}

func assign(tok string, lhs, rhs []json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(&kinds.AssignStmt{KindField: "AssignStmt", Lhs: lhs, Tok: tok, Rhs: rhs})
	return b
}

func field(names []string, typ json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(&kinds.Field{KindField: "Field", Names: names, Type: typ})
	return b
}

func TestGoldenPathHelloProgram(t *testing.T) {
	// Build the following program entirely from JSON node structs:
	//
	//   package main
	//
	//   import "fmt"
	//
	//   func main() {
	//       fmt.Println(true)
	//   }
	//
	// Expected output: "true\n"

	importSpecJSON, _ := json.Marshal(&kinds.ImportSpec{KindField: "ImportSpec", Path: "fmt"})
	importDeclJSON, _ := json.Marshal(&kinds.ImportDecl{
		KindField: "ImportDecl",
		Specs:     []json.RawMessage{importSpecJSON},
	})

	fmtIdentJSON, _ := json.Marshal(&kinds.Ident{KindField: "Ident", Name: "fmt"})
	selectorJSON, _ := json.Marshal(&kinds.SelectorExpr{
		KindField: "SelectorExpr",
		X:         fmtIdentJSON,
		Sel:       "Println",
	})
	trueIdentJSON, _ := json.Marshal(&kinds.Ident{KindField: "Ident", Name: "true"})

	callJSON, _ := json.Marshal(&kinds.CallExpr{
		KindField: "CallExpr",
		Fun:       selectorJSON,
		Args:      []json.RawMessage{trueIdentJSON},
		Ellipsis:  false,
	})
	exprStmtJSON, _ := json.Marshal(&kinds.ExprStmt{KindField: "ExprStmt", X: callJSON})
	bodyJSON, _ := json.Marshal(&kinds.BlockStmt{
		KindField: "BlockStmt",
		List:      []json.RawMessage{exprStmtJSON},
	})
	funcTypeJSON, _ := json.Marshal(&kinds.FuncType{
		KindField: "FuncType",
		Params:    []json.RawMessage{},
	})
	funcDeclJSON, _ := json.Marshal(&kinds.FuncDecl{
		KindField: "FuncDecl",
		Name:      "main",
		Type:      funcTypeJSON,
		Body:      bodyJSON,
	})

	importDecl, err := kinds.UnmarshalNode(importDeclJSON)
	if err != nil {
		t.Fatalf("UnmarshalNode importDecl: %v", err)
	}
	importDeclAST, err := importDecl.ToAST()
	if err != nil {
		t.Fatalf("importDecl.ToAST: %v", err)
	}

	funcDecl, err := kinds.UnmarshalNode(funcDeclJSON)
	if err != nil {
		t.Fatalf("UnmarshalNode funcDecl: %v", err)
	}
	funcDeclAST, err := funcDecl.ToAST()
	if err != nil {
		t.Fatalf("funcDecl.ToAST: %v", err)
	}

	fset := token.NewFileSet()
	file := &ast.File{
		Name:  &ast.Ident{Name: "main"},
		Decls: []ast.Decl{importDeclAST.(ast.Decl), funcDeclAST.(ast.Decl)},
	}

	var buf bytes.Buffer
	err = format.Node(&buf, fset, file)
	if err != nil {
		t.Fatalf("go/format must succeed with NoPos nodes: %v", err)
	}
	src := buf.String()

	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	err = os.WriteFile(goFile, []byte(src), 0644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := exec.Command("go", "run", goFile)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go run must succeed; source:\n%s\nerror: %v", src, err)
	}
	if string(out) != "true\n" {
		t.Errorf("expected %q, got %q; source:\n%s", "true\n", string(out), src)
	}
}

// TestGoldenChannelSendReceive generates and runs:
//
//	func main() {
//	    ch := make(chan int, 1)
//	    ch <- 42
//	    v := <-ch
//	    fmt.Println(v)
//	}
func TestGoldenChannelSendReceive(t *testing.T) {
	// ch := make(chan int, 1)
	chanType, _ := json.Marshal(&kinds.ChanType{KindField: "ChanType", Dir: "BOTH", Value: ident("int")})
	makeCall := call(ident("make"), chanType, intLit("1"))
	assignCh := assign(":=", []json.RawMessage{ident("ch")}, []json.RawMessage{makeCall})

	// ch <- 42
	send, _ := json.Marshal(&kinds.SendStmt{KindField: "SendStmt", Chan: ident("ch"), Value: intLit("42")})

	// v := <-ch
	recv, _ := json.Marshal(&kinds.UnaryExpr{KindField: "UnaryExpr", Op: "<-", X: ident("ch")})
	assignV := assign(":=", []json.RawMessage{ident("v")}, []json.RawMessage{recv})

	// fmt.Println(v)
	println := exprStmt(call(sel(ident("fmt"), "Println"), ident("v")))

	mainBody := block(assignCh, send, assignV, println)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{
		buildDecl(t, importDecl("fmt")),
		buildDecl(t, mainFn),
	})
	if strings.TrimSpace(out) != "42" {
		t.Errorf("expected 42, got %q", out)
	}
}

// TestGoldenGoroutineAndSelect generates and runs:
//
//	func main() {
//	    ch := make(chan string, 1)
//	    go func() { ch <- "hello" }()
//	    select {
//	    case msg := <-ch:
//	        fmt.Println(msg)
//	    }
//	}
func TestGoldenGoroutineAndSelect(t *testing.T) {
	// ch := make(chan string, 1)
	chanType, _ := json.Marshal(&kinds.ChanType{KindField: "ChanType", Dir: "BOTH", Value: ident("string")})
	makeCall := call(ident("make"), chanType, intLit("1"))
	assignCh := assign(":=", []json.RawMessage{ident("ch")}, []json.RawMessage{makeCall})

	// go func() { ch <- "hello" }()
	send, _ := json.Marshal(&kinds.SendStmt{KindField: "SendStmt", Chan: ident("ch"), Value: strLit(`"hello"`)})
	closureBody := block(send)
	ft, _ := json.Marshal(&kinds.FuncType{KindField: "FuncType", Params: []json.RawMessage{}})
	closure, _ := json.Marshal(&kinds.FuncLit{KindField: "FuncLit", Type: ft, Body: closureBody})
	closureCall, _ := json.Marshal(&kinds.CallExpr{KindField: "CallExpr", Fun: closure, Args: []json.RawMessage{}})
	goStmt, _ := json.Marshal(&kinds.GoStmt{KindField: "GoStmt", Call: closureCall})

	// select { case msg := <-ch: fmt.Println(msg) }
	recv, _ := json.Marshal(&kinds.UnaryExpr{KindField: "UnaryExpr", Op: "<-", X: ident("ch")})
	commAssign := assign(":=", []json.RawMessage{ident("msg")}, []json.RawMessage{recv})
	println := exprStmt(call(sel(ident("fmt"), "Println"), ident("msg")))
	commClause, _ := json.Marshal(&kinds.CommClause{
		KindField: "CommClause",
		Comm:      commAssign,
		Body:      []json.RawMessage{println},
	})
	selectBody := block(commClause)
	selectStmt, _ := json.Marshal(&kinds.SelectStmt{KindField: "SelectStmt", Body: selectBody})

	mainBody := block(assignCh, goStmt, selectStmt)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{
		buildDecl(t, importDecl("fmt")),
		buildDecl(t, mainFn),
	})
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("expected hello, got %q", out)
	}
}

// TestGoldenDeferAndClosure generates and runs:
//
//	func main() {
//	    defer fmt.Println("world")
//	    fmt.Println("hello")
//	}
func TestGoldenDeferAndClosure(t *testing.T) {
	// defer fmt.Println("world")
	deferCall, _ := json.Marshal(&kinds.CallExpr{
		KindField: "CallExpr",
		Fun:       sel(ident("fmt"), "Println"),
		Args:      []json.RawMessage{strLit(`"world"`)},
	})
	deferStmt, _ := json.Marshal(&kinds.DeferStmt{KindField: "DeferStmt", Call: deferCall})

	// fmt.Println("hello")
	printHello := exprStmt(call(sel(ident("fmt"), "Println"), strLit(`"hello"`)))

	mainBody := block(deferStmt, printHello)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{
		buildDecl(t, importDecl("fmt")),
		buildDecl(t, mainFn),
	})
	// defer runs after return, so "hello" prints first
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Errorf("expected hello\\nworld, got %q", out)
	}
}

// TestGoldenTypeSwitch generates and runs:
//
//	func describe(v interface{}) string {
//	    switch v.(type) {
//	    case int:    return "int"
//	    case string: return "string"
//	    default:     return "other"
//	    }
//	}
//	func main() { fmt.Println(describe(42)) }
func TestGoldenTypeSwitch(t *testing.T) {
	// switch v.(type) { case int: return "int" ... }
	caseInt, _ := json.Marshal(&kinds.CaseClause{
		KindField: "CaseClause",
		List:      []json.RawMessage{ident("int")},
		Body:      []json.RawMessage{returnStmt(strLit(`"int"`))},
	})
	caseStr, _ := json.Marshal(&kinds.CaseClause{
		KindField: "CaseClause",
		List:      []json.RawMessage{ident("string")},
		Body:      []json.RawMessage{returnStmt(strLit(`"string"`))},
	})
	caseDefault, _ := json.Marshal(&kinds.CaseClause{
		KindField: "CaseClause",
		List:      nil, // default
		Body:      []json.RawMessage{returnStmt(strLit(`"other"`))},
	})

	// type switch: switch v.(type)
	// Use ExprStmt (not AssignStmt) so we get plain `switch v.(type)` not `switch _ := v.(type)`
	typeAssertExpr, _ := json.Marshal(&kinds.TypeAssertExpr{KindField: "TypeAssertExpr", X: ident("v"), Type: nil})
	assignStmt, _ := json.Marshal(&kinds.ExprStmt{KindField: "ExprStmt", X: typeAssertExpr})
	typeSwitchBody := block(caseInt, caseStr, caseDefault)
	typeSwitch, _ := json.Marshal(&kinds.TypeSwitchStmt{
		KindField: "TypeSwitchStmt",
		Assign:    assignStmt,
		Body:      typeSwitchBody,
	})

	// func describe(v interface{}) string
	ifaceType, _ := json.Marshal(&kinds.InterfaceType{KindField: "InterfaceType", Methods: []json.RawMessage{}})
	param := field([]string{"v"}, ifaceType)
	result := field(nil, ident("string"))
	describeFn := funcDecl("describe",
		[]json.RawMessage{param},
		[]json.RawMessage{result},
		block(typeSwitch),
	)

	// func main() { fmt.Println(describe(42)) }
	descCall := call(ident("describe"), intLit("42"))
	mainBody := block(exprStmt(call(sel(ident("fmt"), "Println"), descCall)))
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{
		buildDecl(t, importDecl("fmt")),
		buildDecl(t, describeFn),
		buildDecl(t, mainFn),
	})
	if strings.TrimSpace(out) != "int" {
		t.Errorf("expected int, got %q", out)
	}
}

// TestGoldenRangeOverSlice generates and runs:
//
//	func main() {
//	    nums := []int{1, 2, 3}
//	    sum := 0
//	    for _, v := range nums {
//	        sum += v
//	    }
//	    fmt.Println(sum)
//	}
func TestGoldenRangeOverSlice(t *testing.T) {
	// nums := []int{1, 2, 3}
	sliceType, _ := json.Marshal(&kinds.ArrayType{KindField: "ArrayType", Elt: ident("int")})
	one, two, three := intLit("1"), intLit("2"), intLit("3")
	lit, _ := json.Marshal(&kinds.CompositeLit{KindField: "CompositeLit", Type: sliceType, Elts: []json.RawMessage{one, two, three}})
	assignNums := assign(":=", []json.RawMessage{ident("nums")}, []json.RawMessage{lit})

	// sum := 0
	assignSum := assign(":=", []json.RawMessage{ident("sum")}, []json.RawMessage{intLit("0")})

	// sum += v
	addAssign := assign("+=", []json.RawMessage{ident("sum")}, []json.RawMessage{ident("v")})

	// for _, v := range nums
	rangeStmt, _ := json.Marshal(&kinds.RangeStmt{
		KindField: "RangeStmt",
		Key:       ident("_"),
		Value:     ident("v"),
		Tok:       ":=",
		X:         ident("nums"),
		Body:      block(addAssign),
	})

	// fmt.Println(sum)
	println := exprStmt(call(sel(ident("fmt"), "Println"), ident("sum")))

	mainBody := block(assignNums, assignSum, rangeStmt, println)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{
		buildDecl(t, importDecl("fmt")),
		buildDecl(t, mainFn),
	})
	if strings.TrimSpace(out) != "6" {
		t.Errorf("expected 6, got %q", out)
	}
}

// TestGoldenForLoop generates and runs a classic 3-clause for loop summing 1..5.
func TestGoldenForLoop(t *testing.T) {
	// for i := 0; i < 5; i++ { sum += i+1 }
	// Actually: for i := 1; i <= 5; i++ { sum += i }
	// sum := 0
	assignSum := assign(":=", []json.RawMessage{ident("sum")}, []json.RawMessage{intLit("0")})

	// sum += i
	addAssign := assign("+=", []json.RawMessage{ident("sum")}, []json.RawMessage{ident("i")})

	// i++
	inc, _ := json.Marshal(&kinds.IncDecStmt{KindField: "IncDecStmt", X: ident("i"), Tok: "++"})

	// for i := 1; i <= 5; i++
	cond, _ := json.Marshal(&kinds.BinaryExpr{KindField: "BinaryExpr", X: ident("i"), Op: "<=", Y: intLit("5")})
	init := assign(":=", []json.RawMessage{ident("i")}, []json.RawMessage{intLit("1")})
	forStmt, _ := json.Marshal(&kinds.ForStmt{
		KindField: "ForStmt",
		Init:      init,
		Cond:      cond,
		Post:      inc,
		Body:      block(addAssign),
	})

	println := exprStmt(call(sel(ident("fmt"), "Println"), ident("sum")))
	mainBody := block(assignSum, forStmt, println)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{buildDecl(t, importDecl("fmt")), buildDecl(t, mainFn)})
	if strings.TrimSpace(out) != "15" {
		t.Errorf("expected 15, got %q", out)
	}
}

// TestGoldenExprSwitch generates and runs an expression switch.
func TestGoldenExprSwitch(t *testing.T) {
	// x := 2; switch x { case 1: fmt.Println("one") case 2: fmt.Println("two") default: fmt.Println("other") }
	assignX := assign(":=", []json.RawMessage{ident("x")}, []json.RawMessage{intLit("2")})

	case1, _ := json.Marshal(&kinds.CaseClause{KindField: "CaseClause", List: []json.RawMessage{intLit("1")}, Body: []json.RawMessage{exprStmt(call(sel(ident("fmt"), "Println"), strLit(`"one"`)))}})
	case2, _ := json.Marshal(&kinds.CaseClause{KindField: "CaseClause", List: []json.RawMessage{intLit("2")}, Body: []json.RawMessage{exprStmt(call(sel(ident("fmt"), "Println"), strLit(`"two"`)))}})
	caseDefault, _ := json.Marshal(&kinds.CaseClause{KindField: "CaseClause", List: nil, Body: []json.RawMessage{exprStmt(call(sel(ident("fmt"), "Println"), strLit(`"other"`)))}})

	switchStmt, _ := json.Marshal(&kinds.SwitchStmt{KindField: "SwitchStmt", Tag: ident("x"), Body: block(case1, case2, caseDefault)})
	mainBody := block(assignX, switchStmt)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{buildDecl(t, importDecl("fmt")), buildDecl(t, mainFn)})
	if strings.TrimSpace(out) != "two" {
		t.Errorf("expected two, got %q", out)
	}
}

// TestGoldenIfElse generates and runs if/else.
func TestGoldenIfElse(t *testing.T) {
	// x := 10; if x > 5 { fmt.Println("big") } else { fmt.Println("small") }
	assignX := assign(":=", []json.RawMessage{ident("x")}, []json.RawMessage{intLit("10")})
	cond, _ := json.Marshal(&kinds.BinaryExpr{KindField: "BinaryExpr", X: ident("x"), Op: ">", Y: intLit("5")})
	thenBlock := block(exprStmt(call(sel(ident("fmt"), "Println"), strLit(`"big"`))))
	elseBlock := block(exprStmt(call(sel(ident("fmt"), "Println"), strLit(`"small"`))))
	ifStmt, _ := json.Marshal(&kinds.IfStmt{KindField: "IfStmt", Cond: cond, Body: thenBlock, Else: elseBlock})
	mainBody := block(assignX, ifStmt)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{buildDecl(t, importDecl("fmt")), buildDecl(t, mainFn)})
	if strings.TrimSpace(out) != "big" {
		t.Errorf("expected big, got %q", out)
	}
}

// TestGoldenMultipleReturnAndError generates and runs a function returning (int, error).
func TestGoldenMultipleReturnAndError(t *testing.T) {
	// func divide(a, b int) (int, error) {
	//     if b == 0 { return 0, errors.New("div by zero") }
	//     return a / b, nil
	// }
	// func main() { v, err := divide(10, 2); if err != nil { ... } fmt.Println(v) }
	errPkg := ident("errors")
	newErr := call(sel(errPkg, "New"), strLit(`"div by zero"`))
	zeroCond, _ := json.Marshal(&kinds.BinaryExpr{KindField: "BinaryExpr", X: ident("b"), Op: "==", Y: intLit("0")})
	guardBody := block(returnStmt(intLit("0"), newErr))
	guard, _ := json.Marshal(&kinds.IfStmt{KindField: "IfStmt", Cond: zeroCond, Body: guardBody})

	div, _ := json.Marshal(&kinds.BinaryExpr{KindField: "BinaryExpr", X: ident("a"), Op: "/", Y: ident("b")})
	nilIdent, _ := json.Marshal(&kinds.Ident{KindField: "Ident", Name: "nil"})
	divBody := block(guard, returnStmt(div, nilIdent))

	intType := ident("int")
	errType := ident("error")
	params := []json.RawMessage{field([]string{"a", "b"}, intType)}
	results := []json.RawMessage{field(nil, intType), field(nil, errType)}
	divideFn := funcDecl("divide", params, results, divBody)

	// main: v, err := divide(10, 2); if err != nil { panic(...) }; fmt.Println(v)
	divCall := call(ident("divide"), intLit("10"), intLit("2"))
	assignVErr := assign(":=", []json.RawMessage{ident("v"), ident("err")}, []json.RawMessage{divCall})
	errCheck, _ := json.Marshal(&kinds.BinaryExpr{KindField: "BinaryExpr", X: ident("err"), Op: "!=", Y: nilIdent})
	panicCall := exprStmt(call(ident("panic"), ident("err")))
	errIf, _ := json.Marshal(&kinds.IfStmt{KindField: "IfStmt", Cond: errCheck, Body: block(panicCall)})
	println := exprStmt(call(sel(ident("fmt"), "Println"), ident("v")))
	mainBody := block(assignVErr, errIf, println)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{
		buildDecl(t, importDecl("errors", "fmt")),
		buildDecl(t, divideFn),
		buildDecl(t, mainFn),
	})
	if strings.TrimSpace(out) != "5" {
		t.Errorf("expected 5, got %q", out)
	}
}

// TestGoldenMapOperations generates and runs map set/get.
func TestGoldenMapOperations(t *testing.T) {
	// m := map[string]int{}; m["a"] = 1; m["b"] = 2; fmt.Println(m["a"] + m["b"])
	mapType, _ := json.Marshal(&kinds.MapType{KindField: "MapType", Key: ident("string"), Value: ident("int")})
	emptyLit, _ := json.Marshal(&kinds.CompositeLit{KindField: "CompositeLit", Type: mapType, Elts: []json.RawMessage{}})
	assignM := assign(":=", []json.RawMessage{ident("m")}, []json.RawMessage{emptyLit})

	// m["a"] = 1
	idxA, _ := json.Marshal(&kinds.IndexExpr{KindField: "IndexExpr", X: ident("m"), Index: strLit(`"a"`)})
	setA := assign("=", []json.RawMessage{idxA}, []json.RawMessage{intLit("1")})
	idxB, _ := json.Marshal(&kinds.IndexExpr{KindField: "IndexExpr", X: ident("m"), Index: strLit(`"b"`)})
	setB := assign("=", []json.RawMessage{idxB}, []json.RawMessage{intLit("2")})

	// fmt.Println(m["a"] + m["b"])
	sum, _ := json.Marshal(&kinds.BinaryExpr{KindField: "BinaryExpr", X: idxA, Op: "+", Y: idxB})
	println := exprStmt(call(sel(ident("fmt"), "Println"), sum))

	mainBody := block(assignM, setA, setB, println)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{buildDecl(t, importDecl("fmt")), buildDecl(t, mainFn)})
	if strings.TrimSpace(out) != "3" {
		t.Errorf("expected 3, got %q", out)
	}
}

// TestGoldenClosureCapture generates a closure that captures a variable.
func TestGoldenClosureCapture(t *testing.T) {
	// n := 10; add := func(x int) int { return n + x }; fmt.Println(add(5))
	assignN := assign(":=", []json.RawMessage{ident("n")}, []json.RawMessage{intLit("10")})

	// func(x int) int { return n + x }
	xParam := field([]string{"x"}, ident("int"))
	intResult := field(nil, ident("int"))
	addBody := block(returnStmt(func() json.RawMessage {
		b, _ := json.Marshal(&kinds.BinaryExpr{KindField: "BinaryExpr", X: ident("n"), Op: "+", Y: ident("x")})
		return b
	}()))
	ft, _ := json.Marshal(&kinds.FuncType{KindField: "FuncType", Params: []json.RawMessage{xParam}, Results: []json.RawMessage{intResult}})
	closure, _ := json.Marshal(&kinds.FuncLit{KindField: "FuncLit", Type: ft, Body: addBody})
	assignAdd := assign(":=", []json.RawMessage{ident("add")}, []json.RawMessage{closure})

	// fmt.Println(add(5))
	addCall := call(ident("add"), intLit("5"))
	println := exprStmt(call(sel(ident("fmt"), "Println"), addCall))

	mainBody := block(assignN, assignAdd, println)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{buildDecl(t, importDecl("fmt")), buildDecl(t, mainFn)})
	if strings.TrimSpace(out) != "15" {
		t.Errorf("expected 15, got %q", out)
	}
}

// TestGoldenStructAndMethod generates a struct type + method + instantiation.
func TestGoldenStructAndMethod(t *testing.T) {
	// type Point struct { X, Y int }
	// func (p Point) Sum() int { return p.X + p.Y }
	// func main() { pt := Point{X: 3, Y: 4}; fmt.Println(pt.Sum()) }
	intType := ident("int")
	xyField := field([]string{"X", "Y"}, intType)
	structType, _ := json.Marshal(&kinds.StructType{KindField: "StructType", Fields: []json.RawMessage{xyField}})
	typeSpec, _ := json.Marshal(&kinds.TypeSpec{KindField: "TypeSpec", Name: "Point", Type: structType})
	typeDecl, _ := json.Marshal(&kinds.TypeDecl{KindField: "TypeDecl", Specs: []json.RawMessage{typeSpec}})

	// func (p Point) Sum() int { return p.X + p.Y }
	pxSel := sel(ident("p"), "X")
	pySel := sel(ident("p"), "Y")
	sumExpr, _ := json.Marshal(&kinds.BinaryExpr{KindField: "BinaryExpr", X: pxSel, Op: "+", Y: pySel})
	sumBody := block(returnStmt(sumExpr))
	recv := field([]string{"p"}, ident("Point"))
	recvJSON, _ := json.Marshal(recv)
	ft, _ := json.Marshal(&kinds.FuncType{KindField: "FuncType", Params: []json.RawMessage{}, Results: []json.RawMessage{field(nil, intType)}})
	sumFn, _ := json.Marshal(&kinds.FuncDecl{KindField: "FuncDecl", Recv: recvJSON, Name: "Sum", Type: ft, Body: sumBody})

	// Point{X: 3, Y: 4}
	kv3, _ := json.Marshal(&kinds.KeyValueExpr{KindField: "KeyValueExpr", Key: ident("X"), Value: intLit("3")})
	kv4, _ := json.Marshal(&kinds.KeyValueExpr{KindField: "KeyValueExpr", Key: ident("Y"), Value: intLit("4")})
	ptLit, _ := json.Marshal(&kinds.CompositeLit{KindField: "CompositeLit", Type: ident("Point"), Elts: []json.RawMessage{kv3, kv4}})
	assignPt := assign(":=", []json.RawMessage{ident("pt")}, []json.RawMessage{ptLit})
	println := exprStmt(call(sel(ident("fmt"), "Println"), call(sel(ident("pt"), "Sum"))))
	mainBody := block(assignPt, println)
	mainFn := funcDecl("main", nil, nil, mainBody)

	out := runProgram(t, []ast.Decl{
		buildDecl(t, importDecl("fmt")),
		buildDecl(t, typeDecl),
		buildDecl(t, sumFn),
		buildDecl(t, mainFn),
	})
	if strings.TrimSpace(out) != "7" {
		t.Errorf("expected 7, got %q", out)
	}
}
