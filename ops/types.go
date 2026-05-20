// Namespace: goast/ops
// Type-aware tools: ast_find_refs, ast_find_def, ast_find_impls
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/meta"
	"github.com/mattdurham/grv/selector"
	"golang.org/x/tools/go/packages"
)

// loadTypedPkg loads the package containing filePath using go/packages with
// full type information. cfg.Dir is set to the file's directory so "." resolves
// to the package under the cursor.
func loadTypedPkg(filePath string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedTypesInfo |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedFiles |
			packages.NeedImports |
			packages.NeedName,
		Dir: filepath.Dir(filePath),
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found in %s", filepath.Dir(filePath))
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("package errors: %v", pkg.Errors[0])
	}
	return pkg, nil
}

// fileInPkg finds the *ast.File in pkg whose base name matches filePath.
// go/packages uses its own fset, so we match by file basename.
func fileInPkg(pkg *packages.Package, filePath string) (*ast.File, error) {
	base := filepath.Base(filePath)
	for _, f := range pkg.Syntax {
		pos := pkg.Fset.Position(f.Pos())
		if filepath.Base(pos.Filename) == base {
			return f, nil
		}
	}
	return nil, fmt.Errorf("file %s not found in package", filePath)
}

// identAtPath navigates to the node at steps in file f (parsed by editor, not
// pkg) and then finds the matching *ast.Ident in the pkg's own AST using
// line+col from fset.
func identAtPath(f *ast.File, fset *token.FileSet, steps []selector.PathStep, pkg *packages.Package, pkgFile *ast.File) (*ast.Ident, error) {
	node, _, err := selector.Navigate(f, steps)
	if err != nil {
		return nil, err
	}

	// Extract the identifier to look up. For a FuncDecl, use the name ident.
	var ident *ast.Ident
	switch n := node.(type) {
	case *ast.FuncDecl:
		ident = n.Name
	case *ast.TypeSpec:
		ident = n.Name
	case *ast.Ident:
		ident = n
	case *ast.Field:
		if len(n.Names) > 0 {
			ident = n.Names[0]
		}
	}
	if ident == nil {
		return nil, fmt.Errorf("no identifier at path")
	}

	// Map position from editor fset → pkg fset using base filename + line + col.
	pos := fset.Position(ident.Pos())
	target := findIdentAt(pkgFile, pkg.Fset, filepath.Base(pos.Filename), pos.Line, pos.Column)
	if target == nil {
		return nil, fmt.Errorf("identifier %q not found in typed AST (line %d col %d)", ident.Name, pos.Line, pos.Column)
	}
	return target, nil
}

// findIdentAt searches the AST rooted at f for an *ast.Ident at the given
// base filename / line / column (using pkg's fset).
func findIdentAt(f *ast.File, fset *token.FileSet, baseFilename string, line, col int) *ast.Ident {
	var found *ast.Ident
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil || found != nil {
			return false
		}
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		pos := fset.Position(id.Pos())
		if filepath.Base(pos.Filename) == baseFilename && pos.Line == line && pos.Column == col {
			found = id
			return false
		}
		return true
	})
	return found
}

// ---- ast_find_refs ----

// ASTFindRefsArgs is the argument struct for ast_find_refs.
type ASTFindRefsArgs struct {
	File  string          `json:"file"`
	Path  json.RawMessage `json:"path"`
	Scope string          `json:"scope,omitempty"` // "file" (default) or "package"
}

// RefResult is one entry in the ast_find_refs response.
type RefResult struct {
	File string              `json:"file"`
	Path []selector.PathStep `json:"path"`
	Kind string              `json:"kind"`
	Line int                 `json:"line"`
}

// HandleASTFindRefs implements the ast_find_refs tool.
func HandleASTFindRefs(args ASTFindRefsArgs) (json.RawMessage, error) {
	if args.File == "" {
		return errResult("file is required")
	}
	if len(args.Path) == 0 || string(args.Path) == "null" || string(args.Path) == "[]" {
		return errResult("path is required")
	}

	f, fset, _, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return errResult(fmt.Sprintf("parse path: %v", err))
	}

	pkg, err := loadTypedPkg(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("load package: %v", err))
	}

	pkgFile, err := fileInPkg(pkg, args.File)
	if err != nil {
		return errResult(err.Error())
	}

	targetIdent, err := identAtPath(f, fset, steps, pkg, pkgFile)
	if err != nil {
		return errResult(err.Error())
	}

	obj := pkg.TypesInfo.ObjectOf(targetIdent)
	if obj == nil {
		// Fall back: return empty list for unresolved idents
		return okResult([]RefResult{})
	}

	// Collect all references across the scope.
	var searchFiles []*ast.File
	if args.Scope == "package" {
		searchFiles = pkg.Syntax
	} else {
		searchFiles = []*ast.File{pkgFile}
	}

	ancestors := buildPkgAncestors(searchFiles)
	var results []RefResult
	for _, sf := range searchFiles {
		sfPos := pkg.Fset.Position(sf.Pos())
		ast.Inspect(sf, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if id == targetIdent {
				return true // skip the declaration itself
			}
			if pkg.TypesInfo.ObjectOf(id) == obj {
				pos := pkg.Fset.Position(id.Pos())
				nodePath := buildPath(id, ancestors)
				results = append(results, RefResult{
					File: sfPos.Filename,
					Path: nodePath,
					Kind: "Ident",
					Line: pos.Line,
				})
			}
			return true
		})
	}

	if results == nil {
		results = []RefResult{}
	}
	return okResult(results)
}

