// Namespace: goast/kinds/decl
// Kind: VarDecl
// go/ast: *ast.GenDecl (Tok=VAR)
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type VarDecl struct {
	KindField string            `json:"kind"`
	Specs     []json.RawMessage `json:"specs"`
}

func (n *VarDecl) Kind() string { return "VarDecl" }

func (n *VarDecl) ToAST() (ast.Node, error) {
	specs, err := unmarshalSpecList(n.Specs)
	if err != nil {
		return nil, err
	}
	result := &ast.GenDecl{Tok: token.VAR, Specs: specs}
	if len(specs) > 1 {
		result.Lparen = token.Pos(1)
	}
	return result, nil
}

func (n *VarDecl) FromAST(node ast.Node) error {
	d := node.(*ast.GenDecl)
	n.KindField = "VarDecl"
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

func init() { register("VarDecl", func() Node { return &VarDecl{} }) }
