// Namespace: goast/kinds/expr
// Kind: CallExpr
// go/ast: *ast.CallExpr
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type CallExpr struct {
	KindField string            `json:"kind"`
	Fun       json.RawMessage   `json:"fun"`
	Args      []json.RawMessage `json:"args"`
	Ellipsis  bool              `json:"ellipsis"`
}

func (n *CallExpr) Kind() string { return "CallExpr" }

func (n *CallExpr) ToAST() (ast.Node, error) {
	fun, err := unmarshalExpr(n.Fun, "CallExpr.Fun")
	if err != nil {
		return nil, err
	}
	args, err := unmarshalExprList(n.Args)
	if err != nil {
		return nil, err
	}
	result := &ast.CallExpr{Fun: fun, Args: args}
	if n.Ellipsis {
		result.Ellipsis = token.Pos(1)
	}
	return result, nil
}

func (n *CallExpr) FromAST(node ast.Node) error {
	c := node.(*ast.CallExpr)
	n.KindField = "CallExpr"
	var err error
	n.Fun, err = MarshalExpr(c.Fun)
	if err != nil {
		return err
	}
	n.Args = make([]json.RawMessage, 0, len(c.Args))
	for _, arg := range c.Args {
		m, err := MarshalExpr(arg)
		if err != nil {
			return err
		}
		n.Args = append(n.Args, m)
	}
	n.Ellipsis = c.Ellipsis != token.NoPos
	return nil
}

func init() { register("CallExpr", func() Node { return &CallExpr{} }) }
