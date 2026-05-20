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

---

## 2026-05-20 - Task Received (Tier 2)

Add 4 Tier 2 tools to the goast MCP server:
1. ast_rename — rename identifier at declaration site + all references in same file
2. ast_node_at — given file + line + col, return innermost node + its structural path
3. ast_find_symbols — find declarations matching a name glob across a directory
4. ast_find — structural search: find all nodes matching a partial node tree (absent fields = wildcards)

Starting Tier 2 brainstorm...

---

## 2026-05-20 - Research Findings (Tier 2)

### Codebase State

Tier 1 is fully implemented. Key packages confirmed present and working:

- `kinds/` — 50 bidirectional JSON ↔ go/ast node types, `MarshalNode(ast.Node) (json.RawMessage, error)` in `kinds/marshal.go`
- `selector/selector.go` — `Navigate(file *ast.File, steps []PathStep) (ast.Node, ParentContext, error)` fully implemented with ~85 step kinds
- `editor/editor.go` — `Edit(path, dryRun, fn)`, `ParseFile(path)` 
- `meta/meta.go` — `Compute(fset, src, node, parent, depth) Meta`, `FileInfo(fset, src, file) Meta`
- `ops/imports.go` — already uses `golang.org/x/tools/go/ast/astutil` (AddNamedImport, DeleteImport)
- `ops/query.go` — canonical tool handler pattern: typed args struct, `toolError()`, `navError()`, `mcp.NewTypedToolHandler`
- `ops/replace.go` — canonical edit pattern: `editor.Edit(path, dryRun, fn)`

**`golang.org/x/tools v0.45.0`** is already in go.mod — `astutil.Apply` is available at no extra cost.

### Tool Handler Pattern (from ops/query.go)

```go
type ASTXxxArgs struct {
    File string `json:"file"`
    Path json.RawMessage `json:"path"`
    // ...
}

func HandleASTXxx(ctx context.Context, req mcp.CallToolRequest, args ASTXxxArgs) (*mcp.CallToolResult, error) {
    f, fset, src, err := editor.ParseFile(args.File)
    if err != nil {
        return toolError(fmt.Sprintf("parse: %v", err)), nil
    }
    // ... work ...
    b, _ := json.Marshal(result)
    return mcp.NewToolResultText(string(b)), nil
}
```

`toolError()` and `navError()` are defined in `ops/query.go` and accessible within the `ops` package.

### editor.Edit Pattern (from ops/replace.go)

For write operations:
```go
result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, fset *token.FileSet) error {
    // mutate f ...
    return nil
})
// result.Changed, result.Diff
```

### astutil.Apply Signature

```go
// golang.org/x/tools/go/ast/astutil
func Apply(root ast.Node, pre, post ApplyFunc) ast.Node

type ApplyFunc func(*Cursor) bool

type Cursor struct { ... }
func (c *Cursor) Node() ast.Node
func (c *Cursor) Parent() ast.Node
func (c *Cursor) Name() string   // structural field name in parent
func (c *Cursor) Index() int     // -1 if not in a slice
func (c *Cursor) Replace(n ast.Node)
```

`pre` returns false to stop descending into children. `post` runs after children.

### path.Match for Glob Patterns

`path.Match(pattern, name)` — standard library, supports `*` wildcard. For case-insensitive
matching, both sides should be lowercased. This is suitable for `ast_find_symbols`.

### os.ReadDir for Directory Walking

```go
entries, err := os.ReadDir(dir)
for _, entry := range entries {
    if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
        // parse this file
    }
}
```

Non-recursive by design — `ast_find_symbols` scope is single directory, not subtree.

### token.FileSet Line/Offset Arithmetic

For `ast_node_at`, converting line+col to byte offset:

```go
// After ParseFile returns fset:
tokenFile := fset.File(f.Pos())  // *token.File for this file
lineStart := tokenFile.LineStart(line)  // token.Pos of line start
offset := fset.Position(lineStart).Offset + (col - 1)  // byte offset
```

Then find the innermost node containing that offset by walking the AST and checking
`fset.Position(node.Pos()).Offset <= offset < fset.Position(node.End()).Offset`.

**Caveat**: `tokenFile.LineStart(line)` panics if `line` is out of range. Must bounds-check
against `tokenFile.LineCount()`.

**Alternative**: Use `fset.Position(f.Pos()).Offset` as base, then compute directly from
the source bytes. But LineStart is cleaner.

### MarshalNode + JSON Field Comparison for ast_find

The structural matching approach for `ast_find`:

1. Caller provides a pattern as `json.RawMessage` (a partial JSON node tree).
2. Walk the file AST with `astutil.Apply`.
3. For each visited node, call `kinds.MarshalNode(node)` to get its JSON.
4. Compare the pattern JSON against the actual JSON using recursive field matching:
   - Parse both as `map[string]json.RawMessage`.
   - For each key in pattern: if the key exists in actual AND both are non-null, recursively compare.
   - If a pattern field is absent or `null` → wildcard (match anything).
   - Primitive fields (strings, numbers, booleans) compared by value equality.

This requires no knowledge of specific node schemas — it works generically via JSON.

**Key insight**: `json.RawMessage` comparison can be done by unmarshaling both sides to
`interface{}` and using `reflect.DeepEqual`. But a recursive map comparison is cleaner and
avoids reflect.

**Performance concern**: Calling `MarshalNode` on every visited node during a file walk is
O(n * average_node_size). For typical Go files (1000 nodes), this is fast enough. No caching
needed.

### Path Reconstruction for ast_node_at

The hard part of `ast_node_at` is building the `[]selector.PathStep` path from the file root
down to the found node. Options:

**Option A: Walk with parent tracking.**
Do a manual AST walk using `ast.Inspect`, maintaining a stack of (node, pathStep) pairs.
When the target node is found, return the accumulated path.

```go
type frame struct {
    node ast.Node
    step selector.PathStep
}
var stack []frame

ast.Inspect(f, func(n ast.Node) bool {
    if n == nil { stack = stack[:len(stack)-1]; return false }
    if containsOffset(fset, n, offset) {
        stack = append(stack, frame{n, stepFor(n, parent)})
        return true
    }
    return false
})
```

Problem: `ast.Inspect` does not provide parent context. Need to track it manually.

**Option B: astutil.Apply with cursor.**
`astutil.Apply` provides `c.Parent()` and `c.Name()` and `c.Index()`. This is exactly what's
needed to build a PathStep for the current node relative to its parent.

Walk pre-order. When a node's range contains the offset, it's a candidate. Track the deepest
(most specific) candidate seen.

**Option C: Recursive walk with path accumulation.**
Write a recursive helper that walks the AST, tracking the current path. When the target node
is found, return the accumulated path.

**Chosen: Option C** — a recursive walk that builds path steps as it descends. This is the
most controllable and easiest to reason about. The path-building logic mirrors the selector's
step logic in reverse.

However, building a full path from node-to-root is inherently tied to knowing the AST structure.
The returned path does not need to be maximally specific — it needs to be sufficient for the
selector to navigate back to the same node. So we can build a "summary path" that uses the
selector's step vocabulary.

**Practical approach**: Walk the AST keeping a parent map `map[ast.Node]ast.Node`. When the
innermost target node is found, walk up the parent chain to reconstruct the path steps.

```go
// Build parent map
parents := map[ast.Node]ast.Node{}
ast.Inspect(f, func(n ast.Node) bool {
    if n == nil { return false }
    // For each child of n, record n as its parent
    // (ast.Inspect doesn't give us children directly)
    return true
})
```

This doesn't work easily with `ast.Inspect` alone. Better to use `astutil.Apply` which gives
`c.Parent()`.

**Final approach for ast_node_at path reconstruction**:

Walk with `astutil.Apply`, collect all (node, parent, name, index) tuples for nodes whose
range contains the target offset. After the walk, find the deepest (smallest range) node.
Then walk up through parents to build path steps. The parent lookup uses the tuples collected.

