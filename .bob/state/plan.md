# Implementation Plan: goast Tier 2

## Overview

Add four new MCP tools to the goast server: `ast_rename` (single-file identifier rename via
AST walk), `ast_node_at` (line+col to structural path), `ast_find_symbols` (cross-file
declaration search with glob), and `ast_find` (structural pattern matching with wildcard
fields). These land in two new files (`ops/rename.go`, `ops/lsp.go`) plus additions to
`server.go`. All four tools build on existing packages: `editor`, `selector`, `kinds`,
`meta`, and `golang.org/x/tools/go/ast/astutil` (already in go.mod v0.45.0).

---

## Files to Create

1. `ops/rename.go` — `HandleASTRename`: navigate to declaration, extract old name, walk AST
   with astutil.Apply renaming all matching Idents.
2. `ops/rename_test.go` — TDD tests for HandleASTRename.
3. `ops/lsp.go` — `HandleASTNodeAt`, `HandleASTFindSymbols`, `HandleASTFind`, plus shared
   helpers: `collectAncestors`, `buildPath`, `nodeToPathStep`, `matchPattern`.
4. `ops/lsp_test.go` — TDD tests for all three lsp.go handlers.

## Files to Modify

1. `server.go` — register four new tools: `ast_rename`, `ast_node_at`, `ast_find_symbols`,
   `ast_find`.

---

## Testdata Prerequisite

Before writing tests, check existing testdata files:
```
ls /home/matt/source/goast-worktrees/goast-mcp-server/testdata/
```
Tests will use `os.CreateTemp` + `t.TempDir()` for isolated fixtures rather than shared
testdata files. This keeps tests hermetic and avoids cross-test coupling.

---

## Implementation Steps

### Phase 1: Tests First (TDD)

#### Step 1.1: Create ops/rename_test.go

File: `ops/rename_test.go`
Package: `package ops`

Check existing test files (ops/replace_test.go or similar) for the writeTempFile helper
pattern. All rename tests use `writeTempFile` that creates a temp .go file in t.TempDir().

**Helper:**
```go
func writeTempFile(t *testing.T, content string) string {
    t.Helper()
    f, err := os.CreateTemp(t.TempDir(), "*.go")
    if err != nil { t.Fatal(err) }
    if _, err := f.WriteString(content); err != nil { t.Fatal(err) }
    if err := f.Close(); err != nil { t.Fatal(err) }
    return f.Name()
}
```
(Check if this already exists in another ops test file; if so, reuse.)

**Test cases:**

```
TestHandleASTRename_Function
  fixture:
    package p
    func Add(x, y int) int { return Add(x, y) }
  args: Path=[{kind:"FuncDecl",name:"Add"}], To="Sum", DryRun=false
  assert: file on disk contains "Sum", does not contain "Add"
  assert: result JSON "changed" == true
  assert: result JSON "diff" is non-empty

TestHandleASTRename_DryRun
  fixture: same as above
  args: DryRun=true
  assert: result "diff" is non-empty (contains "Sum")
  assert: file on disk UNCHANGED (still contains "Add")

TestHandleASTRename_Type
  fixture:
    package p
    type Dog struct{ Name string }
    func NewDog() Dog { return Dog{} }
  args: Path=[{kind:"TypeSpec",name:"Dog"}], To="Animal"
  assert: file updated — contains "Animal", does not contain "Dog"

TestHandleASTRename_Method
  fixture:
    package p
    type Dog struct{}
    func (d *Dog) Greet() {}
    func Use() { d := &Dog{}; d.Greet() }
  args: Path=[{kind:"FuncDecl",name:"Greet",recv:"*Dog"}], To="Hello"
  assert: file contains "Hello", does not contain "Greet"

TestHandleASTRename_BadPath
  fixture: any valid Go file (e.g., "package p\nfunc Foo() {}")
  args: Path=[{kind:"FuncDecl",name:"NonExistent"}], To="Bar"
  assert: result IsError==true OR result text contains "not found"

TestHandleASTRename_EmptyTo
  fixture: same as above
  args: Path=[{kind:"FuncDecl",name:"Foo"}], To=""
  assert: result IsError==true OR result text contains "empty"
  assert: file on disk UNCHANGED
```

**Step 1.2: Verify rename tests fail**
- [ ] `cd /home/matt/source/goast-worktrees/goast-mcp-server && go test ./ops/ -run TestHandleASTRename`
- [ ] Confirm: "undefined: HandleASTRename" or compile error
- [ ] This proves tests are real

#### Step 1.3: Create ops/lsp_test.go

File: `ops/lsp_test.go`
Package: `package ops`

**Inline fixtures** (use writeTempFile from above):

Fixture A (basic.go — functions with control flow):
```go
package p

func Add(x, y int) int {
	if x > 0 {
		return x + y
	}
	return y
}

func subtract(x, y int) int {
	return x - y
}
```

Fixture B (types.go — struct and method):
```go
package p

type Dog struct {
	Name string
}

func (d *Dog) Greet() string {
	return d.Name
}
```

Fixture C (calls.go — fmt.Println calls):
```go
package p

import "fmt"

func hello() {
	fmt.Println("hi")
	if true {
		fmt.Println("nested")
	}
}
```

**HandleASTNodeAt test cases:**

