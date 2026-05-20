// Namespace: goast/ops
// Read tools: ast_list, ast_query, ast_query_many, ast_meta
package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/lthiery/goast/editor"
	"github.com/lthiery/goast/kinds"
	"github.com/lthiery/goast/meta"
	"github.com/lthiery/goast/selector"
	"github.com/mark3labs/mcp-go/mcp"
)

// ErrorResponse is the JSON error shape returned for navigation failures.
type ErrorResponse struct {
	Error     string             `json:"error"`
	AtStep    int                `json:"at_step,omitempty"`
	Step      *selector.PathStep `json:"step,omitempty"`
	Available []string           `json:"available,omitempty"`
}

func toolError(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(msg)
}

func navError(err *selector.NavigateError) *mcp.CallToolResult {
	resp := ErrorResponse{
		Error:     err.Error(),
		AtStep:    err.AtStep,
		Step:      &err.Step,
		Available: err.Available,
	}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultError(string(b))
}

// ASTListArgs is the argument struct for ast_list.
type ASTListArgs struct {
	File string `json:"file"`
}

// ASTListItem is one entry in the ast_list response.
type ASTListItem struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	Recv string `json:"recv,omitempty"`
	Line int    `json:"line"`
}

// HandleASTList implements the ast_list tool.
func HandleASTList(ctx context.Context, req mcp.CallToolRequest, args ASTListArgs) (*mcp.CallToolResult, error) {
	f, fset, _, err := editor.ParseFile(args.File)
	if err != nil {
		return toolError(fmt.Sprintf("parse: %v", err)), nil
	}

	var items []ASTListItem
	for _, decl := range f.Decls {
		pos := fset.Position(decl.Pos())
		switch d := decl.(type) {
		case *ast.FuncDecl:
			item := ASTListItem{Kind: "FuncDecl", Name: d.Name.Name, Line: pos.Line}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				item.Recv = recvTypeString(d.Recv.List[0])
			}
			items = append(items, item)
		case *ast.GenDecl:
			switch d.Tok {
			case token.IMPORT:
				for _, spec := range d.Specs {
					is := spec.(*ast.ImportSpec)
					name := ""
					if is.Name != nil {
						name = is.Name.Name
					}
					items = append(items, ASTListItem{
						Kind: "ImportDecl",
						Name: name,
						Line: fset.Position(is.Pos()).Line,
					})
				}
			case token.TYPE:
				for _, spec := range d.Specs {
					ts := spec.(*ast.TypeSpec)
					items = append(items, ASTListItem{
						Kind: "TypeDecl",
						Name: ts.Name.Name,
						Line: fset.Position(ts.Pos()).Line,
					})
				}
			case token.VAR:
				items = append(items, ASTListItem{Kind: "VarDecl", Line: pos.Line})
			case token.CONST:
				items = append(items, ASTListItem{Kind: "ConstDecl", Line: pos.Line})
			}
		}
	}

	b, _ := json.Marshal(items)
	return mcp.NewToolResultText(string(b)), nil
}

