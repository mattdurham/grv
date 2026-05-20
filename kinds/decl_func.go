// Namespace: goast/kinds/decl
// Kind: FuncDecl
// go/ast: *ast.FuncDecl
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type FuncDecl struct {
	KindField string          `json:"kind"`
	Recv      json.RawMessage `json:"recv,omitempty"`
	Name      string          `json:"name"`
	Type      json.RawMessage `json:"type"`
	Body      json.RawMessage `json:"body,omitempty"`
}

func (n *FuncDecl) Kind() string { return "FuncDecl" }

func (n *FuncDecl) ToAST() (ast.Node, error) {
	result := &ast.FuncDecl{
		Name: &ast.Ident{Name: n.Name},
	}

	if len(n.Recv) > 0 && string(n.Recv) != "null" {
		recvNode, err := UnmarshalNode(n.Recv)
		if err != nil {
			return nil, fmt.Errorf("FuncDecl.Recv: %w", err)
		}
		recvAST, err := recvNode.ToAST()
		if err != nil {
			return nil, fmt.Errorf("FuncDecl.Recv: %w", err)
		}
		recvField, ok := recvAST.(*ast.Field)
		if !ok {
			return nil, fmt.Errorf("FuncDecl.Recv: expected *ast.Field, got %T", recvAST)
		}
		result.Recv = &ast.FieldList{List: []*ast.Field{recvField}}
	}

	typeNode, err := UnmarshalNode(n.Type)
	if err != nil {
		return nil, fmt.Errorf("FuncDecl.Type: %w", err)
	}
	typeAST, err := typeNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("FuncDecl.Type: %w", err)
	}
	funcType, ok := typeAST.(*ast.FuncType)
	if !ok {
		return nil, fmt.Errorf("FuncDecl.Type: expected *ast.FuncType, got %T", typeAST)
	}
	result.Type = funcType

	if len(n.Body) > 0 && string(n.Body) != "null" {
		bodyNode, err := UnmarshalNode(n.Body)
		if err != nil {
			return nil, fmt.Errorf("FuncDecl.Body: %w", err)
		}
		bodyAST, err := bodyNode.ToAST()
		if err != nil {
			return nil, fmt.Errorf("FuncDecl.Body: %w", err)
		}
		body, ok := bodyAST.(*ast.BlockStmt)
		if !ok {
			return nil, fmt.Errorf("FuncDecl.Body: expected *ast.BlockStmt, got %T", bodyAST)
		}
		result.Body = body
	}

	return result, nil
}

func (n *FuncDecl) FromAST(node ast.Node) error {
	d := node.(*ast.FuncDecl)
	n.KindField = "FuncDecl"
	n.Name = d.Name.Name

	if d.Recv != nil && len(d.Recv.List) > 0 {
		f := &Field{}
		if err := f.FromAST(d.Recv.List[0]); err != nil {
			return err
		}
		var err error
		n.Recv, err = json.Marshal(f)
		if err != nil {
			return err
		}
	}

	var err error
	n.Type, err = MarshalNode(d.Type)
	if err != nil {
		return err
	}
	if d.Body != nil {
		n.Body, err = MarshalNode(d.Body)
		if err != nil {
			return err
		}
	}
	return nil
}

func init() { register("FuncDecl", func() Node { return &FuncDecl{} }) }
