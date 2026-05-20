# goast — Go AST MCP Server

## Problem

AI agents editing Go code via text patches are unreliable. They manipulate strings without understanding structure, producing wrong hunk offsets, fabricated git hashes, context drift, and malformed diffs.

Root cause: agents express *where* to change (line 139, context lines) rather than *what* to change semantically.

## Solution

An MCP server that exposes fully bidirectional Go AST operations. Claude reads code as structured JSON node trees and writes code by constructing those same trees. The server handles parsing, AST construction, formatting, and writing. No raw text, no snippets, no line numbers.

---

## Node Schema

Every AST node is a JSON object with a `"kind"` discriminator field. The remaining fields mirror the corresponding `go/ast` struct. Nodes compose recursively — an `IfStmt` contains a `BinaryExpr` which contains `SelectorExpr` nodes, etc.

`ast_query` returns node trees. `ast_insert` and `ast_replace` accept them.

### Expressions

```json
{ "kind": "Ident", "name": "string" }

{ "kind": "BasicLit", "tok": "INT|FLOAT|IMAG|CHAR|STRING", "value": "string" }

{ "kind": "Ellipsis", "elt": <Expr|null> }

{ "kind": "FuncLit", "type": <FuncType>, "body": <BlockStmt> }

{ "kind": "CompositeLit", "type": <Expr|null>, "elts": [<Expr>] }

{ "kind": "ParenExpr", "x": <Expr> }

{ "kind": "SelectorExpr", "x": <Expr>, "sel": "string" }

{ "kind": "IndexExpr", "x": <Expr>, "index": <Expr> }

{ "kind": "IndexListExpr", "x": <Expr>, "indices": [<Expr>] }

{ "kind": "SliceExpr", "x": <Expr>, "low": <Expr|null>, "high": <Expr|null>, "max": <Expr|null> }

{ "kind": "TypeAssertExpr", "x": <Expr>, "type": <Expr|null> }

{ "kind": "CallExpr", "fun": <Expr>, "args": [<Expr>], "ellipsis": false }

{ "kind": "StarExpr", "x": <Expr> }

{ "kind": "UnaryExpr", "op": "string", "x": <Expr> }

{ "kind": "BinaryExpr", "x": <Expr>, "op": "string", "y": <Expr> }

{ "kind": "KeyValueExpr", "key": <Expr>, "value": <Expr> }
```

`op` values are Go operator tokens: `+`, `-`, `*`, `/`, `%`, `&`, `|`, `^`, `<<`, `>>`, `&^`, `&&`, `||`, `==`, `!=`, `<`, `<=`, `>`, `>=`. Unary: `+`, `-`, `!`, `^`, `*`, `&`, `<-`.

### Type expressions

```json
{ "kind": "ArrayType", "len": <Expr|null>, "elt": <Expr> }
```
`len: null` means slice type (`[]T`). `len` present means array type (`[N]T`).

```json
{ "kind": "StructType", "fields": [<Field>] }

{ "kind": "InterfaceType", "methods": [<Field>] }

{ "kind": "FuncType", "type_params": [<Field>], "params": [<Field>], "results": [<Field>] }

{ "kind": "MapType", "key": <Expr>, "value": <Expr> }

{ "kind": "ChanType", "dir": "SEND|RECV|BOTH", "value": <Expr> }
```

`StarExpr` doubles as pointer type (`*T`) and pointer dereference — same node, context determines meaning.

### Field

Reused for struct fields, interface methods, and function parameters/results.

```json
{
  "kind": "Field",
  "names": ["string"],
  "type": <Expr>,
  "tag": "string|null"
}
```

`names` is empty/null for embedded struct fields and unnamed function parameters.

### Statements

```json
{ "kind": "BlockStmt", "list": [<Stmt>] }

{
  "kind": "IfStmt",
  "init": <Stmt|null>,
  "cond": <Expr>,
  "body": <BlockStmt>,
  "else": <Stmt|null>
}

{
  "kind": "ForStmt",
  "init": <Stmt|null>,
  "cond": <Expr|null>,
  "post": <Stmt|null>,
  "body": <BlockStmt>
}

{
  "kind": "RangeStmt",
  "key": <Expr|null>,
  "value": <Expr|null>,
  "tok": ":=|=|ILLEGAL",
  "x": <Expr>,
  "body": <BlockStmt>
}

{
  "kind": "SwitchStmt",
  "init": <Stmt|null>,
  "tag": <Expr|null>,
  "body": <BlockStmt>
}

{
  "kind": "TypeSwitchStmt",
  "init": <Stmt|null>,
  "assign": <Stmt>,
  "body": <BlockStmt>
}

{ "kind": "SelectStmt", "body": <BlockStmt> }

{ "kind": "CaseClause", "list": [<Expr>], "body": [<Stmt>] }
```
`list: null` means default case.

