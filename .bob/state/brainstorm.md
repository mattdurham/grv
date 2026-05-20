# Brainstorm

## 2026-05-19 00:00:00 - Task Received

Build the goast MCP server as designed in design.md — a Go MCP server that exposes Go AST
read/write operations to Claude via structured JSON node trees. No raw text editing, no line
numbers, no snippets. Fully bidirectional JSON ↔ go/ast.

Starting brainstorm process...

---

## 2026-05-19 00:05:00 - Research Findings

### MCP Library: mark3labs/mcp-go v0.54.0

Already in go.mod. This is the right choice — it is well-maintained, has stdio transport built-in,
and provides the exact APIs needed.

**Key APIs confirmed working:**

`server.NewMCPServer(name, version, server.WithToolCapabilities(false))` — creates server.
`s.AddTool(tool, handler)` — registers a tool with its handler.
`server.ServeStdio(s)` — starts the stdio loop (handles SIGTERM/SIGINT, reads from os.Stdin, writes to os.Stdout).

Tool definition pattern:
```go
tool := mcp.NewTool("ast_query",
    mcp.WithDescription("..."),
    mcp.WithString("file", mcp.Required(), mcp.Description("...")),
    mcp.WithAny("path", mcp.Description("..."), mcp.Required()),
)
s.AddTool(tool, handler)
```

For tools with complex schemas (path arrays, node objects), use `mcp.NewToolWithRawSchema` or
`mcp.WithRawInputSchema(json.RawMessage(...))` for full JSON Schema control.

Tool handler signature:
```go
func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)
```

Arguments bound via `req.BindArguments(&args)` — uses json.Marshal/Unmarshal internally.
Response via `mcp.NewToolResultJSON(data)` or `mcp.NewToolResultError(msg)`.

**Typed handler pattern** (reduces boilerplate):
```go
s.AddTool(tool, mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args MyArgs) (*mcp.CallToolResult, error) {
    ...
}))
```

**Critical note on stdout**: All logging must go to stderr. The stdio transport uses stdout for
JSON-RPC — any non-JSON written to stdout corrupts the protocol.

### Confirmed: token.NoPos works for inserted/replaced nodes

Tested empirically: `go/format.Node` on a `*ast.File` correctly formats nodes that were
constructed with `token.NoPos` (zero-position). The printer ignores invalid positions for
content it inserts (new nodes), and only uses positions for spacing relative to existing nodes.

This is the core architectural unlock: `ToAST()` can return `go/ast` nodes with all positions
set to `token.NoPos` — `go/format` handles them correctly when applied to the full file.

Test output confirmed:
- Inserting IfStmt with NoPos positions: formats correctly with proper indentation
- Replacing sub-expressions with NoPos nodes: formats correctly
- Direct slice mutation (append splice): works correctly for delete and insert

### go/ast Direct Mutation vs astutil.Apply

Two approaches tested:

**Direct mutation**: Navigate to parent, splice/assign the slice or field directly.
- For delete: `body.List = append(body.List[:i], body.List[i+1:]...)` — works
- For replace: `ifStmt.Cond = newExpr` — works
- For insert: `body.List = append(body.List[:i], append([]ast.Stmt{newStmt}, body.List[i:]...)...)` — works

**astutil.Apply**: Tree-walking with pre/post callbacks, Cursor provides Replace/Delete/InsertBefore/InsertAfter.
- Good for pattern-based operations (ast_find, ast_rename)
- More complex to use for indexed operations (need to count during traversal)
- Apply is the right tool for ast_find and ast_rename (scan whole tree)

**Decision**: Use direct mutation for ast_insert/ast_replace/ast_delete (the selector already
finds the parent and index). Use astutil.Apply for ast_find, ast_rename, and pattern matching.

### astutil.AddNamedImport / DeleteImport

`astutil.AddNamedImport(fset, f, name, path)` — adds import, returns bool (added or already present).
`astutil.DeleteImport(fset, f, path)` — removes import by path, returns bool.

Both work correctly and preserve import grouping. Tested empirically.

### go-difflib for Unified Diff Output

