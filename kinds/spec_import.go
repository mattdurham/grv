// Namespace: goast/kinds/spec
// Kind: ImportSpec
// go/ast: *ast.ImportSpec
package kinds

import (
	"go/ast"
	"go/token"
	"strconv"
)

type ImportSpec struct {
	KindField string  `json:"kind"`
	Name      *string `json:"name,omitempty"`
	Path      string  `json:"path"`
}

func (n *ImportSpec) Kind() string { return "ImportSpec" }

func (n *ImportSpec) ToAST() (ast.Node, error) {
	result := &ast.ImportSpec{
		Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(n.Path)},
	}
	if n.Name != nil {
		result.Name = &ast.Ident{Name: *n.Name}
	}
	return result, nil
}

func (n *ImportSpec) FromAST(node ast.Node) error {
	s := node.(*ast.ImportSpec)
	n.KindField = "ImportSpec"
	path, err := strconv.Unquote(s.Path.Value)
	if err != nil {
		// fallback: strip quotes manually
		path = s.Path.Value
		if len(path) >= 2 && path[0] == '"' {
			path = path[1 : len(path)-1]
		}
	}
	n.Path = path
	if s.Name != nil {
		name := s.Name.Name
		n.Name = &name
	}
	return nil
}

func init() { register("ImportSpec", func() Node { return &ImportSpec{} }) }