```json
{ "kind": "CommClause", "comm": <Stmt|null>, "body": [<Stmt>] }

{ "kind": "AssignStmt", "lhs": [<Expr>], "tok": ":=|=|+=|...", "rhs": [<Expr>] }

{ "kind": "ReturnStmt", "results": [<Expr>] }

{ "kind": "ExprStmt", "x": <Expr> }

{ "kind": "SendStmt", "chan": <Expr>, "value": <Expr> }

{ "kind": "IncDecStmt", "x": <Expr>, "tok": "++|--" }

{ "kind": "DeclStmt", "decl": <GenDecl> }

{ "kind": "GoStmt", "call": <CallExpr> }

{ "kind": "DeferStmt", "call": <CallExpr> }

{ "kind": "BranchStmt", "tok": "break|continue|goto|fallthrough", "label": "string|null" }

{ "kind": "LabeledStmt", "label": "string", "stmt": <Stmt> }
```

### Declarations

```json
{
  "kind": "FuncDecl",
  "recv": <Field|null>,
  "name": "string",
  "type": <FuncType>,
  "body": <BlockStmt|null>
}
```

`GenDecl` is split into four logical kinds by token:

```json
{ "kind": "ImportDecl", "specs": [<ImportSpec>] }

{ "kind": "ConstDecl", "specs": [<ValueSpec>] }

{ "kind": "TypeDecl", "specs": [<TypeSpec>] }

{ "kind": "VarDecl", "specs": [<ValueSpec>] }
```

### Specs

```json
{ "kind": "ImportSpec", "name": "string|null", "path": "string" }

{ "kind": "ValueSpec", "names": ["string"], "type": <Expr|null>, "values": [<Expr>] }

{ "kind": "TypeSpec", "name": "string", "type_params": [<Field>], "type": <Expr> }
```

### Example: complete IfStmt node

```json
{
  "kind": "IfStmt",
  "init": null,
  "cond": {
    "kind": "BinaryExpr",
    "x": {
      "kind": "SelectorExpr",
      "x": { "kind": "Ident", "name": "l" },
      "sel": "pos"
    },
    "op": ">=",
    "y": {
      "kind": "CallExpr",
      "fun": { "kind": "Ident", "name": "len" },
      "args": [
        {
          "kind": "SelectorExpr",
          "x": { "kind": "Ident", "name": "l" },
          "sel": "input"
        }
      ],
      "ellipsis": false
    }
  },
  "body": {
    "kind": "BlockStmt",
    "list": [
      {
        "kind": "ReturnStmt",
        "results": [{ "kind": "Ident", "name": "EOF" }]
      }
    ]
  },
  "else": null
}
```

---

## go.mod and Import Support

### go.mod operations

`gomod_read` — return the parsed go.mod as structured JSON:

```json
{
  "module": "github.com/example/repo",
  "go": "1.22",
  "require": [
    { "path": "golang.org/x/tools", "version": "v0.20.0", "indirect": false }
  ],
  "replace": [
    { "old": "github.com/foo/bar", "new": "../bar", "new_version": "" }
  ],
  "exclude": []
}
```

`gomod_require` — add or update a require directive:

```json
{
  "path": "golang.org/x/tools",
  "version": "v0.21.0",
  "indirect": false
}
```

`gomod_drop_require` — remove a require directive:

```json
{ "path": "golang.org/x/tools" }
```

`gomod_replace` — add or update a replace directive:

```json
{
  "old": "github.com/foo/bar",
  "new": "../bar",
  "new_version": ""
}
```

`gomod_drop_replace` — remove a replace directive:

```json
{ "old": "github.com/foo/bar" }
```

Implementation uses `golang.org/x/mod/modfile` for parsing and writing — same library `go mod` itself uses. Preserves comments and formatting.

### Import operations in Go files

`ast_add_import` — add an import to a file. Merges into existing import block; no-ops if already present.

```json
{
  "file": "handler.go",
  "path": "net/http",
  "alias": ""
}
```

`alias` values: `""` (none), `"."` (dot import), `"_"` (blank import), or any identifier.

`ast_delete_import` — remove an import by path:

```json
{
  "file": "handler.go",
  "path": "fmt"
}
```

`ast_list_imports` — return all imports in a file with their aliases:

```json
{
  "file": "handler.go"
}
```

Returns array of `{ path, alias, used }` — `used` indicates whether the import is referenced in the file body (helps identify unused imports before deletion).

---

## Metadata

Every node returned by `ast_query` or `ast_query_many` carries a `"meta"` field: a flat dictionary of derived, read-only properties computed from the AST at query time. Metadata is never written to source and never persisted — it exists only in the response.

A dedicated `ast_meta` tool returns only the metadata for a node (no tree), useful when the agent only needs summary information without the full recursive structure.

### Universal fields (every node)

| Field | Type | Description |
|---|---|---|
| `line` | int | Start line (1-based) |
| `end_line` | int | End line (1-based) |
| `col` | int | Start column (1-based) |
| `byte_offset` | int | Start byte offset in file |
| `byte_end` | int | End byte offset in file |
| `parent_kind` | string | Kind of the immediate parent node |
| `depth` | int | Nesting depth from file root (file=0, top-level decl=1, etc.) |

