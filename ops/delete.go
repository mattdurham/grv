// Namespace: goast/ops
// Write tool: ast_delete
package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/lthiery/goast/editor"
	"github.com/lthiery/goast/selector"
	"github.com/mark3labs/mcp-go/mcp"
)

// ASTDeleteArgs is the argument struct for ast_delete.
type ASTDeleteArgs struct {
	File   string          `json:"file"`
	Path   json.RawMessage `json:"path"`
	DryRun bool            `json:"dry_run"`
}

// HandleASTDelete implements the ast_delete tool.
func HandleASTDelete(ctx context.Context, req mcp.CallToolRequest, args ASTDeleteArgs) (*mcp.CallToolResult, error) {
	if isReadonly(args.File) {
		return toolError(fmt.Sprintf("file is readonly: %s", args.File)), nil
	}
	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return toolError(fmt.Sprintf("parse path: %v", err)), nil
	}

	result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, _ *token.FileSet) error {
		_, parentCtx, navErr := selector.Navigate(f, steps)
		if navErr != nil {
			return navErr
		}
		return deleteFromList(parentCtx)
	})
	if err != nil {
		if ne, ok := err.(*selector.NavigateError); ok {
			return navError(ne), nil
		}
		return toolError(err.Error()), nil
	}

	resp := map[string]interface{}{
		"changed": result.Changed,
		"diff":    result.Diff,
	}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}

func deleteFromList(ctx selector.ParentContext) error {
	idx := ctx.Index
	if idx < 0 {
		return fmt.Errorf("cannot delete scalar field %q", ctx.FieldName)
	}

	switch parent := ctx.Parent.(type) {
	case *ast.BlockStmt:
		if idx >= len(parent.List) {
			return fmt.Errorf("index %d out of range for BlockStmt.List", idx)
		}
		parent.List = append(parent.List[:idx], parent.List[idx+1:]...)
	case *ast.FieldList:
		if idx >= len(parent.List) {
			return fmt.Errorf("index %d out of range for FieldList.List", idx)
		}
		parent.List = append(parent.List[:idx], parent.List[idx+1:]...)
	case *ast.File:
		if idx >= len(parent.Decls) {
			return fmt.Errorf("index %d out of range for File.Decls", idx)
		}
		parent.Decls = append(parent.Decls[:idx], parent.Decls[idx+1:]...)
	case *ast.CallExpr:
		if idx >= len(parent.Args) {
			return fmt.Errorf("index %d out of range for CallExpr.Args", idx)
		}
		parent.Args = append(parent.Args[:idx], parent.Args[idx+1:]...)
	case *ast.CompositeLit:
		if idx >= len(parent.Elts) {
			return fmt.Errorf("index %d out of range for CompositeLit.Elts", idx)
		}
		parent.Elts = append(parent.Elts[:idx], parent.Elts[idx+1:]...)
	case *ast.CaseClause:
		if ctx.FieldName == "List" {
			if idx >= len(parent.List) {
				return fmt.Errorf("index %d out of range for CaseClause.List", idx)
			}
			parent.List = append(parent.List[:idx], parent.List[idx+1:]...)
		} else {
			if idx >= len(parent.Body) {
				return fmt.Errorf("index %d out of range for CaseClause.Body", idx)
			}
			parent.Body = append(parent.Body[:idx], parent.Body[idx+1:]...)
		}
	case *ast.CommClause:
		if idx >= len(parent.Body) {
			return fmt.Errorf("index %d out of range for CommClause.Body", idx)
		}
		parent.Body = append(parent.Body[:idx], parent.Body[idx+1:]...)
	case *ast.GenDecl:
		if idx >= len(parent.Specs) {
			return fmt.Errorf("index %d out of range for GenDecl.Specs", idx)
		}
		parent.Specs = append(parent.Specs[:idx], parent.Specs[idx+1:]...)
	default:
		return fmt.Errorf("deleteFromList: unsupported parent type %T", ctx.Parent)
	}
	return nil
}
