// Namespace: goast/kinds/stmt
// Kind: BranchStmt
// go/ast: *ast.BranchStmt
package kinds

import "go/ast"

type BranchStmt struct {
	KindField string  `json:"kind"`
	Tok       string  `json:"tok"`             // "break"|"continue"|"goto"|"fallthrough"
	Label     *string `json:"label,omitempty"` // nil if no label
}

func (n *BranchStmt) Kind() string { return "BranchStmt" }

func (n *BranchStmt) ToAST() (ast.Node, error) {
	tok := tokenFromString(n.Tok)
	result := &ast.BranchStmt{Tok: tok}
	if n.Label != nil {
		result.Label = &ast.Ident{Name: *n.Label}
	}
	return result, nil
}

func (n *BranchStmt) FromAST(node ast.Node) error {
	s := node.(*ast.BranchStmt)
	n.KindField = "BranchStmt"
	n.Tok = s.Tok.String()
	if s.Label != nil {
		label := s.Label.Name
		n.Label = &label
	}
	return nil
}

func init() { register("BranchStmt", func() Node { return &BranchStmt{} }) }
