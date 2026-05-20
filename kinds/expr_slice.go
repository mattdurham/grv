// Namespace: goast/kinds/expr
// Kind: SliceExpr
// go/ast: *ast.SliceExpr
package kinds

import (
	"encoding/json"
	"go/ast"
)

type SliceExpr struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
	Low       json.RawMessage `json:"low,omitempty"`
	High      json.RawMessage `json:"high,omitempty"`
	Max       json.RawMessage `json:"max,omitempty"`
}

func (n *SliceExpr) Kind() string { return "SliceExpr" }

func (n *SliceExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "SliceExpr.X")
	if err != nil {
		return nil, err
	}
	low, err := unmarshalExpr(n.Low, "SliceExpr.Low")
	if err != nil {
		return nil, err
	}
	high, err := unmarshalExpr(n.High, "SliceExpr.High")
	if err != nil {
		return nil, err
	}
	max, err := unmarshalExpr(n.Max, "SliceExpr.Max")
	if err != nil {
		return nil, err
	}
	slice3 := max != nil
	return &ast.SliceExpr{X: x, Low: low, High: high, Max: max, Slice3: slice3}, nil
}

func (n *SliceExpr) FromAST(node ast.Node) error {
	s := node.(*ast.SliceExpr)
	n.KindField = "SliceExpr"
	var err error
	n.X, err = MarshalExpr(s.X)
	if err != nil {
		return err
	}
	if s.Low != nil {
		n.Low, err = MarshalExpr(s.Low)
		if err != nil {
			return err
		}
	}
	if s.High != nil {
		n.High, err = MarshalExpr(s.High)
		if err != nil {
			return err
		}
	}
	if s.Max != nil {
		n.Max, err = MarshalExpr(s.Max)
		if err != nil {
			return err
		}
	}
	return nil
}

func init() { register("SliceExpr", func() Node { return &SliceExpr{} }) }