```
TestHandleASTNodeAt_FuncDecl
  fixture A
  line=3, col=6   (the "A" in "Add" on the func declaration line)
  assert: result has "path" containing a step with kind=="FuncDecl" AND name=="Add"
  assert: result "node" JSON has kind=="FuncDecl"

TestHandleASTNodeAt_IfStmt
  fixture A
  line=4, col=2   (the "if" keyword)
  assert: result "path" contains a step with kind=="IfStmt"

TestHandleASTNodeAt_ReturnsMeta
  fixture A
  line=3, col=1
  assert: result JSON has "meta" object with "line" key > 0

TestHandleASTNodeAt_OutOfRange
  fixture A
  line=9999, col=1
  assert: result IsError==true OR result text contains "out of range" OR "line"

TestHandleASTNodeAt_ColZero
  fixture A
  line=1, col=0   (col is 1-based; 0 is invalid)
  assert: result IsError==true
```

**HandleASTFindSymbols test cases:**

```
TestHandleASTFindSymbols_ExactName
  tmpDir with file containing fixture A
  query="Add", kinds=nil
  assert: results has one entry with name=="Add", kind=="FuncDecl"

TestHandleASTFindSymbols_GlobAll
  tmpDir with file containing fixture A
  query="*"
  assert: results contains entries for "Add" and "subtract"

TestHandleASTFindSymbols_GlobPrefix
  tmpDir with file containing fixture A
  query="A*"
  assert: "Add" in results, "subtract" NOT in results

TestHandleASTFindSymbols_KindFilter
  tmpDir with one file containing fixture A AND fixture B merged
  query="*", kinds=["TypeSpec"]
  assert: "Dog" in results
  assert: "Add", "subtract", "Greet" NOT in results

TestHandleASTFindSymbols_CaseInsensitive
  tmpDir with file containing fixture B
  query="dog"
  assert: results contains entry with name=="Dog"

TestHandleASTFindSymbols_RecvField
  tmpDir with file containing fixture B
  query="Greet"
  assert: result entry has recv=="*Dog"

TestHandleASTFindSymbols_NoMatch
  tmpDir with file containing fixture A
  query="ZZZNonExistent"
  assert: results is empty array (not error)

TestHandleASTFindSymbols_MultiFile
  tmpDir with two separate files (fixture A and fixture B)
  query="*"
  assert: results contains symbols from BOTH files
  assert: each result has correct "file" field
```

**HandleASTFind test cases:**

```
TestHandleASTFind_IfStmt
  fixture A (has one IfStmt)
  pattern={"kind":"IfStmt"}
  assert: len(results) >= 1
  assert: results[0].node JSON kind=="IfStmt"

TestHandleASTFind_CallExprPrintln
  fixture C (has two fmt.Println calls)
  pattern={"kind":"CallExpr","fun":{"kind":"SelectorExpr","sel":"Println"}}
  assert: len(results) == 2

TestHandleASTFind_BinaryExprWithOp
  fixture A (has x > 0 and x + y)
  pattern={"kind":"BinaryExpr","op":">"}
  assert: len(results) >= 1
  assert: all results have op==">" in node JSON

TestHandleASTFind_Wildcard
  fixture A
  pattern={"kind":"BinaryExpr"}   (no op, x, y = all wildcard)
  assert: len(results) >= 2 (x>0 and x+y)

TestHandleASTFind_NoMatch
  fixture A
  pattern={"kind":"SelectStmt"}
  assert: results is empty array

TestHandleASTFind_ScopeDir
  tmpDir with fixture A (has IfStmt) and fixture B (no IfStmt)
  args: dir=tmpDir, pattern={"kind":"IfStmt"}
  assert: len(results) >= 1
  assert: all result "file" fields point to the fixture A file, not fixture B
  (fixture B has no IfStmt so only A contributes matches)

TestHandleASTFind_ResultHasPath
  fixture A, pattern={"kind":"IfStmt"}
  assert: results[0].path is non-empty
  assert: path contains a step with kind=="FuncDecl" (the if is inside Add)

TestHandleASTFind_ResultHasMeta
  fixture A, pattern={"kind":"ReturnStmt"}
  assert: len(results) >= 1
  assert: results[0].meta has "line" > 0
```

**Step 1.4: Verify lsp tests fail**
- [ ] `go test ./ops/ -run "TestHandleASTNodeAt|TestHandleASTFindSymbols|TestHandleASTFind"`
- [ ] Confirm compile error: "undefined: HandleASTNodeAt" etc.

---

### Phase 2: Implement ops/rename.go

**File header:**
```go
// Namespace: goast/ops
// Write tool: ast_rename — single-file identifier rename (AST approximation)
package ops
```

**Step 2.1: ASTRenameArgs**
```go
type ASTRenameArgs struct {
    File   string          `json:"file"`
    Path   json.RawMessage `json:"path"`   // []selector.PathStep — declaration site
    To     string          `json:"to"`
    DryRun bool            `json:"dry_run"`
}
```

**Step 2.2: HandleASTRename**
```go
func HandleASTRename(ctx context.Context, req mcp.CallToolRequest, args ASTRenameArgs) (*mcp.CallToolResult, error) {
    if args.To == "" {
        return toolError("to: new name cannot be empty"), nil
    }

    var steps []selector.PathStep
    if err := json.Unmarshal(args.Path, &steps); err != nil {
        return toolError(fmt.Sprintf("parse path: %v", err)), nil
    }

    result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, fset *token.FileSet) error {
        node, _, navErr := selector.Navigate(f, steps)
        if navErr != nil {
            return navErr
        }
        oldName, extractErr := extractDeclName(node)
        if extractErr != nil {
            return extractErr
        }
        astutil.Apply(f, func(c *astutil.Cursor) bool {
            if ident, ok := c.Node().(*ast.Ident); ok && ident.Name == oldName {
                c.Replace(&ast.Ident{Name: args.To})
            }
            return true
        }, nil)
        return nil
    })
    if err != nil {
        if ne, ok := err.(*selector.NavigateError); ok {
            return navError(ne), nil
        }
        return toolError(err.Error()), nil
    }

    resp := map[string]interface{}{"changed": result.Changed, "diff": result.Diff}
    b, _ := json.Marshal(resp)
    return mcp.NewToolResultText(string(b)), nil
}
```

