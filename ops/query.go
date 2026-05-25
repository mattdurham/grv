// Namespace: goast/ops
// Read tools: ast_list, ast_query, ast_query_many, ast_meta
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/kinds"
	"github.com/mattdurham/grv/meta"
	"github.com/mattdurham/grv/selector"
)

// astNodeName returns the primary declared name for a node, if any.
func astNodeName(node ast.Node) string {
	switch n := node.(type) {
	case *ast.FuncDecl:
		return n.Name.Name
	case *ast.TypeSpec:
		return n.Name.Name
	case *ast.ValueSpec:
		if len(n.Names) > 0 {
			return n.Names[0].Name
		}
	case *ast.Ident:
		return n.Name
	}
	return ""
}

// ErrorResponse is the JSON error shape returned for navigation failures.
type ErrorResponse struct {
	Error     string             `json:"error"`
	AtStep    int                `json:"at_step,omitempty"`
	Step      *selector.PathStep `json:"step,omitempty"`
	Available []string           `json:"available,omitempty"`
}

func errResult(msg string) (json.RawMessage, error) {
	return nil, fmt.Errorf("%s", msg)
}

func navErrResult(err *selector.NavigateError) (json.RawMessage, error) {
	resp := ErrorResponse{
		Error:     err.Error(),
		AtStep:    err.AtStep,
		Step:      &err.Step,
		Available: err.Available,
	}
	b, _ := json.Marshal(resp)
	return nil, fmt.Errorf("%s", string(b))
}

func okResult(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return json.RawMessage(b), nil
}

// ASTListArgs is the argument struct for ast_list.
type ASTListArgs struct {
	File string `json:"file"`
	Dir  string `json:"dir,omitempty"`
}

// ASTListItem is one entry in the ast_list response.
type ASTListItem struct {
	Kind      string    `json:"kind"`
	Name      string    `json:"name,omitempty"`
	Recv      string    `json:"recv,omitempty"`
	Line      int       `json:"line"`
	Namespace string    `json:"namespace,omitempty"` // <import-path>#<Name>
	Readonly  bool      `json:"readonly"`
	Meta      meta.Meta `json:"meta,omitempty"`
}

// HandleASTList implements the ast_list tool.
func HandleASTList(args ASTListArgs) (json.RawMessage, error) {
	if args.Dir != "" && args.File == "" {
		return handleASTListDir(args.Dir)
	}
	f, fset, _, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	pkgPath := packageImportPath(filepath.Dir(args.File))
	ro := isReadonly(args.File)

	ns := func(name string) string {
		if name == "" {
			return ""
		}
		return pkgPath + "#" + name
	}

	var items []ASTListItem
	for _, decl := range f.Decls {
		pos := fset.Position(decl.Pos())
		switch d := decl.(type) {
		case *ast.FuncDecl:
			item := ASTListItem{
				Kind:      "FuncDecl",
				Name:      d.Name.Name,
				Line:      pos.Line,
				Namespace: ns(d.Name.Name),
				Readonly:  ro,
			}
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
						Kind:     "ImportDecl",
						Name:     name,
						Line:     fset.Position(is.Pos()).Line,
						Readonly: ro,
					})
				}
			case token.TYPE:
				for _, spec := range d.Specs {
					ts := spec.(*ast.TypeSpec)
					items = append(items, ASTListItem{
						Kind:      "TypeDecl",
						Name:      ts.Name.Name,
						Line:      fset.Position(ts.Pos()).Line,
						Namespace: ns(ts.Name.Name),
						Readonly:  ro,
					})
				}
			case token.VAR:
				items = append(items, ASTListItem{Kind: "VarDecl", Line: pos.Line, Readonly: ro})
			case token.CONST:
				items = append(items, ASTListItem{Kind: "ConstDecl", Line: pos.Line, Readonly: ro})
			}
		}
	}

	if DefaultHookRunner != nil && len(items) > 0 {
		hookMeta := mergeHookMeta(meta.Meta{}, args.File, nil)
		if len(hookMeta) > 0 {
			for i := range items {
				items[i].Meta = hookMeta
			}
		}
	}

	return okResult(items)
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
	Node      json.RawMessage `json:"node"`
	Namespace string          `json:"namespace,omitempty"` // <import-path>#<DeclName>
	Readonly  bool            `json:"readonly"`
	Meta      meta.Meta       `json:"meta,omitempty"`
}