### File-level metadata

Returned by `ast_list` and `ast_meta` when path is empty.

| Field | Type | Description |
|---|---|---|
| `package` | string | Package name |
| `line_count` | int | Total lines in file |
| `decl_count` | int | Total top-level declarations |
| `func_count` | int | Number of FuncDecl nodes |
| `type_count` | int | Number of TypeSpec nodes |
| `import_count` | int | Number of imported paths |
| `has_init` | bool | File has an `init()` function |

### FuncDecl metadata

| Field | Type | Description |
|---|---|---|
| `exported` | bool | Name starts with uppercase |
| `is_method` | bool | Has a receiver |
| `recv_type` | string | Receiver type (e.g. `*Handler`), empty if function |
| `param_count` | int | Number of parameters |
| `result_count` | int | Number of return values |
| `has_error_return` | bool | Last result type is `error` |
| `stmt_count` | int | Direct statements in body (non-recursive) |
| `cyclomatic_complexity` | int | McCabe complexity (1 + branching nodes: if/for/range/case/select/&&/\|\|) |
| `is_variadic` | bool | Last parameter is variadic (`...T`) |

### TypeSpec metadata

| Field | Type | Description |
|---|---|---|
| `exported` | bool | Name starts with uppercase |
| `underlying_kind` | string | `struct`, `interface`, `alias`, `array`, `slice`, `map`, `chan`, `func`, `ident` |
| `is_alias` | bool | Declared with `=` |
| `has_type_params` | bool | Generic type (Go 1.18+) |

### StructType metadata

| Field | Type | Description |
|---|---|---|
| `field_count` | int | Number of fields |
| `has_embedded` | bool | Has at least one embedded (anonymous) field |
| `exported_field_count` | int | Number of exported fields |

### InterfaceType metadata

| Field | Type | Description |
|---|---|---|
| `method_count` | int | Number of method signatures |
| `embed_count` | int | Number of embedded interfaces/types |
| `is_empty` | bool | No methods or embeds |

### IfStmt / ForStmt / RangeStmt metadata

| Field | Type | Description |
|---|---|---|
| `has_init` | bool | Has an init statement (IfStmt, ForStmt) |
| `has_else` | bool | Has else branch (IfStmt) |
| `else_is_if` | bool | Else branch is another IfStmt (else-if chain) |
| `body_stmt_count` | int | Direct statements in body |

### SwitchStmt / TypeSwitchStmt metadata

| Field | Type | Description |
|---|---|---|
| `case_count` | int | Number of CaseClause nodes |
| `has_default` | bool | Has a default case |

### SelectStmt metadata

| Field | Type | Description |
|---|---|---|
| `case_count` | int | Number of CommClause nodes |
| `has_default` | bool | Has a default case |

### CallExpr metadata

| Field | Type | Description |
|---|---|---|
| `arg_count` | int | Number of arguments |
| `is_variadic_call` | bool | Last argument uses `...` spread |
| `callee` | string | Best-effort callee name (e.g. `fmt.Println`, `l.advance`) |

### Field metadata (struct field / param / result)

| Field | Type | Description |
|---|---|---|
| `exported` | bool | First name starts with uppercase |
| `is_embedded` | bool | No names (anonymous embed) |
| `has_tag` | bool | Has a struct tag |
| `name_count` | int | Number of names (e.g. `x, y int` → 2) |

### Example query response shape

```json
{
  "kind": "FuncDecl",
  "name": "ServeHTTP",
  "recv": { "kind": "Field", ... },
  "type": { "kind": "FuncType", ... },
  "body": { "kind": "BlockStmt", ... },
  "meta": {
    "line": 42,
    "end_line": 61,
    "col": 1,
    "byte_offset": 1204,
    "byte_end": 1587,
    "parent_kind": "File",
    "depth": 1,
    "exported": true,
    "is_method": true,
    "recv_type": "*Handler",
    "param_count": 2,
    "result_count": 0,
    "has_error_return": false,
    "stmt_count": 8,
    "cyclomatic_complexity": 4,
    "is_variadic": false
  }
}
```

`"meta"` is always flat — no nested objects. If a field doesn't apply to the node kind it is omitted entirely.

---

## Metadata Hooks

Hooks extend the `"meta"` dictionary with externally derived data. Each hook is a subprocess — the server calls it with node context on stdin and merges the returned key-value pairs into `"meta"` under a namespaced prefix.

Hooks are defined in a config file (`goast.toml`) at the project root or passed via MCP server startup arguments.

### Configuration

