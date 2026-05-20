// Namespace: goast/kinds/expr
// Kind: CompositeLit
// go/ast: *ast.CompositeLit
package kinds

import (
	"encoding/json"
	"go/ast"
)

type CompositeLit struct {
	KindField string            `json:"kind"`
	Type      json.RawMessage   `json:"type,omitempty"`
	Elts      []json.RawMessage `json:"elts"`
}

func (n *CompositeLit) Kind() string { return "CompositeLit" }

func (n *CompositeLit) ToAST() (ast.Node, error) {
	var typ ast.Expr
	if len(n.Type) > 0 && string(n.Type) != "null" {
		var err error
		typ, err = unmarshalExpr(n.Type, "CompositeLit.Type")
		if err != nil {
			return nil, err
		}
	}
	elts, err := unmarshalExprList(n.Elts)
	if err != nil {
		return nil, err
	}
	return &ast.CompositeLit{Type: typ, Elts: elts}, nil
}

func (n *CompositeLit) FromAST(node ast.Node) error {
	c := node.(*ast.CompositeLit)
	n.KindField = "CompositeLit"
	var err error
	if c.Type != nil {
		n.Type, err = MarshalExpr(c.Type)
		if err != nil {
			return err
		}
	}
	n.Elts = make([]json.RawMessage, 0, len(c.Elts))
	for _, elt := range c.Elts {
		m, err := MarshalExpr(elt)
		if err != nil {
			return err
		}
		n.Elts = append(n.Elts, m)
	}
	return nil
}

func init() { register("CompositeLit", func() Node { return &CompositeLit{} }) }
