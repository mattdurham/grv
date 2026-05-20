// Namespace: goast/kinds/spec
// Kind: TypeSpec
// go/ast: *ast.TypeSpec
package kinds

import (
	"encoding/json"
	"go/ast"
)

type TypeSpec struct {
	KindField  string            `json:"kind"`
	Name       string            `json:"name"`
	TypeParams []json.RawMessage `json:"type_params,omitempty"`
	Type       json.RawMessage   `json:"type"`
}

func (n *TypeSpec) Kind() string { return "TypeSpec" }

func (n *TypeSpec) ToAST() (ast.Node, error) {
	result := &ast.TypeSpec{
		Name: &ast.Ident{Name: n.Name},
	}
	if len(n.TypeParams) > 0 {
		fields, err := unmarshalFieldList(n.TypeParams)
		if err != nil {
			return nil, err
		}
		result.TypeParams = &ast.FieldList{List: fields}
	}
	typ, err := unmarshalExpr(n.Type, "TypeSpec.Type")
	if err != nil {
		return nil, err
	}
	result.Type = typ
	return result, nil
}

func (n *TypeSpec) FromAST(node ast.Node) error {
	s := node.(*ast.TypeSpec)
	n.KindField = "TypeSpec"
	n.Name = s.Name.Name
	var err error
	if s.TypeParams != nil && len(s.TypeParams.List) > 0 {
		n.TypeParams, err = marshalFields(s.TypeParams.List)
		if err != nil {
			return err
		}
	}
	n.Type, err = MarshalExpr(s.Type)
	return err
}

func init() { register("TypeSpec", func() Node { return &TypeSpec{} }) }
