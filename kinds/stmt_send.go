// Namespace: goast/kinds/stmt
// Kind: SendStmt
// go/ast: *ast.SendStmt
package kinds

import (
	"encoding/json"
	"go/ast"
)

type SendStmt struct {
	KindField string          `json:"kind"`
	Chan      json.RawMessage `json:"chan"`
	Value     json.RawMessage `json:"value"`
}

func (n *SendStmt) Kind() string { return "SendStmt" }

func (n *SendStmt) ToAST() (ast.Node, error) {
	ch, err := unmarshalExpr(n.Chan, "SendStmt.Chan")
	if err != nil {
		return nil, err
	}
	value, err := unmarshalExpr(n.Value, "SendStmt.Value")
	if err != nil {
		return nil, err
	}
	return &ast.SendStmt{Chan: ch, Value: value}, nil
}

func (n *SendStmt) FromAST(node ast.Node) error {
	s := node.(*ast.SendStmt)
	n.KindField = "SendStmt"
	var err error
	n.Chan, err = MarshalExpr(s.Chan)
	if err != nil {
		return err
	}
	n.Value, err = MarshalExpr(s.Value)
	return err
}

func init() { register("SendStmt", func() Node { return &SendStmt{} }) }