This is O(n) in file nodes, which is fine.

### Spec-Driven Modules in Scope (Tier 2)

No spec-driven modules in scope. The `ops/` directory has no SPECS.md, NOTES.md, TESTS.md,
BENCHMARKS.md, or CLAUDE.md. No constraints from spec-driven enforcement.

---

## 2026-05-20 - Approaches Considered (Tier 2)

### ast_rename: Approach A — Walk and Rename All Matching Idents

Use `astutil.Apply` to walk the entire file AST. For every `*ast.Ident` whose `.Name` equals
`oldName`, call `c.Replace()` with a new `*ast.Ident{Name: newName}`.

**Obtain oldName**: Use `selector.Navigate(f, path)` to reach the declaration node. Extract
the name from there (e.g., `funcDecl.Name.Name`, `typeSpec.Name.Name`, etc.).

**Approximation**: This renames ALL idents with the matching name in the file — including ones
in different scopes (inner functions, nested type declarations). For the common case (renaming
a top-level function or type), this is correct. For shadowed variable rename, it will
over-rename. Document this limitation clearly in the tool description.

**Pros**: Simple, ~30 lines, uses established astutil.Apply pattern already in ops/imports.go.
**Cons**: Over-renames in shadow/scope scenarios.

### ast_rename: Approach B — Scope-Aware Walk Using go/types

Load the package with `go/types`, use `types.Info.Defs` and `types.Info.Uses` to find the
exact object, then rename only idents that reference that exact object.

**Pros**: Precise — no false renames.
**Cons**: Requires go/types setup, go list invocation, 1-5s latency. Explicitly out of scope
for Tier 2 per the task brief ("AST-only (no go/types)").

**Decision**: Approach A. Document the approximation.

---

### ast_node_at: Approach A — astutil.Apply with Ancestor Tracking

Walk the file with `astutil.Apply`. In the pre-function, if the current node's Pos..End range
contains the target byte offset, record it (node, parent, field name, index). Track the
"deepest" (narrowest range) match. After the walk, reverse-build the path.

**Reverse path building**: Starting from the innermost node, use the recorded parent chain
to emit PathStep objects. This requires mapping (parent type, field name, index) →
`selector.PathStep`. This mapping mirrors `selector.go` but in reverse.

**Pros**: Clean, uses cursor's parent info directly.
**Cons**: Reverse-mapping (parent, fieldName, index) → PathStep vocabulary is non-trivial.
         Not every AST node type has a corresponding selector step. Some intermediate nodes
         (e.g., `*ast.FieldList`) are not addressable by the selector vocabulary.

**Simplification**: Return a "best effort" path that stops at the first addressable ancestor.
The returned path should be valid for use with `ast_query`/`ast_replace`. If the innermost
node is inside a position not directly addressable by selector steps (e.g., inside a FieldList),
return the path to the nearest addressable ancestor.

### ast_node_at: Approach B — Walk and Return Node + Meta Only

Don't attempt to reconstruct the full path. Instead:
- Return the node's JSON (via MarshalNode), its meta (Compute), and the node's kind.
- Let the caller use ast_query to re-fetch by a manually constructed path if they need it.

**Pros**: Much simpler to implement — just find the innermost node, no path reconstruction.
**Cons**: The design.md spec says "Returns the innermost node at that position, its full path,
and its meta. The path can then be used directly with ast_query, ast_replace, etc."
Path reconstruction is required by spec.

**Decision**: Approach A, with the simplification of returning the path to the nearest
fully addressable ancestor node.

The path reconstruction needs a reverse-map function:
```go
func nodeToPathStep(node ast.Node, parent ast.Node, fieldName string, index int) (selector.PathStep, bool)
```
That returns a PathStep and whether the step is representable in selector vocabulary.

This function covers the main cases:
- `*ast.FuncDecl` in `*ast.File` → `{Kind: "FuncDecl", Name: fd.Name.Name}`
- `*ast.GenDecl` in `*ast.File` based on Tok → `{Kind: "ImportDecl"}` etc.
- `*ast.IfStmt` in `*ast.BlockStmt` with index → `{Kind: "IfStmt", Index: &idx}`
- `*ast.BlockStmt` as FuncDecl.Body → `{Kind: "Body"}`
- etc.

This is ~100-150 lines of type-switch code.

---

### ast_find_symbols: Approach A — ReadDir + Parse + Scan Decls

```
1. os.ReadDir(dir) — non-recursive
2. For each .go file: editor.ParseFile → *ast.File
3. Walk f.Decls:
   - *ast.FuncDecl: check name vs glob, check kinds filter
   - *ast.GenDecl: iterate specs, check names vs glob
4. Build {file, path, kind, name, recv, meta} per match
```

Glob matching: `path.Match(query, name)` for case-sensitive; lowercase both for
case-insensitive.

**Path field in result**: The selector path to navigate back to this declaration. For a
FuncDecl named "Foo": `[{kind: "FuncDecl", name: "Foo"}]`. For a TypeSpec named "Bar":
`[{kind: "TypeSpec", name: "Bar"}]`.

**Pros**: Simple, fast, no dependencies beyond stdlib.
**Cons**: None significant. This is straightforward.

---

### ast_find: Approach A — MarshalNode + JSON Field Comparison (Recommended)

```
1. Unmarshal pattern from json.RawMessage → map[string]json.RawMessage
2. Walk file with astutil.Apply
3. For each node: kinds.MarshalNode(node) → json.RawMessage
4. Unmarshal actual as map[string]json.RawMessage
5. matchPattern(patternMap, actualMap) recursive:
   - For each key in patternMap:
     - If key == "kind": must match exactly
     - If patternMap[key] is null or missing: wildcard (skip)
     - If both are objects: recurse
     - If primitive: compare JSON bytes (or unmarshal to interface{} and compare)
6. Collect matching nodes with path + meta
```

**Path for matched nodes**: Same problem as ast_node_at — need to know position in tree.
Use the same parent-tracking approach: record (node, parent, fieldName, index) during the walk,
then build path steps for matched nodes.

**Alternative for path**: Since ast_find is a search, we can return the meta (which includes
line/col/offset) and let the caller use ast_node_at to get the path if needed. But the spec
says to return `{ path, node, meta }` — path is required.

**Practical shortcut for ast_find path**: Since ast_find returns top-level or nested matches,
and the most common use case is finding top-level patterns (IfStmt in function body, CallExpr
anywhere), we can build path steps for well-known ancestor types and stop at the first
addressable node. This reuses the same path reconstruction logic from ast_node_at.

### ast_find: Approach B — Compile Pattern to Predicate

Parse the pattern JSON into a Node struct (via kinds.UnmarshalNode), then use the Node's
typed fields for matching rather than raw JSON comparison.

**Pros**: Cleaner matching logic — typed fields make null vs absent unambiguous.
**Cons**: The Node structs store children as `json.RawMessage`, so comparison would still
require JSON parsing of children. The recursion is essentially the same as Approach A.
Additionally, this only works for patterns where the root kind is known. Approach A's generic
JSON comparison is more flexible and simpler to implement.

**Decision**: Approach A for ast_find. Generic JSON field comparison is the right abstraction.

---

## 2026-05-20 - Recommendation (Tier 2)

### Chosen Approaches

**ast_rename**: Approach A — astutil.Apply walk, rename all `*ast.Ident` matching oldName.
Obtain oldName from selector.Navigate + extract from declaration node. Document approximation.

**ast_node_at**: Approach A — astutil.Apply walk collecting (node, parent, field, index) tuples
for all nodes whose range contains the target offset. Find innermost, reconstruct path using a
`nodeToPathStep` reverse-mapper. Return path to nearest fully-addressable ancestor.

**ast_find_symbols**: Approach A — os.ReadDir + parse each .go file + scan f.Decls + glob match.
Case-insensitive matching by lowercasing both pattern and name.

**ast_find**: Approach A — generic JSON map comparison. astutil.Apply walk, MarshalNode each
node, recursive pattern match via map[string]json.RawMessage comparison.

