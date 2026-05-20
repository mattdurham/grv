// Namespace: goast/kinds/stmt
// Kind: ForStmt
// go/ast: *ast.ForStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type ForStmt struct {
	KindField string          `json:"kind"`
	Init      json.RawMessage `json:"init,omitempty"`
	Cond      json.RawMessage `json:"cond,omitempty"`
	Post      json.RawMessage `json:"post,omitempty"`
	Body      json.RawMessage `json:"body"`
}

func (n *ForStmt) Kind() string { return "ForStmt" }

func (n *ForStmt) ToAST() (ast.Node, error) {
	var init, post ast.Stmt
	var cond ast.Expr
	var err error
	if len(n.Init) > 0 && string(n.Init) != "null" {
		init, err = unmarshalStmt(n.Init, "ForStmt.Init")
		if err != nil {
			return nil, err
		}
	}
	if len(n.Cond) > 0 && string(n.Cond) != "null" {
		cond, err = unmarshalExpr(n.Cond, "ForStmt.Cond")
		if err != nil {
			return nil, err
		}
	}
	if len(n.Post) > 0 && string(n.Post) != "null" {
		post, err = unmarshalStmt(n.Post, "ForStmt.Post")
		if err != nil {
			return nil, err
		}
	}
	bodyNode, err := UnmarshalNode(n.Body)
	if err != nil {
		return nil, fmt.Errorf("ForStmt.Body: %w", err)
	}
	bodyAST, err := bodyNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("ForStmt.Body: %w", err)
	}
	body, ok := bodyAST.(*ast.BlockStmt)
	if !ok {
		return nil, fmt.Errorf("ForStmt.Body: expected *ast.BlockStmt, got %T", bodyAST)
	}
	return &ast.ForStmt{Init: init, Cond: cond, Post: post, Body: body}, nil
}

func (n *ForStmt) FromAST(node ast.Node) error {
	s := node.(*ast.ForStmt)
	n.KindField = "ForStmt"
	var err error
	if s.Init != nil {
		n.Init, err = MarshalStmt(s.Init)
		if err != nil {
			return err
		}
	}
	if s.Cond != nil {
		n.Cond, err = MarshalExpr(s.Cond)
		if err != nil {
			return err
		}
	}
	if s.Post != nil {
		n.Post, err = MarshalStmt(s.Post)
		if err != nil {
			return err
		}
	}
	n.Body, err = MarshalNode(s.Body)
	return err
}

func init() { register("ForStmt", func() Node { return &ForStmt{} }) }
