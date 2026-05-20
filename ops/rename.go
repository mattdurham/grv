// Namespace: goast/ops
// Write tool: ast_rename — single-file identifier rename (AST approximation)
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/selector"
	"golang.org/x/tools/go/ast/astutil"
)

// ASTRenameArgs is the argument struct for ast_rename.
type ASTRenameArgs struct {
	File   string          `json:"file"`
	Path   json.RawMessage `json:"path"`
	To     string          `json:"to"`
	DryRun bool            `json:"dry_run"`
}

// HandleASTRename implements the ast_rename tool.
func HandleASTRename(args ASTRenameArgs) (json.RawMessage, error) {
	if isReadonly(args.File) {
		return errResult(fmt.Sprintf("file is readonly: %s", args.File))
	}
	if args.To == "" {
		return errResult("to: new name cannot be empty")
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return errResult(fmt.Sprintf("parse path: %v", err))
	}

	result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, fset *token.FileSet) error {
		node, _, navErr := selector.Navigate(f, steps)
		if navErr != nil {
			return navErr
		}
		oldName, extractErr := extractDeclName(node)
		if extractErr != nil {
			return extractErr
		}
		astutil.Apply(f, func(c *astutil.Cursor) bool {
			if ident, ok := c.Node().(*ast.Ident); ok && ident.Name == oldName {
				c.Replace(&ast.Ident{Name: args.To})
			}
			return true
		}, nil)
		return nil
	})
	if err != nil {
		if ne, ok := err.(*selector.NavigateError); ok {
			return navErrResult(ne)
		}
		return errResult(err.Error())
	}

	resp := map[string]interface{}{"changed": result.Changed, "diff": result.Diff}
	return okResult(resp)
}

func extractDeclName(node ast.Node) (string, error) {
	switch n := node.(type) {
	case *ast.FuncDecl:
		return n.Name.Name, nil
	case *ast.TypeSpec:
		return n.Name.Name, nil
	case *ast.ValueSpec:
		if len(n.Names) == 0 {
			return "", fmt.Errorf("ValueSpec has no names")
		}
		return n.Names[0].Name, nil
	case *ast.Field:
		if len(n.Names) == 0 {
			return "", fmt.Errorf("field has no names (embedded field)")
		}
		return n.Names[0].Name, nil
	case *ast.Ident:
		return n.Name, nil
	default:
		return "", fmt.Errorf("unsupported node type %T for rename", node)
	}
}
