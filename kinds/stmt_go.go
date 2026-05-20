// Namespace: goast/kinds/stmt
// Kind: GoStmt
// go/ast: *ast.GoStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
)

type GoStmt struct {
	KindField string          `json:"kind"`
	Call      json.RawMessage `json:"call"`
}

func (n *GoStmt) Kind() string { return "GoStmt" }

func (n *GoStmt) ToAST() (ast.Node, error) {
	callNode, err := UnmarshalNode(n.Call)
	if err != nil {
		return nil, fmt.Errorf("GoStmt.Call: %w", err)
	}
	callAST, err := callNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("GoStmt.Call: %w", err)
	}
	call, ok := callAST.(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("GoStmt.Call: expected *ast.CallExpr, got %T", callAST)
	}
	return &ast.GoStmt{Go: token.NoPos, Call: call}, nil
}

func (n *GoStmt) FromAST(node ast.Node) error {
	s := node.(*ast.GoStmt)
	n.KindField = "GoStmt"
	var err error
	n.Call, err = MarshalNode(s.Call)
	return err
}

func init() { register("GoStmt", func() Node { return &GoStmt{} }) }