`github.com/pmezard/go-difflib v1.0.0` is already available in the module cache.

```go
diff := difflib.UnifiedDiff{
    A: difflib.SplitLines(before), B: difflib.SplitLines(after),
    FromFile: path, ToFile: path, Context: 3,
}
text, _ := difflib.GetUnifiedDiffString(diff)
```

Produces standard unified diff. This is the right library — no extra dependencies.

### golang.org/x/mod/modfile

`modfile.Parse(file, data, nil)` — parses go.mod to `*modfile.File`
`f.AddRequire(path, vers)` — adds/updates require
`f.DropRequire(path)` — removes require  
`f.AddReplace(oldPath, oldVers, newPath, newVers)` — adds/updates replace
`f.DropReplace(oldPath, oldVers)` — removes replace
`f.Format()` — produces updated go.mod bytes (preserves comments, formatting)

All confirmed available and working. Uses the same library as the `go` toolchain itself.

### JSON Discriminated Union Unmarshaling

Tested the two-phase peek pattern:
```go
func UnmarshalNode(data json.RawMessage) (Node, error) {
    var peek struct{ Kind string `json:"kind"` }
    json.Unmarshal(data, &peek)
    factory := registry[peek.Kind]
    node := factory()
    json.Unmarshal(data, node)
    return node, nil
}
```

Key insight: each Node struct stores its child nodes as `json.RawMessage` fields, not as `Node`
interface fields. This avoids circular type embedding issues in Go. The `ToAST()` method calls
`UnmarshalNode()` recursively on its `json.RawMessage` children, then calls `ToAST()` on each.

This pattern is clean and requires no special JSON unmarshaler on the struct types.

### golang.org/x/tools/go/ast/astutil.Apply

```go
// Cursor methods available:
func (c *Cursor) Node() ast.Node
func (c *Cursor) Parent() ast.Node
func (c *Cursor) Name() string    // field name in parent
func (c *Cursor) Index() int      // index in slice, or -1
func (c *Cursor) Replace(n ast.Node)
func (c *Cursor) Delete()
func (c *Cursor) InsertAfter(n ast.Node)
func (c *Cursor) InsertBefore(n ast.Node)
```

Used for: ast_find (structural search), ast_rename (identifier scanning),
ast_find_refs (reference finding within a file scope).

### go/types for LSP Operations

`go/types.Config{}.Check(path, fset, files, info)` — type-checks a package.
`golang.org/x/tools/go/packages.Load(cfg, patterns...)` — loads packages with type info.

Cross-file LSP ops (ast_find_refs package scope, ast_find_def, ast_find_impls) require
`go/types` + `go/packages`. These are Tier 3 operations — slow (invokes `go list`), need a
working module at the project root. Single-file variants (ast_find_refs file scope) can use
pure AST analysis.

### MCP Tool Schema for Complex Parameters

For tools that take array/object parameters (path, node):
- `mcp.WithAny("path", mcp.Description("..."), mcp.Required())` — accepts any JSON value
- `mcp.NewToolWithRawSchema(name, desc, rawSchema)` — full JSON Schema control

Using `BindArguments` with struct fields typed as `json.RawMessage` correctly captures
complex JSON parameters without type conversion loss.

```go
type ASTQueryArgs struct {
    File   string          `json:"file"`
    Path   json.RawMessage `json:"path"`   // []PathStep
    DryRun bool            `json:"dry_run"`
    Node   json.RawMessage `json:"node"`   // Node (for replace/insert)
}
```

---

## 2026-05-19 00:20:00 - Spec-Driven Modules in Scope

No spec-driven modules detected in scope. This is a greenfield project with no existing SPECS.md,
NOTES.md, TESTS.md, BENCHMARKS.md, or CLAUDE.md files. No spec constraints to honor.

---

## 2026-05-19 00:25:00 - Implementation Challenges Analysis

### Challenge 1: JSON Discriminated Union Unmarshaling (50 kinds)

**Problem**: The `"node"` parameter in ast_insert/ast_replace is a JSON object whose Go type
depends on `"kind"`. Standard `encoding/json` cannot handle this without custom unmarshaling.