// HandleASTQuery implements the ast_query tool.
func HandleASTQuery(args ASTQueryArgs) (json.RawMessage, error) {
	f, fset, src, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	// Empty path → file-level info
	if len(args.Path) == 0 || string(args.Path) == "null" || string(args.Path) == "[]" {
		m := meta.FileInfo(fset, src, f)
		resp := ASTQueryResponse{Meta: m}
		return okResult(resp)
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return errResult(fmt.Sprintf("parse path: %v", err))
	}

	node, _, navErr := selector.Navigate(f, steps)
	if navErr != nil {
		if ne, ok := navErr.(*selector.NavigateError); ok {
			return navErrResult(ne)
		}
		return errResult(navErr.Error())
	}

	nodeJSON, err := kinds.MarshalNode(node)
	if err != nil {
		return errResult(fmt.Sprintf("marshal node: %v", err))
	}

	resp := ASTQueryResponse{
		Node:     nodeJSON,
		Readonly: isReadonly(args.File),
	}

	// Namespace: derive from file's import path + declaration name
	if name := astNodeName(node); name != "" {
		resp.Namespace = packageImportPath(filepath.Dir(args.File)) + "#" + name
	}

	// Compute metadata
	resp.Meta = meta.Compute(fset, src, node, nil, len(steps))
	resp.Meta = mergeHookMeta(resp.Meta, args.File, nil)

	return okResult(resp)
}

// ASTQueryManyArgs is the argument struct for ast_query_many.
type ASTQueryManyArgs struct {
	File  string            `json:"file"`
	Paths []json.RawMessage `json:"paths"`
}

// HandleASTQueryMany implements the ast_query_many tool.
func HandleASTQueryMany(args ASTQueryManyArgs) (json.RawMessage, error) {
	f, fset, src, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	results := make([]ASTQueryResponse, 0, len(args.Paths))
	for _, pathJSON := range args.Paths {
		var steps []selector.PathStep
		if err := json.Unmarshal(pathJSON, &steps); err != nil {
			return errResult(fmt.Sprintf("parse path: %v", err))
		}

		node, _, navErr := selector.Navigate(f, steps)
		if navErr != nil {
			if ne, ok := navErr.(*selector.NavigateError); ok {
				return navErrResult(ne)
			}
			return errResult(navErr.Error())
		}

		nodeJSON, err := kinds.MarshalNode(node)
		if err != nil {
			return errResult(fmt.Sprintf("marshal node: %v", err))
		}

		resp := ASTQueryResponse{Node: nodeJSON}
		resp.Meta = meta.Compute(fset, src, node, nil, len(steps))
		results = append(results, resp)
	}

	return okResult(results)
}

// ASTMetaArgs is the argument struct for ast_meta.
type ASTMetaArgs struct {
	File  string          `json:"file"`
	Path  json.RawMessage `json:"path"`
	Hooks []string        `json:"hooks,omitempty"`
}

// HandleASTMeta implements the ast_meta tool.
func HandleASTMeta(args ASTMetaArgs) (json.RawMessage, error) {
	f, fset, src, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	if len(args.Path) == 0 || string(args.Path) == "null" || string(args.Path) == "[]" {
		m := meta.FileInfo(fset, src, f)
		m = mergeHookMeta(m, args.File, args.Hooks)
		return okResult(m)
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return errResult(fmt.Sprintf("parse path: %v", err))
	}

	node, _, navErr := selector.Navigate(f, steps)
	if navErr != nil {
		if ne, ok := navErr.(*selector.NavigateError); ok {
			return navErrResult(ne)
		}
		return errResult(navErr.Error())
	}

	m := meta.Compute(fset, src, node, nil, len(steps))
	m = mergeHookMeta(m, args.File, args.Hooks)
	return okResult(m)
}

func handleASTListDir(dir string) (json.RawMessage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errResult(fmt.Sprintf("read dir: %v", err))
	}
	var merged []ASTListItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		result, err := HandleASTList(ASTListArgs{File: filepath.Join(dir, e.Name())})
		if err != nil {
			continue
		}
		var items []ASTListItem
		if err := json.Unmarshal(result, &items); err != nil {
			continue
		}
		merged = append(merged, items...)
	}
	if merged == nil {
		merged = []ASTListItem{}
	}
	return okResult(merged)
}
