// Namespace: goast/kinds/expr
// Kind: Ellipsis
// go/ast: *ast.Ellipsis
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type Ellipsis struct {
	KindField string          `json:"kind"`
	Elt       json.RawMessage `json:"elt,omitempty"`
}

func (n *Ellipsis) Kind() string { return "Ellipsis" }

func (n *Ellipsis) ToAST() (ast.Node, error) {
	var elt ast.Expr
	if len(n.Elt) > 0 && string(n.Elt) != "null" {
		var err error
		elt, err = unmarshalExpr(n.Elt, "Ellipsis.Elt")
		if err != nil {
			return nil, err
		}
	}
	return &ast.Ellipsis{Ellipsis: token.NoPos, Elt: elt}, nil
}

func (n *Ellipsis) FromAST(node ast.Node) error {
	e := node.(*ast.Ellipsis)
	n.KindField = "Ellipsis"
	if e.Elt != nil {
		var err error
		n.Elt, err = MarshalExpr(e.Elt)
		if err != nil {
			return err
		}
	}
	return nil
}

func init() { register("Ellipsis", func() Node { return &Ellipsis{} }) }