func recvTypeString(field *ast.Field) string {
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return "*" + id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// ASTQueryArgs is the argument struct for ast_query.
type ASTQueryArgs struct {
	File string          `json:"file"`
	Path json.RawMessage `json:"path"`
}

// ASTQueryResponse is the response for ast_query.
type ASTQueryResponse struct {
	Node   json.RawMessage `json:"node"`
	Source string          `json:"source,omitempty"`
	Meta   meta.Meta       `json:"meta,omitempty"`
}

// HandleASTQuery implements the ast_query tool.
func HandleASTQuery(ctx context.Context, req mcp.CallToolRequest, args ASTQueryArgs) (*mcp.CallToolResult, error) {
	f, fset, src, err := editor.ParseFile(args.File)
	if err != nil {
		return toolError(fmt.Sprintf("parse: %v", err)), nil
	}

	// Empty path → file-level info
	if len(args.Path) == 0 || string(args.Path) == "null" || string(args.Path) == "[]" {
		m := meta.FileInfo(fset, src, f)
		resp := ASTQueryResponse{Meta: m}
		b, _ := json.Marshal(resp)
		return mcp.NewToolResultText(string(b)), nil
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return toolError(fmt.Sprintf("parse path: %v", err)), nil
	}

	node, _, navErr := selector.Navigate(f, steps)
	if navErr != nil {
		if ne, ok := navErr.(*selector.NavigateError); ok {
			return navError(ne), nil
		}
		return toolError(navErr.Error()), nil
	}

	nodeJSON, err := kinds.MarshalNode(node)
	if err != nil {
		return toolError(fmt.Sprintf("marshal node: %v", err)), nil
	}

	resp := ASTQueryResponse{Node: nodeJSON}

	// Extract source text
	pos := fset.Position(node.Pos())
	end := fset.Position(node.End())
	if pos.IsValid() && end.IsValid() && end.Offset <= len(src) {
		resp.Source = string(src[pos.Offset:end.Offset])
	}

	// Compute metadata
	resp.Meta = meta.Compute(fset, src, node, nil, len(steps))

	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}

// ASTQueryManyArgs is the argument struct for ast_query_many.
type ASTQueryManyArgs struct {
	File  string            `json:"file"`
	Paths []json.RawMessage `json:"paths"`
}

// HandleASTQueryMany implements the ast_query_many tool.
func HandleASTQueryMany(ctx context.Context, req mcp.CallToolRequest, args ASTQueryManyArgs) (*mcp.CallToolResult, error) {
	f, fset, src, err := editor.ParseFile(args.File)
	if err != nil {
		return toolError(fmt.Sprintf("parse: %v", err)), nil
	}

	results := make([]ASTQueryResponse, 0, len(args.Paths))
	for _, pathJSON := range args.Paths {
		var steps []selector.PathStep
		if err := json.Unmarshal(pathJSON, &steps); err != nil {
			return toolError(fmt.Sprintf("parse path: %v", err)), nil
		}

		node, _, navErr := selector.Navigate(f, steps)
		if navErr != nil {
			if ne, ok := navErr.(*selector.NavigateError); ok {
				return navError(ne), nil
			}
			return toolError(navErr.Error()), nil
		}

		nodeJSON, err := kinds.MarshalNode(node)
		if err != nil {
			return toolError(fmt.Sprintf("marshal node: %v", err)), nil
		}

		resp := ASTQueryResponse{Node: nodeJSON}
		pos := fset.Position(node.Pos())
		end := fset.Position(node.End())
		if pos.IsValid() && end.IsValid() && end.Offset <= len(src) {
			resp.Source = string(src[pos.Offset:end.Offset])
		}
		resp.Meta = meta.Compute(fset, src, node, nil, len(steps))
		results = append(results, resp)
	}

	b, _ := json.Marshal(results)
	return mcp.NewToolResultText(string(b)), nil
}

// ASTMetaArgs is the argument struct for ast_meta.
type ASTMetaArgs struct {
	File string          `json:"file"`
	Path json.RawMessage `json:"path"`
}

// HandleASTMeta implements the ast_meta tool.
func HandleASTMeta(ctx context.Context, req mcp.CallToolRequest, args ASTMetaArgs) (*mcp.CallToolResult, error) {
	f, fset, src, err := editor.ParseFile(args.File)
	if err != nil {
		return toolError(fmt.Sprintf("parse: %v", err)), nil
	}

	if len(args.Path) == 0 || string(args.Path) == "null" || string(args.Path) == "[]" {
		m := meta.FileInfo(fset, src, f)
		b, _ := json.Marshal(m)
		return mcp.NewToolResultText(string(b)), nil
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return toolError(fmt.Sprintf("parse path: %v", err)), nil
	}

	node, _, navErr := selector.Navigate(f, steps)
	if navErr != nil {
		if ne, ok := navErr.(*selector.NavigateError); ok {
			return navError(ne), nil
		}
		return toolError(navErr.Error()), nil
	}

	m := meta.Compute(fset, src, node, nil, len(steps))
	b, _ := json.Marshal(m)
	return mcp.NewToolResultText(string(b)), nil
}
