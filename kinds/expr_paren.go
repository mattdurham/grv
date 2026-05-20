// Namespace: goast/kinds/expr
// Kind: ParenExpr
// go/ast: *ast.ParenExpr
package kinds

import (
	"encoding/json"
	"go/ast"
)

type ParenExpr struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
}

func (n *ParenExpr) Kind() string { return "ParenExpr" }

func (n *ParenExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "ParenExpr.X")
	if err != nil {
		return nil, err
	}
	return &ast.ParenExpr{X: x}, nil
}

func (n *ParenExpr) FromAST(node ast.Node) error {
	p := node.(*ast.ParenExpr)
	n.KindField = "ParenExpr"
	var err error
	n.X, err = MarshalExpr(p.X)
	return err
}

func init() { register("ParenExpr", func() Node { return &ParenExpr{} }) }