---

### Implementation Strategy

#### File: `ops/rename.go`

```go
// ASTRenameArgs
type ASTRenameArgs struct {
    File   string          `json:"file"`
    Path   json.RawMessage `json:"path"`   // path to declaration site
    To     string          `json:"to"`
    DryRun bool            `json:"dry_run"`
}

func HandleASTRename(ctx, req, args) (*mcp.CallToolResult, error) {
    // 1. Parse file
    // 2. Navigate to declaration node to extract oldName
    //    - FuncDecl: fd.Name.Name
    //    - TypeSpec: ts.Name.Name
    //    - ValueSpec: vs.Names[0].Name (first name)
    //    - Field: field.Names[0].Name
    //    - Ident directly: id.Name
    // 3. editor.Edit: astutil.Apply, replace all *ast.Ident{Name==oldName}
    //    with *ast.Ident{Name: args.To}
    //    NOTE: also rename the declaration ident itself
    // 4. Return diff
}
```

**Rename of declaration ident**: The declaration node's own Ident (e.g., `funcDecl.Name`)
must also be renamed. Since we rename ALL matching Idents including that one, it's handled
automatically — no special case needed.

**Key subtlety**: For `selector.Navigate` to work, the file must be parsed fresh in `Edit`.
Navigate the parsed-in-edit file, not the pre-edit file. The pattern in ops/replace.go:
navigate inside the `editor.Edit` callback.

#### File: `ops/lsp.go`

Three handlers in one file:

**HandleASTNodeAt**:
```go
type ASTNodeAtArgs struct {
    File string `json:"file"`
    Line int    `json:"line"`
    Col  int    `json:"col"`   // 1-based column
}

// Response:
type ASTNodeAtResponse struct {
    Path []selector.PathStep `json:"path"`
    Node json.RawMessage     `json:"node"`
    Meta meta.Meta           `json:"meta"`
}
```

Implementation:
1. ParseFile → f, fset, src
2. `tokenFile := fset.File(f.Pos())`; bounds-check line vs `tokenFile.LineCount()`
3. `lineStart := tokenFile.LineStart(line)`; `targetOffset := fset.Position(lineStart).Offset + (col - 1)`
4. Walk AST with astutil.Apply, collect `nodeInfo{node, parent, fieldName, index}` for all
   nodes where `fset.Position(n.Pos()).Offset <= targetOffset < fset.Position(n.End()).Offset`.
   Track innermost (smallest End-Pos range).
5. From innermost node, walk up through collected nodeInfo chain building PathSteps.
6. MarshalNode the innermost node, Compute meta.

**HandleASTFindSymbols**:
```go
type ASTFindSymbolsArgs struct {
    Dir   string   `json:"dir"`
    Query string   `json:"query"`           // glob pattern, e.g. "Handle*"
    Kinds []string `json:"kinds,omitempty"` // filter by kind: FuncDecl, TypeSpec, etc.
}

type SymbolMatch struct {
    File string              `json:"file"`
    Path []selector.PathStep `json:"path"`
    Kind string              `json:"kind"`
    Name string              `json:"name"`
    Recv string              `json:"recv,omitempty"`
    Meta meta.Meta           `json:"meta"`
}
```

Implementation:
1. os.ReadDir(args.Dir)
2. For each .go file: editor.ParseFile
3. Walk f.Decls:
   - FuncDecl: name = fd.Name.Name, recv = recvTypeString(...)
   - GenDecl type: each TypeSpec.Name.Name
   - GenDecl var/const: each ValueSpec.Names
4. `path.Match(strings.ToLower(query), strings.ToLower(name))`
5. Filter by args.Kinds if non-empty
6. Build path: `[]selector.PathStep{{Kind: "FuncDecl", Name: name}}` etc.
7. meta = meta.Compute(fset, src, node, nil, 1)

**HandleASTFind**:
```go
type ASTFindArgs struct {
    File    string          `json:"file"`
    Dir     string          `json:"dir,omitempty"`
    Pattern json.RawMessage `json:"pattern"` // partial node tree
}

type FindMatch struct {
    Path []selector.PathStep `json:"path"`
    Node json.RawMessage     `json:"node"`
    Meta meta.Meta           `json:"meta"`
}
```

Implementation (single file, scope="file" default):
1. ParseFile
2. Unmarshal pattern to `map[string]json.RawMessage`
3. astutil.Apply walk, collecting (node, parent, fieldName, index)
4. For each node: kinds.MarshalNode → unmarshal to map → matchPattern(patternMap, actualMap)
5. On match: build PathSteps from ancestor chain, MarshalNode, Compute meta
6. Collect all matches, return sorted by source order

For `dir` scope: iterate .go files in dir, run per-file, aggregate results.

#### File: server.go (additions)

Register 4 new tools:
- `ast_rename` with file, path, to, dry_run
- `ast_node_at` with file, line, col
- `ast_find_symbols` with dir, query, kinds (optional array)
- `ast_find` with file (or dir), pattern (object)

---

### Key Implementation Decisions

**Decision 1: ast_rename extracts oldName inside editor.Edit callback.**
Navigate the freshly-parsed AST (inside the edit fn), not a pre-parsed one. This avoids
any position-invalidation issues from re-parsing.

**Decision 2: ast_find uses generic JSON map comparison, not typed Node comparison.**
No need to know node-type-specific field semantics. Works for all 50 kinds uniformly.
Null pattern fields vs absent pattern fields are both treated as wildcards.

**Decision 3: ast_node_at path reconstruction stops at selector-vocabulary boundary.**
If the innermost node is not directly addressable by the selector (e.g., it's inside a
FieldList or a position marker), return the nearest addressable ancestor and note the
limitation in the response. This is better than returning an invalid path.

**Decision 4: ast_find_symbols uses path.Match for glob, lowercase for case-insensitivity.**
Standard library, zero dependencies. `"*"` matches any sequence of non-separator characters.
For symbols, there are no path separators, so `*` matches any substring effectively.

**Decision 5: For ast_find with dir scope, process files sequentially.**
No goroutines. Sequential is simpler, sufficient for typical directory sizes (< 50 files).

**Decision 6: Reuse the same nodeInfoChain approach for both ast_node_at and ast_find.**
Both need to reconstruct a path from a found node back to the file root. Extract a shared
`buildPath(ancestors []nodeAncestor) []selector.PathStep` helper.

---

### Risks and Mitigations

**Risk 1: ast_node_at path reconstruction is complex.**
The nodeToPathStep reverse-mapper needs ~150 lines of type-switch coverage. Missing cases
return empty path rather than panicking. Test with diverse node types in ops_test.go.

**Risk 2: ast_rename over-renames in shadow scenarios.**
Document explicitly: "AST-only rename — renames all identifiers with the same name in the
file regardless of scope. Use go/types-based rename (Tier 3) for precise cross-scope rename."