**Step 2.3: extractDeclName**
```go
func extractDeclName(node ast.Node) (string, error) {
    switch n := node.(type) {
    case *ast.FuncDecl:
        return n.Name.Name, nil
    case *ast.TypeSpec:
        return n.Name.Name, nil
    case *ast.ValueSpec:
        if len(n.Names) == 0 {
            return "", fmt.Errorf("ValueSpec has no names")
        }
        return n.Names[0].Name, nil
    case *ast.Field:
        if len(n.Names) == 0 {
            return "", fmt.Errorf("Field has no names (embedded field)")
        }
        return n.Names[0].Name, nil
    case *ast.Ident:
        return n.Name, nil
    default:
        return "", fmt.Errorf("unsupported node type %T for rename", node)
    }
}
```

**Imports for rename.go:**
```go
import (
    "context"
    "encoding/json"
    "fmt"
    "go/ast"
    "go/token"

    "github.com/lthiery/goast/editor"
    "github.com/lthiery/goast/selector"
    "github.com/mark3labs/mcp-go/mcp"
    "golang.org/x/tools/go/ast/astutil"
)
```

**Step 2.4: Run rename tests — all should pass**
- [ ] `go test ./ops/ -run TestHandleASTRename -v`

---

### Phase 3: Implement ops/lsp.go

**File header:**
```go
// Namespace: goast/ops
// LSP-style tools: ast_node_at, ast_find_symbols, ast_find
package ops
```

**Imports:**
```go
import (
    "context"
    "encoding/json"
    "fmt"
    "go/ast"
    "go/token"
    "os"
    "path"
    "path/filepath"
    "strings"

    "github.com/lthiery/goast/editor"
    "github.com/lthiery/goast/kinds"
    "github.com/lthiery/goast/meta"
    "github.com/lthiery/goast/selector"
    "github.com/mark3labs/mcp-go/mcp"
    "golang.org/x/tools/go/ast/astutil"
)
```

#### Step 3.1: Shared infrastructure (implement first, before handlers)

**nodeAncestor struct:**
```go
type nodeAncestor struct {
    node      ast.Node
    parent    ast.Node
    fieldName string
    index     int // -1 for scalar fields
}
```

**collectAncestors:**
```go
func collectAncestors(f *ast.File) map[ast.Node]nodeAncestor {
    result := make(map[ast.Node]nodeAncestor)
    astutil.Apply(f, func(c *astutil.Cursor) bool {
        n := c.Node()
        if n == nil {
            return false
        }
        result[n] = nodeAncestor{
            node:      n,
            parent:    c.Parent(),
            fieldName: c.Name(),
            index:     c.Index(),
        }
        return true
    }, nil)
    return result
}
```

**buildPath:**
```go
func buildPath(target ast.Node, ancestors map[ast.Node]nodeAncestor) []selector.PathStep {
    var steps []selector.PathStep
    current := target
    for {
        anc, ok := ancestors[current]
        if !ok {
            break
        }
        if _, isFile := anc.parent.(*ast.File); isFile || anc.parent == nil {
            // Add this node's step relative to File, then stop.
            step, ok := nodeToPathStep(current, anc.parent, anc.fieldName, anc.index, ancestors)
            if ok {
                steps = append(steps, step)
            }
            break
        }
        step, ok := nodeToPathStep(current, anc.parent, anc.fieldName, anc.index, ancestors)
        if ok {
            steps = append(steps, step)
        }
        current = anc.parent
    }
    // Reverse: collected from target→root, need root→target.
    for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
        steps[i], steps[j] = steps[j], steps[i]
    }
    return steps
}
```

**nodeToPathStep** — maps (node, parent, fieldName, index) to a PathStep in selector vocabulary.
Returns (step, true) if representable, (zero, false) otherwise.

This is the most complex helper (~120-150 lines). Key cases:

```go
func nodeToPathStep(node ast.Node, parent ast.Node, fieldName string, sliceIndex int, ancestors map[ast.Node]nodeAncestor) (selector.PathStep, bool) {
    switch n := node.(type) {
    case *ast.FuncDecl:
        step := selector.PathStep{Kind: "FuncDecl", Name: n.Name.Name}
        if n.Recv != nil && len(n.Recv.List) > 0 {
            step.Recv = recvTypeString(n.Recv.List[0])
        }
        return step, true

    case *ast.TypeSpec:
        return selector.PathStep{Kind: "TypeSpec", Name: n.Name.Name}, true

    case *ast.GenDecl:
        switch n.Tok.String() {
        case "type":   return selector.PathStep{Kind: "TypeDecl"}, true
        case "var":    return selector.PathStep{Kind: "VarDecl"}, true
        case "const":  return selector.PathStep{Kind: "ConstDecl"}, true
        case "import": return selector.PathStep{Kind: "ImportDecl"}, true
        }

    case *ast.BlockStmt:
        if fieldName == "Body" {
            return selector.PathStep{Kind: "Body"}, true
        }

    case *ast.FieldList:
        switch fieldName {
        case "Params":  return selector.PathStep{Kind: "Params"}, true
        case "Results": return selector.PathStep{Kind: "Results"}, true
        }

    case *ast.StructType:
        return selector.PathStep{Kind: "StructType"}, true

    case *ast.InterfaceType:
        return selector.PathStep{Kind: "InterfaceType"}, true

    case *ast.Field:
        idx := sliceIndex
        return selector.PathStep{Kind: "Field", Index: &idx}, true
    }

    // Scalar field steps (fieldName determines the step kind).
    switch fieldName {
    case "Cond":   return selector.PathStep{Kind: "Cond"}, true
    case "Init":   return selector.PathStep{Kind: "Init"}, true
    case "Post":   return selector.PathStep{Kind: "Post"}, true
    case "Else":   return selector.PathStep{Kind: "Else"}, true
    case "Tag":    return selector.PathStep{Kind: "Tag"}, true
    case "X":      return selector.PathStep{Kind: "X"}, true
    case "Y":      return selector.PathStep{Kind: "Y"}, true
    case "Fun":    return selector.PathStep{Kind: "Fun"}, true
    case "Sel":    return selector.PathStep{Kind: "Sel"}, true
    case "Key":    return selector.PathStep{Kind: "Key"}, true
    case "Value":  return selector.PathStep{Kind: "Value"}, true
    }

    // Indexed stmt slice steps: IfStmt[N], ForStmt[N], etc.
    // The index N is the nth occurrence of that kind in the parent block.
    if fieldName == "List" || fieldName == "Body" {
        kindName := stmtKindName(node)
        if kindName != "" {
            nth := nthIndexOfKind(node, parent, sliceIndex)
            return selector.PathStep{Kind: kindName, Index: &nth}, true
        }
    }

    // Args[N], Elts[N], Lhs[N], Rhs[N] — indexed expr slices
    switch fieldName {
    case "Args":
        return selector.PathStep{Kind: "Args", Index: &sliceIndex}, true
    case "Elts":
        return selector.PathStep{Kind: "Elts", Index: &sliceIndex}, true
    case "Lhs":
        return selector.PathStep{Kind: "Lhs", Index: &sliceIndex}, true
    case "Rhs":
        return selector.PathStep{Kind: "Rhs", Index: &sliceIndex}, true
    }

    return selector.PathStep{}, false
}
```

**stmtKindName** — maps ast.Node type to its selector Kind string:
```go
func stmtKindName(n ast.Node) string {
    switch n.(type) {
    case *ast.IfStmt:         return "IfStmt"
    case *ast.ForStmt:        return "ForStmt"
    case *ast.RangeStmt:      return "RangeStmt"
    case *ast.SwitchStmt:     return "SwitchStmt"
    case *ast.TypeSwitchStmt: return "TypeSwitchStmt"
    case *ast.SelectStmt:     return "SelectStmt"
    case *ast.AssignStmt:     return "AssignStmt"
    case *ast.ReturnStmt:     return "ReturnStmt"
    case *ast.ExprStmt:       return "ExprStmt"
    case *ast.GoStmt:         return "GoStmt"
    case *ast.DeferStmt:      return "DeferStmt"
    case *ast.CaseClause:     return "CaseClause"
    case *ast.CommClause:     return "CommClause"
    }
    return ""
}
```

**nthIndexOfKind** — count how many same-kind siblings appear before sliceIndex in parent:
```go
func nthIndexOfKind(node ast.Node, parent ast.Node, sliceIndex int) int {
    list := stmtListOf(parent)
    if list == nil {
        return 0
    }
    targetType := fmt.Sprintf("%T", node)
    count := 0
    for i := 0; i < sliceIndex && i < len(list); i++ {
        if fmt.Sprintf("%T", list[i]) == targetType {
            count++
        }
    }
    return count
}

func stmtListOf(n ast.Node) []ast.Stmt {
    switch v := n.(type) {
    case *ast.BlockStmt:  return v.List
    case *ast.CaseClause: return v.Body
    case *ast.CommClause: return v.Body
    }
    return nil
}
```

#### Step 3.2: HandleASTNodeAt