**Solution (confirmed)**: Two-phase peek pattern. Each Node struct stores child nodes as
`json.RawMessage` (lazy), calling `UnmarshalNode()` recursively inside `ToAST()`. This is
clean, Go-idiomatic, and avoids any circular type issues.

Registry map `map[string]func() Node` provides O(1) dispatch. 50 entries, trivial.

**Difficulty**: Low — pattern is clear and mechanical.

### Challenge 2: Path Selector Traversal

**Problem**: Navigate `[{"kind":"FuncDecl","name":"advance"}, {"kind":"IfStmt","index":0}, {"kind":"Cond"}]`
from `*ast.File` to a specific `ast.Expr`.

**Solution**: The `selector` package implements a `Navigate(file *ast.File, steps []Step) (ast.Node, parentContext, error)` function. Each step is a type switch + filter:

```go
case "FuncDecl":
    for _, decl := range file.Decls {
        if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == step.Name {
            current = fd
        }
    }
case "Body":
    if fd, ok := current.(*ast.FuncDecl); ok {
        current = fd.Body
    }
case "IfStmt":
    // find the Nth IfStmt in the current BlockStmt.List
    block := current.(*ast.BlockStmt)
    count := 0
    for _, stmt := range block.List {
        if ifStmt, ok := stmt.(*ast.IfStmt); ok {
            if count == step.Index { current = ifStmt; break }
            count++
        }
    }
case "Cond":
    current = current.(*ast.IfStmt).Cond
```

The `parentContext` return is essential for ast_delete and ast_insert — it holds (parentNode,
fieldName, index) so the caller knows how to mutate the parent list.

For ast_replace, navigate to the node's parent, then assign the new node to the right field.

**Design for error context**: When navigation fails at step N, return:
- which step failed
- what was at that point in the AST
- what alternatives were available

```go
type NavigateError struct {
    AtStep    int
    Step      PathStep
    Available []string  // "IfStmt[0]", "ReturnStmt[1]", etc.
}
```

**Difficulty**: Medium — requires a lot of type-switch cases (~85 step kinds) but is systematic.

### Challenge 3: ToAST Position Fields

**Problem**: Many `go/ast` struct fields hold `token.Pos` for keyword/punctuation positions
(e.g., `IfStmt.If`, `BlockStmt.Lbrace`, `ForStmt.For`). `go/format` uses these for spacing.

**Solution (confirmed empirically)**: Using `token.NoPos` (zero value) for all position fields
is safe. `go/format.Node` on a `*ast.File` correctly handles zero-positioned nodes in the
context of an existing file. The printer re-derives spacing from surrounding nodes.

**Key fields that need `token.NoPos`**: IfStmt.If, IfStmt.Else, ForStmt.For, BlockStmt.Lbrace/Rbrace,
FuncType.Func, ChanType.Arrow, CallExpr.Ellipsis, etc.

`ToAST()` can set all `token.Pos` fields to `token.NoPos` — this is the right default.

**Difficulty**: Low — zero-value approach confirmed working.

### Challenge 4: FuncType.TypeParams and Generic Types

**Problem**: Go 1.18+ generics use `FuncType.TypeParams` and `TypeSpec.TypeParams`. The
`go/ast` struct uses `*ast.FieldList` for type parameters, same as regular params.

**Solution**: Handle `TypeParams` as `[]*Field` in the JSON schema — same as `Params` and
`Results`. When `TypeParams` is null/empty, omit from the `*ast.FuncType.TypeParams` field.

**Difficulty**: Low — same structure as existing field lists.

### Challenge 5: Comment Preservation

**Problem**: `go/printer` preserves doc comments (attached to declarations) and inline comments
(attached to statements via `ast.CommentGroup`). However, freestanding comments between
statements (not attached to any node) can be lost when the surrounding statements are moved,
deleted, or replaced.

**Solution**: Document this limitation clearly. The design.md already acknowledges it:
"Freestanding comments between statements may be lost after structural edits. This is a
known go/ast limitation."

