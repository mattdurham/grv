// Namespace: goast/kinds/stmt
// Kind: IncDecStmt
// go/ast: *ast.IncDecStmt
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
)

type IncDecStmt struct {
	KindField string          `json:"kind"`
	X         json.RawMessage `json:"x"`
	Tok       string          `json:"tok"` // "++"|"--"
}

func (n *IncDecStmt) Kind() string { return "IncDecStmt" }

func (n *IncDecStmt) ToAST() (ast.Node, error) {
	x, err := unmarshalExpr(n.X, "IncDecStmt.X")
	if err != nil {
		return nil, err
	}
	tok := tokenFromString(n.Tok)
	if tok == 0 {
		return nil, fmt.Errorf("IncDecStmt.Tok: unknown token %q", n.Tok)
	}
	return &ast.IncDecStmt{X: x, Tok: tok}, nil
}

func (n *IncDecStmt) FromAST(node ast.Node) error {
	s := node.(*ast.IncDecStmt)
	n.KindField = "IncDecStmt"
	var err error
	n.X, err = MarshalExpr(s.X)
	if err != nil {
		return err
	}
	n.Tok = s.Tok.String()
	return nil
}

func init() { register("IncDecStmt", func() Node { return &IncDecStmt{} }) }