```toml
[[hooks]]
name    = "git"
command = ["git", "log", "-1", "--format=%H|%an|%ae|%ar", "--"]
scope   = "file"      # "file" | "func" | "node" | "all"
cache   = true        # cache result per file for the session lifetime
timeout = "2s"

[[hooks]]
name    = "blame"
command = ["git", "blame", "--porcelain", "-L"]
scope   = "func"      # only runs for FuncDecl nodes
kinds   = ["FuncDecl", "TypeSpec"]   # further filter by kind; omit for all in scope
cache   = true
timeout = "5s"

[[hooks]]
name    = "bugginess"
command = ["bugginess-score"]   # any executable on PATH
scope   = "file"
cache   = true
timeout = "10s"

[[hooks]]
name    = "coverage"
command = ["go-coverage-hook"]
scope   = "func"
kinds   = ["FuncDecl"]
cache   = true
timeout = "3s"
```

**`scope` values:**
- `file` — hook runs once per file, result applied to every node in that file
- `func` — hook runs once per FuncDecl (or filtered kinds), result applied to that node
- `node` — hook runs for every queried node individually (expensive; use sparingly)
- `all` — hook runs once at server startup, result applied globally to all nodes

### Hook stdin (JSON)

The server writes a single JSON object to the hook's stdin:

```json
{
  "file":    "internal/lexer/lexer.go",
  "package": "github.com/example/repo/internal/lexer",
  "node": {
    "kind":        "FuncDecl",
    "name":        "advance",
    "line":        42,
    "end_line":    61,
    "byte_offset": 1204,
    "byte_end":    1587
  }
}
```

For `scope: "file"` hooks, `"node"` is omitted. For `scope: "all"` hooks, both `"file"` and `"node"` are omitted.

### Hook stdout (JSON)

The hook writes a flat JSON object to stdout. All values must be string, number, or bool — no nested objects.

```json
{
  "last_commit":  "a3f9c12",
  "last_author":  "alice",
  "last_email":   "alice@example.com",
  "last_modified": "3 days ago",
  "commit_count": 47
}
```

These are merged into `"meta"` under the hook's name as a dot-prefix:

```json
"meta": {
  "line": 42,
  "end_line": 61,
  ...
  "git.last_commit":   "a3f9c12",
  "git.last_author":   "alice",
  "git.last_email":    "alice@example.com",
  "git.last_modified": "3 days ago",
  "git.commit_count":  47,
  "bugginess.score":   0.72,
  "bugginess.bug_fix_count": 8,
  "coverage.pct":      84.3,
  "coverage.covered":  true
}
```

### Error handling

- Hook exits non-zero or times out: its keys are omitted from meta; a `"<name>.error"` key is added with the error message
- Hook emits invalid JSON: same treatment — fail silently, add error key
- Hook stderr is captured and surfaced only in `"<name>.error"` on failure, not on success

This keeps metadata failures non-fatal — a broken git hook doesn't break code queries.

### Caching

`cache: true` hooks are memoized per (file, hook name) pair for the lifetime of the MCP server process. The cache is keyed on file path + mtime. If the file changes on disk (mtime advances), the cache entry is invalidated and the hook re-runs on the next query.

`scope: "all"` results are cached for the entire process lifetime with no invalidation.

### Built-in hooks (bundled, no config required)

| Hook name | Command | Scope | What it provides |
|---|---|---|---|
| `git` | `git log -1 --format=...` | `file` | `git.last_commit`, `git.last_author`, `git.last_modified`, `git.commit_count` |
| `blame` | `git blame --porcelain -L <line>,<end>` | `func` | `blame.author`, `blame.commit`, `blame.summary` for the node's line range |

Built-ins are enabled by default and can be disabled in config:

```toml
[builtin]
git   = true
blame = true
```

### `ast_meta` tool with hook control

```json
{
  "file": "lexer.go",
  "path": [{ "kind": "FuncDecl", "name": "advance" }],
  "hooks": ["git", "blame", "bugginess"]
}
```

The `"hooks"` field is an optional allowlist. If omitted, all configured hooks run. Pass `[]` to suppress all hooks and return only AST-derived meta.

---

## JSON Path Format

Selectors navigate to a node within a file before reading or replacing it. Each step is an object with a `kind` field and optional filters.

```json
[
  { "kind": "FuncDecl", "name": "advance" },
  { "kind": "IfStmt", "index": 0 },
  { "kind": "Cond" }
]
```

### Step kinds and filter fields

