// Namespace: goast/kinds/stmt
// Kind: CommClause
// go/ast: *ast.CommClause
package kinds

import (
	"encoding/json"
	"go/ast"
)

type CommClause struct {
	KindField string            `json:"kind"`
	Comm      json.RawMessage   `json:"comm,omitempty"`
	Body      []json.RawMessage `json:"body"`
}

func (n *CommClause) Kind() string { return "CommClause" }

func (n *CommClause) ToAST() (ast.Node, error) {
	var comm ast.Stmt
	if len(n.Comm) > 0 && string(n.Comm) != "null" {
		var err error
		comm, err = unmarshalStmt(n.Comm, "CommClause.Comm")
		if err != nil {
			return nil, err
		}
	}
	body, err := unmarshalStmtList(n.Body)
	if err != nil {
		return nil, err
	}
	return &ast.CommClause{Comm: comm, Body: body}, nil
}

func (n *CommClause) FromAST(node ast.Node) error {
	c := node.(*ast.CommClause)
	n.KindField = "CommClause"
	var err error
	if c.Comm != nil {
		n.Comm, err = MarshalStmt(c.Comm)
		if err != nil {
			return err
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

func init() { register("CommClause", func() Node { return &CommClause{} }) }