Practical mitigation: Before writing, compare pre/post comment counts and warn via the diff
response if comments appear to have been dropped.

**Difficulty**: Low to document, impossible to fully fix (it's a go/printer limitation).

### Challenge 6: ast_rename Single-File

**Problem**: Rename `nextToken` → `scanToken` within one file. Need to find all uses of the
identifier (not just string occurrences) and rename them.

**Solution without go/types**: Use `astutil.Apply` to walk the AST. For the declaration site
(FuncDecl.Name), record the `*ast.Ident` pointer. For all call sites and references in the file,
check if the `*ast.Ident.Name` matches AND the position is reachable from that declaration.

Pure AST approach (no type info): walk the file, rename all `*ast.Ident` with matching name
that appear in a call/selector position. This is a safe approximation for single-file rename —
it will miss shadowed variables but that's acceptable for Tier 2.

Cross-file rename (`scope: "package"`) requires `go/types` for accurate resolution.

**Difficulty**: Medium for single-file (AST-only), Hard for cross-file (requires go/types).

### Challenge 7: ast_find Structural Search

**Problem**: Match a pattern tree against all nodes in a file, where absent fields are wildcards.

**Solution**: Recursive match function:
```go
func matchNode(pattern, actual Node) bool {
    if pattern.Kind() != actual.Kind() { return false }
    // for each field in pattern: if set, compare; if nil/zero, skip
}
```

For JSON nodes: compare field by field. If a field in the pattern is `null` or absent, it's a
wildcard. If present, it must match exactly (recursively).

Use `astutil.Apply` to visit every node, convert via `FromAST`, then match against the pattern.

**Difficulty**: Medium — pattern matching logic needs careful null/absent handling.

### Challenge 8: LSP Operations (ast_find_refs, ast_find_def, ast_find_impls)

**Problem**: These require type resolution to work accurately. Pure AST analysis will give
false positives for same-name identifiers in different scopes.

**Solution approach**:
- `ast_find_refs` with `scope: "file"` — pure AST analysis is sufficient (fast, no go list)
- `ast_find_refs` with `scope: "package"` — requires `go/types.Check()` on all files in dir
- `ast_find_def` — requires `go/types` to resolve import paths
- `ast_find_impls` — requires `go/types` for interface satisfaction checking

`go/packages.Load` with `packages.NeedTypes | packages.NeedTypesInfo` is the right API for
cross-file loading. It invokes `go list` internally — takes 1-5 seconds for typical packages.

**Mitigation**: Parse-on-every-call (no caching) is fine for file-scoped ops. For package-scoped
ops, accept the latency. Document it.

**Difficulty**: Hard — go/types setup is complex, requires proper module configuration.

### Challenge 9: ast_extract_func

**Problem**: Extract statements[3:7] into a new function. Need to:
1. Find all variables used in statements that were declared before statement 3 → parameters
2. Find all variables declared in statements and used after statement 7 → return values
3. Generate function signature, declaration, and call site

This requires walking the extracted statements and their context, doing scope analysis.

Pure AST approach (without go/types): track `*ast.Ident` usage and declaration patterns.
Look for `:=` or `var` declarations in the extracted range, check if those idents appear
after the range. Look for idents used in the range but declared before it → params.

**Difficulty**: Very Hard — scope analysis without type info is unreliable. Best deferred to Tier 3.

### Challenge 10: Atomic Writes

**Solution**:
```go
func writeAtomic(path string, content []byte) error {
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, ".goast-*.go")
    if err != nil { return err }
    defer os.Remove(tmp.Name()) // cleanup on failure
    if _, err := tmp.Write(content); err != nil { tmp.Close(); return err }
    if err := tmp.Close(); err != nil { return err }
    return os.Rename(tmp.Name(), path)
}
```

Temp file in same directory ensures rename is atomic (same filesystem).

**Difficulty**: Trivial.

---

## 2026-05-19 00:35:00 - Approaches Considered

### Approach 1: Full Design Implementation (All Tiers)

Implement all tools as specified in design.md including Tier 3 LSP operations and refactoring tools.

**Pros:**
- Complete feature set from day one
- All tools available for integration testing

**Cons:**
- LSP operations (ast_find_refs package scope, ast_find_def, ast_find_impls) require go/packages
  which is significantly more complex (go list invocation, module setup, slow startup)
- ast_extract_func and ast_inline_func are research-grade implementations — risky scope
- Longer time to working v1
- Cross-file rename needs go/types — adds significant complexity

**Fits existing patterns:** Yes, follows design.md exactly.

### Approach 2: Tiered Phased Build (Recommended)

Build in tiers, with a working server after Tier 1 that Claude can immediately use.

**Tier 1 (working v1 — core, ~60% of value):**
- `kinds/` package: all 50 Node structs with `ToAST`/`FromAST`
- `selector/` package: path navigation, parentContext tracking
- `editor/` package: parse→edit→format→write cycle, atomic writes, diff generation
- `diff/` package: go-difflib wrapper
- `ops/query.go`: ast_list, ast_query, ast_query_many, ast_meta (no hooks)
- `ops/insert.go`: ast_insert
- `ops/replace.go`: ast_replace
- `ops/delete.go`: ast_delete
- `ops/imports.go`: ast_add_import, ast_delete_import, ast_list_imports
- `ops/gomod.go`: gomod_read, gomod_require, gomod_drop_require, gomod_replace, gomod_drop_replace
- `server.go` + `main.go`

**Tier 2 (~25% additional value):**
- `ops/rename.go`: ast_rename single-file (AST-only, no go/types)
- `ops/lsp.go`: ast_node_at, ast_find_symbols, ast_find (structural search)
- Metadata hooks system (goast.toml, subprocess hooks, file-scope cache)

**Tier 3 (~15% advanced value):**
- Cross-file LSP: ast_find_refs (package scope), ast_find_def, ast_find_impls
- Cross-file rename (go/types-based)
- ast_extract_func, ast_inline_func

**Pros:**
- Working server after Tier 1 (~3-4 days of focused work)
- Risk-free: each tier delivers independent value
- Tier 3 LSP ops are the most complex — can defer or simplify if needed
- Allows Claude to start using goast immediately on Tier 1

**Cons:**
- Tier 3 ops will be deferred
- Some tools in the MCP spec won't be registered initially

**Fits existing patterns:** Yes, design.md explicitly calls out a similar tier structure.

### Approach 3: Minimal Viable Server (Read-Only First)

Build only read tools (ast_list, ast_query, ast_query_many) first, then add write tools.

**Pros:**
- Smallest possible working deliverable
- Good for testing the schema/tool registration approach

**Cons:**
- Read-only is only ~20% of the value
- The hard problems (ToAST, atomic writes) are deferred, not avoided
- Less useful to Claude right away

**Fits existing patterns:** Partial — under-builds the design.

---

## 2026-05-19 00:45:00 - Recommendation

### Chosen Approach: Approach 2 — Tiered Phased Build

**Rationale:**

The tiered approach delivers a working, genuinely useful server (50+ node kinds, bidirectional
JSON↔AST, read/write ops, imports, go.mod) without gambling on the high-risk Tier 3 items first.
The LSP operations require `go/packages` + `go/types` which adds significant complexity and
latency — they are better tackled after the core is solid.

All the technically difficult elements (token.NoPos handling, discriminated union unmarshaling,
selector traversal) are confirmed working by prototype testing. The remaining work is largely
mechanical — 50 `ToAST`/`FromAST` implementations, 85 selector step cases.

**Build Order Within Tier 1:**

1. **`kinds/` package** — Node interface, registry, all 50 structs with ToAST/FromAST.
   Start with the most-used: Ident, BasicLit, BinaryExpr, UnaryExpr, CallExpr, SelectorExpr,
   BlockStmt, IfStmt, ForStmt, RangeStmt, AssignStmt, ReturnStmt, ExprStmt, FuncDecl,
   FuncType, Field. Then the rest.

2. **`selector/` package** — Navigate(file, steps) → (node, parentCtx, error).
   Include all ~85 step kinds from the design's path vocabulary table. Write selector_test.go
   with testdata/*.go files and round-trip tests.

3. **`diff/` package** — Thin wrapper around go-difflib. DiffFiles(before, after []byte, path string) string.

4. **`editor/` package** — ParseAndEdit(path string, fn func(*ast.File, *token.FileSet) error) (diff string, err error).
   Handles: parse, call fn to mutate AST, format, atomically write, generate diff.
   Also editor.DryRun variant that formats but doesn't write.

5. **`ops/query.go`** — ast_list, ast_query, ast_query_many, ast_meta.
   ast_meta computes metadata from ast.Node without hooks initially.

6. **`ops/insert.go`**, **`ops/replace.go`**, **`ops/delete.go`** — wire selector + editor.

7. **`ops/imports.go`** — wire astutil.AddNamedImport / DeleteImport + ast_list_imports.

8. **`ops/gomod.go`** — wire modfile parse/edit/format + atomic write.

9. **`server.go`** — register all Tier 1 tools.

10. **`main.go`** — `server.NewMCPServer` + `server.ServeStdio`.

**Key Design Decisions:**

- **Child nodes as json.RawMessage in structs**: Avoids circular type issues. ToAST calls
  `UnmarshalNode()` recursively. This is the confirmed correct approach.

- **token.NoPos for all constructed nodes**: Confirmed safe via prototype testing. No need to
  fake positions.

- **Direct mutation for insert/replace/delete**: Navigate to parent list via selector, splice
  directly. No need for astutil.Apply for these operations — cleaner and more efficient.

- **astutil.Apply for ast_find and ast_rename**: Tree-scanning operations benefit from Apply's
  cursor with Replace/Delete primitives.

- **Error context on navigation failure**: Return `NavigateError{AtStep, Step, Available}` so
  Claude can immediately understand what went wrong and try the correct path.

- **Tool argument binding**: Use `mcp.NewTypedToolHandler` + struct args with `json.RawMessage`
  fields for path and node. Avoids boilerplate BindArguments calls.

- **No in-memory state**: Parse-on-every-call. No per-call cache. This simplifies the server
  dramatically and is fast enough (go/parser is ~1ms for typical files).

- **Unified diff in response**: Every write tool returns `"diff"` field. Claude can inspect
  what changed without re-querying. Dry run mode returns diff without writing.

**Tier 2 Implementation Notes:**

- `ast_rename` single-file: Walk AST with astutil.Apply, rename all Ident nodes with matching
  name. This is an approximation (no type resolution) but correct for the common case.
- `ast_find` structural search: Walk AST with astutil.Apply, convert each node via FromAST,
  compare against pattern using a recursive field matcher.
- `ast_node_at`: Walk AST, find innermost node whose Pos()..End() range contains the target
  byte offset. Reverse-navigate to build the path.

**Tier 3 Implementation Notes:**

- Use `golang.org/x/tools/go/packages.Load` with `NeedTypes | NeedTypesInfo | NeedSyntax`.
- Cache the type-checked package per (dir, file-mtimes) for the server's lifetime.
- All Tier 3 tools require a valid Go module at the project root.

**Risks Identified:**

- **Selector completeness**: The ~85 step kinds in the path vocabulary are many. Missing a step
  kind means that path is navigable by Claude but will fail at runtime. Mitigation: write a
  comprehensive selector_test.go with testdata covering all step kinds before moving to ops/.

- **GenDecl split into ImportDecl/ConstDecl/TypeDecl/VarDecl**: The JSON schema splits what
  `go/ast` calls `GenDecl` into 4 logical kinds. `FromAST` for these 4 must inspect
  `genDecl.Tok` to know which JSON kind to produce. `ToAST` for each must produce a
  `*ast.GenDecl` with the right `Tok` field. This is correct but requires careful attention.

- **FieldList as []*Field vs *ast.FieldList**: go/ast uses `*ast.FieldList` for params/results/
  type params/struct fields/interface methods. The JSON schema uses flat `[]Field`. ToAST for
  FuncType, StructType, InterfaceType must construct `*ast.FieldList` from the flat list.
  This includes setting `FieldList.Opening/Closing` to NoPos. Confirmed safe.

- **CaseClause.List null = default**: `list: null` in JSON means `case <nil>` in ast, which
  is the default case. Implementation must check for nil slice and set `CaseClause.List = nil`
  (not empty slice).

- **go/types latency for Tier 3**: `packages.Load` invokes `go list` which can take 2-10
  seconds on cold runs. Mitigation: document this, accept the latency, potentially add a
  timeout parameter to the tool.

**Open Questions:**

1. Should all 50 Node kinds be implemented in Tier 1, or defer some rare ones (IndexListExpr for
   generics index expressions, SelectStmt, CommClause, LabeledStmt, BranchStmt)? Recommendation:
   implement all 50 in Tier 1 — they're mechanical and the incompleteness would frustrate Claude.

2. Should `ast_meta` with hooks be Tier 1 or Tier 2? Recommendation: Tier 2. Hooks add complexity
   (subprocess management, caching, goast.toml parsing) and aren't needed for the core edit loop.
   ast_meta without hooks (pure AST-derived meta) is Tier 1.

3. `"source"` field in ast_query response (return source text alongside node tree, per
   design.md Open Questions)? Recommendation: Yes, include it in Tier 1. Extract the source
   bytes for the node's Pos..End range from the original source. This requires the original
   source bytes to be threaded through the query path.

---

## 2026-05-19 00:55:00 - Concrete Implementation Plan Preview

### Directory Structure (as designed)

```
goast/
  main.go          — 20 lines: NewMCPServer + ServeStdio
  server.go        — tool registration (~200 lines)
  kinds/
    node.go        — Node interface, registry, UnmarshalNode (~50 lines)
    expr_*.go      — 16 expression kinds (~30-50 lines each)
    type_*.go      — 6 type kinds
    field.go
    stmt_*.go      — 19 statement kinds
    decl_*.go      — 5 declaration kinds
    spec_*.go      — 3 spec kinds
  selector/
    selector.go    — Navigate() function (~400 lines of type switches)
    selector_test.go
  editor/
    editor.go      — parse→edit→format→write cycle (~100 lines)
    editor_test.go
  ops/
    query.go       — ast_list, ast_query, ast_query_many, ast_meta
    insert.go      — ast_insert
    replace.go     — ast_replace
    delete.go      — ast_delete
    rename.go      — ast_rename
    imports.go     — import tools
    gomod.go       — go.mod tools
    lsp.go         — ast_node_at, ast_find, ast_find_symbols (Tier 2+)
    refactor.go    — ast_extract_func, ast_inline_func (Tier 3)
  diff/
    diff.go        — DiffFiles() wrapper
  testdata/
    *.go           — fixed Go files for tests
```

### Node Interface

```go
// kinds/node.go
type Node interface {
    Kind() string
    ToAST() (ast.Node, error)
    FromAST(ast.Node) error
}

var registry = map[string]func() Node{ ... }

func UnmarshalNode(data json.RawMessage) (Node, error) {
    if len(data) == 0 || string(data) == "null" { return nil, nil }
    var peek struct{ Kind string `json:"kind"` }
    if err := json.Unmarshal(data, &peek); err != nil { return nil, err }
    factory, ok := registry[peek.Kind]
    if !ok { return nil, fmt.Errorf("unknown kind %q", peek.Kind) }
    n := factory()
    return n, json.Unmarshal(data, n)
}
```

### Example Kind Implementation (IfStmt)

```go
// kinds/stmt_if.go
// Namespace: goast/kinds/stmt
// Kind: IfStmt
// go/ast: *ast.IfStmt
package kinds

import (
    "encoding/json"
    "go/ast"
    "go/token"
)

type IfStmt struct {
    KindField string          `json:"kind"`
    Init      json.RawMessage `json:"init,omitempty"`  // Stmt|null
    Cond      json.RawMessage `json:"cond"`             // Expr
    Body      json.RawMessage `json:"body"`             // BlockStmt
    Else      json.RawMessage `json:"else,omitempty"`   // Stmt|null
}

func (s *IfStmt) Kind() string { return "IfStmt" }

func (s *IfStmt) ToAST() (ast.Node, error) {
    result := &ast.IfStmt{If: token.NoPos}
    if len(s.Init) > 0 && string(s.Init) != "null" {
        initNode, err := UnmarshalNode(s.Init)
        if err != nil { return nil, err }
        initAST, err := initNode.ToAST()
        if err != nil { return nil, err }
        result.Init = initAST.(ast.Stmt)
    }
    condNode, err := UnmarshalNode(s.Cond)
    if err != nil { return nil, err }
    condAST, err := condNode.ToAST()
    if err != nil { return nil, err }
    result.Cond = condAST.(ast.Expr)
    // ... body, else
    return result, nil
}

func (s *IfStmt) FromAST(node ast.Node) error {
    ifStmt := node.(*ast.IfStmt)
    s.KindField = "IfStmt"
    if ifStmt.Init != nil {
        var initNode IfStmt // placeholder, use actual kind
        _ = initNode
        // convert ifStmt.Init → json.RawMessage via FromAST dispatch
    }
    // etc.
    return nil
}
```

`FromAST` dispatch needs a `MarshalNode(ast.Node) (json.RawMessage, error)` helper that:
1. Identifies the concrete ast.Node type
2. Creates the corresponding Node struct
3. Calls FromAST
4. Marshals to JSON

This is the mirror of `UnmarshalNode`.

### Selector Package Core

```go
// selector/selector.go
type PathStep struct {
    Kind  string `json:"kind"`
    Name  string `json:"name,omitempty"`
    Recv  string `json:"recv,omitempty"`
    Index int    `json:"index"`  // -1 means "unset" (use 0 as default isn't safe, use pointer or sentinel)
}

// Actually use *int for index to distinguish "not set" from "0":
type PathStep struct {
    Kind  string  `json:"kind"`
    Name  string  `json:"name,omitempty"`
    Recv  string  `json:"recv,omitempty"`
    Index *int    `json:"index,omitempty"`
}

type ParentContext struct {
    Parent    ast.Node
    FieldName string  // "List", "Specs", "Args", etc.
    Index     int     // position of child in slice, or -1 if not in slice
}

type NavigateError struct {
    AtStep    int
    Step      PathStep
    Available []string
}

func Navigate(file *ast.File, steps []PathStep) (ast.Node, ParentContext, error)
```

---

## 2026-05-19 01:00:00 - BRAINSTORM COMPLETE

**Status:** Complete
**Recommendation:** Tiered Phased Build — Tier 1 first (50 kinds + core read/write ops + imports + go.mod), then Tier 2 (rename, ast_find, hooks), then Tier 3 (LSP + refactoring).
**Next Phase:** PLAN

Key confirmed findings for the planner:
1. `token.NoPos` is safe for all constructed AST nodes — confirmed by prototype
2. Child nodes stored as `json.RawMessage` in kind structs, recursively resolved in ToAST — confirmed working
3. Direct slice mutation (not astutil.Apply) for insert/delete/replace at known indices — confirmed working
4. `astutil.Apply` for tree-scan operations (ast_find, ast_rename) — confirmed working
5. `astutil.AddNamedImport`/`DeleteImport` for import management — confirmed working
6. `modfile.Parse/Format` + mutation methods for go.mod — confirmed working
7. `go-difflib` for unified diff — confirmed working, already in module cache
8. `mark3labs/mcp-go v0.54.0` stdio transport + `mcp.NewTypedToolHandler` pattern — confirmed ready

Hardest implementation parts (in order):
1. `FromAST` + `MarshalNode` helper — dispatching on concrete `ast.Node` type (50 type-switch arms)
2. `selector.Navigate` — 85 step kinds, parentContext tracking
3. `ast_rename` single-file — AST-only identifier scanning without type resolution
4. Tier 3 LSP ops — require go/packages setup (defer)

Ready for workflow-planner agent to create detailed implementation plan.