**Risk 3: ast_find performance on large files.**
MarshalNode on every node in a 5000-line file could be slow. Profiling baseline: a typical
Go file has ~2000-5000 AST nodes; MarshalNode is ~10µs per node → ~20-50ms total. Acceptable.
If too slow, add a pre-filter on node Kind before MarshalNode (cheapest check: node type
assertion matches the pattern's kind field).

**Risk 4: ast_find_symbols ReadDir on large directories.**
ReadDir is O(files). Parsing each .go file adds ~1ms per file. For 100 files → 100ms.
Acceptable. No parallel parsing needed for typical directories.

**Risk 5: tokenFile.LineStart panics on out-of-range line.**
Bounds-check: `if line < 1 || line > tokenFile.LineCount() { return toolError(...) }`.
Also check col bounds (1 to line length).

---

### Open Questions

1. **ast_find pattern matching: should array fields require same-length match or allow
   subset matching?** E.g., if pattern has `"args": [{"kind":"Ident","name":"x"}]` — does
   this match a CallExpr with 3 args where the first is `x`? Or only a CallExpr with exactly
   1 arg named `x`? Recommendation: require exact array length match for simplicity. Wildcard
   = absent field, not a subset-match array. Document this.

2. **ast_find_symbols case sensitivity**: Recommendation: case-insensitive by default (lowercase
   both sides). The query `"Handle*"` should match `"handleRequest"` and `"HandleResponse"`.

3. **ast_node_at col is 1-based or 0-based?** Convention in editors is 1-based. The meta
   package uses 1-based (`pos.Column`). Use 1-based throughout, subtract 1 when computing
   byte offset: `targetOffset = fset.Position(lineStart).Offset + (col - 1)`.

---

## 2026-05-20 - BRAINSTORM COMPLETE

**Status:** Complete
**Recommendation:** Four tools in two files (ops/rename.go, ops/lsp.go):
  - ast_rename: astutil.Apply walk, rename all Ident nodes matching oldName (AST approximation)
  - ast_node_at: LineStart+col byte offset, innermost node walk, path reverse-reconstruction
  - ast_find_symbols: os.ReadDir + parse + scan f.Decls + path.Match glob
  - ast_find: astutil.Apply + MarshalNode + recursive JSON map comparison (absent=wildcard)
**Next Phase:** PLAN

Key technical decisions for the implementer:
1. ast_rename extracts oldName via selector.Navigate inside editor.Edit callback (fresh parse)
2. ast_rename renames ALL *ast.Ident with matching name — document scope approximation
3. ast_node_at uses token.FileSet.File(f.Pos()).LineStart(line) for line→offset; bounds-check first
4. ast_node_at path reconstruction: collect (node, parent, fieldName, index) with astutil.Apply,
   then reverse-walk to build []selector.PathStep using a nodeToPathStep mapper
5. ast_find uses generic map[string]json.RawMessage comparison — no node-type-specific logic
6. ast_find array fields require exact-length match (not subset); absent field = wildcard
7. ast_find_symbols: path.Match + lowercase both sides for case-insensitive glob
8. Shared path reconstruction helper reused by ast_node_at and ast_find
9. Register all 4 tools in server.go

---

## 2026-05-20 10:53:27 - Tier 3 Research: Type-Aware LSP Operations

### Task
Implement 3 tools using go/types: ast_find_refs, ast_find_def, ast_find_impls.
All use go/packages.Load for package-scope type resolution.

---

## 2026-05-20 10:53:27 - Research Findings

### Question 1: go/packages LoadMode flags

**Required flags** (confirmed by experiment):
```go
packages.NeedTypesInfo | packages.NeedTypes | packages.NeedSyntax | packages.NeedFiles | packages.NeedImports | packages.NeedName
```

- `NeedTypesInfo` — provides `TypesInfo.Uses` and `TypesInfo.Defs` maps
- `NeedTypes` — provides `pkg.Types` (`*types.Package`) needed for `scope.Lookup()` and `types.Implements()`
- `NeedSyntax` — provides `pkg.Syntax` (`[]*ast.File`), needed to walk AST and match idents
- `NeedFiles` — provides `pkg.GoFiles` (file path list), useful for resolving filenames
- `NeedImports` — needed so that cross-package `Uses` entries have their objects fully resolved
- `NeedName` — provides `pkg.Name` and `pkg.PkgPath`

`NeedFiles` is technically optional but costs little and helps with filename normalization.

**Loading pattern** — always use `"."` with `Dir` set:
```go
cfg := &packages.Config{
    Mode: packages.NeedTypesInfo | packages.NeedTypes | packages.NeedSyntax |
          packages.NeedFiles | packages.NeedImports | packages.NeedName,
    Dir: filepath.Dir(filePath),
}
pkgs, err := packages.Load(cfg, ".")
```

**Do NOT use `"file="+filePath`**: Both patterns produce identical results in practice (confirmed experimentally), but `"."` + `Dir` is cleaner — it loads the whole package the file belongs to, which is exactly what we need for package-scope operations. The `file=` pattern also works but semantically means "the package containing this specific file", which is the same result but less explicit.

**Latency**: ~50ms per call on a small package (ops/, ~800 lines, 8 files). This is acceptable for tool invocations but prohibits calling per-identifier. Call once per tool handler, use the result. No in-process caching needed — each MCP tool call is independent and 50ms is well within interactive tolerance.

### Question 2: ast_find_refs implementation

**File scope (fast, AST-only)**:
1. Navigate to node with `selector.Navigate(f, steps)`
2. Extract declaration name string via `extractDeclName(node)` (already exists in `rename.go`)
3. Walk AST with `astutil.Apply`, collect all `*ast.Ident` where `ident.Name == declName`
4. Return as list of `{file, path, kind, line}` using `buildPath()` (already in `lsp.go`)

Note: File-scope find_refs is approximate (same as `ast_rename`) — it matches by name string, not by type system. Document this limitation in the tool description.

**Package scope (type-accurate)**:
1. Navigate to node, extract name ident's position (line, col) from editor's fset
2. Call `loadPackage(filePath)` to get `*packages.Package`
3. Scan `pkg.TypesInfo.Defs` to find the `*ast.Ident` at that file:line:col → get `declObj types.Object`
4. Walk all files in `pkg.Syntax`, check `pkg.TypesInfo.Uses[ident] == declObj`
5. For each matching ident, build the path in that file's AST

**Extracting declaration name ident from a path**: The path navigation returns an `ast.Node`. The name ident is extracted differently per node type:
```go
func extractNameIdent(node ast.Node) (*ast.Ident, bool) {
    switch n := node.(type) {
    case *ast.FuncDecl:  return n.Name, true
    case *ast.TypeSpec:  return n.Name, true
    case *ast.Field:     if len(n.Names) > 0 { return n.Names[0], true }
    case *ast.ValueSpec: if len(n.Names) > 0 { return n.Names[0], true }
    case *ast.Ident:     return n, true
    }
    return nil, false
}
```

**Bridge between editor fset and pkg.Fset**: `editor.ParseFile` and `packages.Load` use independent `token.FileSet` instances — `token.Pos` values are NOT comparable across them. The bridge is the textual position `(filename, line, column)`:
- Get position from editor fset: `editorFset.Position(nameIdent.Pos())` → `{Filename, Line, Column}`
- In pkg.Syntax, find matching ident: `pkg.Fset.Position(ident.Pos()).Line == targetLine && .Column == targetCol`

This was confirmed experimentally — both fsets produce the same line/col for the same source location.

### Question 3: ast_find_def implementation

**Given file + line + col (from path navigation)**:
1. Navigate to node via path, extract the target name ident
2. Get its position from editor fset
3. `loadPackage(filePath)` to get `*packages.Package`
4. Find matching `*ast.Ident` in `pkg.Syntax` at that position
5. Look up `pkg.TypesInfo.Uses[ident]` to get the `types.Object`
6. If nil, it's a declaration site — no definition to jump to (or return self)
7. If non-nil, `obj.Pos()` gives the definition position

**Handling built-ins and external symbols**:
- Built-in functions (`len`, `cap`, `make`, etc.): `obj.Pos().IsValid()` returns `false`. Return `{"builtin": true, "name": "len"}` instead of a file location.
- External package symbols (e.g., `fmt.Println`): `obj.Pos().IsValid()` is `true`, and `pkg.Fset.Position(obj.Pos()).Filename` points to a file in the Go module cache (e.g., `$GOPATH/pkg/mod/...`). Return `{"external": true, "package": obj.Pkg().Path(), "name": obj.Name()}` if the file is not within the project directory, or return the file path if it's readable.
- Same-package symbols: `obj.Pos().IsValid()` is `true` and `Filename` is within the package dir. Return `{file, line, col}`.

Confirmed: `pkg.Fset.Position(builtinObj.Pos())` for `len` returns `{Filename: "", Line: -1, Column: -1}` (invalid position).

### Question 4: ast_find_impls implementation

**From TypeSpec path → *types.Interface**:
```go
// 1. Navigate to TypeSpec
node, _, _ := selector.Navigate(f, steps)  // returns *ast.TypeSpec
ts := node.(*ast.TypeSpec)

// 2. Get position of name ident
namePos := editorFset.Position(ts.Name.Pos())

// 3. In pkg.TypesInfo.Defs, find object at that position
var ifaceType *types.Interface
for ident, obj := range pkg.TypesInfo.Defs {
    p := pkg.Fset.Position(ident.Pos())
    if p.Line == namePos.Line && p.Column == namePos.Column {
        underlying := obj.Type().Underlying()
        if iface, ok := underlying.(*types.Interface); ok {
            ifaceType = iface
        }
        break
    }
}
```

**Walking implementors**:
```go
scope := pkg.Types.Scope()
for _, name := range scope.Names() {
    obj := scope.Lookup(name)
    typeName, ok := obj.(*types.TypeName)
    if !ok || typeName.IsAlias() {
        continue
    }
    T := typeName.Type()
    // Check both value and pointer receivers
    if types.Implements(T, ifaceType) || types.Implements(types.NewPointer(T), ifaceType) {
        // Found an implementor
    }
}
```

Experimentally confirmed:
- `Cat` (value receiver `String()`) → `types.Implements(Cat, Stringer) = true`
- `Dog` (pointer receiver `*Dog.String()`) → `types.Implements(Dog, Stringer) = false`, `types.Implements(*Dog, Stringer) = true`
- The interface itself (`Stringer`) → `types.Implements(Stringer, Stringer) = true` — filter this out
- `Fish` (no method) → both `false`

**Important**: Also check `typeName.Type() == ifaceType.Underlying()` to skip the interface itself — or simply skip `*types.Interface` underlying types.

### Question 5: loadPackage shared helper

```go
// loadPackage loads the Go package containing filePath using go/packages.
// Returns the first package with no errors, or an error if loading fails.
func loadPackage(filePath string) (*packages.Package, *token.FileSet, error) {
    cfg := &packages.Config{
        Mode: packages.NeedTypesInfo | packages.NeedTypes | packages.NeedSyntax |
              packages.NeedFiles | packages.NeedImports | packages.NeedName,
        Dir: filepath.Dir(filePath),
    }
    pkgs, err := packages.Load(cfg, ".")
    if err != nil {
        return nil, nil, fmt.Errorf("packages.Load: %w", err)
    }
    if len(pkgs) == 0 {
        return nil, nil, fmt.Errorf("no packages found in %s", filepath.Dir(filePath))
    }
    pkg := pkgs[0]
    // Collect non-fatal package errors (type errors etc.) but still return the package
    // because TypesInfo is often populated even with type errors
    var errMsgs []string
    for _, e := range pkg.Errors {
        errMsgs = append(errMsgs, e.Msg)
    }
    if pkg.TypesInfo == nil {
        return nil, nil, fmt.Errorf("type info not available (errors: %v)", errMsgs)
    }
    return pkg, pkg.Fset, nil
}
```

Note: Return `pkg.Fset` separately — the caller needs it to convert `token.Pos` values to file positions.

### Question 6: Testability strategy

**Decision: Use testdata/typesdata/ as a mini Go module.**

Confirmed working:
```
testdata/typesdata/
  go.mod     (module example.com/typesdata; go 1.21)
  types.go   (package typesdata — interfaces, structs, functions)
```

`packages.Load` with `Dir: "testdata/typesdata"` successfully loads and type-checks this mini module in 40-50ms.

**Why NOT use goast module itself as test target**:
1. Tests would depend on goast's own source structure — brittle (renames, moves break tests)
2. Circular: testing the tool that analyzes goast using goast
3. Slower: loading all of ops/ (8 files) is slower than a tiny module
4. Harder to control: can't add a deliberate "Fish doesn't implement Stringer" to goast source

**Why testdata/typesdata/ is better**:
1. Stable: the mini module content is test-controlled
2. Intentional: can add interfaces, implementors, non-implementors deliberately
3. Fast: tiny module loads quickly
4. Isolated: independent go.mod means no coupling to main module's dependencies

**The writeTempModule helper** for tests:
```go
// writeTempModule creates a temporary Go module with the given source file.
// Returns the directory containing the module.
func writeTempModule(t *testing.T, source string) string {
    t.Helper()
    dir := t.TempDir()
    if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/typetest\n\ngo 1.21\n"), 0644); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(source), 0644); err != nil {
        t.Fatal(err)
    }
    return dir
}
```

Tests then use `filePath := filepath.Join(dir, "types.go")` as input.

**Alternative: pre-committed testdata/typesdata/** — commit a fixed mini module so tests don't regenerate it on every run. This is slightly faster (avoids WriteFile) but both approaches are fine.

**Recommendation**: Commit `testdata/typesdata/` as a fixed mini module. Its content can be expanded as new test scenarios are needed. Use `writeTempModule` for tests that need custom source not covered by the fixed fixtures.

---

## 2026-05-20 10:53:27 - Approaches Considered (Tier 3)

### Approach 1: Separate ops/types.go file, shared loadPackage helper

Implement all 3 tools in a new `ops/types.go` file following the exact pattern of `ops/lsp.go`. Export handler functions (`HandleASTFindRefs`, `HandleASTFindDef`, `HandleASTFindImpls`). Keep `loadPackage` as an unexported helper in the same file.

**Pros:**
- Follows existing file layout (one concern per file)
- Shared `loadPackage` is co-located with its users
- Easy to add a `typesdata` test fixture alongside `ops/types_test.go`
- Consistent with `ops/lsp.go`, `ops/rename.go` pattern

**Cons:**
- `loadPackage` is only used by this one file (not truly shared across files)
- All 3 tools in one file may get large (~400-500 lines)

**Fits existing patterns:** Yes — `ops/lsp.go` is ~600 lines with 4 tools.

### Approach 2: Split into ops/refs.go, ops/def.go, ops/impls.go with shared ops/pkgload.go

Three separate files for the three tools. A fourth file `ops/pkgload.go` exports the `loadPackage` helper.

**Pros:**
- Finer granularity
- `loadPackage` clearly shared

**Cons:**
- Unnecessary fragmentation for 3 closely related tools
- More files to navigate
- Against the pattern (existing tools are grouped by feature, not split per-operation)

**Fits existing patterns:** No — existing grouping is by feature cluster.

### Approach 3: Single ops/types.go with inlined package loading

No shared helper — each handler calls `packages.Load` directly inline.

**Pros:**
- Maximum simplicity
- No helper function abstraction

**Cons:**
- Duplication of ~10 lines of packages.Load setup in 3 places
- Error handling inconsistency risk
- Harder to update LoadMode flags

**Fits existing patterns:** No — existing code extracts helpers (toolError, navError, buildPath, etc.)

---

## 2026-05-20 10:53:27 - Recommendation (Tier 3)

### Chosen Approach: Approach 1 — Single ops/types.go + shared loadPackage

**Rationale:**
- Matches `ops/lsp.go` pattern exactly: one file, multiple related tools, shared unexported helpers
- `loadPackage` is a natural private helper alongside its 3 callers
- Single `ops/types_test.go` using `testdata/typesdata/` fixture module
- Keeps the diff reviewable in one place

**Implementation Strategy:**

1. **Create `testdata/typesdata/`** — mini Go module with:
   - `Stringer` interface (value method)
   - `Dog` (pointer receiver), `Cat` (value receiver), `Fish` (no impl)
   - `Add`, `Subtract`, `Call` functions (where `Call` uses `Add` — for find_refs)
   - Commit it; tests reference `../../testdata/typesdata/types.go`

2. **Implement `loadPackage`** in `ops/types.go`:
   - Signature: `loadPackage(filePath string) (*packages.Package, error)`
   - `pkg.Fset` is returned via `pkg` directly (`pkg.Fset` is a field on `*packages.Package`)
   - Mode: `NeedTypesInfo | NeedTypes | NeedSyntax | NeedFiles | NeedImports | NeedName`
   - Error if `pkg.TypesInfo == nil`

3. **Implement `extractNameIdentFromPath`** in `ops/types.go`:
   - Parse file, navigate path, extract `*ast.Ident` name and its `fset.Position()`
   - Returns `(identName string, line int, col int, err error)`

4. **Implement `findIdentInPkg`** — given `(pkg, filename, line, col)`, scan `pkg.Syntax` to find matching `*ast.Ident`:
   - Match on `filepath.Base(filename)` for robustness (temp dirs vs absolute paths)
   - Actually: use full path match when available, fall back to base name
   - Returns `(*ast.Ident, bool)`

5. **Implement `HandleASTFindRefs`**:
   - File scope: `extractDeclName(node)` + `astutil.Apply` name-string match
   - Package scope: `loadPackage` + `findIdentInPkg` + `TypesInfo.Defs[ident]` + walk all syntax files checking `TypesInfo.Uses[ident] == declObj`

6. **Implement `HandleASTFindDef`**:
   - `loadPackage` + `findIdentInPkg` + `TypesInfo.Uses[ident]`
   - If `obj.Pos().IsValid()`: return file + line + col + path reconstruction
   - If `!obj.Pos().IsValid()`: return `{"builtin": true, "name": obj.Name()}`
   - Also handle: `TypesInfo.Defs[ident]` for the case where the path points at a declaration (return self)

7. **Implement `HandleASTFindImpls`**:
   - `loadPackage` + `findIdentInPkg` + `TypesInfo.Defs[ident]`
   - `obj.Type().Underlying().(*types.Interface)` — error if not interface
   - Walk `pkg.Types.Scope().Names()`, check `types.Implements(T, iface)` and `types.Implements(types.NewPointer(T), iface)`
   - Skip the interface type itself from results

8. **Register all 3 in server.go**

**Key Decisions:**

- **fset bridge via line+col**: Both `editor.ParseFile`'s fset and `packages.Load`'s `pkg.Fset` are independent; bridge via textual position `(line, col)` confirmed working.
- **File-scope find_refs is name-only**: Document in tool description. Same approximation as `ast_rename`.
- **Package-scope includes the declaration site**: Optionally include the declaration in find_refs results (design.md is silent; include it with a `"is_decl": true` marker).
- **`loadPackage` returns `*packages.Package` not `(*packages.Package, *token.FileSet)`**: `pkg.Fset` is a public field on `packages.Package`, no need to return separately.
- **50ms latency is acceptable**: Per design.md's constraint "package scope = use go/packages, accept it".
- **testdata/typesdata is a committed fixture**: Avoids per-test module creation overhead.

**Risks Identified:**

- **File in temp dir**: `writeTempFile` puts files in OS temp dirs — `packages.Load` with `Dir: filepath.Dir(tempFile)` will fail because there's no `go.mod` in the temp dir. Tier 3 tests MUST use `writeTempModule` (dir + go.mod). File-scope tests can still use `writeTempFile`. 
  Mitigation: Use `testdata/typesdata/` as primary fixture; only use `writeTempModule` for custom scenarios.

- **pkg.Errors with TypesInfo present**: `packages.Load` may return non-nil errors (e.g., import cycle, type errors) but still populate `TypesInfo`. Check `pkg.TypesInfo != nil` before proceeding; log errors but don't fail.

- **Column numbers in fset bridge**: Confirmed working for top-level declarations (col 6 for `func F`, col 2 for struct fields). Edge case: generated code or files with non-UTF8 content. Low risk for the target use case.

- **`selector.Navigate` uses editor's fset**: The `editor.ParseFile` parse is separate from `packages.Load`. This adds ~1-2ms for a second parse of the same file. Acceptable.

**Open Questions:**

- Should `ast_find_def` also check `pkg.TypesInfo.Defs[ident]` (i.e., return self if the path IS the declaration)? Recommendation: yes — return `{"is_decl": true, ...}` to avoid confusing "no definition found" with "this is the definition".

---

## 2026-05-20 10:53:27 - BRAINSTORM COMPLETE (Tier 3)

**Status:** Complete
**Recommendation:** Single ops/types.go + shared loadPackage + testdata/typesdata/ mini-module
**Next Phase:** PLAN

Ready for workflow-planner agent to create detailed implementation plan for Tier 3 tools.

---

## 2026-05-20 - Task Received (Module Rename + CLI Daemon)

grv — rename Go module from github.com/lthiery/goast → github.com/mattdurham/grv, rename binary
from goast → grv, and add a CLI with daemon architecture:
- grv (no args / piped stdin) → stdio MCP server (existing behavior)
- grv start [dir] → start per-CWD daemon in background
- grv stop [dir] → stop daemon
- grv status → show running daemons
Daemon serves MCP tools over Unix socket with 1-hour idle timeout.
Constraint C (from brainstorm-prompt.md): daemon is infrastructure for future caching; existing
MCP clients use stdio as before.

Starting brainstorm process...

---

## 2026-05-20 - Research Findings (Module Rename + CLI Daemon)

### Import Path Scope

24 Go files contain the old import path `github.com/lthiery/goast`:

```
server.go
ops/lsp_test.go       ops/rename.go         ops/rename_test.go
editor/editor_test.go ops/replace.go        ops/gomod.go
kinds/golden_test.go  editor/editor.go      ops/types.go
ops/types_test.go     ops/delete.go         kinds/kinds_test.go
ops/readonly_test.go  ops/file_test.go      meta/meta_test.go
ops/imports.go        ops/directory_test.go selector/selector_test.go
ops/query.go          ops/file.go           ops/lsp.go
ops/insert.go         ops/ops_test.go
```

All are internal package imports (e.g., `github.com/lthiery/goast/ops`, `.../kinds`,
`.../selector`). No external callers. The rename is a pure mechanical substitution:
`go mod edit -module github.com/mattdurham/grv` + sed over 24 files.

go.mod current module: `github.com/lthiery/goast`, go version 1.25.5.

### mcp-go v0.54.0: StdioServer.Listen API

The key discovery: `ServeStdio` is a convenience wrapper. The underlying method is:

```go
func (s *StdioServer) Listen(ctx context.Context, stdin io.Reader, stdout io.Writer) error
```

This means serving MCP over a Unix socket connection is straightforward:
```go
stdioServer := server.NewStdioServer(mcpServer)
conn, _ := listener.Accept()
go stdioServer.Listen(ctx, conn, conn)  // conn implements both io.Reader and io.Writer
```

No custom JSON-RPC loop needed. The existing StdioServer handles all protocol framing.

One important limitation: `stdioSession` is a package-level variable (not per-connection) in
the current mcp-go implementation. This means the StdioServer is designed for one concurrent
client. For the daemon use case (one connection at a time from the MCP client), this is fine.

### Current main.go / server.go

`main.go` (19 lines): creates MCPServer, calls RegisterTools, calls ServeStdio.
`server.go` (~280 lines): RegisterTools function registering 25 tools via mcp.NewTypedToolHandler.

The binary is named by `go build -o goast` or by module path inference. No hardcoded binary name
in source. Rename just requires changing the module path and the string literals "goast" in
main.go (server name, log prefix).

### go.mod Dependencies

No cobra, no CLI framework. Current deps: mark3labs/mcp-go, google/jsonschema-go, google/uuid,
pmezard/go-difflib, santhosh-tekuri/jsonschema, spf13/cast, yosida95/uritemplate,
golang.org/x/{mod,sync,text,tools}.

All standard library packages needed for the daemon (os/exec, net, syscall, os/signal) are
available without new dependencies.

### Spec-Driven Modules

No SPECS.md, NOTES.md, TESTS.md, BENCHMARKS.md, or CLAUDE.md present in any directory.
No spec constraints. No NOTE invariants in .go files.

### mcp-go StdioServer Limitation: stdioSessionInstance

Inspecting the Listen code:
```go
// line 539-540 of stdio.go:
if err := s.server.RegisterSession(ctx, &stdioSessionInstance); err != nil {
```

`stdioSessionInstance` is a package-level `stdioSession` variable — not a local. This means
concurrent calls to `Listen` on the same `StdioServer` would share session state. For the
daemon pattern (one MCP client connecting via socket, one connection active at a time), this
is not a problem. If concurrent connections were needed, we'd need to create a new
`StdioServer` per connection. For now: accept one connection at a time.

---

## 2026-05-20 - Approaches Considered (Module Rename + CLI Daemon)

### Approach 1: Manual dispatch on os.Args (Recommended)

4 subcommands: no args (stdio), start, stop, status.
No external CLI framework. Dispatch on `os.Args[1]` with a switch statement.

```go
func main() {
    if len(os.Args) < 2 || isStdioMode() {
        runStdio()
        return
    }
    switch os.Args[1] {
    case "start":  runStart(os.Args[2:])
    case "stop":   runStop(os.Args[2:])
    case "status": runStatus()
    case "daemon": runDaemon(os.Args[2:])  // internal, called by start
    default:       runStdio()  // unknown arg → fall through to stdio
    }
}
```

`isStdioMode()` checks `!term.IsTerminal(int(os.Stdin.Fd()))` to detect piped stdin (MCP client
spawning the process). This means `grv` with no args on a terminal shows usage; `grv` with
piped stdin acts as stdio MCP server (backward-compatible with existing MCP configs).

**Pros:**
- Zero new dependencies
- Fits in ~250 lines of new code in main.go + cmd/ package
- Perfectly adequate for 4 subcommands
- Easy to read and maintain

**Cons:**
- No auto-generated --help formatting
- Manual flag parsing for start/stop (just --dir)

**Fits existing patterns:** Yes — minimal dependencies philosophy matches the project.

### Approach 2: Cobra

Use `github.com/spf13/cobra` for subcommand routing.

**Pros:**
- Auto-generated help text
- Standard --flag parsing

**Cons:**
- New dependency (cobra is heavy: pflag, etc.)
- Overkill for 4 subcommands
- The project already avoids heavy dependencies

**Fits existing patterns:** No.

### Approach 3: Subpackages per command (cmd/start/, cmd/stop/, etc.)

Organize each subcommand in its own subpackage.

**Pros:**
- Good separation for larger CLIs

**Cons:**
- Overkill for 4 simple commands
- More files to navigate
- The logic is simple enough to live in one cmd/daemon.go file

**Fits existing patterns:** Partially.

---

### Daemon Socket Path Design

**Decision: `~/.grv/<base58(sha256(absdir))[:8]>.sock`**

Rationale:
- Same binary serves multiple directories — need to disambiguate
- 8 chars of base58(sha256) gives 48 bits of entropy — collision probability negligible
- Files: `.sock`, `.pid`, `.log` — all in `~/.grv/`

Implementation:
```go
func grvDir() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".grv")
}

func socketHash(absdir string) string {
    h := sha256.Sum256([]byte(absdir))
    // base58 encode first 6 bytes → 8 chars
    return base58Encode(h[:6])
}

func socketPath(absdir string) string {
    return filepath.Join(grvDir(), socketHash(absdir)+".sock")
}
```

For base58: implement a minimal encoder (alphabet is standard Bitcoin base58: 123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz). ~20 lines of code. No dependency.

Alternative: hex encoding. Simpler but longer (12 hex chars vs 8 base58). Either works. Use
hex for simplicity — 8 hex chars (32 bits of hash prefix) is sufficient for personal use.

**Final decision: hex encoding of first 4 bytes of sha256** — no base58 dependency, human-readable,
sufficient uniqueness: `fmt.Sprintf("%x", sha256.Sum256([]byte(absdir))[:4])` → 8 hex chars.

---

### MCP over Unix Socket

Using `StdioServer.Listen(ctx, conn, conn)`:

```go
// In daemon mode:
ln, err := net.Listen("unix", sockPath)
mcpSvr := server.NewMCPServer("grv", "0.1.0", server.WithToolCapabilities(false))
RegisterTools(mcpSvr)
stdioSvr := server.NewStdioServer(mcpSvr)

for {
    conn, err := ln.Accept()
    if err != nil { break }
    go func(c net.Conn) {
        defer c.Close()
        touchActivity()  // reset idle timer
        stdioSvr.Listen(ctx, c, c)
    }(conn)
}
```

Because `stdioSessionInstance` is package-level, we should serialize connections rather than
serve concurrently. Simplest approach: accept one at a time (no goroutine for Accept):

```go
for {
    conn, err := ln.Accept()
    if err != nil { break }
    touchActivity()
    stdioSvr.Listen(ctx, conn, conn)  // blocks until client closes
    conn.Close()
}
```

This is correct for stdio MCP protocol — MCP clients connect, do their work, disconnect.
No concurrent sessions needed.

Note: `server.NewStdioServer` needs to be called once, outside the loop. But the
`stdioSessionInstance` package var means state persists across connections (e.g., initialized
flag). Check if `stdioSession.Initialize()` is idempotent — it stores via `atomic.Bool.Store`,
so repeated initialization is safe.

---

### Daemon Start / Re-exec Pattern

Go has no `fork()`. Standard pattern: re-exec with a sentinel subcommand.

```
grv start [dir]
  1. Compute absdir (default: cwd)
  2. Check pidFile — if exists and process alive: "daemon already running"
  3. os.MkdirAll(grvDir())
  4. Open logFile for append
  5. cmd := exec.Command(os.Args[0], "daemon", "--socket", sockPath, "--dir", absdir)
     cmd.Stdout = logFile
     cmd.Stderr = logFile
     cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}  // detach from terminal
  6. cmd.Start()  // returns immediately, no Wait
  7. Write cmd.Process.Pid to pidFile
  8. Wait up to 1 second for sockPath to appear (poll every 50ms)
  9. Report success or timeout

grv daemon --socket SOCK --dir DIR
  1. os.MkdirAll(grvDir())
  2. net.Listen("unix", sockPath)
  3. Idle timeout goroutine
  4. Signal handler for SIGTERM → cleanup + exit
  5. Accept loop
```

`Setsid: true` creates a new session, detaching the daemon from the controlling terminal.
The daemon's stdin is inherited as nil (cmd.Stdin = nil → /dev/null effectively).

**PID file race condition**: `grv start` writes the PID after `cmd.Start()`. The daemon itself
could also write the PID from inside. Simpler: have `grv start` write the PID (it has the Pid
from `cmd.Process.Pid`). No need for daemon to write its own PID.

**Is-alive check for PID file**: `syscall.Kill(pid, 0)` returns nil if process exists,
`os.ErrProcessDone` or `syscall.ESRCH` if dead. Use this in `grv stop` and `grv status`.

---

### Idle Timeout

```go
var lastActivity atomic.Int64  // Unix nanoseconds

func touchActivity() {
    lastActivity.Store(time.Now().UnixNano())
}

func idleWatcher(timeout time.Duration) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        last := time.Unix(0, lastActivity.Load())
        if time.Since(last) > timeout {
            log.Println("idle timeout reached, exiting")
            os.Exit(0)
        }
    }
}
```

`touchActivity()` is called:
1. On each new connection accepted
2. Optionally: wrapped around each tool handler via server middleware

For option 2, mcp-go v0.54.0 has `s.Use(mw ...ToolHandlerMiddleware)`. A middleware that
calls `touchActivity()` on each tool call is clean:

```go
mcpSvr.Use(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        touchActivity()
        return next(ctx, req)
    }
})
```

Check the `ToolHandlerMiddleware` type in mcp-go — if available, use it. If not, touching
per-connection is sufficient (any connection resets the timer).

---

### grv stop

```go
func runStop(args []string) {
    absdir := resolveDir(args)
    sockPath := socketPath(absdir)
    pidPath := pidPath(absdir)

    pidBytes, err := os.ReadFile(pidPath)
    if err != nil { fmt.Println("no daemon running"); return }
    pid, _ := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
    
    proc, err := os.FindProcess(pid)
    if err != nil || syscall.Kill(pid, 0) != nil {
        os.Remove(pidPath)
        os.Remove(sockPath)
        fmt.Println("stale PID file removed")
        return
    }
    proc.Signal(syscall.SIGTERM)
    fmt.Printf("sent SIGTERM to pid %d\n", pid)
    os.Remove(pidPath)
    os.Remove(sockPath)
}
```

---

### grv status

```go
func runStatus() {
    grvDir := grvDir()
    entries, _ := os.ReadDir(grvDir)
    for _, e := range entries {
        if !strings.HasSuffix(e.Name(), ".pid") { continue }
        pidBytes, _ := os.ReadFile(filepath.Join(grvDir, e.Name()))
        pid, _ := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
        alive := syscall.Kill(pid, 0) == nil
        hash := strings.TrimSuffix(e.Name(), ".pid")
        status := "running"
        if !alive { status = "dead (stale)" }
        fmt.Printf("%s  pid=%d  %s\n", hash, pid, status)
    }
}
```

No need to reverse the hash to a directory path — the hash is sufficient for identification.
If we want human-readable output, store the absdir in a `.dir` file alongside the `.pid`.

---

## 2026-05-20 - Approaches Considered (final structure)

### Approach A: Everything in main.go (Simple)

Expand main.go to ~300 lines with all subcommand logic inline.

**Pros:** Single file, no package boundary overhead.
**Cons:** main.go becomes a grab-bag. Hard to test individual subcommands.

### Approach B: cmd/ subpackage (Recommended)

```
main.go           — parse os.Args, dispatch
cmd/
  stdio.go        — runStdio()
  daemon.go       — runDaemon(), Accept loop, idle timeout
  start.go        — runStart(), re-exec
  stop.go         — runStop()
  status.go       — runStatus()
  paths.go        — grvDir(), socketPath(), pidPath(), logPath(), socketHash()
```

**Pros:**
- Clean separation, each file ~50-80 lines
- cmd/paths.go is shared utility (no duplication)
- Testable: cmd/ package functions can be unit-tested
- Follows standard Go project layout

**Cons:**
- Slightly more files

**Fits existing patterns:** Yes — the project uses subpackages (ops/, editor/, kinds/, etc.).

### Approach C: Single cmd.go file at top level

All subcommand logic in a `cmd.go` file alongside main.go (same package main).

**Pros:** No subpackage needed.
**Cons:** Everything in package main. Less clean.

---

## 2026-05-20 - Recommendation (Module Rename + CLI Daemon)

### Chosen Approach

**Module rename**: Mechanical — `go mod edit` + sed over 24 files + update string literals.

**CLI structure**: Approach B — `cmd/` subpackage with manual dispatch. No cobra.

**Socket path**: `~/.grv/<hex8>.sock` where hex8 = first 4 bytes of sha256(absdir) as hex.
Companion files: `.pid` (same prefix) and `.log` (same prefix). `.dir` file to store human-
readable absdir for `grv status`.

**Daemon start**: Re-exec pattern — `grv start` spawns `grv daemon --socket S --dir D` via
`exec.Command` with `Setsid: true`, redirects stdout/stderr to logFile, calls `cmd.Start()`.

**MCP over Unix socket**: `server.NewStdioServer(mcpSvr)` + `stdioSvr.Listen(ctx, conn, conn)`.
Accept one connection at a time (sequential, not concurrent) — safe because stdioSession is
package-level in mcp-go.

**Idle timeout**: `atomic.Int64` lastActivity, background ticker every 5 minutes, exit after
1 hour idle. Touch on each new connection accepted.

**stdin detection for stdio mode**:
```go
func isStdioMode() bool {
    stat, _ := os.Stdin.Stat()
    return (stat.Mode() & os.ModeCharDevice) == 0  // stdin is a pipe, not a terminal
}
```

**Implementation Strategy:**

1. Module rename:
   - `go mod edit -module github.com/mattdurham/grv`
   - `find . -name "*.go" | xargs sed -i 's|github.com/lthiery/goast|github.com/mattdurham/grv|g'`
   - Update string literals in main.go: "goast" → "grv" (server name, log prefix)
   - `go build ./...` to verify

2. Create `cmd/paths.go`: grvDir, socketHash, socketPath, pidPath, logPath, dirFilePath

3. Create `cmd/daemon.go`: runDaemon + idleWatcher + touchActivity

4. Create `cmd/start.go`: runStart with re-exec + PID write + socket poll

5. Create `cmd/stop.go`: runStop with SIGTERM

6. Create `cmd/status.go`: runStatus reading .pid files

7. Update `main.go`:
   - Change log prefix to "grv: "
   - Add dispatch: `runStdio` (default/piped), `start`, `stop`, `status`, `daemon`
   - `runStdio` = existing behavior (RegisterTools + ServeStdio)

**Key Decisions:**

- **No new dependencies**: crypto/sha256 (stdlib), net (stdlib), syscall (stdlib), os/exec (stdlib).
- **Sequential daemon connections**: One MCP client at a time — matches MCP protocol design.
- **`grv daemon` is internal**: Not documented as user-facing. Called only by `grv start`.
- **Default behavior when args unknown**: Fall through to stdio mode for backward compatibility.
- **Daemon architecture is Constraint C**: Daemon exists for future caching warmup; MCP clients
  continue to use stdio. The daemon is not yet used by MCP clients in this iteration.

**Risks Identified:**

- **stdioSessionInstance package-level var**: If mcp-go v0.54.0 changes this to a local var
  in a future version, concurrent connections would work. For now, sequential is safe.
  Mitigation: wrap Accept loop without goroutine.

- **macOS socket path length limit**: Unix socket paths have a 104-char limit on macOS (vs 108
  on Linux). `~/.grv/` + 8 chars + `.sock` = well under limit. No risk.

- **`Setsid` on Linux vs macOS**: `syscall.SysProcAttr{Setsid: true}` works on both. No issue.

- **Race between grv start writing PID and socket appearing**: The socket is created inside
  `runDaemon` after `net.Listen`. The PID is written by `runStart` after `cmd.Start()`. Since
  `cmd.Start()` returns before the daemon has called `net.Listen`, the socket poll is needed.
  500ms-1s should be sufficient for the daemon to reach the Listen call.

**Open Questions:**

- Should `grv start` without a dir argument default to cwd or refuse? Recommendation: default
  to cwd (same as how go tools work: `go build` uses cwd by default).

- Should the stdio mode print anything to stderr on terminal invocation (no piped stdin, no
  known subcommand)? Recommendation: print usage to stderr and exit 1.

---

## 2026-05-20 - BRAINSTORM COMPLETE (Module Rename + CLI Daemon)

**Status:** Complete
**Recommendation:** Manual dispatch CLI in cmd/ subpackage, re-exec daemon pattern, StdioServer.Listen over Unix socket
**Next Phase:** PLAN

Key findings for the planner:
1. 24 files need import path substitution: `sed -i 's|github.com/lthiery/goast|github.com/mattdurham/grv|g'`
2. `StdioServer.Listen(ctx, io.Reader, io.Writer)` is the correct API for Unix socket serving — no custom protocol loop needed
3. stdioSessionInstance is package-level in mcp-go v0.54.0 — accept connections sequentially, not concurrently
4. Re-exec pattern: `grv start` spawns `grv daemon --socket S --dir D` via exec.Command with SysProcAttr{Setsid: true}
5. Socket path: `~/.grv/<hex(sha256(absdir)[:4])>.sock` — 8 hex chars, no new deps
6. Idle timeout: `atomic.Int64` lastActivity + 5-minute ticker + 1-hour threshold + os.Exit(0)
7. stdin detection: `(stat.Mode() & os.ModeCharDevice) == 0` distinguishes pipe from terminal
8. No new dependencies required — all needed packages are in stdlib

