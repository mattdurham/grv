// Namespace: goast/kinds/type
// Kind: StructType
// go/ast: *ast.StructType
package kinds

import (
	"encoding/json"
	"go/ast"
)

type StructType struct {
	KindField string            `json:"kind"`
	Fields    []json.RawMessage `json:"fields"`
}

func (n *StructType) Kind() string { return "StructType" }

func (n *StructType) ToAST() (ast.Node, error) {
	fields, err := unmarshalFieldList(n.Fields)
	if err != nil {
		return nil, err
	}
	return &ast.StructType{Fields: &ast.FieldList{List: fields}}, nil
}

func (n *StructType) FromAST(node ast.Node) error {
	s := node.(*ast.StructType)
	n.KindField = "StructType"
	if s.Fields == nil {
		n.Fields = []json.RawMessage{}
		return nil
	}
	var err error
	n.Fields, err = marshalFields(s.Fields.List)
	return err
}

func init() { register("StructType", func() Node { return &StructType{} }) }
