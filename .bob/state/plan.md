# Implementation Plan: Tier 3 Type-Aware LSP Tools

## Overview

Add three type-aware tools (`ast_find_refs`, `ast_find_def`, `ast_find_impls`) to the goast
MCP server using `go/packages` for full type resolution. These tools live in a single new
file `ops/types.go` sharing a `loadPackage` helper. A committed mini-module
`testdata/typesdata/` provides stable, intentional fixtures for all three tests. After
registration in `server.go`, the total tool count reaches 22.

---

## Files to Create

1. `testdata/typesdata/go.mod` — mini Go module declaration
2. `testdata/typesdata/types.go` — package typesdata with Stringer interface, Dog/Cat/Fish types, Add/Sum3 functions
3. `ops/types.go` — three tool handlers plus loadPackage, extractNameIdent, findIdentInPkg helpers
4. `ops/types_test.go` — all tests using testdata/typesdata/

## Files to Modify

1. `server.go` — register ast_find_refs, ast_find_def, ast_find_impls (19 → 22 tools)

---

## Implementation Steps

### Phase 1: Testdata Fixture (Task A)

**Step 1.1: Create `testdata/typesdata/go.mod`**

Exact content:
```
module example.com/typesdata

go 1.21
```

**Step 1.2: Create `testdata/typesdata/types.go`**

Exact content (line numbers matter — tests reference specific lines):

```go
package typesdata

// Stringer is a simple interface for types that can represent themselves as a string.
type Stringer interface {
	String() string
}

// Dog implements Stringer via a pointer receiver.
type Dog struct {
	Name string
}

func (d *Dog) String() string {
	return d.Name
}

// Cat implements Stringer via a value receiver.
type Cat struct {
	Name string
}

func (c Cat) String() string {
	return c.Name
}

// Fish does not implement Stringer (no methods).
type Fish struct{}

// Add adds two integers.
func Add(a, b int) int {
	return a + b
}

// Sum3 calls Add twice, providing call sites for find_refs tests.
func Sum3(a, b, c int) int {
	return Add(Add(a, b), c)
}
```

This provides:
- `Stringer` interface — target for ast_find_impls tests
- `Dog` (pointer receiver `*Dog`) — implements Stringer via pointer
- `Cat` (value receiver) — implements Stringer directly
- `Fish` (no methods) — must NOT appear in impls results
- `Add` function — has two call sites in Sum3, for ast_find_refs tests
- `Sum3` — the call sites for Add

---

### Phase 2: Write Tests First (TDD)

**Step 2.1: Create `ops/types_test.go`**

Package declaration: `package ops_test`

Imports:
```go
import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    "github.com/lthiery/goast/ops"
    "github.com/mark3labs/mcp-go/mcp"
)
```

Helper functions in this file:

```go
// typesdataFile returns the absolute path to testdata/typesdata/types.go.
// go test sets cwd to the package directory (ops/), so go up one level.
func typesdataFile(t *testing.T) string {
    t.Helper()
    abs, err := filepath.Abs(filepath.Join("..", "testdata", "typesdata", "types.go"))
    if err != nil {
        t.Fatalf("typesdataFile: %v", err)
    }
    return abs
}

// writeTempModule creates a temp dir with go.mod + types.go for package-scope tests.
// Required because packages.Load needs a valid go.mod. Do NOT use writeTempFile
// (from rename_test.go) for Tier 3 tests — bare .go files without go.mod fail.
func writeTempModule(t *testing.T, source string) string {
    t.Helper()
    dir := t.TempDir()
    gomod := "module example.com/typetest\n\ngo 1.21\n"
    if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(source), 0644); err != nil {
        t.Fatal(err)
    }
    return dir
}

// extractText returns the text content of a CallToolResult without fataling on IsError.
// Use this in error-case tests; use resultText (from ops_test.go) for success cases.
func extractText(t *testing.T, result *mcp.CallToolResult) string {
    t.Helper()
    if len(result.Content) == 0 {
        return ""
    }
    tc, ok := result.Content[0].(mcp.TextContent)
    if !ok {
        t.Fatalf("expected TextContent, got %T", result.Content[0])
    }
    return tc.Text
}
```

**Step 2.2: Tests for HandleASTFindRefs**

