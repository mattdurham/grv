// Namespace: goast/kinds/type
// Kind: ArrayType
// go/ast: *ast.ArrayType
package kinds

import (
	"encoding/json"
	"go/ast"
)

type ArrayType struct {
	KindField string          `json:"kind"`
	Len       json.RawMessage `json:"len,omitempty"`
	Elt       json.RawMessage `json:"elt"`
}

func (n *ArrayType) Kind() string { return "ArrayType" }

func (n *ArrayType) ToAST() (ast.Node, error) {
	var length ast.Expr
	if len(n.Len) > 0 && string(n.Len) != "null" {
		var err error
		length, err = unmarshalExpr(n.Len, "ArrayType.Len")
		if err != nil {
			return nil, err
		}
	}
	elt, err := unmarshalExpr(n.Elt, "ArrayType.Elt")
	if err != nil {
		return nil, err
	}
	return &ast.ArrayType{Len: length, Elt: elt}, nil
}

func (n *ArrayType) FromAST(node ast.Node) error {
	a := node.(*ast.ArrayType)
	n.KindField = "ArrayType"
	var err error
	if a.Len != nil {
		n.Len, err = MarshalExpr(a.Len)
		if err != nil {
			return err
		}
	}
	n.Elt, err = MarshalExpr(a.Elt)
	return err
}

func init() { register("ArrayType", func() Node { return &ArrayType{} }) }
