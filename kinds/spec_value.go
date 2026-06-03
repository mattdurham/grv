// Namespace: goast/kinds/spec
// Kind: ValueSpec
// go/ast: *ast.ValueSpec
package kinds

import (
	"encoding/json"
	"go/ast"
)

type ValueSpec struct {
	KindField string            `json:"kind"`
	Names     []string          `json:"names"`
	Type      json.RawMessage   `json:"type,omitempty"`
	Values    []json.RawMessage `json:"values,omitempty"`
}

func (n *ValueSpec) Kind() string { return "ValueSpec" }

func (n *ValueSpec) ToAST() (ast.Node, error) {
	result := &ast.ValueSpec{}
	for _, name := range n.Names {
		result.Names = append(result.Names, &ast.Ident{Name: name})
	}
	if len(n.Type) > 0 && string(n.Type) != "null" {
		typ, err := unmarshalExpr(n.Type, "ValueSpec.Type")
		if err != nil {
			return nil, err
		}
		result.Type = typ
	}
	values, err := unmarshalExprList(n.Values)
	if err != nil {
		return nil, err
	}
	if len(values) > 0 {
		result.Values = values
	}
	return result, nil
}

func (n *ValueSpec) FromAST(node ast.Node) error {
	s := node.(*ast.ValueSpec)
	n.KindField = "ValueSpec"
	for _, name := range s.Names {
		n.Names = append(n.Names, name.Name)
	}
	var err error
	if s.Type != nil {
		n.Type, err = MarshalExpr(s.Type)
		if err != nil {
			return err
		}
	}
	if len(s.Values) > 0 {
		n.Values = make([]json.RawMessage, 0, len(s.Values))
		for _, val := range s.Values {
			m, err := MarshalExpr(val)
			if err != nil {
				return err
			}
			n.Values = append(n.Values, m)
		}
	}
	return nil
}

func init() { register("ValueSpec", func() Node { return &ValueSpec{} }) }
