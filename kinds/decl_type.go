// Namespace: goast/kinds/decl
// Kind: TypeDecl
// go/ast: *ast.GenDecl (Tok=TYPE)
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type TypeDecl struct {
	KindField string            `json:"kind"`
	Specs     []json.RawMessage `json:"specs"`
}

func (n *TypeDecl) Kind() string { return "TypeDecl" }

func (n *TypeDecl) ToAST() (ast.Node, error) {
	specs, err := unmarshalSpecList(n.Specs)
	if err != nil {
		return nil, err
	}
	result := &ast.GenDecl{Tok: token.TYPE, Specs: specs}
	if len(specs) > 1 {
		result.Lparen = token.Pos(1)
	}
	return result, nil
}

func (n *TypeDecl) FromAST(node ast.Node) error {
	d := node.(*ast.GenDecl)
	n.KindField = "TypeDecl"
	n.Specs = make([]json.RawMessage, 0, len(d.Specs))
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		s := &TypeSpec{}
		if err := s.FromAST(ts); err != nil {
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

func init() { register("TypeDecl", func() Node { return &TypeDecl{} }) }