```go
func TestHandleASTFindRefs_FileScope_Add(t *testing.T) {
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})

    result, err := ops.HandleASTFindRefs(ctx, emptyReq, ops.ASTFindRefsArgs{
        File:  file,
        Path:  pathJSON,
        Scope: "file",
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.IsError {
        t.Fatalf("tool error: %v", result.Content)
    }
    var refs []ops.RefResult
    if err := json.Unmarshal([]byte(extractText(t, result)), &refs); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    // File scope (AST name-string match): Add appears in decl + 2 calls in Sum3 = >= 3
    if len(refs) < 3 {
        t.Errorf("expected >= 3 refs for Add (file scope), got %d", len(refs))
    }
    for _, r := range refs {
        if r.Line <= 0 {
            t.Errorf("ref has zero line: %+v", r)
        }
    }
}

func TestHandleASTFindRefs_PackageScope_Add(t *testing.T) {
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})

    result, err := ops.HandleASTFindRefs(ctx, emptyReq, ops.ASTFindRefsArgs{
        File:  file,
        Path:  pathJSON,
        Scope: "package",
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.IsError {
        t.Fatalf("tool error: %v", result.Content)
    }
    var refs []ops.RefResult
    if err := json.Unmarshal([]byte(extractText(t, result)), &refs); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    // Package scope (type-accurate): Add is used twice in Sum3
    useCount := 0
    for _, r := range refs {
        if !r.IsDecl {
            useCount++
        }
    }
    if useCount < 2 {
        t.Errorf("expected >= 2 use-site refs for Add (package scope), got %d (total: %d)", useCount, len(refs))
    }
}

func TestHandleASTFindRefs_DefaultScopeIsFile(t *testing.T) {
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})

    result, err := ops.HandleASTFindRefs(ctx, emptyReq, ops.ASTFindRefsArgs{
        File: file,
        Path: pathJSON,
        // Scope omitted — defaults to "file"
    })
    if err != nil || result.IsError {
        t.Fatal("default scope (empty string) should succeed and default to file")
    }
}

func TestHandleASTFindRefs_BadPath(t *testing.T) {
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Nonexistent"}})

    result, err := ops.HandleASTFindRefs(ctx, emptyReq, ops.ASTFindRefsArgs{
        File:  file,
        Path:  pathJSON,
        Scope: "file",
    })
    if err != nil {
        t.Fatalf("unexpected Go error: %v", err)
    }
    if !result.IsError {
        t.Error("expected tool error for nonexistent FuncDecl path")
    }
}
```

**Step 2.3: Tests for HandleASTFindDef**

```go
func TestHandleASTFindDef_DeclSite(t *testing.T) {
    // Navigating to the declaration itself should return is_decl=true
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Add"}})

    result, err := ops.HandleASTFindDef(ctx, emptyReq, ops.ASTFindDefArgs{
        File: file,
        Path: pathJSON,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.IsError {
        t.Fatalf("tool error: %v", result.Content)
    }
    var def ops.DefResult
    if err := json.Unmarshal([]byte(extractText(t, result)), &def); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if def.File == "" {
        t.Error("expected non-empty file in def result")
    }
    // When navigating to the declaration, is_decl should be true
    if !def.IsDecl {
        t.Error("expected is_decl=true when path points to declaration site")
    }
}

func TestHandleASTFindDef_TypeSpec(t *testing.T) {
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Dog"}})

    result, err := ops.HandleASTFindDef(ctx, emptyReq, ops.ASTFindDefArgs{
        File: file,
        Path: pathJSON,
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.IsError {
        t.Fatalf("tool error: %v", result.Content)
    }
    var def ops.DefResult
    if err := json.Unmarshal([]byte(extractText(t, result)), &def); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if def.File == "" {
        t.Error("expected non-empty file for Dog TypeSpec definition")
    }
    if def.Line <= 0 {
        t.Errorf("expected positive line number, got %d", def.Line)
    }
}

func TestHandleASTFindDef_BadPath(t *testing.T) {
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "FuncDecl", "name": "Ghost"}})

    result, err := ops.HandleASTFindDef(ctx, emptyReq, ops.ASTFindDefArgs{
        File: file,
        Path: pathJSON,
    })
    if err != nil {
        t.Fatalf("unexpected Go error: %v", err)
    }
    if !result.IsError {
        t.Error("expected tool error for nonexistent path")
    }
}
```

**Step 2.4: Tests for HandleASTFindImpls**

