// Namespace: goast/kinds/type
// Kind: FuncType
// go/ast: *ast.FuncType
package kinds

import (
	"encoding/json"
	"go/ast"
)

type FuncType struct {
	KindField  string            `json:"kind"`
	TypeParams []json.RawMessage `json:"type_params,omitempty"`
	Params     []json.RawMessage `json:"params"`
	Results    []json.RawMessage `json:"results,omitempty"`
}

func (n *FuncType) Kind() string { return "FuncType" }

func (n *FuncType) ToAST() (ast.Node, error) {
	params, err := unmarshalFieldList(n.Params)
	if err != nil {
		return nil, err
	}
	result := &ast.FuncType{
		Params: &ast.FieldList{List: params},
	}
	if len(n.Results) > 0 {
		results, err := unmarshalFieldList(n.Results)
		if err != nil {
			return nil, err
		}
		result.Results = &ast.FieldList{List: results}
	}
	if len(n.TypeParams) > 0 {
		typeParams, err := unmarshalFieldList(n.TypeParams)
		if err != nil {
			return nil, err
		}
		result.TypeParams = &ast.FieldList{List: typeParams}
	}
	return result, nil
}

func (n *FuncType) FromAST(node ast.Node) error {
	f := node.(*ast.FuncType)
	n.KindField = "FuncType"
	var err error
	if f.Params != nil {
		n.Params, err = marshalFields(f.Params.List)
		if err != nil {
			return err
		}
	} else {
		n.Params = []json.RawMessage{}
	}
	if f.Results != nil && len(f.Results.List) > 0 {
		n.Results, err = marshalFields(f.Results.List)
		if err != nil {
			return err
		}
	}
	if f.TypeParams != nil && len(f.TypeParams.List) > 0 {
		n.TypeParams, err = marshalFields(f.TypeParams.List)
		if err != nil {
			return err
		}
	}
	return nil
}

func init() { register("FuncType", func() Node { return &FuncType{} }) }