| Kind | Filters | Notes |
|------|---------|-------|
| `FuncDecl` | `name`, `recv` | `recv`: `*T` or `T` |
| `TypeDecl` | — | Top-level type group |
| `TypeSpec` | `name` | Specific named type within a TypeDecl |
| `VarDecl` | — | Top-level var group |
| `ConstDecl` | — | Top-level const group |
| `ImportDecl` | — | Top-level import group |
| `StructType` | — | Struct body |
| `InterfaceType` | — | Interface body |
| `Field` | `name`, `index` | Struct field, interface method, or parameter |
| `Body` | — | Function/method body (BlockStmt) |
| `Params` | — | FuncType parameter list |
| `Results` | — | FuncType result list |
| `IfStmt` | `index` | Nth if in current block |
| `ForStmt` | `index` | Nth for in current block |
| `RangeStmt` | `index` | Nth range in current block |
| `SwitchStmt` | `index` | Nth switch in current block |
| `TypeSwitchStmt` | `index` | Nth type switch in current block |
| `SelectStmt` | `index` | Nth select in current block |
| `CaseClause` | `index` | Nth case in switch body |
| `CommClause` | `index` | Nth case in select body |
| `AssignStmt` | `index` | Nth assignment in current block |
| `ReturnStmt` | `index` | Nth return in current block |
| `ExprStmt` | `index` | Nth expression statement |
| `GoStmt` | `index` | Nth go statement |
| `DeferStmt` | `index` | Nth defer statement |
| `Stmt` | `index` | Nth statement of any kind |
| `Cond` | — | Condition of IfStmt/ForStmt/SwitchStmt |
| `Init` | — | Init clause of IfStmt/ForStmt/SwitchStmt |
| `Post` | — | Post clause of ForStmt |
| `Else` | — | Else branch of IfStmt |
| `Tag` | — | Tag expression of SwitchStmt |
| `Lhs` | `index` | Nth LHS expr of AssignStmt |
| `Rhs` | `index` | Nth RHS expr of AssignStmt |
| `Key` | — | Key of RangeStmt or KeyValueExpr |
| `Value` | — | Value of RangeStmt, KeyValueExpr, SendStmt |
| `X` | — | Operand/receiver in binary, unary, selector, index, etc. |
| `Y` | — | Right operand of BinaryExpr |
| `Fun` | — | Function expression of CallExpr |
| `Args` | `index` | Nth argument of CallExpr |
| `Sel` | — | Selected field of SelectorExpr |
| `Elts` | `index` | Nth element of CompositeLit |

`index` is zero-based throughout.

---

## MCP Tools

### Read tools

#### `ast_list`
List all top-level declarations in a file.

```json
{ "file": "lexer.go" }
```

Returns array of `{ kind, name, recv, line }` summaries.

---

#### `ast_query`
Return the JSON node tree at a path.

```json
{
  "file": "lexer.go",
  "path": [{ "kind": "FuncDecl", "name": "advance" }]
}
```

Returns the full node tree rooted at the matched node.

---

#### `ast_query_many`
Run multiple queries in one call.

```json
{
  "file": "lexer.go",
  "paths": [
    [{ "kind": "FuncDecl", "name": "advance" }],
    [{ "kind": "TypeSpec", "name": "Token" }]
  ]
}
```

Returns array of node trees, same shape as `ast_query`.

---

### Write tools

All write tools share these behaviors:
- Parse the file fresh on each call
- Build the modified AST
- Format with `go/format`
- Return a unified diff
- Write to disk only if `dry_run` is false (default false)
- Atomic write: temp file + `os.Rename`

#### `ast_insert`
Insert a node into a list (block statements, struct fields, function args, etc.) at a given index.

```json
{
  "file": "lexer.go",
  "path": [
    { "kind": "FuncDecl", "name": "advance" },
    { "kind": "Body" }
  ],
  "index": 0,
  "node": {
    "kind": "IfStmt",
    "cond": { "kind": "BinaryExpr", "x": ..., "op": ">=", "y": ... },
    "body": { "kind": "BlockStmt", "list": [...] }
  },
  "dry_run": false
}
```

`index: -1` appends to end of list.

---

#### `ast_replace`
Replace the node at a path with a new node tree.

```json
{
  "file": "lexer.go",
  "path": [
    { "kind": "FuncDecl", "name": "advance" },
    { "kind": "IfStmt", "index": 0 },
    { "kind": "Cond" }
  ],
  "node": {
    "kind": "BinaryExpr",
    "x": { "kind": "Ident", "name": "tok" },
    "op": "==",
    "y": { "kind": "Ident", "name": "EOF" }
  },
  "dry_run": false
}
```

---

#### `ast_delete`
Remove the node at a path from its parent list.

```json
{
  "file": "lexer.go",
  "path": [
    { "kind": "FuncDecl", "name": "advance" },
    { "kind": "Body" },
    { "kind": "Stmt", "index": 2 }
  ],
  "dry_run": false
}
```

---

#### `ast_rename`
Rename an identifier at its declaration site and update all references within the same file.

```json
{
  "file": "lexer.go",
  "path": [{ "kind": "FuncDecl", "name": "nextToken" }],
  "to": "scanToken",
  "dry_run": false
}
```

---

## LSP-style Operations

Higher-level operations that complement the path system. These answer questions like "what is at this position?", "where is this used?", "what implements this interface?" — the kind of queries an editor makes over LSP, but returning structured AST data rather than text locations.

All cross-file operations accept either `"file"` (single file) or `"dir"` (all `.go` files in a directory, non-recursive) or `"package"` (import path, resolved via `go list`).

---

### `ast_node_at`
Given a file position (line + column), return the node at that position and its path from the file root. Bridges position-based navigation (from a human reading source) into the structural path system.

```json
{
  "file": "lexer.go",
  "line": 47,
  "col": 12
}
```

Returns the innermost node at that position, its full path, and its meta. The path can then be used directly with `ast_query`, `ast_replace`, etc.

