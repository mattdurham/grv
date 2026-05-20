// Namespace: goast/kinds/decl
// Kind: ConstDecl
// go/ast: *ast.GenDecl (Tok=CONST)
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type ConstDecl struct {
	KindField string            `json:"kind"`
	Specs     []json.RawMessage `json:"specs"`
}

func (n *ConstDecl) Kind() string { return "ConstDecl" }

func (n *ConstDecl) ToAST() (ast.Node, error) {
	specs, err := unmarshalSpecList(n.Specs)
	if err != nil {
		return nil, err
	}
	result := &ast.GenDecl{Tok: token.CONST, Specs: specs}
	if len(specs) > 1 {
		result.Lparen = token.Pos(1)
	}
	return result, nil
}

func (n *ConstDecl) FromAST(node ast.Node) error {
	d := node.(*ast.GenDecl)
	n.KindField = "ConstDecl"
	n.Specs = make([]json.RawMessage, 0, len(d.Specs))
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		s := &ValueSpec{}
		if err := s.FromAST(vs); err != nil {
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

func init() { register("ConstDecl", func() Node { return &ConstDecl{} }) }
