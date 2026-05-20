// Namespace: goast/kinds/stmt
// Kind: ExprStmt
// go/ast: *ast.ExprStmt
package kinds

import (
	"encoding/json"
	"go/ast"
)

type ExprStmt struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
}

func (n *ExprStmt) Kind() string { return "ExprStmt" }

func (n *ExprStmt) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "ExprStmt.X")
	if err != nil {
		return nil, err
	}
	return &ast.ExprStmt{X: x}, nil
}

func (n *ExprStmt) FromAST(node ast.Node) error {
	s := node.(*ast.ExprStmt)
	n.KindField = "ExprStmt"
	var err error
	n.X, err = MarshalExpr(s.X)
	return err
}

func init() { register("ExprStmt", func() Node { return &ExprStmt{} }) }