```go
type ASTNodeAtArgs struct {
    File string `json:"file"`
    Line int    `json:"line"` // 1-based
    Col  int    `json:"col"`  // 1-based
}

type ASTNodeAtResponse struct {
    Path   []selector.PathStep `json:"path"`
    Node   json.RawMessage     `json:"node"`
    Source string              `json:"source,omitempty"`
    Meta   meta.Meta           `json:"meta,omitempty"`
}

func HandleASTNodeAt(ctx context.Context, req mcp.CallToolRequest, args ASTNodeAtArgs) (*mcp.CallToolResult, error) {
    f, fset, src, err := editor.ParseFile(args.File)
    if err != nil {
        return toolError(fmt.Sprintf("parse: %v", err)), nil
    }

    tokenFile := fset.File(f.Pos())
    if args.Line < 1 || args.Line > tokenFile.LineCount() {
        return toolError(fmt.Sprintf("line %d out of range (file has %d lines)", args.Line, tokenFile.LineCount())), nil
    }
    if args.Col < 1 {
        return toolError(fmt.Sprintf("col %d out of range (must be >= 1)", args.Col)), nil
    }

    lineStart := tokenFile.LineStart(args.Line)
    lineStartOffset := fset.Position(lineStart).Offset
    targetOffset := lineStartOffset + (args.Col - 1)
    if targetOffset >= len(src) {
        return toolError(fmt.Sprintf("col %d out of range for line %d", args.Col, args.Line)), nil
    }

    ancestors := collectAncestors(f)

    var best ast.Node
    bestSpan := -1
    for node := range ancestors {
        pos := fset.Position(node.Pos())
        end := fset.Position(node.End())
        if !pos.IsValid() || !end.IsValid() {
            continue
        }
        if pos.Offset <= targetOffset && targetOffset < end.Offset {
            span := end.Offset - pos.Offset
            if bestSpan < 0 || span < bestSpan {
                best = node
                bestSpan = span
            }
        }
    }
    if best == nil {
        return toolError(fmt.Sprintf("no node found at line %d col %d", args.Line, args.Col)), nil
    }

    nodePath := buildPath(best, ancestors)

    nodeJSON, err := kinds.MarshalNode(best)
    if err != nil {
        return toolError(fmt.Sprintf("marshal node: %v", err)), nil
    }

    var sourceFrag string
    pos := fset.Position(best.Pos())
    end := fset.Position(best.End())
    if pos.IsValid() && end.IsValid() && end.Offset <= len(src) {
        sourceFrag = string(src[pos.Offset:end.Offset])
    }

    m := meta.Compute(fset, src, best, nil, len(nodePath))
    resp := ASTNodeAtResponse{Path: nodePath, Node: nodeJSON, Source: sourceFrag, Meta: m}
    b, _ := json.Marshal(resp)
    return mcp.NewToolResultText(string(b)), nil
}
```

#### Step 3.3: HandleASTFindSymbols

```go
type ASTFindSymbolsArgs struct {
    Dir   string   `json:"dir"`
    Query string   `json:"query"`
    Kinds []string `json:"kinds,omitempty"`
}

type SymbolResult struct {
    File string              `json:"file"`
    Path []selector.PathStep `json:"path"`
    Kind string              `json:"kind"`
    Name string              `json:"name"`
    Recv string              `json:"recv,omitempty"`
    Line int                 `json:"line"`
    Meta meta.Meta           `json:"meta,omitempty"`
}

func HandleASTFindSymbols(ctx context.Context, req mcp.CallToolRequest, args ASTFindSymbolsArgs) (*mcp.CallToolResult, error) {
    if args.Dir == "" {
        return toolError("dir is required"), nil
    }
    if args.Query == "" {
        args.Query = "*"
    }

    entries, err := os.ReadDir(args.Dir)
    if err != nil {
        return toolError(fmt.Sprintf("readdir: %v", err)), nil
    }

    kindsSet := make(map[string]bool, len(args.Kinds))
    for _, k := range args.Kinds {
        kindsSet[k] = true
    }

    var results []SymbolResult
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
            continue
        }
        filePath := filepath.Join(args.Dir, entry.Name())
        f, fset, src, parseErr := editor.ParseFile(filePath)
        if parseErr != nil {
            continue
        }
        results = append(results, scanSymbols(f, fset, src, filePath, args.Query, kindsSet)...)
    }

    if results == nil {
        results = []SymbolResult{}
    }
    b, _ := json.Marshal(results)
    return mcp.NewToolResultText(string(b)), nil
}
```

**scanSymbols helper:**
```go
func scanSymbols(f *ast.File, fset *token.FileSet, src []byte, filePath, query string, kindsFilter map[string]bool) []SymbolResult {
    queryLower := strings.ToLower(query)
    var results []SymbolResult

    matchName := func(name string) bool {
        ok, _ := path.Match(queryLower, strings.ToLower(name))
        return ok
    }
    kindAllowed := func(k string) bool {
        return len(kindsFilter) == 0 || kindsFilter[k]
    }

    for _, decl := range f.Decls {
        switch d := decl.(type) {
        case *ast.FuncDecl:
            if !kindAllowed("FuncDecl") || !matchName(d.Name.Name) {
                continue
            }
            step := selector.PathStep{Kind: "FuncDecl", Name: d.Name.Name}
            recv := ""
            if d.Recv != nil && len(d.Recv.List) > 0 {
                recv = recvTypeString(d.Recv.List[0])
                step.Recv = recv
            }
            results = append(results, SymbolResult{
                File: filePath,
                Path: []selector.PathStep{step},
                Kind: "FuncDecl",
                Name: d.Name.Name,
                Recv: recv,
                Line: fset.Position(d.Pos()).Line,
                Meta: meta.Compute(fset, src, d, nil, 1),
            })

        case *ast.GenDecl:
            for _, spec := range d.Specs {
                switch s := spec.(type) {
                case *ast.TypeSpec:
                    if !kindAllowed("TypeSpec") || !matchName(s.Name.Name) {
                        continue
                    }
                    results = append(results, SymbolResult{
                        File: filePath,
                        Path: []selector.PathStep{{Kind: "TypeSpec", Name: s.Name.Name}},
                        Kind: "TypeSpec",
                        Name: s.Name.Name,
                        Line: fset.Position(s.Pos()).Line,
                        Meta: meta.Compute(fset, src, s, nil, 1),
                    })
                case *ast.ValueSpec:
                    specKind := "VarSpec"
                    if d.Tok.String() == "const" {
                        specKind = "ConstSpec"
                    }
                    if !kindAllowed(specKind) {
                        continue
                    }
                    for _, nameIdent := range s.Names {
                        if !matchName(nameIdent.Name) {
                            continue
                        }
                        results = append(results, SymbolResult{
                            File: filePath,
                            Path: []selector.PathStep{{Kind: specKind, Name: nameIdent.Name}},
                            Kind: specKind,
                            Name: nameIdent.Name,
                            Line: fset.Position(s.Pos()).Line,
                        })
                    }
                }
            }
        }
    }
    return results
}
```

