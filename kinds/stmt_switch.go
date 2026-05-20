// Namespace: goast/kinds/stmt
// Kind: SwitchStmt
// go/ast: *ast.SwitchStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type SwitchStmt struct {
	KindField string          `json:"kind"`
	Init      json.RawMessage `json:"init,omitempty"`
	Tag       json.RawMessage `json:"tag,omitempty"`
	Body      json.RawMessage `json:"body"`
}

func (n *SwitchStmt) Kind() string { return "SwitchStmt" }

func (n *SwitchStmt) ToAST() (ast.Node, error) {
	var init ast.Stmt
	var tag ast.Expr
	var err error
	if len(n.Init) > 0 && string(n.Init) != "null" {
		init, err = unmarshalStmt(n.Init, "SwitchStmt.Init")
		if err != nil {
			return nil, err
		}
	}
	if len(n.Tag) > 0 && string(n.Tag) != "null" {
		tag, err = unmarshalExpr(n.Tag, "SwitchStmt.Tag")
		if err != nil {
			return nil, err
		}
	}
	bodyNode, err := UnmarshalNode(n.Body)
	if err != nil {
		return nil, fmt.Errorf("SwitchStmt.Body: %w", err)
	}
	bodyAST, err := bodyNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("SwitchStmt.Body: %w", err)
	}
	body, ok := bodyAST.(*ast.BlockStmt)
	if !ok {
		return nil, fmt.Errorf("SwitchStmt.Body: expected *ast.BlockStmt, got %T", bodyAST)
	}
	return &ast.SwitchStmt{Init: init, Tag: tag, Body: body}, nil
}

func (n *SwitchStmt) FromAST(node ast.Node) error {
	s := node.(*ast.SwitchStmt)
	n.KindField = "SwitchStmt"
	var err error
	if s.Init != nil {
		n.Init, err = MarshalStmt(s.Init)
		if err != nil {
			return err
		}
	}
	if s.Tag != nil {
		n.Tag, err = MarshalExpr(s.Tag)
		if err != nil {
			return err
		}
	}
	n.Body, err = MarshalNode(s.Body)
	return err
}

func init() { register("SwitchStmt", func() Node { return &SwitchStmt{} }) }
