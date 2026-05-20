// Namespace: goast/kinds/expr
// Kind: KeyValueExpr
// go/ast: *ast.KeyValueExpr
package kinds

import (
	"encoding/json"
	"go/ast"
)

type KeyValueExpr struct {
	KindField string          `json:"kind"`
	Key       json.RawMessage `json:"key"`
	Value     json.RawMessage `json:"value"`
}

func (n *KeyValueExpr) Kind() string { return "KeyValueExpr" }

func (n *KeyValueExpr) ToAST() (ast.Node, error) {
	key, err := unmarshalExpr(n.Key, "KeyValueExpr.Key")
	if err != nil {
		return nil, err
	}
	value, err := unmarshalExpr(n.Value, "KeyValueExpr.Value")
	if err != nil {
		return nil, err
	}
	return &ast.KeyValueExpr{Key: key, Value: value}, nil
}

func (n *KeyValueExpr) FromAST(node ast.Node) error {
	k := node.(*ast.KeyValueExpr)
	n.KindField = "KeyValueExpr"
	var err error
	n.Key, err = MarshalExpr(k.Key)
	if err != nil {
		return err
	}
	n.Value, err = MarshalExpr(k.Value)
	return err
}

func init() { register("KeyValueExpr", func() Node { return &KeyValueExpr{} }) }
