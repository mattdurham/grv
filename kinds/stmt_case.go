// Namespace: goast/kinds/stmt
// Kind: CaseClause
// go/ast: *ast.CaseClause
// NOTE: list=null means default case (not empty case)
package kinds

import (
	"encoding/json"
	"go/ast"
)

type CaseClause struct {
	KindField string            `json:"kind"`
	List      []json.RawMessage `json:"list"` // null = default case
	Body      []json.RawMessage `json:"body"`
}

func (n *CaseClause) Kind() string { return "CaseClause" }

func (n *CaseClause) ToAST() (ast.Node, error) {
	result := &ast.CaseClause{}
	// nil List = default case; non-nil (even empty) = case clause
	if n.List != nil {
		list, err := unmarshalExprList(n.List)
		if err != nil {
			return nil, err
		}
		result.List = list
	}
	body, err := unmarshalStmtList(n.Body)
	if err != nil {
		return nil, err
	}
	result.Body = body
	return result, nil
}

func (n *CaseClause) FromAST(node ast.Node) error {
	c := node.(*ast.CaseClause)
	n.KindField = "CaseClause"
	// Preserve nil distinction: nil=default, non-nil=case
	if c.List != nil {
		n.List = make([]json.RawMessage, 0, len(c.List))
		for _, expr := range c.List {
			m, err := MarshalExpr(expr)
			if err != nil {
				return err
			}
			n.List = append(n.List, m)
		}
	}
	n.Body = make([]json.RawMessage, 0, len(c.Body))
	for _, stmt := range c.Body {
		m, err := MarshalStmt(stmt)
		if err != nil {
			return err
		}
		n.Body = append(n.Body, m)
	}
	return nil
}

func init() { register("CaseClause", func() Node { return &CaseClause{} }) }
