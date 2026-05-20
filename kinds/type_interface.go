// Namespace: goast/kinds/type
// Kind: InterfaceType
// go/ast: *ast.InterfaceType
package kinds

import (
	"encoding/json"
	"go/ast"
)

type InterfaceType struct {
	KindField string            `json:"kind"`
	Methods   []json.RawMessage `json:"methods"`
}

func (n *InterfaceType) Kind() string { return "InterfaceType" }

func (n *InterfaceType) ToAST() (ast.Node, error) {
	fields, err := unmarshalFieldList(n.Methods)
	if err != nil {
		return nil, err
	}
	return &ast.InterfaceType{Methods: &ast.FieldList{List: fields}}, nil
}

func (n *InterfaceType) FromAST(node ast.Node) error {
	iface := node.(*ast.InterfaceType)
	n.KindField = "InterfaceType"
	if iface.Methods == nil {
		n.Methods = []json.RawMessage{}
		return nil
	}
	var err error
	n.Methods, err = marshalFields(iface.Methods.List)
	return err
}

func init() { register("InterfaceType", func() Node { return &InterfaceType{} }) }
