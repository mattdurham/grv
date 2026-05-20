// Namespace: goast/kinds/decl
// Kind: ImportDecl
// go/ast: *ast.GenDecl (Tok=IMPORT)
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type ImportDecl struct {
	KindField string            `json:"kind"`
	Specs     []json.RawMessage `json:"specs"`
}

func (n *ImportDecl) Kind() string { return "ImportDecl" }

func (n *ImportDecl) ToAST() (ast.Node, error) {
	specs, err := unmarshalSpecList(n.Specs)
	if err != nil {
		return nil, err
	}
	result := &ast.GenDecl{Tok: token.IMPORT, Specs: specs}
	if len(specs) > 1 {
		result.Lparen = token.Pos(1)
	}
	return result, nil
}

func (n *ImportDecl) FromAST(node ast.Node) error {
	d := node.(*ast.GenDecl)
	n.KindField = "ImportDecl"
	n.Specs = make([]json.RawMessage, 0, len(d.Specs))
	for _, spec := range d.Specs {
		is, ok := spec.(*ast.ImportSpec)
		if !ok {
			continue
		}
		s := &ImportSpec{}
		if err := s.FromAST(is); err != nil {
			return err
		}
		m, err := json.Marshal(s)
		if err != nil {
			return err
		}
		n.Specs = append(n.Specs, m)
	}
	return nil
}

func init() { register("ImportDecl", func() Node { return &ImportDecl{} }) }
