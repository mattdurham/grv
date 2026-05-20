// Namespace: goast/kinds/stmt
// Kind: BlockStmt
// go/ast: *ast.BlockStmt
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type BlockStmt struct {
	KindField string            `json:"kind"`
	List      []json.RawMessage `json:"list"`
}

func (n *BlockStmt) Kind() string { return "BlockStmt" }

func (n *BlockStmt) ToAST() (ast.Node, error) {
	list, err := unmarshalStmtList(n.List)
	if err != nil {
		return nil, err
	}
	return &ast.BlockStmt{Lbrace: token.NoPos, List: list, Rbrace: token.NoPos}, nil
}

func (n *BlockStmt) FromAST(node ast.Node) error {
	b := node.(*ast.BlockStmt)
	n.KindField = "BlockStmt"
	n.List = make([]json.RawMessage, 0, len(b.List))
	for _, stmt := range b.List {
		m, err := MarshalStmt(stmt)
		if err != nil {
			return err
		}
		n.List = append(n.List, m)
	}
	return nil
}

func init() { register("BlockStmt", func() Node { return &BlockStmt{} }) }
