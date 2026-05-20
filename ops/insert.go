// Namespace: goast/ops
// Write tool: ast_insert
package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/lthiery/goast/editor"
	"github.com/lthiery/goast/kinds"
	"github.com/lthiery/goast/selector"
	"github.com/mark3labs/mcp-go/mcp"
)

// ASTInsertArgs is the argument struct for ast_insert.
type ASTInsertArgs struct {
	File   string          `json:"file"`
	Path   json.RawMessage `json:"path"`
	Index  int             `json:"index"`
	Node   json.RawMessage `json:"node"`
	DryRun bool            `json:"dry_run"`
}

// HandleASTInsert implements the ast_insert tool.
func HandleASTInsert(ctx context.Context, req mcp.CallToolRequest, args ASTInsertArgs) (*mcp.CallToolResult, error) {
	if isReadonly(args.File) {
		return toolError(fmt.Sprintf("file is readonly: %s", args.File)), nil
	}
	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return toolError(fmt.Sprintf("parse path: %v", err)), nil
	}

	kindNode, err := kinds.UnmarshalNode(args.Node)
	if err != nil {
		return toolError(fmt.Sprintf("parse node: %v", err)), nil
	}

	result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, _ *token.FileSet) error {
		target, parentCtx, navErr := selector.Navigate(f, steps)
		if navErr != nil {
			return navErr
		}
		newNode, toErr := kindNode.ToAST()
		if toErr != nil {
			return fmt.Errorf("ToAST: %w", toErr)
		}
		// Try inserting into the target node itself first (it may be a list container).
		// Fall back to inserting via the parent context.
		if err := insertIntoNode(target, newNode, args.Index); err == nil {
			return nil
		}
		return insertIntoList(parentCtx, newNode, args.Index)
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

// insertIntoNode tries to insert newNode into target as a direct list container.
func insertIntoNode(target ast.Node, newNode ast.Node, index int) error {
	switch t := target.(type) {
	case *ast.BlockStmt:
		stmt, ok := newNode.(ast.Stmt)
		if !ok {
			return fmt.Errorf("expected ast.Stmt for BlockStmt, got %T", newNode)
		}
		t.List = insertAtStmt(t.List, stmt, index)
		return nil
	case *ast.FieldList:
		field, ok := newNode.(*ast.Field)
		if !ok {
			return fmt.Errorf("expected *ast.Field for FieldList, got %T", newNode)
		}
		t.List = insertAtField(t.List, field, index)
		return nil
	case *ast.File:
		decl, ok := newNode.(ast.Decl)
		if !ok {
			return fmt.Errorf("expected ast.Decl for File, got %T", newNode)
		}
		t.Decls = insertAtDecl(t.Decls, decl, index)
		return nil
	case *ast.CallExpr:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr for CallExpr, got %T", newNode)
		}
		t.Args = insertAtExpr(t.Args, expr, index)
		return nil
	case *ast.CompositeLit:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr for CompositeLit, got %T", newNode)
		}
		t.Elts = insertAtExpr(t.Elts, expr, index)
		return nil
	}
	return fmt.Errorf("not a list container: %T", target)
}

func insertIntoList(ctx selector.ParentContext, newNode ast.Node, index int) error {
	switch parent := ctx.Parent.(type) {
	case *ast.BlockStmt:
		stmt, ok := newNode.(ast.Stmt)
		if !ok {
			return fmt.Errorf("expected ast.Stmt, got %T", newNode)
		}
		parent.List = insertAtStmt(parent.List, stmt, index)
	case *ast.FieldList:
		field, ok := newNode.(*ast.Field)
		if !ok {
			return fmt.Errorf("expected *ast.Field, got %T", newNode)
		}
		parent.List = insertAtField(parent.List, field, index)
	case *ast.File:
		decl, ok := newNode.(ast.Decl)
		if !ok {
			return fmt.Errorf("expected ast.Decl, got %T", newNode)
		}
		parent.Decls = insertAtDecl(parent.Decls, decl, index)
	case *ast.CallExpr:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr, got %T", newNode)
		}
		parent.Args = insertAtExpr(parent.Args, expr, index)
	case *ast.CompositeLit:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr, got %T", newNode)
		}
		parent.Elts = insertAtExpr(parent.Elts, expr, index)
	case *ast.CaseClause:
		if ctx.FieldName == "List" {
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr, got %T", newNode)
			}
			parent.List = insertAtExpr(parent.List, expr, index)
		} else {
			stmt, ok := newNode.(ast.Stmt)
			if !ok {
				return fmt.Errorf("expected ast.Stmt, got %T", newNode)
			}
			parent.Body = insertAtStmt(parent.Body, stmt, index)
		}
	case *ast.CommClause:
		stmt, ok := newNode.(ast.Stmt)
		if !ok {
			return fmt.Errorf("expected ast.Stmt, got %T", newNode)
		}
		parent.Body = insertAtStmt(parent.Body, stmt, index)
	default:
		return fmt.Errorf("cannot insert into %T (field %q)", ctx.Parent, ctx.FieldName)
	}
	return nil
}

func insertAtStmt(list []ast.Stmt, item ast.Stmt, index int) []ast.Stmt {
	if index < 0 || index >= len(list) {
		return append(list, item)
	}
	list = append(list, nil)
	copy(list[index+1:], list[index:])
	list[index] = item
	return list
}

func insertAtField(list []*ast.Field, item *ast.Field, index int) []*ast.Field {
	if index < 0 || index >= len(list) {
		return append(list, item)
	}
	list = append(list, nil)
	copy(list[index+1:], list[index:])
	list[index] = item
	return list
}

func insertAtDecl(list []ast.Decl, item ast.Decl, index int) []ast.Decl {
	if index < 0 || index >= len(list) {
		return append(list, item)
	}
	list = append(list, nil)
	copy(list[index+1:], list[index:])
	list[index] = item
	return list
}

func insertAtExpr(list []ast.Expr, item ast.Expr, index int) []ast.Expr {
	if index < 0 || index >= len(list) {
		return append(list, item)
	}
	list = append(list, nil)
	copy(list[index+1:], list[index:])
	list[index] = item
	return list
}
