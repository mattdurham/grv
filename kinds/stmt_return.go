// Namespace: goast/kinds/stmt
// Kind: ReturnStmt
// go/ast: *ast.ReturnStmt
package kinds

import (
	"encoding/json"
	"go/ast"
)

type ReturnStmt struct {
	KindField string            `json:"kind"`
	Results   []json.RawMessage `json:"results,omitempty"`
}

func (n *ReturnStmt) Kind() string { return "ReturnStmt" }

func (n *ReturnStmt) ToAST() (ast.Node, error) {
	results, err := unmarshalExprList(n.Results)
	if err != nil {
		return nil, err
	}
	return &ast.ReturnStmt{Results: results}, nil
}

func (n *ReturnStmt) FromAST(node ast.Node) error {
	s := node.(*ast.ReturnStmt)
	n.KindField = "ReturnStmt"
	n.Results = make([]json.RawMessage, 0, len(s.Results))
	for _, expr := range s.Results {
		m, err := MarshalExpr(expr)
		if err != nil {
			return err
		}
		n.Results = append(n.Results, m)
	}
	return nil
}

func init() { register("ReturnStmt", func() Node { return &ReturnStmt{} }) }