```json
{
  "path": [
    { "kind": "FuncDecl", "name": "advance" },
    { "kind": "Body" },
    { "kind": "IfStmt", "index": 0 },
    { "kind": "Cond" }
  ],
  "node": { "kind": "BinaryExpr", ... },
  "meta": { ... }
}
```

---

### `ast_find_refs`
Find all references to the identifier at the given path within a file or package. Returns a list of locations, each with its path and the node that contains the reference.

```json
{
  "file": "lexer.go",
  "path": [{ "kind": "FuncDecl", "name": "advance" }],
  "scope": "file"
}
```

`scope`: `"file"` (default) or `"package"` (all files in the same package directory).

Returns:
```json
[
  {
    "file": "lexer.go",
    "path": [
      { "kind": "FuncDecl", "name": "Next" },
      { "kind": "Body" },
      { "kind": "ExprStmt", "index": 2 }
    ],
    "kind": "CallExpr",
    "line": 83
  }
]
```

---

### `ast_find_def`
Follow an identifier to its declaration. Resolves local variables, function calls, type names, struct fields, imported names.

```json
{
  "file": "lexer.go",
  "path": [
    { "kind": "FuncDecl", "name": "Next" },
    { "kind": "Body" },
    { "kind": "ExprStmt", "index": 0 },
    { "kind": "X" },
    { "kind": "Sel" }
  ]
}
```

Returns the definition location and node:
```json
{
  "file": "lexer.go",
  "path": [{ "kind": "FuncDecl", "name": "advance" }],
  "node": { "kind": "FuncDecl", ... },
  "meta": { ... }
}
```

For definitions in other files (imported packages), returns `"file"` and `"path"` into that file if it is on disk; returns `"external": true` with package path and symbol name if it resolves to a module dependency.

---

### `ast_find_impls`
Given an interface type, find all concrete types in scope that implement it. Uses `go/types` for full type-system resolution.

```json
{
  "file": "handler.go",
  "path": [{ "kind": "TypeSpec", "name": "Handler" }],
  "scope": "package"
}
```

Returns array of `{ file, path, type_name, meta }` for each implementing type.

---

### `ast_find_symbols`
Search for declarations matching a name pattern across files. Pattern supports `*` glob and case-insensitive matching.

```json
{
  "dir": "./internal/",
  "query": "Handle*",
  "kinds": ["FuncDecl", "TypeSpec"]
}
```

Returns array of `{ file, path, kind, name, recv, meta }` summaries — same shape as `ast_list` but across multiple files.

---

### `ast_find`
Structural search: find all nodes matching a partial node tree. Absent fields in the pattern are wildcards — they match any value. Present fields must match exactly.

```json
{
  "file": "lexer.go",
  "pattern": {
    "kind": "IfStmt",
    "cond": {
      "kind": "BinaryExpr",
      "op": "!=",
      "y": { "kind": "Ident", "name": "nil" }
    }
  }
}
```

This finds every `if x != nil { ... }` in the file regardless of what `x` is (the `"x"` field is absent → wildcard).

```json
{
  "file": "handler.go",
  "pattern": {
    "kind": "CallExpr",
    "fun": {
      "kind": "SelectorExpr",
      "sel": "Errorf"
    }
  }
}
```

Finds every `x.Errorf(...)` call regardless of receiver.

Returns array of `{ path, node, meta }` for every match, in source order.

Scope can be widened: `"file"` (default), `"dir"`, or `"package"`.

---

### `ast_rename` (cross-file)
Rename an identifier across all files in a package. Builds on `go/types` for accurate resolution — renames only the declaration and its genuine references, not coincidental uses of the same string.

```json
{
  "package": "./internal/lexer",
  "file": "lexer.go",
  "path": [{ "kind": "FuncDecl", "name": "nextToken" }],
  "to": "scanToken",
  "dry_run": false
}
```

Returns a map of `{ file → unified_diff }` for every file that changed.

---

### `ast_extract_func`
Extract a range of statements into a new function. Analyzes the selected statements for inputs (variables used but not declared inside) and outputs (variables declared inside and used after), generates the function signature and call site automatically.

```json
{
  "file": "handler.go",
  "path": [
    { "kind": "FuncDecl", "name": "ServeHTTP" },
    { "kind": "Body" }
  ],
  "from": 3,
  "to": 7,
  "name": "validateRequest",
  "dry_run": false
}
```

`from`/`to` are statement indices (inclusive). Returns the new function node and the replacement call site node, plus a diff.

---

### `ast_inline_func`
Inline a function call — replace a call site with the body of the called function, substituting arguments for parameters. Single-file only.

```json
{
  "file": "lexer.go",
  "path": [
    { "kind": "FuncDecl", "name": "Next" },
    { "kind": "Body" },
    { "kind": "ExprStmt", "index": 0 }
  ],
  "dry_run": false
}
```

Returns an error if the callee has multiple return sites (too complex to inline safely).

---

## Architecture

