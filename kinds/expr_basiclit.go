// Namespace: goast/kinds/expr
// Kind: BasicLit
// go/ast: *ast.BasicLit
package kinds

import (
	"fmt"
	"go/ast"
	"go/token"
)

type BasicLit struct {
	KindField string `json:"kind"`
	Tok       string `json:"tok"` // "INT", "FLOAT", "IMAG", "CHAR", "STRING"
	Value     string `json:"value"`
}

func (n *BasicLit) Kind() string { return "BasicLit" }

func (n *BasicLit) ToAST() (ast.Node, error) {
	tok, err := tokKindFromString(n.Tok)
	if err != nil {
		return nil, fmt.Errorf("BasicLit.Tok: %w", err)
	}
	return &ast.BasicLit{Kind: tok, Value: n.Value}, nil
}

func (n *BasicLit) FromAST(node ast.Node) error {
	b := node.(*ast.BasicLit)
	n.KindField = "BasicLit"
	n.Tok = b.Kind.String()
	n.Value = b.Value
	return nil
}

func tokKindFromString(s string) (token.Token, error) {
	switch s {
	case "INT":
		return token.INT, nil
	case "FLOAT":
		return token.FLOAT, nil
	case "IMAG":
		return token.IMAG, nil
	case "CHAR":
		return token.CHAR, nil
	case "STRING":
		return token.STRING, nil
	default:
		if s == "BOOL" {
			return token.ILLEGAL, fmt.Errorf("unknown token kind %q: true/false are Ident nodes, not BasicLit", s)
		}
		return token.ILLEGAL, fmt.Errorf("unknown token kind %q (valid: INT, FLOAT, IMAG, CHAR, STRING)", s)
	}
}

func init() { register("BasicLit", func() Node { return &BasicLit{} }) }
