// Namespace: goast/ops
// Import tools: ast_add_import, ast_delete_import, ast_list_imports
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mattdurham/grv/editor"
	"golang.org/x/tools/go/ast/astutil"
)

// AddImportArgs is the argument struct for ast_add_import.
type AddImportArgs struct {
	File  string `json:"file"`
	Path  string `json:"path"`
	Alias string `json:"alias"`
}

// DeleteImportArgs is the argument struct for ast_delete_import.
type DeleteImportArgs struct {
	File string `json:"file"`
	Path string `json:"path"`
}

// ListImportsArgs is the argument struct for ast_list_imports.
type ListImportsArgs struct {
	File string `json:"file"`
}

// ImportInfo is one entry in the ast_list_imports response.
type ImportInfo struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
	Used  bool   `json:"used"`
}

// HandleAddImport implements the ast_add_import tool.
func HandleAddImport(args AddImportArgs) (json.RawMessage, error) {
	if isReadonly(args.File) {
		return errResult(fmt.Sprintf("file is readonly: %s", args.File))
	}
	result, err := editor.Edit(args.File, false, func(f *ast.File, fset *token.FileSet) error {
		if args.Alias == "" {
			astutil.AddImport(fset, f, args.Path)
		} else {
			astutil.AddNamedImport(fset, f, args.Alias, args.Path)
		}
		return nil
	})
	if err != nil {
		return errResult(fmt.Sprintf("add import: %v", err))
	}
	resp := map[string]interface{}{
		"changed": result.Changed,
		"diff":    result.Diff,
	}
	return okResult(resp)
}

// HandleDeleteImport implements the ast_delete_import tool.
func HandleDeleteImport(args DeleteImportArgs) (json.RawMessage, error) {
	result, err := editor.Edit(args.File, false, func(f *ast.File, fset *token.FileSet) error {
		astutil.DeleteImport(fset, f, args.Path)
		return nil
	})
	if err != nil {
		return errResult(fmt.Sprintf("delete import: %v", err))
	}
	resp := map[string]interface{}{
		"changed": result.Changed,
		"diff":    result.Diff,
	}
	return okResult(resp)
}

// HandleListImports implements the ast_list_imports tool.
func HandleListImports(args ListImportsArgs) (json.RawMessage, error) {
	f, _, _, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	// Collect all used idents to detect import usage
	usedPkgs := collectUsedIdents(f)

	var imports []ImportInfo
	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1] // strip quotes
		}
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		// Determine effective package name for usage check
		pkgName := alias
		if pkgName == "" || pkgName == "." || pkgName == "_" {
			// use last segment of path
			pkgName = lastPathSegment(path)
		}
		used := pkgName == "." || usedPkgs[pkgName]
		imports = append(imports, ImportInfo{Path: path, Alias: alias, Used: used})
	}

	return okResult(imports)
}

func collectUsedIdents(f *ast.File) map[string]bool {
	used := map[string]bool{}
	for _, decl := range f.Decls {
		ast.Inspect(decl, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok {
					used[id.Name] = true
				}
			}
			return true
		})
	}
	return used
}

func lastPathSegment(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