One struct per file. Each kind lives in its own file under `kinds/`. Functions on that struct stay in the same file. From the MCP caller's perspective there are no files — just node kinds and paths.

Each file carries a namespace comment so the file layout can be reconstructed from the source:

```go
// Namespace: goast/kinds/stmt
// Kind: IfStmt
// go/ast: *ast.IfStmt
package kinds
```

```
goast/
  main.go                  — MCP server entrypoint (stdio transport)
  server.go                — tool registration and dispatch
  kinds/
    node.go                — Node interface, kind registry, JSON unmarshal dispatch
    expr_ident.go          — Ident
    expr_basiclit.go       — BasicLit
    expr_binary.go         — BinaryExpr
    expr_unary.go          — UnaryExpr
    expr_star.go           — StarExpr
    expr_paren.go          — ParenExpr
    expr_call.go           — CallExpr
    expr_selector.go       — SelectorExpr
    expr_index.go          — IndexExpr
    expr_indexlist.go      — IndexListExpr
    expr_slice.go          — SliceExpr
    expr_typeassert.go     — TypeAssertExpr
    expr_funclit.go        — FuncLit
    expr_compositelit.go   — CompositeLit
    expr_keyvalue.go       — KeyValueExpr
    expr_ellipsis.go       — Ellipsis
    type_array.go          — ArrayType
    type_struct.go         — StructType
    type_interface.go      — InterfaceType
    type_func.go           — FuncType
    type_map.go            — MapType
    type_chan.go           — ChanType
    field.go               — Field
    stmt_block.go          — BlockStmt
    stmt_if.go             — IfStmt
    stmt_for.go            — ForStmt
    stmt_range.go          — RangeStmt
    stmt_switch.go         — SwitchStmt
    stmt_typeswitch.go     — TypeSwitchStmt
    stmt_select.go         — SelectStmt
    stmt_case.go           — CaseClause
    stmt_comm.go           — CommClause
    stmt_assign.go         — AssignStmt
    stmt_return.go         — ReturnStmt
    stmt_expr.go           — ExprStmt
    stmt_send.go           — SendStmt
    stmt_go.go             — GoStmt
    stmt_defer.go          — DeferStmt
    stmt_incdec.go         — IncDecStmt
    stmt_labeled.go        — LabeledStmt
    stmt_branch.go         — BranchStmt
    stmt_decl.go           — DeclStmt
    decl_func.go           — FuncDecl
    decl_import.go         — ImportDecl
    decl_const.go          — ConstDecl
    decl_type.go           — TypeDecl
    decl_var.go            — VarDecl
    spec_import.go         — ImportSpec
    spec_value.go          — ValueSpec
    spec_type.go           — TypeSpec
  selector/
    selector.go            — JSON path → go/ast node traversal
    selector_test.go
  editor/
    editor.go              — parse → edit → format → write cycle
    editor_test.go
  ops/
    query.go               — ast_list, ast_query, ast_query_many, ast_meta
    insert.go              — ast_insert
    replace.go             — ast_replace
    delete.go              — ast_delete
    rename.go              — ast_rename (single-file and cross-file)
    imports.go             — ast_add_import, ast_delete_import, ast_list_imports
    gomod.go               — gomod_read, gomod_require, gomod_drop_require, gomod_replace, gomod_drop_replace
    lsp.go                 — ast_node_at, ast_find_refs, ast_find_def, ast_find_impls, ast_find_symbols, ast_find
    refactor.go            — ast_extract_func, ast_inline_func
  diff/
    diff.go                — unified diff generation (pre/post source bytes)
  testdata/
    *.go                   — fixed Go files for selector and editor tests
```

### Node interface

```go
type Node interface {
    Kind() string
    ToAST() (ast.Node, error)   // JSON node → go/ast node
    FromAST(ast.Node) error     // go/ast node → JSON node
}
```

JSON unmarshalling dispatches on `"kind"` via a registry map:

```go
var registry = map[string]func() Node{
    "IfStmt":     func() Node { return &IfStmt{} },
    "BinaryExpr": func() Node { return &BinaryExpr{} },
    // ...
}
```

`UnmarshalNode(data []byte) (Node, error)` peeks at the `kind` field, looks up the constructor, and unmarshals into the concrete type.

---

## Key Implementation Decisions

### MCP transport
stdio, following the MCP spec. No HTTP server needed for local use with Claude Code.

### Parse-on-every-call
No in-memory cache. Each tool call parses the file fresh. Simpler, avoids stale state if the file is modified externally. Fast enough for single-file operations.

### go/ast ↔ Node conversion
Each kind file implements `ToAST` (for writes) and `FromAST` (for reads). `ToAST` is the critical path — it constructs a valid `go/ast` node that `go/format` can serialize. Invalid combinations (e.g., `BinaryExpr` with an unknown operator) are caught here before any file is touched.

### Comment preservation
`go/printer` preserves doc comments and inline comments attached to nodes. Freestanding comments between statements may be lost after structural edits. This is a known `go/ast` limitation. Document it; do not apply structural edits to comment-heavy regions without warning.

