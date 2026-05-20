// Namespace: goast/kinds/stmt
// Kind: TypeSwitchStmt
// go/ast: *ast.TypeSwitchStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type TypeSwitchStmt struct {
	KindField string          `json:"kind"`
	Init      json.RawMessage `json:"init,omitempty"`
	Assign    json.RawMessage `json:"assign"`
	Body      json.RawMessage `json:"body"`
}

func (n *TypeSwitchStmt) Kind() string { return "TypeSwitchStmt" }

func (n *TypeSwitchStmt) ToAST() (ast.Node, error) {
	var init ast.Stmt
	var err error
	if len(n.Init) > 0 && string(n.Init) != "null" {
		init, err = unmarshalStmt(n.Init, "TypeSwitchStmt.Init")
		if err != nil {
			return nil, err
		}
	}
	assign, err := unmarshalStmt(n.Assign, "TypeSwitchStmt.Assign")
	if err != nil {
		return nil, err
	}
	bodyNode, err := UnmarshalNode(n.Body)
	if err != nil {
		return nil, fmt.Errorf("TypeSwitchStmt.Body: %w", err)
	}
	bodyAST, err := bodyNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("TypeSwitchStmt.Body: %w", err)
	}
	body, ok := bodyAST.(*ast.BlockStmt)
	if !ok {
		return nil, fmt.Errorf("TypeSwitchStmt.Body: expected *ast.BlockStmt, got %T", bodyAST)
	}
	return &ast.TypeSwitchStmt{Init: init, Assign: assign, Body: body}, nil
}

func (n *TypeSwitchStmt) FromAST(node ast.Node) error {
	s := node.(*ast.TypeSwitchStmt)
	n.KindField = "TypeSwitchStmt"
	var err error
	if s.Init != nil {
		n.Init, err = MarshalStmt(s.Init)
		if err != nil {
			return err
		}
	}
	n.Assign, err = MarshalStmt(s.Assign)
	if err != nil {
		return err
	}
	n.Body, err = MarshalNode(s.Body)
	return err
}

func init() { register("TypeSwitchStmt", func() Node { return &TypeSwitchStmt{} }) }