```go
func TestHandleASTFindImpls_Stringer(t *testing.T) {
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Stringer"}})

    result, err := ops.HandleASTFindImpls(ctx, emptyReq, ops.ASTFindImplsArgs{
        File:  file,
        Path:  pathJSON,
        Scope: "package",
    })
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.IsError {
        t.Fatalf("tool error: %v", result.Content)
    }
    var impls []ops.ImplResult
    if err := json.Unmarshal([]byte(extractText(t, result)), &impls); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    valueImpls := map[string]bool{}
    ptrImpls := map[string]bool{}
    for _, impl := range impls {
        if impl.Pointer {
            ptrImpls[impl.TypeName] = true
        } else {
            valueImpls[impl.TypeName] = true
        }
    }

    // Cat has value receiver String() → implements Stringer directly (Pointer=false)
    if !valueImpls["Cat"] {
        t.Errorf("expected Cat as value implementor; valueImpls=%v ptrImpls=%v", valueImpls, ptrImpls)
    }
    // Dog has pointer receiver *Dog.String() → only *Dog implements Stringer (Pointer=true)
    if !ptrImpls["Dog"] {
        t.Errorf("expected Dog as pointer implementor; valueImpls=%v ptrImpls=%v", valueImpls, ptrImpls)
    }
    // Fish has no methods → must NOT appear
    if valueImpls["Fish"] || ptrImpls["Fish"] {
        t.Error("Fish should not implement Stringer")
    }
    // Stringer itself must NOT appear (interface excluded from results)
    if valueImpls["Stringer"] || ptrImpls["Stringer"] {
        t.Error("Stringer interface itself should not appear in impl results")
    }
    // Line number should be populated for each result
    for _, impl := range impls {
        if impl.Line <= 0 {
            t.Errorf("impl %s has zero line", impl.TypeName)
        }
    }
}

func TestHandleASTFindImpls_NotAnInterface(t *testing.T) {
    file := typesdataFile(t)
    // Dog is a struct, not an interface — should return error
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Dog"}})

    result, err := ops.HandleASTFindImpls(ctx, emptyReq, ops.ASTFindImplsArgs{
        File:  file,
        Path:  pathJSON,
        Scope: "package",
    })
    if err != nil {
        t.Fatalf("unexpected Go error: %v", err)
    }
    if !result.IsError {
        t.Error("expected tool error when TypeSpec is not an interface")
    }
}

func TestHandleASTFindImpls_BadPath(t *testing.T) {
    file := typesdataFile(t)
    pathJSON, _ := json.Marshal([]map[string]string{{"kind": "TypeSpec", "name": "Ghost"}})

    result, err := ops.HandleASTFindImpls(ctx, emptyReq, ops.ASTFindImplsArgs{
        File:  file,
        Path:  pathJSON,
        Scope: "package",
    })
    if err != nil {
        t.Fatalf("unexpected Go error: %v", err)
    }
    if !result.IsError {
        t.Error("expected tool error for nonexistent TypeSpec")
    }
}
```

**Step 2.5: Run tests to confirm they fail**

```bash
cd /home/matt/source/goast-worktrees/goast-mcp-server
go test ./ops/ -run 'TestHandleAST(FindRefs|FindDef|FindImpls)' -v 2>&1 | head -20
```

Expected: compilation failure (ops.HandleASTFindRefs etc. do not exist). Confirms TDD gate.

---

### Phase 3: Implementation (ops/types.go)

**Step 3.1: File header and imports**

```go
// Namespace: goast/ops
// Type-aware LSP tools: ast_find_refs, ast_find_def, ast_find_impls
// These tools invoke go/packages.Load (~50ms per call). No caching.
// All three tools require the target file to be inside a valid Go module (go.mod required).
package ops

import (
    "context"
    "encoding/json"
    "fmt"
    "go/ast"
    "go/types"
    "os"
    "path/filepath"
    "strings"

    "github.com/lthiery/goast/editor"
    "github.com/lthiery/goast/selector"
    "github.com/mark3labs/mcp-go/mcp"
    "golang.org/x/tools/go/ast/astutil"
    "golang.org/x/tools/go/packages"
)
```

**Step 3.2: `loadPackage` helper**

```go
// loadPackage loads the Go package containing filePath with full type information.
// Config uses Dir=filepath.Dir(filePath) and pattern ".".
// Returns error if TypesInfo is nil (indicates load failure).
func loadPackage(filePath string) (*packages.Package, error) {
    cfg := &packages.Config{
        Mode: packages.NeedTypesInfo | packages.NeedTypes | packages.NeedSyntax |
              packages.NeedFiles | packages.NeedImports | packages.NeedName,
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
    if pkg.TypesInfo == nil {
        var msgs []string
        for _, e := range pkg.Errors {
            msgs = append(msgs, e.Msg)
        }
        return nil, fmt.Errorf("type info unavailable — ensure file is in a Go module (errors: %v)", msgs)
    }
    return pkg, nil
}
```

**Step 3.3: `extractNameIdent` helper**

```go
// extractNameIdent returns the primary name *ast.Ident from a navigated node.
// FuncDecl → funcDecl.Name; TypeSpec → typeSpec.Name; Field → first name;
// ValueSpec → first name; Ident → itself.
func extractNameIdent(node ast.Node) (*ast.Ident, bool) {
    switch n := node.(type) {
    case *ast.FuncDecl:
        return n.Name, true
    case *ast.TypeSpec:
        return n.Name, true
    case *ast.Field:
        if len(n.Names) > 0 {
            return n.Names[0], true
        }
    case *ast.ValueSpec:
        if len(n.Names) > 0 {
            return n.Names[0], true
        }
    case *ast.Ident:
        return n, true
    }
    return nil, false
}
```

