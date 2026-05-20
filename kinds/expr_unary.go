// Namespace: goast/kinds/expr
// Kind: UnaryExpr
// go/ast: *ast.UnaryExpr
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type UnaryExpr struct {
	KindField string          `json:"kind"`
	Op        string          `json:"op"`
	X         json.RawMessage `json:"x"`
}

func (n *UnaryExpr) Kind() string { return "UnaryExpr" }

func (n *UnaryExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "UnaryExpr.X")
	if err != nil {
		return nil, err
	}
	op := tokenFromString(n.Op)
	if op == 0 {
		return nil, fmt.Errorf("UnaryExpr.Op: unknown operator %q", n.Op)
	}
	return &ast.UnaryExpr{Op: op, X: x}, nil
}

func (n *UnaryExpr) FromAST(node ast.Node) error {
	u := node.(*ast.UnaryExpr)
	n.KindField = "UnaryExpr"
	n.Op = u.Op.String()
	var err error
	n.X, err = MarshalExpr(u.X)
	return err
}

func init() { register("UnaryExpr", func() Node { return &UnaryExpr{} }) }
