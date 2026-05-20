// Namespace: goast/kinds/stmt
// Kind: DeclStmt
// go/ast: *ast.DeclStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type DeclStmt struct {
	KindField string          `json:"kind"`
	Decl      json.RawMessage `json:"decl"`
}

func (n *DeclStmt) Kind() string { return "DeclStmt" }

func (n *DeclStmt) ToAST() (ast.Node, error) {
	declNode, err := UnmarshalNode(n.Decl)
	if err != nil {
		return nil, fmt.Errorf("DeclStmt.Decl: %w", err)
	}
	declAST, err := declNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("DeclStmt.Decl: %w", err)
	}
	decl, ok := declAST.(ast.Decl)
	if !ok {
		return nil, fmt.Errorf("DeclStmt.Decl: expected ast.Decl, got %T", declAST)
	}
	return &ast.DeclStmt{Decl: decl}, nil
}

func (n *DeclStmt) FromAST(node ast.Node) error {
	s := node.(*ast.DeclStmt)
	n.KindField = "DeclStmt"
	var err error
	n.Decl, err = MarshalDecl(s.Decl)
	return err
}

func init() { register("DeclStmt", func() Node { return &DeclStmt{} }) }