**Step 3.4: `findIdentInPkg` helper**

Bridges from editor fset position (line, col) to a `*ast.Ident` in pkg.Syntax.
Uses `filepath.Base(filename)` for filename matching (handles temp dir path differences).

```go
// findIdentInPkg scans pkg.Syntax to find the *ast.Ident at (line, col) in filename.
// line and col are 1-based (matching token.Position.Line and .Column).
// Returns nil if no matching ident is found.
func findIdentInPkg(pkg *packages.Package, filename string, line, col int) *ast.Ident {
    base := filepath.Base(filename)
    var found *ast.Ident
    for _, syntax := range pkg.Syntax {
        if found != nil {
            break
        }
        astutil.Apply(syntax, func(c *astutil.Cursor) bool {
            if found != nil {
                return false
            }
            ident, ok := c.Node().(*ast.Ident)
            if !ok {
                return true
            }
            pos := pkg.Fset.Position(ident.Pos())
            if filepath.Base(pos.Filename) == base && pos.Line == line && pos.Column == col {
                found = ident
                return false
            }
            return true
        }, nil)
    }
    return found
}
```

**Step 3.5: `isExternalFile` helper**

```go
// isExternalFile returns true if filename is in the Go module cache (GOPATH/pkg/mod).
func isExternalFile(filename string) bool {
    gopath := os.Getenv("GOPATH")
    if gopath == "" {
        gopath = filepath.Join(os.Getenv("HOME"), "go")
    }
    return strings.HasPrefix(filename, filepath.Join(gopath, "pkg", "mod"))
}
```

**Step 3.6: ASTFindRefsArgs, RefResult, HandleASTFindRefs**

```go
// ASTFindRefsArgs is the argument struct for ast_find_refs.
type ASTFindRefsArgs struct {
    File  string          `json:"file"`
    Path  json.RawMessage `json:"path"`  // []selector.PathStep to declaration
    Scope string          `json:"scope"` // "file" (default) | "package"
}

// RefResult is one entry in the ast_find_refs response.
type RefResult struct {
    File   string              `json:"file"`
    Path   []selector.PathStep `json:"path"`
    Line   int                 `json:"line"`
    Col    int                 `json:"col"`
    IsDecl bool                `json:"is_decl,omitempty"`
}

// HandleASTFindRefs implements the ast_find_refs tool.
func HandleASTFindRefs(ctx context.Context, req mcp.CallToolRequest, args ASTFindRefsArgs) (*mcp.CallToolResult, error) {
    f, fset, _, err := editor.ParseFile(args.File)
    if err != nil {
        return toolError(fmt.Sprintf("parse: %v", err)), nil
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

    nameIdent, ok := extractNameIdent(node)
    if !ok {
        return toolError("path does not resolve to a named node (FuncDecl, TypeSpec, Field, ValueSpec, or Ident)"), nil
    }

    scope := args.Scope
    if scope == "" {
        scope = "file"
    }

    switch scope {
    case "file":
        return handleFindRefsFileScope(args.File, f, fset, nameIdent.Name), nil
    case "package":
        namePos := fset.Position(nameIdent.Pos())
        return handleFindRefsPkgScope(args.File, namePos.Line, namePos.Column), nil
    default:
        return toolError(fmt.Sprintf("unknown scope %q (use \"file\" or \"package\")", scope)), nil
    }
}
```

`handleFindRefsFileScope` (unexported, file-local):

```go
func handleFindRefsFileScope(filePath string, f *ast.File, fset *token.FileSet, name string) *mcp.CallToolResult {
    ancestors := collectAncestors(f) // from lsp.go
    var refs []RefResult
    astutil.Apply(f, func(c *astutil.Cursor) bool {
        ident, ok := c.Node().(*ast.Ident)
        if !ok {
            return true
        }
        if ident.Name != name {
            return true
        }
        pos := fset.Position(ident.Pos())
        p := buildPath(ident, ancestors) // from lsp.go
        isDecl := false
        // Idents in Defs (i.e., declaration sites) can be detected by checking if
        // the parent is a FuncDecl/TypeSpec/Field/ValueSpec with matching name
        // For file-scope we use a simpler heuristic: check if parent is a declaration node
        refs = append(refs, RefResult{
            File:   filePath,
            Path:   p,
            Line:   pos.Line,
            Col:    pos.Column,
            IsDecl: isDecl,
        })
        return true
    }, nil)
    b, _ := json.Marshal(refs)
    return mcp.NewToolResultText(string(b))
}
```

