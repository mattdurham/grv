// Namespace: goast/kinds/expr
// Kind: StarExpr
// go/ast: *ast.StarExpr
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type StarExpr struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
}

func (n *StarExpr) Kind() string { return "StarExpr" }

func (n *StarExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "StarExpr.X")
	if err != nil {
		return nil, err
	}
	return &ast.StarExpr{Star: token.NoPos, X: x}, nil
}

func (n *StarExpr) FromAST(node ast.Node) error {
	s := node.(*ast.StarExpr)
	n.KindField = "StarExpr"
	var err error
	n.X, err = MarshalExpr(s.X)
	return err
}

func init() { register("StarExpr", func() Node { return &StarExpr{} }) }
