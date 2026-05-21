// Namespace: goast/ops
// Tool: ast_place — smart file routing for new declarations
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/kinds"
)

// ASTPlaceArgs is the argument struct for ast_place.
type ASTPlaceArgs struct {
	Dir    string          `json:"dir"`               // package directory
	Node   json.RawMessage `json:"node"`              // declaration to place
	DryRun bool            `json:"dry_run,omitempty"` // preview without writing
}

// ASTPlaceResult is the response for ast_place.
type ASTPlaceResult struct {
	File      string `json:"file"`      // chosen file (relative to Dir)
	Namespace string `json:"namespace"` // canonical qualified name: <import-path>#<DeclName>
	Reason    string `json:"reason"`    // why this file was chosen
	Created   bool   `json:"created"`   // true if file did not exist and was created
	Diff      string `json:"diff"`
	Changed   bool   `json:"changed"`
}

// HandleASTPlace routes a new declaration to the correct file and inserts it.
//
// Routing rules:
//   - struct type Foo          → foo.go (created if missing)
//   - method on *Foo / Foo     → foo.go (where Foo is declared)
//   - func NewFoo(...)         → foo.go
//   - typed const (enum)       → <typename>.go (e.g. Status → status.go)
//   - untyped const            → constants.go
//   - var declaration          → constants.go (or existing var-heavy file)
//   - free function            → file with most functions, or <pkgname>.go
func HandleASTPlace(args ASTPlaceArgs) (json.RawMessage, error) {
	if args.Dir == "" {
		return nil, fmt.Errorf("dir is required")
	}

	kindNode, err := kinds.UnmarshalNode(args.Node)
	if err != nil {
		return nil, fmt.Errorf("parse node: %w", err)
	}
	newDecl, err := kindNode.ToAST()
	if err != nil {
		return nil, fmt.Errorf("ToAST: %w", err)
	}
	astDecl, ok := newDecl.(ast.Decl)
	if !ok {
		return nil, fmt.Errorf("node must be a declaration, got %T", newDecl)
	}

	pkg, err := scanPackage(args.Dir)
	if err != nil {
		return nil, fmt.Errorf("scan dir: %w", err)
	}

	targetFile, reason, created := routeDecl(astDecl, pkg, args.Dir)

	// Create the file with a package declaration if it doesn't exist yet.
	// We always write the stub (even for dry_run) so editor.Edit can parse it;
	// if dry_run, we remove it on failure or let the diff speak for itself.
	if created {
		content := fmt.Sprintf("package %s\n", pkg.pkgName)
		if err := os.WriteFile(targetFile, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("create %s: %w", targetFile, err)
		}
		// For dry_run: remove the stub if the edit fails or after we have the diff.
		if args.DryRun {
			defer func() {
				// Only remove if the file is still just the package stub
				data, _ := os.ReadFile(targetFile)
				if string(data) == content {
					os.Remove(targetFile)
				}
			}()
		}
	}

	result, err := editor.Edit(targetFile, args.DryRun, func(f *ast.File, _ *token.FileSet) error {
		f.Decls = append(f.Decls, astDecl)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("insert into %s: %w", targetFile, err)
	}

	rel, _ := filepath.Rel(args.Dir, targetFile)
	ns := declNamespace(astDecl, args.Dir)
	res := ASTPlaceResult{
		File:      rel,
		Namespace: ns,
		Reason:    reason,
		Created:   created,
		Diff:      result.Diff,
		Changed:   result.Changed,
	}
	return okResult(res)
}

// routeDecl selects the target file and whether it needs to be created.
func routeDecl(decl ast.Decl, pkg *packageInfo, dir string) (absPath, reason string, created bool) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return routeFunc(d, pkg, dir)
	case *ast.GenDecl:
		return routeGenDecl(d, pkg, dir)
	}
	f := defaultFile(pkg, dir)
	return f, "default", !fileExists(f)
}

func routeFunc(d *ast.FuncDecl, pkg *packageInfo, dir string) (string, string, bool) {
	name := d.Name.Name

	// Method: same file as the receiver type declaration
	if d.Recv != nil && len(d.Recv.List) > 0 {
		recv := baseTypeName(d.Recv.List[0])
		target := fileForType(recv, pkg, dir)
		exists := fileExists(target)
		return target,
			fmt.Sprintf("method on %s → %s", recv, filepath.Base(target)),
			!exists
	}

	// Constructor NewFoo → same file as Foo
	if strings.HasPrefix(name, "New") && len(name) > 3 {
		typeName := name[3:]
		target := fileForType(typeName, pkg, dir)
		exists := fileExists(target)
		return target,
			fmt.Sprintf("constructor for %s → %s", typeName, filepath.Base(target)),
			!exists
	}

	// Free function → main.go if this is a main package, else file with most functions
	if pkg.pkgName == "main" {
		if p := findFile(pkg, "main.go"); p != "" {
			return p, "main package: free functions go in main.go", false
		}
	}
	best, bestCount := "", 0
	for _, fi := range pkg.files {
		if len(fi.funcs) > bestCount {
			bestCount = len(fi.funcs)
			best = fi.path
		}
	}
	if best != "" {
		return best, fmt.Sprintf("most functions (%d) in %s", bestCount, filepath.Base(best)), false
	}
	f := defaultFile(pkg, dir)
	return f, "default", !fileExists(f)
}