Note: for file scope, `IsDecl` is set to false for all refs (it's an approximation — we
don't run type checking). The declaration site is included in the results (it matches by
name string) but IsDecl is not set. This is acceptable for the file-scope approximation.
If precise IsDecl marking is needed, use package scope.

`handleFindRefsPkgScope` (unexported, file-local):

```go
func handleFindRefsPkgScope(filePath string, targetLine, targetCol int) *mcp.CallToolResult {
    pkg, err := loadPackage(filePath)
    if err != nil {
        return toolError(fmt.Sprintf("load package: %v", err))
    }

    pkgIdent := findIdentInPkg(pkg, filePath, targetLine, targetCol)
    if pkgIdent == nil {
        return toolError("identifier not found in type info — verify file is part of the loaded package")
    }

    // Get the declaration object. Check Defs first (ident IS the declaration).
    var declObj types.Object
    if obj, ok := pkg.TypesInfo.Defs[pkgIdent]; ok && obj != nil {
        declObj = obj
    } else if obj, ok := pkg.TypesInfo.Uses[pkgIdent]; ok && obj != nil {
        declObj = obj
    }
    if declObj == nil {
        return toolError("identifier has no type info object")
    }

    var refs []RefResult
    for _, syntax := range pkg.Syntax {
        ancestors := make(map[ast.Node]nodeAncestor) // local per-file
        astutil.Apply(syntax, func(c *astutil.Cursor) bool {
            n := c.Node()
            if n == nil {
                return false
            }
            ancestors[n] = nodeAncestor{
                node: n, parent: c.Parent(),
                fieldName: c.Name(), index: c.Index(),
            }
            return true
        }, nil)

        astutil.Apply(syntax, func(c *astutil.Cursor) bool {
            ident, ok := c.Node().(*ast.Ident)
            if !ok {
                return true
            }
            isDecl := pkg.TypesInfo.Defs[ident] == declObj
            isUse := pkg.TypesInfo.Uses[ident] == declObj
            if !isDecl && !isUse {
                return true
            }
            pos := pkg.Fset.Position(ident.Pos())
            p := buildPath(ident, ancestors)
            refs = append(refs, RefResult{
                File:   pos.Filename,
                Path:   p,
                Line:   pos.Line,
                Col:    pos.Column,
                IsDecl: isDecl,
            })
            return true
        }, nil)
    }

    if refs == nil {
        refs = []RefResult{}
    }
    b, _ := json.Marshal(refs)
    return mcp.NewToolResultText(string(b))
}
```

Implementation note on `handleFindRefsPkgScope`: The `collectAncestors` function from
`lsp.go` uses the editor's fset. For the package-scope walk, we need to build ancestors
using the pkg's fset (inside pkg.Syntax). The simplest approach is to duplicate the
`collectAncestors` logic inline per-file in the pkg.Syntax loop, using a local map.
Alternatively, call `collectAncestors(syntax)` — since `collectAncestors` only uses
`astutil.Apply` (no fset), it works on any `ast.Node`. Use that.

Revised: call `ancestors := collectAncestors(syntax)` directly per syntax file.

**Step 3.7: ASTFindDefArgs, DefResult, HandleASTFindDef**

