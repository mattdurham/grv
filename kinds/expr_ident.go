// Namespace: goast/kinds/expr
// Kind: Ident
// go/ast: *ast.Ident
package kinds

import "go/ast"

type Ident struct {
	KindField string `json:"kind"`
	Name      string `json:"name"`
}

func (n *Ident) Kind() string { return "Ident" }

func (n *Ident) ToAST() (ast.Node, error) {
	return &ast.Ident{Name: n.Name}, nil
}

func (n *Ident) FromAST(node ast.Node) error {
	n.KindField = "Ident"
	n.Name = node.(*ast.Ident).Name
	return nil
}

func init() { register("Ident", func() Node { return &Ident{} }) }