// buildPkgAncestors builds the ancestor map for a slice of files (used for
// cross-file operations where each file's own collectAncestors would lose
// cross-file context).
func buildPkgAncestors(files []*ast.File) map[ast.Node]nodeAncestor {
	result := make(map[ast.Node]nodeAncestor)
	for _, f := range files {
		for k, v := range collectAncestors(f) {
			result[k] = v
		}
	}
	return result
}

// ---- ast_find_def ----

// ASTFindDefArgs is the argument struct for ast_find_def.
type ASTFindDefArgs struct {
	File string          `json:"file"`
	Path json.RawMessage `json:"path"`
}

// ASTFindDefResponse is the response for ast_find_def.
type ASTFindDefResponse struct {
	File     string              `json:"file"`
	Path     []selector.PathStep `json:"path,omitempty"`
	Node     json.RawMessage     `json:"node,omitempty"`
	Source   string              `json:"source,omitempty"`
	Meta     meta.Meta           `json:"meta,omitempty"`
	External bool                `json:"external,omitempty"`
	Package  string              `json:"package,omitempty"`
	Symbol   string              `json:"symbol,omitempty"`
}

// HandleASTFindDef implements the ast_find_def tool.
func HandleASTFindDef(args ASTFindDefArgs) (json.RawMessage, error) {
	if args.File == "" {
		return errResult("file is required")
	}
	if len(args.Path) == 0 || string(args.Path) == "null" || string(args.Path) == "[]" {
		return errResult("path is required")
	}

	f, fset, src, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return errResult(fmt.Sprintf("parse path: %v", err))
	}

	pkg, err := loadTypedPkg(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("load package: %v", err))
	}

	pkgFile, err := fileInPkg(pkg, args.File)
	if err != nil {
		return errResult(err.Error())
	}

	targetIdent, identErr := identAtPath(f, fset, steps, pkg, pkgFile)
	if identErr != nil {
		// If the path resolves to a non-ident node (e.g., the FuncDecl itself),
		// we treat the declaration as its own definition.
		node, _, navErr := selector.Navigate(f, steps)
		if navErr != nil {
			return errResult(navErr.Error())
		}
		pos := fset.Position(node.Pos())
		end := fset.Position(node.End())
		var source string
		if pos.IsValid() && end.IsValid() && end.Offset <= len(src) {
			source = string(src[pos.Offset:end.Offset])
		}
		m := meta.Compute(fset, src, node, nil, len(steps))
		resp := ASTFindDefResponse{
			File:   args.File,
			Source: source,
			Meta:   m,
		}
		return okResult(resp)
	}

	obj := pkg.TypesInfo.ObjectOf(targetIdent)
	if obj == nil {
		resp := ASTFindDefResponse{File: args.File}
		return okResult(resp)
	}

	defPos := obj.Pos()
	if !defPos.IsValid() {
		// Builtin
		resp := ASTFindDefResponse{
			File:     "builtin",
			External: true,
			Symbol:   obj.Name(),
		}
		return okResult(resp)
	}

	pkgFsetPos := pkg.Fset.Position(defPos)
	defFile := pkgFsetPos.Filename

	resp := ASTFindDefResponse{File: defFile}

	// Try to build a path in the definition file.
	df, dfset, _, parseErr := editor.ParseFile(defFile)
	if parseErr == nil {
		defFileInPkg, findErr := fileInPkg(pkg, defFile)
		if findErr == nil {
			defIdent := findIdentAt(defFileInPkg, pkg.Fset, filepath.Base(defFile), pkgFsetPos.Line, pkgFsetPos.Column)
			if defIdent != nil {
				ancestors := collectAncestors(df)
				// Map pkg fset position back to editor fset position by name match
				ast.Inspect(df, func(n ast.Node) bool {
					id, ok := n.(*ast.Ident)
					if !ok {
						return true
					}
					if id.Name != defIdent.Name {
						return true
					}
					p := dfset.Position(id.Pos())
					if p.Line == pkgFsetPos.Line {
						nodePath := buildPath(id, ancestors)
						resp.Path = nodePath
						return false
					}
					return true
				})
				_ = ancestors
			}
		}
	}

	// Include meta from the definition node if we can find it.
	if df != nil && resp.Path != nil {
		node, _, navErr := selector.Navigate(df, resp.Path)
		if navErr == nil {
			_, dfset2, src2, _ := editor.ParseFile(defFile)
			m := meta.Compute(dfset2, src2, node, nil, len(resp.Path))
			resp.Meta = m
		}
	}

	return okResult(resp)
}

