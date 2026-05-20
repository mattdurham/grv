// Namespace: goast/kinds/stmt
// Kind: IfStmt
// go/ast: *ast.IfStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type IfStmt struct {
	KindField string          `json:"kind"`
	Init      json.RawMessage `json:"init,omitempty"`
	Cond      json.RawMessage `json:"cond"`
	Body      json.RawMessage `json:"body"`
	Else      json.RawMessage `json:"else,omitempty"`
}

func (n *IfStmt) Kind() string { return "IfStmt" }

func (n *IfStmt) ToAST() (ast.Node, error) {
	var init ast.Stmt
	if len(n.Init) > 0 && string(n.Init) != "null" {
		var err error
		init, err = unmarshalStmt(n.Init, "IfStmt.Init")
		if err != nil {
			return nil, err
		}
	}
	cond, err := unmarshalExpr(n.Cond, "IfStmt.Cond")
	if err != nil {
		return nil, err
	}
	bodyNode, err := UnmarshalNode(n.Body)
	if err != nil {
		return nil, fmt.Errorf("IfStmt.Body: %w", err)
	}
	bodyAST, err := bodyNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("IfStmt.Body: %w", err)
	}
	body, ok := bodyAST.(*ast.BlockStmt)
	if !ok {
		return nil, fmt.Errorf("IfStmt.Body: expected *ast.BlockStmt, got %T", bodyAST)
	}
	var elseStmt ast.Stmt
	if len(n.Else) > 0 && string(n.Else) != "null" {
		elseStmt, err = unmarshalStmt(n.Else, "IfStmt.Else")
		if err != nil {
			return nil, err
		}
	}
	return &ast.IfStmt{Init: init, Cond: cond, Body: body, Else: elseStmt}, nil
}

func (n *IfStmt) FromAST(node ast.Node) error {
	s := node.(*ast.IfStmt)
	n.KindField = "IfStmt"
	var err error
	if s.Init != nil {
		n.Init, err = MarshalStmt(s.Init)
		if err != nil {
			return err
		}
	}
	n.Cond, err = MarshalExpr(s.Cond)
	if err != nil {
		return err
	}
	n.Body, err = MarshalNode(s.Body)
	if err != nil {
		return err
	}
	if s.Else != nil {
		n.Else, err = MarshalStmt(s.Else)
	}
	return err
}

func init() { register("IfStmt", func() Node { return &IfStmt{} }) }
