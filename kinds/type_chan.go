// Namespace: goast/kinds/type
// Kind: ChanType
// go/ast: *ast.ChanType
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type ChanType struct {
	KindField string          `json:"kind"`
	Dir       string          `json:"dir"` // "SEND"|"RECV"|"BOTH"
	Value     json.RawMessage `json:"value"`
}

func (n *ChanType) Kind() string { return "ChanType" }

func (n *ChanType) ToAST() (ast.Node, error) {
	value, err := unmarshalExpr(n.Value, "ChanType.Value")
	if err != nil {
		return nil, err
	}
	var dir ast.ChanDir
	switch n.Dir {
	case "SEND":
		dir = ast.SEND
	case "RECV":
		dir = ast.RECV
	case "BOTH":
		dir = ast.SEND | ast.RECV
	default:
		return nil, fmt.Errorf("ChanType.Dir: unknown direction %q", n.Dir)
	}
	return &ast.ChanType{Dir: dir, Value: value}, nil
}

func (n *ChanType) FromAST(node ast.Node) error {
	c := node.(*ast.ChanType)
	n.KindField = "ChanType"
	switch c.Dir {
	case ast.SEND:
		n.Dir = "SEND"
	case ast.RECV:
		n.Dir = "RECV"
	case ast.SEND | ast.RECV:
		n.Dir = "BOTH"
	default:
		n.Dir = "BOTH"
	}
	var err error
	n.Value, err = MarshalExpr(c.Value)
	return err
}

func init() { register("ChanType", func() Node { return &ChanType{} }) }
