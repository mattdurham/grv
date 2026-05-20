// Namespace: goast/kinds/expr
// Kind: SelectorExpr
// go/ast: *ast.SelectorExpr
package kinds

import (
	"encoding/json"
	"go/ast"
)

type SelectorExpr struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
	Sel       string          `json:"sel"`
}

func (n *SelectorExpr) Kind() string { return "SelectorExpr" }

func (n *SelectorExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "SelectorExpr.X")
	if err != nil {
		return nil, err
	}
	return &ast.SelectorExpr{X: x, Sel: &ast.Ident{Name: n.Sel}}, nil
}

func (n *SelectorExpr) FromAST(node ast.Node) error {
	s := node.(*ast.SelectorExpr)
	n.KindField = "SelectorExpr"
	var err error
	n.X, err = MarshalExpr(s.X)
	if err != nil {
		return err
	}
	n.Sel = s.Sel.Name
	return nil
}

func init() { register("SelectorExpr", func() Node { return &SelectorExpr{} }) }
