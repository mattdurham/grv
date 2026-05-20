// Namespace: goast/kinds/expr
// Kind: IndexExpr
// go/ast: *ast.IndexExpr
package kinds

import (
	"encoding/json"
	"go/ast"
)

type IndexExpr struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
	Index     json.RawMessage `json:"index"`
}

func (n *IndexExpr) Kind() string { return "IndexExpr" }

func (n *IndexExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "IndexExpr.X")
	if err != nil {
		return nil, err
	}
	index, err := unmarshalExpr(n.Index, "IndexExpr.Index")
	if err != nil {
		return nil, err
	}
	return &ast.IndexExpr{X: x, Index: index}, nil
}

func (n *IndexExpr) FromAST(node ast.Node) error {
	e := node.(*ast.IndexExpr)
	n.KindField = "IndexExpr"
	var err error
	n.X, err = MarshalExpr(e.X)
	if err != nil {
		return err
	}
	n.Index, err = MarshalExpr(e.Index)
	return err
}

func init() { register("IndexExpr", func() Node { return &IndexExpr{} }) }
