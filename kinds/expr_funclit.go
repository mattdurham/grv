// Namespace: goast/kinds/expr
// Kind: FuncLit
// go/ast: *ast.FuncLit
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type FuncLit struct {
	KindField string          `json:"kind"`
	Type      json.RawMessage `json:"type"`
	Body      json.RawMessage `json:"body"`
}

func (n *FuncLit) Kind() string { return "FuncLit" }

func (n *FuncLit) ToAST() (ast.Node, error) {
	typeNode, err := UnmarshalNode(n.Type)
	if err != nil {
		return nil, fmt.Errorf("FuncLit.Type: %w", err)
	}
	typeAST, err := typeNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("FuncLit.Type: %w", err)
	}
	funcType, ok := typeAST.(*ast.FuncType)
	if !ok {
		return nil, fmt.Errorf("FuncLit.Type: expected *ast.FuncType, got %T", typeAST)
	}

	bodyNode, err := UnmarshalNode(n.Body)
	if err != nil {
		return nil, fmt.Errorf("FuncLit.Body: %w", err)
	}
	bodyAST, err := bodyNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("FuncLit.Body: %w", err)
	}
	body, ok := bodyAST.(*ast.BlockStmt)
	if !ok {
		return nil, fmt.Errorf("FuncLit.Body: expected *ast.BlockStmt, got %T", bodyAST)
	}

	return &ast.FuncLit{Type: funcType, Body: body}, nil
}

func (n *FuncLit) FromAST(node ast.Node) error {
	f := node.(*ast.FuncLit)
	n.KindField = "FuncLit"
	var err error
	n.Type, err = MarshalNode(f.Type)
	if err != nil {
		return err
	}
	n.Body, err = MarshalNode(f.Body)
	return err
}

func init() { register("FuncLit", func() Node { return &FuncLit{} }) }