func routeGenDecl(d *ast.GenDecl, pkg *packageInfo, dir string) (string, string, bool) {
	tok := d.Tok.String()

	switch tok {
	case "type":
		// Struct/interface/type → <typename>.go (lowercased)
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			target := filepath.Join(dir, strings.ToLower(ts.Name.Name)+".go")
			return target,
				fmt.Sprintf("type %s → %s", ts.Name.Name, filepath.Base(target)),
				!fileExists(target)
		}

	case "const":
		// Typed const (enum): find the type name from the first ValueSpec
		// e.g. const StatusPending Status = iota → status.go
		if typeName := constTypeName(d); typeName != "" {
			target := filepath.Join(dir, strings.ToLower(typeName)+".go")
			return target,
				fmt.Sprintf("const of type %s → %s", typeName, filepath.Base(target)),
				!fileExists(target)
		}
		// Untyped const → constants.go
		target := filepath.Join(dir, "constants.go")
		return target, "untyped constants → constants.go", !fileExists(target)

	case "var":
		// Vars → constants.go (or create it)
		target := filepath.Join(dir, "constants.go")
		if p := findFile(pkg, "vars.go"); p != "" {
			return p, "vars → vars.go", false
		}
		return target, "vars → constants.go", !fileExists(target)
	}

	f := defaultFile(pkg, dir)
	return f, fmt.Sprintf("default for %s", tok), !fileExists(f)
}

// fileForType returns the conventional file path for a type (e.g. Dog → dog.go).
// If a file already declares the type, that file is returned instead.
func fileForType(typeName string, pkg *packageInfo, dir string) string {
	// Check if type is already declared in an existing file
	for _, fi := range pkg.files {
		for _, t := range fi.types {
			if t == typeName {
				return fi.path
			}
		}
	}
	// Conventional path: <typename>.go (lowercase)
	return filepath.Join(dir, strings.ToLower(typeName)+".go")
}

// baseTypeName extracts the bare type name from a receiver field (strips * prefix).
func baseTypeName(field *ast.Field) string {
	t := field.Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// constTypeName returns the explicit type name of the first spec in a const decl,
// or "" if the const is untyped.
func constTypeName(d *ast.GenDecl) string {
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok || vs.Type == nil {
			continue
		}
		if ident, ok := vs.Type.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// packageInfo holds a summary of all Go files in a directory.
type packageInfo struct {
	files   []fileInfo
	pkgName string
}

type fileInfo struct {
	path       string
	name       string
	types      []string
	funcs      []string
	recvTypes  []string
	constCount int
	varCount   int
}

func scanPackage(dir string) (*packageInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pkg := &packageInfo{pkgName: filepath.Base(dir)}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		fullPath := filepath.Join(dir, e.Name())
		fi, err := parseFileInfo(fullPath)
		if err != nil {
			continue
		}
		if pkg.pkgName == filepath.Base(dir) && fi.pkgName != "" {
			pkg.pkgName = fi.pkgName
		}
		pkg.files = append(pkg.files, fi.fileInfo)
	}
	return pkg, nil
}

type parseResult struct {
	fileInfo
	pkgName string
}

func parseFileInfo(path string) (*parseResult, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	fi := &parseResult{pkgName: f.Name.Name}
	fi.path = path
	fi.name = filepath.Base(path)
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			fi.funcs = append(fi.funcs, d.Name.Name)
			if d.Recv != nil && len(d.Recv.List) > 0 {
				fi.recvTypes = append(fi.recvTypes, recvTypeString(d.Recv.List[0]))
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch d.Tok.String() {
				case "type":
					if ts, ok := spec.(*ast.TypeSpec); ok {
						fi.types = append(fi.types, ts.Name.Name)
					}
				case "const":
					fi.constCount++
				case "var":
					fi.varCount++
				}
			}
		}
	}
	return fi, nil
}

func findFile(pkg *packageInfo, name string) string {
	for _, fi := range pkg.files {
		if fi.name == name {
			return fi.path
		}
	}
	return ""
}

func defaultFile(pkg *packageInfo, dir string) string {
	for _, fi := range pkg.files {
		if !strings.Contains(fi.name, "generated") && !strings.HasSuffix(fi.name, "_gen.go") {
			return fi.path
		}
	}
	if len(pkg.files) > 0 {
		return pkg.files[0].path
	}
	return filepath.Join(dir, pkg.pkgName+".go")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// declNamespace returns the canonical namespace for a declaration:
//   <import-path>#<DeclName>
//
// e.g. "github.com/mattdurham/grv/ops#Dog"
// Falls back to <dir>#<name> if go.mod can't be found.
func declNamespace(decl ast.Decl, dir string) string {
	name := declName(decl)
	if name == "" {
		return ""
	}
	importPath := packageImportPath(dir)
	return importPath + "#" + name
}

// declName extracts the primary identifier from a declaration.
func declName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Name.Name
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				return s.Name.Name
			case *ast.ValueSpec:
				if len(s.Names) > 0 {
					return s.Names[0].Name
				}
			case *ast.ImportSpec:
				if s.Path != nil {
					return strings.Trim(s.Path.Value, `"`)
				}
			}
		}
	}
	return ""
}

// packageImportPath finds the Go module import path for a directory by
// reading the nearest go.mod and combining module + relative path.
// PackageImportPath is exported for testing.
var PackageImportPath = packageImportPath

func packageImportPath(dir string) string {
	abs, _ := filepath.Abs(dir)
	// Walk up to find go.mod
	for d := abs; d != filepath.Dir(d); d = filepath.Dir(d) {
		gomod := filepath.Join(d, "go.mod")
		data, err := os.ReadFile(gomod)
		if err != nil {
			continue
		}
		// Extract module path from first "module " line
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				mod := strings.TrimSpace(strings.TrimPrefix(line, "module"))
				// rel path from module root to dir
				rel, _ := filepath.Rel(d, abs)
				if rel == "." || rel == "" {
					return mod
				}
				return mod + "/" + filepath.ToSlash(rel)
			}
		}
	}
	// Fallback: use the directory name
	return filepath.Base(abs)
}