```go
// ASTFindDefArgs is the argument struct for ast_find_def.
type ASTFindDefArgs struct {
    File string          `json:"file"`
    Path json.RawMessage `json:"path"` // []selector.PathStep to the identifier
}

// DefResult is the response for ast_find_def.
type DefResult struct {
    File     string              `json:"file,omitempty"`
    Path     []selector.PathStep `json:"path,omitempty"`
    Line     int                 `json:"line,omitempty"`
    Col      int                 `json:"col,omitempty"`
    Builtin  bool                `json:"builtin,omitempty"`
    External bool                `json:"external,omitempty"`
    Package  string              `json:"package,omitempty"`
    Name     string              `json:"name,omitempty"`
    IsDecl   bool                `json:"is_decl,omitempty"`
}

// HandleASTFindDef implements the ast_find_def tool.
func HandleASTFindDef(ctx context.Context, req mcp.CallToolRequest, args ASTFindDefArgs) (*mcp.CallToolResult, error) {
    f, fset, _, err := editor.ParseFile(args.File)
    if err != nil {
        return toolError(fmt.Sprintf("parse: %v", err)), nil
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

    nameIdent, ok := extractNameIdent(node)
    if !ok {
        return toolError("path does not resolve to a named node"), nil
    }

    namePos := fset.Position(nameIdent.Pos())

    pkg, err := loadPackage(args.File)
    if err != nil {
        return toolError(fmt.Sprintf("load package: %v", err)), nil
    }

    pkgIdent := findIdentInPkg(pkg, args.File, namePos.Line, namePos.Column)
    if pkgIdent == nil {
        return toolError("identifier not found in package type info — check that the file is part of the loaded package"), nil
    }

    // Check if path is the declaration site (Defs)
    if defObj, ok := pkg.TypesInfo.Defs[pkgIdent]; ok && defObj != nil {
        defPos := pkg.Fset.Position(defObj.Pos())
        result := DefResult{
            File:   defPos.Filename,
            Line:   defPos.Line,
            Col:    defPos.Column,
            IsDecl: true,
        }
        b, _ := json.Marshal(result)
        return mcp.NewToolResultText(string(b)), nil
    }

    // It's a use site — resolve to the declaration
    obj := pkg.TypesInfo.Uses[pkgIdent]
    if obj == nil {
        return toolError("identifier has no type info object (not a Def or Use)"), nil
    }

    defPos := obj.Pos()
    if !defPos.IsValid() {
        // Built-in universe object
        result := DefResult{Builtin: true, Name: obj.Name()}
        b, _ := json.Marshal(result)
        return mcp.NewToolResultText(string(b)), nil
    }

    defPosInfo := pkg.Fset.Position(defPos)
    if isExternalFile(defPosInfo.Filename) {
        pkgPath := ""
        if obj.Pkg() != nil {
            pkgPath = obj.Pkg().Path()
        }
        result := DefResult{
            External: true,
            Package:  pkgPath,
            Name:     obj.Name(),
            File:     defPosInfo.Filename,
            Line:     defPosInfo.Line,
        }
        b, _ := json.Marshal(result)
        return mcp.NewToolResultText(string(b)), nil
    }

    result := DefResult{
        File: defPosInfo.Filename,
        Line: defPosInfo.Line,
        Col:  defPosInfo.Column,
    }
    b, _ := json.Marshal(result)
    return mcp.NewToolResultText(string(b)), nil
}
```

**Step 3.8: ASTFindImplsArgs, ImplResult, HandleASTFindImpls**

```go
// ASTFindImplsArgs is the argument struct for ast_find_impls.
type ASTFindImplsArgs struct {
    File  string          `json:"file"`
    Path  json.RawMessage `json:"path"`  // []selector.PathStep to TypeSpec (interface)
    Scope string          `json:"scope"` // "package" (default; file scope not meaningful)
}

// ImplResult is one entry in the ast_find_impls response.
type ImplResult struct {
    File     string `json:"file"`
    TypeName string `json:"type_name"`
    Pointer  bool   `json:"pointer"` // true if *T implements (not T)
    Line     int    `json:"line"`
}

// HandleASTFindImpls implements the ast_find_impls tool.
func HandleASTFindImpls(ctx context.Context, req mcp.CallToolRequest, args ASTFindImplsArgs) (*mcp.CallToolResult, error) {
    f, fset, _, err := editor.ParseFile(args.File)
    if err != nil {
        return toolError(fmt.Sprintf("parse: %v", err)), nil
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

    nameIdent, ok := extractNameIdent(node)
    if !ok {
        return toolError("path does not resolve to a TypeSpec or named node"), nil
    }

    namePos := fset.Position(nameIdent.Pos())

    pkg, err := loadPackage(args.File)
    if err != nil {
        return toolError(fmt.Sprintf("load package: %v", err)), nil
    }

    pkgIdent := findIdentInPkg(pkg, args.File, namePos.Line, namePos.Column)
    if pkgIdent == nil {
        return toolError("interface name not found in package type info"), nil
    }

    declObj := pkg.TypesInfo.Defs[pkgIdent]
    if declObj == nil {
        return toolError("identifier not found in type info Defs — path must point to a type declaration"), nil
    }

    underlying := declObj.Type().Underlying()
    iface, ok := underlying.(*types.Interface)
    if !ok {
        return toolError(fmt.Sprintf("path resolves to %T, not an interface type", underlying)), nil
    }

    scope := pkg.Types.Scope()
    var results []ImplResult
    for _, name := range scope.Names() {
        obj := scope.Lookup(name)
        typeName, ok := obj.(*types.TypeName)
        if !ok || typeName.IsAlias() {
            continue
        }
        T := typeName.Type()
        // Skip interface types (prevents Stringer from appearing in Stringer impls)
        if _, isIface := T.Underlying().(*types.Interface); isIface {
            continue
        }
        valueImpl := types.Implements(T, iface)
        ptrImpl := types.Implements(types.NewPointer(T), iface)
        if !valueImpl && !ptrImpl {
            continue
        }
        defPos := pkg.Fset.Position(obj.Pos())
        pointer := !valueImpl // if only pointer implements, Pointer=true
        results = append(results, ImplResult{
            File:     defPos.Filename,
            TypeName: name,
            Pointer:  pointer,
            Line:     defPos.Line,
        })
    }

    if results == nil {
        results = []ImplResult{}
    }
    b, _ := json.Marshal(results)
    return mcp.NewToolResultText(string(b)), nil
}
```

