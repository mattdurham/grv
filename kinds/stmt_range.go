// Namespace: goast/kinds/stmt
// Kind: RangeStmt
// go/ast: *ast.RangeStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type RangeStmt struct {
	KindField string          `json:"kind"`
	Key       json.RawMessage `json:"key,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
	Tok       string          `json:"tok"`
	X         json.RawMessage `json:"x"`
	Body      json.RawMessage `json:"body"`
}

func (n *RangeStmt) Kind() string { return "RangeStmt" }

func (n *RangeStmt) ToAST() (ast.Node, error) {
	var key, value ast.Expr
	var err error
	if len(n.Key) > 0 && string(n.Key) != "null" {
		key, err = unmarshalExpr(n.Key, "RangeStmt.Key")
		if err != nil {
			return nil, err
		}
	}
	if len(n.Value) > 0 && string(n.Value) != "null" {
		value, err = unmarshalExpr(n.Value, "RangeStmt.Value")
		if err != nil {
			return nil, err
		}
	}
	tok := tokenFromString(n.Tok)
	x, err := unmarshalExpr(n.X, "RangeStmt.X")
	if err != nil {
		return nil, err
	}
	bodyNode, err := UnmarshalNode(n.Body)
	if err != nil {
		return nil, fmt.Errorf("RangeStmt.Body: %w", err)
	}
	bodyAST, err := bodyNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("RangeStmt.Body: %w", err)
	}
	body, ok := bodyAST.(*ast.BlockStmt)
	if !ok {
		return nil, fmt.Errorf("RangeStmt.Body: expected *ast.BlockStmt, got %T", bodyAST)
	}
	return &ast.RangeStmt{Key: key, Value: value, Tok: tok, X: x, Body: body}, nil
}

func (n *RangeStmt) FromAST(node ast.Node) error {
	s := node.(*ast.RangeStmt)
	n.KindField = "RangeStmt"
	var err error
	if s.Key != nil {
		n.Key, err = MarshalExpr(s.Key)
		if err != nil {
			return err
		}
	}
	if s.Value != nil {
		n.Value, err = MarshalExpr(s.Value)
		if err != nil {
			return err
		}
	}
	n.Tok = s.Tok.String()
	n.X, err = MarshalExpr(s.X)
	if err != nil {
		return err
	}
	n.Body, err = MarshalNode(s.Body)
	return err
}

func init() { register("RangeStmt", func() Node { return &RangeStmt{} }) }
