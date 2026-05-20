// Namespace: goast/kinds/stmt
// Kind: AssignStmt
// go/ast: *ast.AssignStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
)

type AssignStmt struct {
	KindField string            `json:"kind"`
	Lhs       []json.RawMessage `json:"lhs"`
	Tok       string            `json:"tok"`
	Rhs       []json.RawMessage `json:"rhs"`
}

func (n *AssignStmt) Kind() string { return "AssignStmt" }

func (n *AssignStmt) ToAST() (ast.Node, error) {
	lhs, err := unmarshalExprList(n.Lhs)
	if err != nil {
		return nil, err
	}
	rhs, err := unmarshalExprList(n.Rhs)
	if err != nil {
		return nil, err
	}
	tok := tokenFromString(n.Tok)
	if tok == token.ILLEGAL {
		return nil, fmt.Errorf("AssignStmt: unknown tok %q", n.Tok)
	}
	return &ast.AssignStmt{Lhs: lhs, Tok: tok, Rhs: rhs}, nil
}

func (n *AssignStmt) FromAST(node ast.Node) error {
	s := node.(*ast.AssignStmt)
	n.KindField = "AssignStmt"
	n.Lhs = make([]json.RawMessage, 0, len(s.Lhs))
	for _, expr := range s.Lhs {
		m, err := MarshalExpr(expr)
		if err != nil {
			return err
		}
		n.Lhs = append(n.Lhs, m)
	}
	n.Tok = s.Tok.String()
	n.Rhs = make([]json.RawMessage, 0, len(s.Rhs))
	for _, expr := range s.Rhs {
		m, err := MarshalExpr(expr)
		if err != nil {
			return err
		}
		n.Rhs = append(n.Rhs, m)
	}
	return nil
}

func init() { register("AssignStmt", func() Node { return &AssignStmt{} }) }