#### Step 3.4: HandleASTFind

```go
type ASTFindArgs struct {
    File    string          `json:"file,omitempty"`
    Dir     string          `json:"dir,omitempty"`
    Pattern json.RawMessage `json:"pattern"`
}

type FindResult struct {
    File   string              `json:"file"`
    Path   []selector.PathStep `json:"path"`
    Node   json.RawMessage     `json:"node"`
    Source string              `json:"source,omitempty"`
    Meta   meta.Meta           `json:"meta,omitempty"`
}

func HandleASTFind(ctx context.Context, req mcp.CallToolRequest, args ASTFindArgs) (*mcp.CallToolResult, error) {
    if args.File == "" && args.Dir == "" {
        return toolError("file or dir is required"), nil
    }
    if len(args.Pattern) == 0 || string(args.Pattern) == "null" {
        return toolError("pattern is required"), nil
    }

    var patternMap map[string]json.RawMessage
    if err := json.Unmarshal(args.Pattern, &patternMap); err != nil {
        return toolError(fmt.Sprintf("parse pattern: %v", err)), nil
    }

    var files []string
    if args.File != "" {
        files = []string{args.File}
    } else {
        entries, err := os.ReadDir(args.Dir)
        if err != nil {
            return toolError(fmt.Sprintf("readdir: %v", err)), nil
        }
        for _, e := range entries {
            if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
                files = append(files, filepath.Join(args.Dir, e.Name()))
            }
        }
    }

    var allResults []FindResult
    for _, filePath := range files {
        f, fset, src, err := editor.ParseFile(filePath)
        if err != nil {
            continue
        }
        allResults = append(allResults, findInFile(f, fset, src, filePath, patternMap)...)
    }

    if allResults == nil {
        allResults = []FindResult{}
    }
    b, _ := json.Marshal(allResults)
    return mcp.NewToolResultText(string(b)), nil
}
```

**findInFile helper:**
```go
func findInFile(f *ast.File, fset *token.FileSet, src []byte, filePath string, patternMap map[string]json.RawMessage) []FindResult {
    ancestors := collectAncestors(f)
    var results []FindResult

    astutil.Apply(f, func(c *astutil.Cursor) bool {
        n := c.Node()
        if n == nil {
            return false
        }
        nodeJSON, err := kinds.MarshalNode(n)
        if err != nil {
            return true
        }
        var actualMap map[string]json.RawMessage
        if err := json.Unmarshal(nodeJSON, &actualMap); err != nil {
            return true
        }
        if !matchPattern(patternMap, actualMap) {
            return true
        }
        nodePath := buildPath(n, ancestors)
        var sourceFrag string
        pos := fset.Position(n.Pos())
        end := fset.Position(n.End())
        if pos.IsValid() && end.IsValid() && end.Offset <= len(src) {
            sourceFrag = string(src[pos.Offset:end.Offset])
        }
        m := meta.Compute(fset, src, n, nil, len(nodePath))
        results = append(results, FindResult{
            File:   filePath,
            Path:   nodePath,
            Node:   nodeJSON,
            Source: sourceFrag,
            Meta:   m,
        })
        return true
    }, nil)

    return results
}
```

**matchPattern helper:**
```go
// matchPattern returns true if all non-null fields in patternMap match actualMap.
// Absent or null pattern fields are wildcards.
// Array fields require exact-length match.
func matchPattern(patternMap, actualMap map[string]json.RawMessage) bool {
    for k, pv := range patternMap {
        // Null or absent = wildcard.
        if len(pv) == 0 || string(pv) == "null" {
            continue
        }
        av, ok := actualMap[k]
        if !ok {
            return false
        }
        // Both objects: recurse.
        if len(pv) > 0 && pv[0] == '{' && len(av) > 0 && av[0] == '{' {
            var pm, am map[string]json.RawMessage
            if json.Unmarshal(pv, &pm) == nil && json.Unmarshal(av, &am) == nil {
                if !matchPattern(pm, am) {
                    return false
                }
                continue
            }
        }
        // Both arrays: exact-length element match.
        if len(pv) > 0 && pv[0] == '[' && len(av) > 0 && av[0] == '[' {
            var pa, aa []json.RawMessage
            if json.Unmarshal(pv, &pa) == nil && json.Unmarshal(av, &aa) == nil {
                if len(pa) != len(aa) {
                    return false
                }
                for i := range pa {
                    var pm2, am2 map[string]json.RawMessage
                    if json.Unmarshal(pa[i], &pm2) == nil && json.Unmarshal(aa[i], &am2) == nil {
                        if !matchPattern(pm2, am2) {
                            return false
                        }
                    } else if string(pa[i]) != string(aa[i]) {
                        return false
                    }
                }
                continue
            }
        }
        // Primitive: raw JSON byte comparison.
        if string(pv) != string(av) {
            return false
        }
    }
    return true
}
```

**Step 3.5: Run lsp tests — all should pass**
- [ ] `go test ./ops/ -run "TestHandleASTNodeAt|TestHandleASTFindSymbols|TestHandleASTFind" -v`

