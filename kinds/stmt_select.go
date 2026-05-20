// Namespace: goast/kinds/stmt
// Kind: SelectStmt
// go/ast: *ast.SelectStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type SelectStmt struct {
	KindField string          `json:"kind"`
	Body      json.RawMessage `json:"body"`
}

func (n *SelectStmt) Kind() string { return "SelectStmt" }

func (n *SelectStmt) ToAST() (ast.Node, error) {
	bodyNode, err := UnmarshalNode(n.Body)
	if err != nil {
		return nil, fmt.Errorf("SelectStmt.Body: %w", err)
	}
	bodyAST, err := bodyNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("SelectStmt.Body: %w", err)
	}
	body, ok := bodyAST.(*ast.BlockStmt)
	if !ok {
		return nil, fmt.Errorf("SelectStmt.Body: expected *ast.BlockStmt, got %T", bodyAST)
	}
	return &ast.SelectStmt{Body: body}, nil
}

func (n *SelectStmt) FromAST(node ast.Node) error {
	s := node.(*ast.SelectStmt)
	n.KindField = "SelectStmt"
	var err error
	n.Body, err = MarshalNode(s.Body)
	return err
}

func init() { register("SelectStmt", func() Node { return &SelectStmt{} }) }
