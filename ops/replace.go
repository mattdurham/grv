// Namespace: goast/ops
// Write tool: ast_replace
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/kinds"
	"github.com/mattdurham/grv/selector"
)

// ASTReplaceArgs is the argument struct for ast_replace.
type ASTReplaceArgs struct {
	File   string          `json:"file"`
	Path   json.RawMessage `json:"path"`
	Node   json.RawMessage `json:"node"`
	DryRun bool            `json:"dry_run"`
}

// HandleASTReplace implements the ast_replace tool.
func HandleASTReplace(args ASTReplaceArgs) (json.RawMessage, error) {
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

	result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, _ *token.FileSet) error {
		_, parentCtx, navErr := selector.Navigate(f, steps)
		if navErr != nil {
			return navErr
		}
		newNode, toErr := kindNode.ToAST()
		if toErr != nil {
			return fmt.Errorf("ToAST: %w", toErr)
		}
		return replaceInParent(parentCtx, newNode)
	})
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

func replaceInParent(ctx selector.ParentContext, newNode ast.Node) error {
	idx := ctx.Index

	switch parent := ctx.Parent.(type) {
	// List replacements
	case *ast.BlockStmt:
		if idx < 0 || idx >= len(parent.List) {
			return fmt.Errorf("index %d out of range for BlockStmt.List (len %d)", idx, len(parent.List))
		}
		stmt, ok := newNode.(ast.Stmt)
		if !ok {
			return fmt.Errorf("expected ast.Stmt, got %T", newNode)
		}
		parent.List[idx] = stmt
	case *ast.FieldList:
		if idx < 0 || idx >= len(parent.List) {
			return fmt.Errorf("index %d out of range for FieldList.List", idx)
		}
		field, ok := newNode.(*ast.Field)
		if !ok {
			return fmt.Errorf("expected *ast.Field, got %T", newNode)
		}
		parent.List[idx] = field
	case *ast.File:
		if idx < 0 || idx >= len(parent.Decls) {
			return fmt.Errorf("index %d out of range for File.Decls", idx)
		}
		decl, ok := newNode.(ast.Decl)
		if !ok {
			return fmt.Errorf("expected ast.Decl, got %T", newNode)
		}
		parent.Decls[idx] = decl
	case *ast.CallExpr:
		if idx >= 0 {
			if idx >= len(parent.Args) {
				return fmt.Errorf("index %d out of range for CallExpr.Args", idx)
			}
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr, got %T", newNode)
			}
			parent.Args[idx] = expr
		} else {
			// Replacing Fun (scalar)
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for CallExpr.Fun, got %T", newNode)
			}
			parent.Fun = expr
		}
	case *ast.CompositeLit:
		if idx >= 0 {
			if idx >= len(parent.Elts) {
				return fmt.Errorf("index %d out of range for CompositeLit.Elts", idx)
			}
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr, got %T", newNode)
			}
			parent.Elts[idx] = expr
		} else {
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for CompositeLit.Type, got %T", newNode)
			}
			parent.Type = expr
		}
	case *ast.CaseClause:
		if ctx.FieldName == "List" && idx >= 0 {
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr, got %T", newNode)
			}
			parent.List[idx] = expr
		} else if ctx.FieldName == "Body" && idx >= 0 {
			stmt, ok := newNode.(ast.Stmt)
			if !ok {
				return fmt.Errorf("expected ast.Stmt, got %T", newNode)
			}
			parent.Body[idx] = stmt
		}
	case *ast.CommClause:
		if idx >= 0 {
			stmt, ok := newNode.(ast.Stmt)
			if !ok {
				return fmt.Errorf("expected ast.Stmt, got %T", newNode)
			}
			parent.Body[idx] = stmt
		} else {
			stmt, ok := newNode.(ast.Stmt)
			if !ok {
				return fmt.Errorf("expected ast.Stmt for CommClause.Comm, got %T", newNode)
			}
			parent.Comm = stmt
		}
	// Scalar field replacements
	case *ast.IfStmt:
		expr, ok := newNode.(ast.Expr)
		switch ctx.FieldName {
		case "Cond":
			if !ok {
				return fmt.Errorf("expected ast.Expr for IfStmt.Cond, got %T", newNode)
			}
			parent.Cond = expr
		case "Init":
			stmt, ok2 := newNode.(ast.Stmt)
			if !ok2 {
				return fmt.Errorf("expected ast.Stmt for IfStmt.Init, got %T", newNode)
			}
			parent.Init = stmt
		case "Body":
			body, ok2 := newNode.(*ast.BlockStmt)
			if !ok2 {
				return fmt.Errorf("expected *ast.BlockStmt for IfStmt.Body, got %T", newNode)
			}
			parent.Body = body
		case "Else":
			stmt, ok2 := newNode.(ast.Stmt)
			if !ok2 {
				return fmt.Errorf("expected ast.Stmt for IfStmt.Else, got %T", newNode)
			}
			parent.Else = stmt
		}
	case *ast.ForStmt:
		switch ctx.FieldName {
		case "Cond":
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for ForStmt.Cond, got %T", newNode)
			}
			parent.Cond = expr
		case "Init":
			stmt, ok := newNode.(ast.Stmt)
			if !ok {
				return fmt.Errorf("expected ast.Stmt for ForStmt.Init, got %T", newNode)
			}
			parent.Init = stmt
		case "Post":
			stmt, ok := newNode.(ast.Stmt)
			if !ok {
				return fmt.Errorf("expected ast.Stmt for ForStmt.Post, got %T", newNode)
			}
			parent.Post = stmt
		case "Body":
			body, ok := newNode.(*ast.BlockStmt)
			if !ok {
				return fmt.Errorf("expected *ast.BlockStmt for ForStmt.Body, got %T", newNode)
			}
			parent.Body = body
		}
	case *ast.RangeStmt:
		switch ctx.FieldName {
		case "X":
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for RangeStmt.X, got %T", newNode)
			}
			parent.X = expr
		case "Key":
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for RangeStmt.Key, got %T", newNode)
			}
			parent.Key = expr
		case "Value":
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for RangeStmt.Value, got %T", newNode)
			}
			parent.Value = expr
		case "Body":
			body, ok := newNode.(*ast.BlockStmt)
			if !ok {
				return fmt.Errorf("expected *ast.BlockStmt for RangeStmt.Body, got %T", newNode)
			}
			parent.Body = body
		}
	case *ast.BinaryExpr:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr, got %T", newNode)
		}
		switch ctx.FieldName {
		case "X":
			parent.X = expr
		case "Y":
			parent.Y = expr
		}
	case *ast.UnaryExpr:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr for UnaryExpr.X, got %T", newNode)
		}
		parent.X = expr
	case *ast.SelectorExpr:
		switch ctx.FieldName {
		case "X":
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for SelectorExpr.X, got %T", newNode)
			}
			parent.X = expr
		case "Sel":
			id, ok := newNode.(*ast.Ident)
			if !ok {
				return fmt.Errorf("expected *ast.Ident for SelectorExpr.Sel, got %T", newNode)
			}
			parent.Sel = id
		}
	case *ast.AssignStmt:
		if idx >= 0 {
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr, got %T", newNode)
			}
			if ctx.FieldName == "Lhs" {
				parent.Lhs[idx] = expr
			} else {
				parent.Rhs[idx] = expr
			}
		}
	case *ast.ReturnStmt:
		if idx >= 0 {
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for ReturnStmt.Results, got %T", newNode)
			}
			parent.Results[idx] = expr
		}
	case *ast.ExprStmt:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr for ExprStmt.X, got %T", newNode)
		}
		parent.X = expr
	case *ast.SwitchStmt:
		switch ctx.FieldName {
		case "Tag":
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr for SwitchStmt.Tag, got %T", newNode)
			}
			parent.Tag = expr
		case "Body":
			body, ok := newNode.(*ast.BlockStmt)
			if !ok {
				return fmt.Errorf("expected *ast.BlockStmt for SwitchStmt.Body, got %T", newNode)
			}
			parent.Body = body
		}
	case *ast.FuncDecl:
		switch ctx.FieldName {
		case "Body":
			body, ok := newNode.(*ast.BlockStmt)
			if !ok {
				return fmt.Errorf("expected *ast.BlockStmt for FuncDecl.Body, got %T", newNode)
			}
			parent.Body = body
		}
	case *ast.GenDecl:
		if idx >= 0 && idx < len(parent.Specs) {
			spec, ok := newNode.(ast.Spec)
			if !ok {
				return fmt.Errorf("expected ast.Spec, got %T", newNode)
			}
			parent.Specs[idx] = spec
		}
	case *ast.StarExpr:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr for StarExpr.X, got %T", newNode)
		}
		parent.X = expr
	case *ast.ParenExpr:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr for ParenExpr.X, got %T", newNode)
		}
		parent.X = expr
	case *ast.IndexExpr:
		switch ctx.FieldName {
		case "X":
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr, got %T", newNode)
			}
			parent.X = expr
		case "Index":
			expr, ok := newNode.(ast.Expr)
			if !ok {
				return fmt.Errorf("expected ast.Expr, got %T", newNode)
			}
			parent.Index = expr
		}
	case *ast.SendStmt:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr, got %T", newNode)
		}
		switch ctx.FieldName {
		case "Chan":
			parent.Chan = expr
		case "Value":
			parent.Value = expr
		}
	case *ast.KeyValueExpr:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr, got %T", newNode)
		}
		switch ctx.FieldName {
		case "Key":
			parent.Key = expr
		case "Value":
			parent.Value = expr
		}
	case *ast.IncDecStmt:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr for IncDecStmt.X, got %T", newNode)
		}
		parent.X = expr
	case *ast.TypeSpec:
		expr, ok := newNode.(ast.Expr)
		if !ok {
			return fmt.Errorf("expected ast.Expr for TypeSpec.Type, got %T", newNode)
		}
		parent.Type = expr
	default:
		return fmt.Errorf("replaceInParent: unsupported parent type %T (field %q)", ctx.Parent, ctx.FieldName)
	}
	return nil
}
