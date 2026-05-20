// Namespace: goast/kinds/type
// Kind: MapType
// go/ast: *ast.MapType
package kinds

import (
	"encoding/json"
	"go/ast"
)

type MapType struct {
	KindField string          `json:"kind"`
	Key       json.RawMessage `json:"key"`
	Value     json.RawMessage `json:"value"`
}

func (n *MapType) Kind() string { return "MapType" }

func (n *MapType) ToAST() (ast.Node, error) {
	key, err := unmarshalExpr(n.Key, "MapType.Key")
	if err != nil {
		return nil, err
	}
	value, err := unmarshalExpr(n.Value, "MapType.Value")
	if err != nil {
		return nil, err
	}
	return &ast.MapType{Key: key, Value: value}, nil
}

func (n *MapType) FromAST(node ast.Node) error {
	m := node.(*ast.MapType)
	n.KindField = "MapType"
	var err error
	n.Key, err = MarshalExpr(m.Key)
	if err != nil {
		return err
	}
	n.Value, err = MarshalExpr(m.Value)
	return err
}

func init() { register("MapType", func() Node { return &MapType{} }) }
