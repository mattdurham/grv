// Namespace: goast/ops
// Write tool: ast_insert
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/kinds"
	"github.com/mattdurham/grv/selector"
)

// ASTInsertArgs is the argument struct for ast_insert.
//
// Target selection (pick one):
//   - File: explicit path to the target .go file
//   - Dir:  auto-route to the canonical file within this package directory
//   - (neither): daemon injects its working directory as Dir automatically
//
// When neither File nor Dir is specified, the daemon fills in Dir from its
// own working directory so callers never need to specify a path.
type ASTInsertArgs struct {
	File   string          `json:"file,omitempty"` // explicit target file
	Dir    string          `json:"dir,omitempty"`  // auto-route within this package dir
	Path   json.RawMessage `json:"path"`
	Index  int             `json:"index"`
	Node   json.RawMessage `json:"node"`
	DryRun bool            `json:"dry_run"`
}

// HandleASTInsert implements the ast_insert tool.
func HandleASTInsert(args ASTInsertArgs) (json.RawMessage, error) {
	// Auto-route: Dir given (or daemon-injected) but no explicit File.
	// handleASTPlaceWithFile runs post-write enforcement internally.
	if args.File == "" && args.Dir != "" {
		placeResult, _, err := handleASTPlaceWithFile(ASTPlaceArgs{
			Dir:    args.Dir,
			Node:   args.Node,
			DryRun: args.DryRun,
		}, DefaultChecksConfig.Enforce)
		if err != nil {
			return errResult(err.Error())
		}
		return placeResult, nil
	}
	if args.File == "" {
		return errResult("file or dir is required")
	}
	if isReadonly(args.File) {
		return errResult(fmt.Sprintf("file is readonly: %s", args.File))
	}
	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return errResult(fmt.Sprintf("parse path: %v", err))
	}

	kindNode, err := kinds.UnmarshalNode(args.Node)
	if err != nil {
		return errResult(fmt.Sprintf("parse node: %v", err))
	}
	if kindNode == nil {
		return errResult("node is required")
	}

	original, _ := os.ReadFile(args.File)
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
		if err := insertIntoNode(target, newNode, args.Index); err != nil {
			if err2 := insertIntoList(parentCtx, newNode, args.Index); err2 != nil {
				return err2
			}
		}
		return nil
	})
	if err == nil && !args.DryRun && result.Changed {
		if err2 := enforcePostWrite(args.File, original, DefaultChecksConfig.Enforce); err2 != nil {
			err = err2
		}
	}
	if err != nil {
		if ne, ok := err.(*selector.NavigateError); ok {
			return navErrResult(ne)
		}
		return errResult(err.Error())
	}

	resp := map[string]interface{}{
		"changed": result.Changed,
		"diff":    result.Diff,
	}
	return okResult(resp)
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