// ---- ast_find_impls ----

// ASTFindImplsArgs is the argument struct for ast_find_impls.
type ASTFindImplsArgs struct {
	File  string          `json:"file"`
	Path  json.RawMessage `json:"path"`
	Scope string          `json:"scope,omitempty"` // "package" (default) or "file"
}

// ImplResult is one entry in the ast_find_impls response.
type ImplResult struct {
	File     string              `json:"file"`
	Path     []selector.PathStep `json:"path,omitempty"`
	TypeName string              `json:"type_name"`
	Meta     meta.Meta           `json:"meta,omitempty"`
}

// HandleASTFindImpls implements the ast_find_impls tool.
func HandleASTFindImpls(args ASTFindImplsArgs) (json.RawMessage, error) {
	if args.File == "" {
		return errResult("file is required")
	}
	if len(args.Path) == 0 || string(args.Path) == "null" || string(args.Path) == "[]" {
		return errResult("path is required")
	}

	f, fset, _, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return errResult(fmt.Sprintf("parse path: %v", err))
	}

	// Navigate to the TypeSpec node.
	node, _, navErr := selector.Navigate(f, steps)
	if navErr != nil {
		return errResult(navErr.Error())
	}
	ts, ok := node.(*ast.TypeSpec)
	if !ok {
		return errResult("path must point to a TypeSpec")
	}
	_ = fset
	_ = ts

	pkg, err := loadTypedPkg(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("load package: %v", err))
	}

	pkgFile, err := fileInPkg(pkg, args.File)
	if err != nil {
		return errResult(err.Error())
	}

	// Find the TypeSpec in the typed AST by name.
	var ifaceObj types.Object
	ast.Inspect(pkgFile, func(n ast.Node) bool {
		if id, ok2 := n.(*ast.Ident); ok2 && id.Name == ts.Name.Name {
			o := pkg.TypesInfo.Defs[id]
			if o != nil {
				ifaceObj = o
				return false
			}
		}
		return true
	})
	if ifaceObj == nil {
		return errResult(fmt.Sprintf("type %s not found in typed AST", ts.Name.Name))
	}

	ifaceType, ok := ifaceObj.Type().Underlying().(*types.Interface)
	if !ok {
		return errResult(fmt.Sprintf("%s is not an interface type", ts.Name.Name))
	}

	// Collect all named types in scope and check if they implement the interface.
	var searchFiles []*ast.File
	if args.Scope == "file" {
		searchFiles = []*ast.File{pkgFile}
	} else {
		searchFiles = pkg.Syntax
	}

	var results []ImplResult
	seen := map[string]bool{}

	for _, sf := range searchFiles {
		sfPosStr := pkg.Fset.Position(sf.Pos()).Filename
		ast.Inspect(sf, func(n ast.Node) bool {
			id, ok2 := n.(*ast.Ident)
			if !ok2 {
				return true
			}
			def := pkg.TypesInfo.Defs[id]
			if def == nil {
				return true
			}
			tn, ok2 := def.(*types.TypeName)
			if !ok2 || tn.IsAlias() {
				return true
			}
			T := tn.Type()
			ptrT := types.NewPointer(T)
			implementsT := types.Implements(T, ifaceType)
			implementsPtr := types.Implements(ptrT, ifaceType)
			if !implementsT && !implementsPtr {
				return true
			}
			// Skip the interface itself.
			if T.Underlying() == ifaceType {
				return true
			}
			typeName := tn.Name()
			if implementsPtr && !implementsT {
				typeName = "*" + typeName
			}
			key := sfPosStr + ":" + typeName
			if seen[key] {
				return true
			}
			seen[key] = true

			results = append(results, ImplResult{
				File:     sfPosStr,
				TypeName: typeName,
			})
			return true
		})
	}

	if results == nil {
		results = []ImplResult{}
	}
	return okResult(results)
}
