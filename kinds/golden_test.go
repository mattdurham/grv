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
	"testing"

	"github.com/lthiery/goast/kinds"
)

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
