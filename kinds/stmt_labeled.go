// Namespace: goast/kinds/stmt
// Kind: LabeledStmt
// go/ast: *ast.LabeledStmt
package kinds

import (
	"encoding/json"
	"go/ast"
)

type LabeledStmt struct {
	KindField string          `json:"kind"`
	Label     string          `json:"label"`
	Stmt      json.RawMessage `json:"stmt"`
}

func (n *LabeledStmt) Kind() string { return "LabeledStmt" }

func (n *LabeledStmt) ToAST() (ast.Node, error) {
	stmt, err := unmarshalStmt(n.Stmt, "LabeledStmt.Stmt")
	if err != nil {
		return nil, err
	}
	return &ast.LabeledStmt{Label: &ast.Ident{Name: n.Label}, Stmt: stmt}, nil
}

func (n *LabeledStmt) FromAST(node ast.Node) error {
	s := node.(*ast.LabeledStmt)
	n.KindField = "LabeledStmt"
	n.Label = s.Label.Name
	var err error
	n.Stmt, err = MarshalStmt(s.Stmt)
	return err
}

func init() { register("LabeledStmt", func() Node { return &LabeledStmt{} }) }