### go.mod
Use `golang.org/x/mod/modfile` — same library the `go` toolchain uses. Preserves formatting and comments within go.mod.

### Diff output
Compare original source bytes to post-format bytes. Myers' algorithm on lines, or `go-difflib`. Return as unified diff string in the tool response.

### Atomicity
Write to temp file in same directory, then `os.Rename`. If `go/format` fails, original file is untouched.

### Error shape
All errors return:
```json
{
  "error": "path not found",
  "at_step": 2,
  "step": { "kind": "IfStmt", "index": 3 },
  "available": ["IfStmt[0]", "IfStmt[1]", "ReturnStmt[2]"]
}
```

---

## Node Count

| Category | Count |
|---|---|
| Expressions | 16 |
| Type expressions | 6 |
| Field | 1 |
| Statements | 19 |
| Declarations | 5 |
| Specs | 3 |
| **Total navigable kinds** | **50** |
| Field accessor path steps | ~35 |
| **Total path vocabulary** | **~85** |

---

## Non-Go File Support

Non-Go files (JSON, YAML, TOML, Markdown, shell scripts, etc.) get simple raw read/write tools — no AST, no JSON node trees.

### `file_read`
Read the raw content of any file.

```json
{ "file": "config.yaml" }
```

Returns: `{ "content": "...", "size": 1234, "readonly": false }`

### `file_write`
Write raw content to any file. Atomic write (temp + rename). Returns a unified diff.

```json
{
  "file": "config.yaml",
  "content": "...",
  "dry_run": false
}
```

Returns: `{ "diff": "...", "changed": true }`

Both tools set `"readonly": true` if the file is in a readonly location (vendor/, stdlib, module cache) and refuse writes to readonly files.

---

## ast_directory — Directory Overview

A comprehensive directory scanner that provides a complete inventory of what's in a directory: Go symbols, non-Go files, and the readonly status of each.

```json
{
  "dir": "./internal/lexer",
  "recursive": false
}
```

Returns a structured inventory:

```json
{
  "go_files": [
    {
      "file": "lexer.go",
      "readonly": false,
      "package": "lexer",
      "structs": [
        { "name": "Lexer", "path": [{"kind":"TypeSpec","name":"Lexer"}], "field_count": 4 }
      ],
      "interfaces": [
        { "name": "Scanner", "path": [...], "method_count": 2 }
      ],
      "functions": [
        { "name": "New", "recv": "", "path": [...] },
        { "name": "Next", "recv": "*Lexer", "path": [...] }
      ],
      "globals": [
        { "kind": "VarDecl", "names": ["defaultBufSize"] },
        { "kind": "ConstDecl", "names": ["EOF"] }
      ]
    }
  ],
  "non_go_files": [
    { "file": "README.md", "size": 1234, "readonly": false },
    { "file": "config.json", "size": 512, "readonly": false }
  ],
  "subdirs": ["internal", "testdata"]
}
```

**Readonly detection** — a file/directory is readonly if it is under:
- `vendor/` (any depth)
- The Go standard library (`GOROOT`)
- The Go module cache (`GOPATH/pkg/mod` or `GOMODCACHE`)
- Has filesystem read-only permission (os.FileMode & 0200 == 0)

Readonly nodes are returned in results but all write tools (`ast_insert`, `ast_replace`, `ast_delete`, `ast_rename`, `file_write`, etc.) return an error if the target file is readonly.

---

## Readonly Metadata Field

All tools that return node or file metadata include `"readonly": bool` in their response. This applies to:
- `ast_list` — each declaration item
- `ast_query` / `ast_query_many` — the meta dictionary
- `ast_find_symbols` — each SymbolResult
- `ast_directory` — each file entry
- `file_read` — top-level field

**isReadonly(path string) bool** — shared helper in ops/readonly.go:
```go
func isReadonly(filePath string) bool {
    abs, _ := filepath.Abs(filePath)
    // vendor
    if strings.Contains(abs, "/vendor/") { return true }
    // stdlib
    if strings.HasPrefix(abs, runtime.GOROOT()) { return true }
    // module cache
    gomod := os.Getenv("GOMODCACHE")
    if gomod == "" { gomod = filepath.Join(os.Getenv("GOPATH"), "pkg", "mod") }
    if strings.HasPrefix(abs, gomod) { return true }
    // filesystem readonly
    info, err := os.Stat(abs)
    if err == nil && info.Mode()&0200 == 0 { return true }
    return false
}
```

---

## Out of Scope (v1)

- Type-aware validation (e.g. ensuring added struct field name doesn't conflict)
- Generated file detection (`.pb.go`, `_mock.go`)
- Multi-file atomic transactions

---

## Open Questions

1. Should `ast_rename` update string literals containing the identifier? Probably not — too ambiguous.
2. Files with build tags: parse with `parser.ParseComments`, ignore tags — they're irrelevant for structural edits.
3. Should `ast_query` also return the source text alongside the node tree, for human readability? Yes — include `"source"` field in response.