---

### Phase 4: Registration (server.go)

**Step 4.1: Add three tool registrations to `RegisterTools`**

After the existing `ast_find` block (currently last Tier 2 tool), add:

```go
    // Tier 3 tools — type-aware LSP operations
    // These call go/packages.Load internally (~50ms per call, requires go.mod).

    // ast_find_refs
    s.AddTool(
        mcp.NewTool("ast_find_refs",
            mcp.WithDescription("Find all references to the identifier at the given selector path. scope=\"file\" uses fast AST-only name matching (approximate, no type resolution). scope=\"package\" uses go/packages for type-accurate resolution (~50ms, requires go.mod). Returns array of {file, path, line, col, is_decl}."),
            mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file containing the declaration")),
            mcp.WithArray("path", mcp.Required(), mcp.Description("Selector path to the declaration site of the identifier")),
            mcp.WithString("scope", mcp.Description("Search scope: \"file\" (default, AST-only approximation) or \"package\" (type-accurate, requires go.mod)")),
        ),
        mcp.NewTypedToolHandler(ops.HandleASTFindRefs),
    )

    // ast_find_def
    s.AddTool(
        mcp.NewTool("ast_find_def",
            mcp.WithDescription("Follow the identifier at the selector path to its declaration. Resolves local variables, function calls, type names, struct fields, imported names. Returns {file, line, col} for same-package defs; {external:true, package, name} for module cache defs; {builtin:true, name} for built-ins; {is_decl:true} if path already points to the declaration. Requires go.mod (~50ms)."),
            mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
            mcp.WithArray("path", mcp.Required(), mcp.Description("Selector path to the identifier to follow (use site or declaration site)")),
        ),
        mcp.NewTypedToolHandler(ops.HandleASTFindDef),
    )

    // ast_find_impls
    s.AddTool(
        mcp.NewTool("ast_find_impls",
            mcp.WithDescription("Find all concrete types in the package that implement the interface at the given selector path. Uses go/types for full type-system resolution. Checks both value receiver (T) and pointer receiver (*T) satisfaction. Excludes the interface type itself. Requires go.mod (~50ms). Returns array of {file, type_name, pointer, line}."),
            mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file containing the interface declaration")),
            mcp.WithArray("path", mcp.Required(), mcp.Description("Selector path to a TypeSpec whose underlying type is an interface")),
            mcp.WithString("scope", mcp.Description("Scope: \"package\" (default and only meaningful value)")),
        ),
        mcp.NewTypedToolHandler(ops.HandleASTFindImpls),
    )
```

**Step 4.2: Verify tool count**

```bash
grep -c 'mcp.NewTool(' /home/matt/source/goast-worktrees/goast-mcp-server/server.go
```

Must output: `22`

---

### Phase 5: Verification

**Step 5.1: Full test suite**

```bash
cd /home/matt/source/goast-worktrees/goast-mcp-server
go test ./...
```

All tests must pass.

**Step 5.2: Tier 3 tests with verbose output**

```bash
go test ./ops/ -run 'TestHandleAST(FindRefs|FindDef|FindImpls)' -v -count=1
```

**Step 5.3: Race detector**

```bash
go test -race ./ops/ -run 'TestHandleAST(FindRefs|FindDef|FindImpls)' -count=1
```

**Step 5.4: Build**

```bash
go build ./...
```

**Step 5.5: Format**

```bash
gofmt -w ops/types.go ops/types_test.go
```

---

## Edge Cases to Handle

### Edge Case 1: No go.mod in file's directory

**Scenario:** `loadPackage` called on a bare .go file without go.mod (e.g., a file written
by `writeTempFile` in test).

**Expected:** Return `toolError` with message "type info unavailable — ensure file is in a
Go module (go.mod required)".

**How:** Check `pkg.TypesInfo == nil` after Load; collect and report `pkg.Errors`.

### Edge Case 2: Built-in identifier (len, cap, make, etc.)

**Scenario:** `ast_find_def` path resolves to a call to `len`.

**Expected:** `DefResult{Builtin: true, Name: "len"}`.

**How:** `obj.Pos().IsValid()` returns false for universe-scope objects (built-ins).

### Edge Case 3: Interface implementing itself excluded from find_impls