---

### Phase 4: server.go — Register Four New Tools

Append to `RegisterTools` in `server.go` after the existing gomod tools, with a comment
`// Tier 2 tools`.

**ast_rename:**
```go
s.AddTool(
    mcp.NewTool("ast_rename",
        mcp.WithDescription("Rename an identifier at its declaration site and update all references within the same file. AST-only approximation — renames all identifiers with the given name in the file regardless of scope. Accurate for top-level declarations; may over-rename shadowed variables."),
        mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
        mcp.WithArray("path", mcp.Required(), mcp.Description("Selector path to the declaration site of the identifier to rename")),
        mcp.WithString("to", mcp.Required(), mcp.Description("New identifier name")),
        mcp.WithBoolean("dry_run", mcp.Description("If true, return diff without writing to disk")),
    ),
    mcp.NewTypedToolHandler(ops.HandleASTRename),
)
```

**ast_node_at:**
```go
s.AddTool(
    mcp.NewTool("ast_node_at",
        mcp.WithDescription("Given a file position (line + column, 1-based), return the innermost AST node at that position, its structural path from the file root, and metadata. The path can be used directly with ast_query, ast_replace, etc."),
        mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
        mcp.WithInteger("line", mcp.Required(), mcp.Description("Line number (1-based)")),
        mcp.WithInteger("col", mcp.Required(), mcp.Description("Column number (1-based)")),
    ),
    mcp.NewTypedToolHandler(ops.HandleASTNodeAt),
)
```

**ast_find_symbols:**
```go
s.AddTool(
    mcp.NewTool("ast_find_symbols",
        mcp.WithDescription("Search for declarations matching a name glob pattern across all .go files in a directory (non-recursive). Pattern supports * wildcard and is case-insensitive. Returns array of {file, path, kind, name, recv, line, meta}."),
        mcp.WithString("dir", mcp.Required(), mcp.Description("Directory to search (non-recursive)")),
        mcp.WithString("query", mcp.Required(), mcp.Description("Glob pattern for symbol name, e.g. \"Handle*\", \"*\", \"Get\"")),
        mcp.WithArray("kinds", mcp.Description("Optional filter by kind: [\"FuncDecl\", \"TypeSpec\", \"VarSpec\", \"ConstSpec\"]. Omit for all.")),
    ),
    mcp.NewTypedToolHandler(ops.HandleASTFindSymbols),
)
```

**ast_find:**
```go
s.AddTool(
    mcp.NewTool("ast_find",
        mcp.WithDescription("Structural search: find all AST nodes matching a partial node tree pattern. Absent or null fields are wildcards. Present fields must match exactly. Array fields require exact-length match. Provide file for single-file search or dir for directory-wide search. Returns array of {file, path, node, source, meta}."),
        mcp.WithString("file", mcp.Description("Absolute path to a single Go source file")),
        mcp.WithString("dir", mcp.Description("Directory to search all .go files (non-recursive)")),
        mcp.WithObject("pattern", mcp.Required(), mcp.Description("Partial node tree. Absent/null fields are wildcards. Example: {\"kind\":\"IfStmt\"} finds all if statements.")),
    ),
    mcp.NewTypedToolHandler(ops.HandleASTFind),
)
```

---

### Phase 5: Verification

- [ ] `cd /home/matt/source/goast-worktrees/goast-mcp-server && go test ./ops/ -run TestHandleASTRename -v`
- [ ] `go test ./ops/ -run "TestHandleASTNodeAt|TestHandleASTFindSymbols|TestHandleASTFind" -v`
- [ ] `go test ./...` — no regressions
- [ ] `go test -race ./ops/`
- [ ] `go build ./...`
- [ ] `go vet ./...`

---

## Edge Cases to Handle

### Edge Case 1: tokenFile.LineStart panics on out-of-range line
**Scenario:** ast_node_at called with line=9999 on a 10-line file.
**Expected:** toolError before LineStart is called.
**Implementation:** `if args.Line < 1 || args.Line > tokenFile.LineCount() { return toolError(...) }`

### Edge Case 2: ast_rename with empty To
**Scenario:** Caller passes `"to": ""`.
**Expected:** toolError without touching the file.
**Implementation:** Guard at top: `if args.To == "" { return toolError(...) }`

### Edge Case 3: ast_find with non-object pattern
**Scenario:** Pattern is `"IfStmt"` (a string) rather than `{"kind":"IfStmt"}`.
**Expected:** toolError from json.Unmarshal to map failing.

### Edge Case 4: ast_find_symbols on empty dir
**Scenario:** Dir exists but contains no .go files.
**Expected:** Empty array returned, not error.
**Implementation:** `if results == nil { results = []SymbolResult{} }`

### Edge Case 5: ast_find with neither file nor dir
**Scenario:** Both File and Dir are "".
**Expected:** toolError("file or dir is required").

### Edge Case 6: col is 0 (off-by-one)
**Scenario:** Caller passes col=0 (0-based vs 1-based confusion).
**Expected:** toolError("col ... out of range (must be >= 1)").

### Edge Case 7: buildPath for non-selector-addressable nodes
**Scenario:** The found node is an *ast.BasicLit deep in an expression with no named selector step.
**Expected:** buildPath returns a partial path stopping at the nearest addressable ancestor.
**Implementation:** nodeToPathStep returns (zero, false) for unknown types; buildPath silently skips them.

