// Namespace: goast/kinds/expr
// Kind: IndexListExpr
// go/ast: *ast.IndexListExpr (generics, Go 1.18+)
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type IndexListExpr struct {
	KindField string            `json:"kind"`
	X         json.RawMessage   `json:"x"`
	Indices   []json.RawMessage `json:"indices"`
}

func (n *IndexListExpr) Kind() string { return "IndexListExpr" }

func (n *IndexListExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "IndexListExpr.X")
	if err != nil {
		return nil, err
	}
	indices, err := unmarshalExprList(n.Indices)
	if err != nil {
		return nil, fmt.Errorf("IndexListExpr.Indices: %w", err)
	}
	return &ast.IndexListExpr{X: x, Indices: indices}, nil
}

func (n *IndexListExpr) FromAST(node ast.Node) error {
	e := node.(*ast.IndexListExpr)
	n.KindField = "IndexListExpr"
	var err error
	n.X, err = MarshalExpr(e.X)
	if err != nil {
		return err
	}
	n.Indices = make([]json.RawMessage, 0, len(e.Indices))
	for _, idx := range e.Indices {
		m, err := MarshalExpr(idx)
		if err != nil {
			return err
		}
		n.Indices = append(n.Indices, m)
	}
	return nil
}

func init() { register("IndexListExpr", func() Node { return &IndexListExpr{} }) }