**Scenario:** `types.Implements(Stringer, StringerIface)` returns true — the interface
satisfies itself.

**Expected:** Stringer must NOT appear in results.

**How:** Skip any named type whose `T.Underlying()` is `*types.Interface`.

### Edge Case 4: findIdentInPkg returns nil

**Scenario:** Column mismatch or the ident is in a different file than expected.

**Expected:** `toolError("identifier not found in package type info")`.

**How:** Nil check on `findIdentInPkg` result; return descriptive error.

### Edge Case 5: External module cache symbol

**Scenario:** `ast_find_def` on a call to `fmt.Println` — resolves to `$GOPATH/pkg/mod/...`.

**Expected:** `DefResult{External: true, Package: "fmt", Name: "Println", File: "/..."}`.

**How:** `isExternalFile(filename)` checks GOPATH/pkg/mod prefix.

### Edge Case 6: Declaration site in find_def

**Scenario:** Path navigates to `FuncDecl("Add")` — the declaration itself.

**Expected:** `DefResult{File: ..., Line: ..., IsDecl: true}` — not an error.

**How:** Check `TypesInfo.Defs[pkgIdent]` before `TypesInfo.Uses[pkgIdent]`.

### Edge Case 7: Pointer vs value receiver for find_impls

**Scenario:** Dog has `*Dog` receiver; Cat has value receiver.

**Expected:** Dog → `{pointer: true}`, Cat → `{pointer: false}`.

**How:** Check value impl first. If `types.Implements(T, iface)` is true → `Pointer=false`.
If false but `types.Implements(types.NewPointer(T), iface)` is true → `Pointer=true`.

---

## Risks and Concerns

### Risk 1: findIdentInPkg column mismatch

**Risk:** Non-ASCII identifiers or unusual source encoding could cause byte-offset column
disagreement between editor fset and pkg.Fset.

**Mitigation:** Use `filepath.Base(filename)` (not full path) for matching. Document as
known limitation in tool description. Standard Go ASCII identifiers (the common case) are
unaffected.

### Risk 2: packages.Load failure in CI

**Risk:** Restricted CI environments without `go` on PATH.

**Mitigation:** `testdata/typesdata/go.mod` has no external dependencies. Requires only
local Go toolchain. The existing test suite already depends on `golang.org/x/tools` (in
go.mod), so the toolchain is available.

### Risk 3: Duplicate type names with existing ops types

**Risk:** `RefResult`, `DefResult`, `ImplResult` might collide with names in other ops files.

**Mitigation:** Grep before implementation: `grep -r 'type.*Result' ops/`. Currently: only
`SymbolResult` and `FindResult` exist. New names are distinct.

### Risk 4: handleFindRefsFileScope needs token.FileSet

**Risk:** `handleFindRefsFileScope` needs fset to get line/col from ident positions, but the
function signature above doesn't include it.

**Mitigation:** Pass `fset *token.FileSet` as a parameter to `handleFindRefsFileScope`. Add
`"go/token"` to imports in `ops/types.go`.

---

## Dependencies

### Internal

- `github.com/lthiery/goast/editor` — `editor.ParseFile`
- `github.com/lthiery/goast/selector` — `selector.Navigate`, `selector.PathStep`, `selector.NavigateError`
- `ops` package internals — `toolError`, `navError`, `buildPath`, `collectAncestors`, `nodeAncestor`

### External (already in go.mod)

- `golang.org/x/tools v0.45.0` — `go/packages`, `golang.org/x/tools/go/ast/astutil`
- `github.com/mark3labs/mcp-go v0.54.0` — MCP handler types

### No new dependencies required.

---

## Success Criteria

- [ ] `testdata/typesdata/go.mod` created
- [ ] `testdata/typesdata/types.go` created with exact content
- [ ] `ops/types_test.go` written; all 11 test functions present; tests fail before implementation
- [ ] `ops/types.go` created with `loadPackage`, `extractNameIdent`, `findIdentInPkg`, `isExternalFile`, `handleFindRefsFileScope`, `handleFindRefsPkgScope`, `HandleASTFindRefs`, `HandleASTFindDef`, `HandleASTFindImpls`
- [ ] All new tests pass
- [ ] All pre-existing tests still pass: `go test ./...`
- [ ] Race detector clean
- [ ] Three tools registered in `server.go`; tool count = 22
- [ ] `go build ./...` succeeds
- [ ] `gofmt` clean
- [ ] Stringer NOT in ast_find_impls results (test: TestHandleASTFindImpls_Stringer)
- [ ] Dog appears as pointer implementor (Pointer=true), Cat as value implementor (Pointer=false)
- [ ] ast_find_def returns is_decl=true when path is the declaration site
- [ ] No functions exceed cyclomatic complexity 40
