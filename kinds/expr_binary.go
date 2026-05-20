// Namespace: goast/kinds/expr
// Kind: BinaryExpr
// go/ast: *ast.BinaryExpr
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type BinaryExpr struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
	Op        string          `json:"op"`
	Y         json.RawMessage `json:"y"`
}

func (n *BinaryExpr) Kind() string { return "BinaryExpr" }

func (n *BinaryExpr) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "BinaryExpr.X")
	if err != nil {
		return nil, err
	}
	y, err := unmarshalExpr(n.Y, "BinaryExpr.Y")
	if err != nil {
		return nil, err
	}
	op := tokenFromString(n.Op)
	if op == 0 {
		return nil, fmt.Errorf("BinaryExpr.Op: unknown operator %q", n.Op)
	}
	return &ast.BinaryExpr{X: x, Op: op, Y: y}, nil
}

func (n *BinaryExpr) FromAST(node ast.Node) error {
	b := node.(*ast.BinaryExpr)
	n.KindField = "BinaryExpr"
	var err error
	n.X, err = MarshalExpr(b.X)
	if err != nil {
		return err
	}
	n.Op = b.Op.String()
	n.Y, err = MarshalExpr(b.Y)
	return err
}

func init() { register("BinaryExpr", func() Node { return &BinaryExpr{} }) }