### Edge Case 8: matchPattern array exact-length requirement
**Scenario:** Pattern `"args":[{"kind":"Ident"}]` vs actual CallExpr with 3 args.
**Expected:** No match — exact array length required.
**Documented in tool description.**

---

## Risks and Mitigations

### Risk 1: nodeToPathStep coverage gaps
**Risk:** Some node types produce incomplete paths.
**Impact:** Non-fatal — path stops at nearest representable ancestor. Valid but less precise.
**Mitigation:** Cover all stmt kinds in stmtKindName. Missing scalar fields default to no step.

### Risk 2: collectAncestors map key collision
**Risk:** Two different nodes could theoretically be equal pointers (impossible in Go — each
node is a separate heap allocation).
**Impact:** None. ast.Node values are interface values pointing to distinct structs.

### Risk 3: ast_find performance on large files
**Risk:** MarshalNode on every node in a 5000-line file.
**Estimate:** ~2000-5000 nodes * ~10µs = 20-50ms. Acceptable.
**Mitigation:** Accept for Tier 2. Document if user reports slowness.

### Risk 4: ast_rename over-renames shadowed names
**Risk:** Renaming a top-level `err` renames all `err` in the file.
**Impact:** Incorrect for shadowed variables; correct for the stated primary use case.
**Mitigation:** Tool description explicitly documents this limitation.

### Risk 5: recvTypeString already defined in query.go
**Risk:** Re-defining it in lsp.go causes a compile error ("redeclared in this block").
**Impact:** Build failure.
**Mitigation:** Do NOT define recvTypeString in lsp.go — it is already accessible from
query.go within the same `ops` package.

---

## Dependencies

### Internal
- `editor.ParseFile(path)` → `(*ast.File, *token.FileSet, []byte, error)`
- `editor.Edit(path, dryRun, fn)` → `(EditResult, error)` where `EditResult.Changed`, `EditResult.Diff`
- `selector.Navigate(f, steps)` → `(ast.Node, ParentContext, error)`
- `selector.PathStep{Kind, Name, Recv, Index}`
- `selector.NavigateError`
- `kinds.MarshalNode(ast.Node)` → `(json.RawMessage, error)`
- `meta.Compute(fset, src, node, nil, depth)` → `meta.Meta`
- `ops.toolError`, `ops.navError`, `ops.recvTypeString` (defined in query.go, same package)

### External (already in go.mod)
- `golang.org/x/tools/go/ast/astutil` v0.45.0 — `Apply`, `Cursor`
- `github.com/mark3labs/mcp-go/mcp` v0.54.0

### No New Dependencies Required

---

## Complexity Analysis

| Function | Estimated Complexity | Notes |
|---|---|---|
| HandleASTRename | 8 | Validate + navigate + walk |
| extractDeclName | 7 | Type switch, 5 arms |
| HandleASTNodeAt | 12 | Offset calc, map iteration for innermost |
| HandleASTFindSymbols | 8 | ReadDir + scan loop |
| scanSymbols | 15 | Nested type switches + filter |
| HandleASTFind | 8 | File list + findInFile loop |
| findInFile | 8 | astutil.Apply walk |
| matchPattern | 12 | Recursive JSON comparison, 3 branches |
| buildPath | 8 | Parent chain walk + reverse |
| nodeToPathStep | 28 | Large type switch, ~20 cases |
| collectAncestors | 3 | Simple walk |
| nthIndexOfKind | 5 | Count loop |
| stmtKindName | 14 | Type switch, ~13 cases |

All under 40. nodeToPathStep at ~28 is highest. If adding more cases pushes past 35,
split into `nodeToPathStepDecl`, `nodeToPathStepStmt`, `nodeToPathStepField`.

---

## Success Criteria

- [ ] All 6 `TestHandleASTRename_*` tests pass
- [ ] All 5 `TestHandleASTNodeAt_*` tests pass
- [ ] All 8 `TestHandleASTFindSymbols_*` tests pass
- [ ] All 8 `TestHandleASTFind_*` tests pass
- [ ] `go test ./...` passes (no Tier 1 regressions)
- [ ] `go test -race ./ops/` passes
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] No function exceeds cyclomatic complexity 40
- [ ] ast_rename tool description documents scope approximation
- [ ] ast_find tool description documents exact array-length matching
- [ ] ast_node_at tool description documents 1-based line/col
- [ ] Four new tools registered in server.go

---

## Notes

- `recvTypeString` is in query.go (same package). Do not redeclare in lsp.go.
- `toolError` and `navError` are in query.go (same package). Use directly.
- Verify `editor.ParseFile` signature matches `(path string) (*ast.File, *token.FileSet, []byte, error)` before use.
- The `path.Match` function (stdlib `path` package, not `path/filepath`) is used for glob matching. Import as `"path"`. There is NO path separator in symbol names, so `*` works as a full-match wildcard.
- `collectAncestors` is called twice in ast_node_at (once to find the best node, once implicitly via buildPath). Alternatively, call it once and pass the map to both uses — minor optimization, implement as single call.
- For `buildPath`, the *ast.File node itself is in the ancestors map with parent==nil. The loop terminates when `anc.parent` is *ast.File (the step is added) or when `anc.parent` is nil (the root, no step).
- Array fields (`"args"`, `"elts"`, `"lhs"`, `"rhs"`) in the Args/Elts/Lhs/Rhs step cases of `nodeToPathStep` use the raw `sliceIndex` (not nth-of-kind), since these are indexed by position not by kind.
