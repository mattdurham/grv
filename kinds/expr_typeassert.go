// Namespace: goast/kinds/expr
// Kind: TypeAssertExpr
// go/ast: *ast.TypeAssertExpr
package kinds

import (
	"encoding/json"
	"go/ast"
)

type TypeAssertExpr struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
	Type      json.RawMessage `json:"type,omitempty"`
}

func (n *TypeAssertExpr) Kind() string { return "TypeAssertExpr" }

func (n *TypeAssertExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "TypeAssertExpr.X")
	if err != nil {
		return nil, err
	}
	typ, err := unmarshalExpr(n.Type, "TypeAssertExpr.Type")
	if err != nil {
		return nil, err
	}
	return &ast.TypeAssertExpr{X: x, Type: typ}, nil
}

func (n *TypeAssertExpr) FromAST(node ast.Node) error {
	t := node.(*ast.TypeAssertExpr)
	n.KindField = "TypeAssertExpr"
	var err error
	n.X, err = MarshalExpr(t.X)
	if err != nil {
		return err
	}
	if t.Type != nil {
		n.Type, err = MarshalExpr(t.Type)
	}
	return err
}

func init() { register("TypeAssertExpr", func() Node { return &TypeAssertExpr{} }) }
