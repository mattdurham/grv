// Namespace: goast/kinds/stmt
// Kind: DeferStmt
// go/ast: *ast.DeferStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
)

type DeferStmt struct {
	KindField string          `json:"kind"`
	Call      json.RawMessage `json:"call"`
}

func (n *DeferStmt) Kind() string { return "DeferStmt" }

func (n *DeferStmt) ToAST() (ast.Node, error) {
	callNode, err := UnmarshalNode(n.Call)
	if err != nil {
		return nil, fmt.Errorf("DeferStmt.Call: %w", err)
	}
	callAST, err := callNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("DeferStmt.Call: %w", err)
	}
	call, ok := callAST.(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("DeferStmt.Call: expected *ast.CallExpr, got %T", callAST)
	}
	return &ast.DeferStmt{Defer: token.NoPos, Call: call}, nil
}

func (n *DeferStmt) FromAST(node ast.Node) error {
	s := node.(*ast.DeferStmt)
	n.KindField = "DeferStmt"
	var err error
	n.Call, err = MarshalNode(s.Call)
	return err
}

func init() { register("DeferStmt", func() Node { return &DeferStmt{} }) }
