# grv — Go AST Tool

grv is a Go AST manipulation tool designed for AI agents and developers who need to read and write Go code structurally — without string patching, line offsets, or raw source text.

All code is represented as **tree notation**: compact, indented node trees that are easy to read, easy to generate, and unambiguous to parse.

## Installation

```sh
go install github.com/mattdurham/grv@latest
```

Or from source:

```sh
git clone https://github.com/mattdurham/grv
cd grv
make install-all   # installs binary + lth-grv Claude Code skill
```

## Quick Start

```sh
# Start the daemon (auto-started on first tool call)
grv start

# Read a function
grv ast_query --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled'

# Read metadata: line range, complexity, git churn, lth memories
grv ast_meta --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled'

# List all declarations in a package
grv ast_list --namespace ops

# Check for rule violations
grv ast_check --dir ops/

# Patch a field without replacing the whole node
grv ast_patch --file ops/checks.go \
  --path 'FuncDecl name=ruleErrorHandled' \
  --ops 'set name "ruleErrorDiscarded"' \
  --dry_run true

# Insert a new statement
grv ast_insert --file ops/checks.go \
  --path 'FuncDecl name=runChecks / BlockStmt' \
  --index 0 \
  --node 'ExprStmt
  x CallExpr ellipsis=false
    fun SelectorExpr sel=Println
      x Ident name=fmt
    args[]
      BasicLit tok=STRING value="\"debug\""' \
  --dry_run true
```

## Tree Notation

grv uses tree notation for all input and output. Every node is one line with scalar attributes inline; children are indented 2 spaces:

```
FuncDecl name=ruleErrorHandled line=201 end_line=246
  type FuncType
    params FieldList
      list[]
        Field names=[fset] type=*token.FileSet
        Field names=[_] type=[]byte
        Field names=[f] type=*ast.File
        Field names=[absFile] type=string
    results FieldList
      list[]
        Field type=[]Violation
  body BlockStmt
    list[]
      AssignStmt tok=":="
        lhs[]
          Ident name=commentLines
        rhs[]
          CallExpr ellipsis=false
            fun Ident name=make
            args[]
              MapType
                key Ident name=int
                value Ident name=bool
      ...
```

Metadata renders as key=val:

```
line=201
end_line=246
cyclomatic_complexity=10
git_churn=1
exported=false
param_count=4
lth.results[]
  score=0.63 layer=3
  summary="..."
  fetch="lth --json get abc123"
```

## Reference

```sh
grv help               # list all tools
grv help <tool>        # args and notes for a specific tool
grv example            # one example per tool
grv example <tool>     # all examples for a tool
grv guide              # tree notation reference index
grv guide notation     # how to read and write tree notation
grv guide fields       # common field names: x, fun, sel, tok, op ...
grv guide nodes        # node kinds with Go ↔ tree examples
grv guide meta         # metadata fields reference
grv grammar            # schemas for all node kinds
```

## Tools

### Reading

| Tool | What it does |
|------|-------------|
| `ast_list` | Top-level declarations in a file or package |
| `ast_query` | Full node tree at a selector path |
| `ast_query_many` | Multiple nodes in one call |
| `ast_meta` | Line range, complexity, git churn, lth memories |
| `ast_directory` | All files and symbols in a directory |
| `ast_find` | Nodes matching a pattern |
| `ast_find_symbols` | Symbols by name across a directory |
| `ast_find_refs` | All references to a symbol |
| `ast_find_def` | Definition of an identifier |
| `ast_find_impls` | Types implementing an interface |
| `ast_node_at` | Node at a specific line/column |
| `ast_check` | Rule violations (nil deref, unchecked errors, etc.) |

### Writing

| Tool | What it does |
|------|-------------|
| `ast_insert` | Insert a node into a list |
| `ast_replace` | Replace a node at a path |
| `ast_delete` | Delete a node |
| `ast_patch` | Mutate named fields without full replacement |
| `ast_rename` | Rename an identifier |
| `ast_insert_many` | Multiple inserts, one atomic write |
| `ast_replace_many` | Multiple replaces, one atomic write |
| `ast_delete_many` | Multiple deletes, one atomic write |
| `ast_add_import` | Add an import |
| `ast_delete_import` | Remove an import |

All write tools:
- Parse the file fresh on each call
- Write atomically (temp file + rename)
- Run configured check rules post-write and restore the original on violation

### Paths

Paths identify nodes within a file using slash-separated tree steps:

```sh
--path 'FuncDecl name=foo'
--path 'FuncDecl name=foo / BlockStmt / AssignStmt'
--path 'TypeSpec name=Violation / StructType / FieldList / Field name=Line'
```

### Ops (ast_patch)

```sh
--ops 'set name "newName"
delete recv
append list
  Ident name=x'
```

### Ops (batch tools)

```sh
--ops 'path FuncDecl name=foo
node
  FuncDecl name=bar
---
path TypeSpec name=Old
node
  TypeSpec name=New'
```

See `grv guide notation` for the full reference.

## Configuration (`grv.yaml`)

```yaml
hooks:
  - name: lth
    command: ["~/bin/lth", "search", "{namespace}", "--brief"]
    scope: file
    cache: true
    immutable: true
    timeout: "30s"

checks:
  enforce: ["all"]   # or specific rules: ["error_handled", "nil_dereference"]
```

### Available check rules

| Rule | Catches |
|------|---------|
| `error_handled` | `val, _ := fn()` — error discarded with blank |
| `error_not_checked` | `fn()` — error return not captured at all |
| `type_assertion_not_checked` | `v := x.(T)` without comma-ok |
| `mutex_not_embedded` | `sync.Mutex` embedded anonymously in struct |
| `channel_size_not_one_or_zero` | `make(chan T, N)` where N>1 with no comment |
| `map_without_size_hint` | `make(map[K]V)` with no size hint |
| `slice_without_capacity` | `make([]T, n)` with no capacity argument |
| `nil_dereference` | SSA-based: dereference of provably-nil pointer (transitive) |

## Invariants

1. grv never returns raw Go source text — all representations are AST node trees
2. AST node trees are the sole bidirectional representation
3. Every write tool parses the file fresh on each call
4. All writes are atomic: temp file + `os.Rename`; original untouched if format fails
5. Readonly detection enforced before any write (vendor/, GOROOT, module cache, permissions)
