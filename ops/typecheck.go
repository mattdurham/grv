// Namespace: goast/ops
// Type-aware check rules — require go/types package loading.
package ops

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// typeAwareRuleFunc is a check that requires full type information.
// fset and f are for the specific file being checked; info covers the whole package.
type typeAwareRuleFunc func(fset *token.FileSet, f *ast.File, info *types.Info, absFile string) []Violation

// typeAwareRules is the registry of rules that need go/types.
var typeAwareRules = map[string]typeAwareRuleFunc{
	"error_not_checked": ruleErrorNotChecked,
}

// loadPackageForFile loads the package containing absFile with full type information.
// Returns nil, nil if the file is not part of a loadable package (e.g. standalone snippet).
func loadPackageForFile(absFile string) (*packages.Package, error) {
	dir := filepath.Dir(absFile)
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports,
		Dir:  dir,
		Fset: token.NewFileSet(),
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil || len(pkgs) == 0 {
		return nil, err
	}
	return pkgs[0], nil
}

// findSyntaxFile returns the *ast.File in pkg.Syntax whose position matches absFile.
func findSyntaxFile(pkg *packages.Package, absFile string) (*ast.File, *token.FileSet) {
	for _, f := range pkg.Syntax {
		pos := pkg.Fset.Position(f.Pos())
		if filepath.Clean(pos.Filename) == filepath.Clean(absFile) {
			return f, pkg.Fset
		}
	}
	return nil, nil
}

// runTypeAwareChecks loads package type info and runs all active type-aware rules.
// Falls back gracefully (returns nil) if the package cannot be loaded.
func runTypeAwareChecks(absFile string, enforce []string) []Violation {
	// Collect which type-aware rules are active.
	active := resolveTypeAwareRules(enforce)
	if len(active) == 0 {
		return nil
	}

	pkg, err := loadPackageForFile(absFile)
	if err != nil || pkg == nil || pkg.TypesInfo == nil {
		return nil // degrade gracefully — type info unavailable
	}

	f, fset := findSyntaxFile(pkg, absFile)
	if f == nil {
		return nil
	}

	var out []Violation
	for _, rule := range active {
		out = append(out, rule(fset, f, pkg.TypesInfo, absFile)...)
	}
	return out
}

// resolveTypeAwareRules returns the active type-aware rule funcs for the enforce list.
func resolveTypeAwareRules(enforce []string) []typeAwareRuleFunc {
	var out []typeAwareRuleFunc
	seen := make(map[string]bool)
	for _, name := range enforce {
		if name == "all" {
			for n, fn := range typeAwareRules {
				if !seen[n] {
					seen[n] = true
					out = append(out, fn)
				}
			}
			continue
		}
		if fn, ok := typeAwareRules[name]; ok && !seen[name] {
			seen[name] = true
			out = append(out, fn)
		}
	}
	return out
}

// ruleErrorNotChecked flags ExprStmt containing a CallExpr whose last return type
// is error — the result is completely discarded with no assignment at all.
//
// This is the type-aware complement to error_handled: error_handled catches the
// `_, err := fn()` pattern where err is explicitly blanked; error_not_checked
// catches `fn()` where the error return is not captured at all.
//
// Suppression: add any comment on the same line.
func ruleErrorNotChecked(fset *token.FileSet, f *ast.File, info *types.Info, absFile string) []Violation {
	// Pre-build comment line set.
	commentLines := make(map[int]bool)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			commentLines[fset.Position(c.Slash).Line] = true
		}
	}

	var violations []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		exprStmt, ok := n.(*ast.ExprStmt)
		if !ok {
			return true
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Look up the signature of the called function.
		var sig *types.Signature
		switch t := info.TypeOf(call.Fun).(type) {
		case *types.Signature:
			sig = t
		default:
			return true
		}

		// Check if the last result is error.
		res := sig.Results()
		if res.Len() == 0 {
			return true
		}
		last := res.At(res.Len() - 1)
		if !types.Implements(last.Type(), errorInterface()) {
			return true
		}

		line := fset.Position(exprStmt.Pos()).Line
		if commentLines[line] {
			return true
		}
		violations = append(violations, Violation{
			File:    absFile,
			Line:    line,
			Rule:    "error_not_checked",
			Message: "function returns an error that is not checked — assign the result or use _ explicitly with a comment",
		})
		return true
	})
	return violations
}

// errorInterface returns the *types.Interface for the built-in error type.
func errorInterface() *types.Interface {
	// error is defined as interface { Error() string }
	errorMethod := types.NewFunc(0, nil, "Error",
		types.NewSignatureType(nil, nil, nil,
			types.NewTuple(),
			types.NewTuple(types.NewVar(0, nil, "", types.Typ[types.String])),
			false,
		),
	)
	return types.NewInterfaceType([]*types.Func{errorMethod}, nil).Complete()
}
