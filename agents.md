# grv — Agent Usage Guide

This guide is for AI agents using grv to read and write Go code. grv exposes Go's AST as a structured tool API — no raw source text, no string patching, no line offsets.

## Core Principle

**Never read or write `.go` files directly.** Use grv tools for all Go code interaction. Use `grv file_read` / `grv file_write` for non-Go files.

```sh
# WRONG — agents must not do this
cat ops/checks.go
sed -i 's/old/new/' ops/checks.go

# RIGHT
grv ast_list --file ops/checks.go
grv ast_replace --file ops/checks.go --path 'FuncDecl name=foo' --node '...'
```

## Bootstrap Pattern

Before starting work on any session:

```sh
grv start               # ensure daemon is running (idempotent)
grv ast_directory --dir ops/   # orient yourself: what's in this package?
```

## Workflow: Read → Understand → Write

### 1. Orient

```sh
# What's in this directory?
grv ast_directory --dir ops/

# What's in this file?
grv ast_list --file ops/checks.go

# Find a symbol by name
grv ast_find_symbols --dir ops/ --query ruleErrorHandled
```

### 2. Read

```sh
# Read a function — outputs tree notation by default
grv ast_query --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled'

# Read metadata: line range, complexity, git churn, relevant memories
grv ast_meta --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled'

# Read multiple nodes in one call
grv ast_query_many --file ops/checks.go \
  --paths '[[{"kind":"FuncDecl","name":"ruleErrorHandled"}],[{"kind":"FuncDecl","name":"runChecks"}]]'
```

The `line=201 end_line=246` fields in the node header tell you where in the source file the node lives.

### 3. Understand the output

See `grv guide` for the full reference. Key things to know:

```
FuncDecl name=foo line=42 end_line=67    ← scalar fields inline on header
  body                                   ← single-object child (no suffix)
    BlockStmt
      list[]                             ← object array ([] suffix)
        AssignStmt tok=":="
          lhs[]
            Ident name=x
          rhs[]
            CallExpr ellipsis=false
              fun Ident name=fmt.Println
              args[]
                BasicLit tok=STRING value="\"hello\""
  type FuncType
    params FieldList
      list[]
        Field names=[ctx] type=context.Context
```

**Common field names** (`grv guide fields` for full list):
- `x` — primary sub-expression (receiver in `SelectorExpr`, operand in `UnaryExpr`)
- `fun` — function being called in `CallExpr`
- `sel` — selected name in `SelectorExpr` (part after the dot)
- `tok` — token: `:=`, `=`, `INT`, `STRING`, `+`, etc.
- `op` — operator in `BinaryExpr` / `UnaryExpr`
- `body` — `BlockStmt` body of a function, if, for, range
- `lhs` / `rhs` — left/right sides of an assignment
- `cond` — condition in `IfStmt` / `ForStmt`

### 4. Write

**Always dry_run first.** Review the diff, then apply.

```sh
# Patch a single field — most surgical option
grv ast_patch --file ops/checks.go \
  --path 'FuncDecl name=ruleErrorHandled' \
  --ops '[{"op":"set","field":"name","value":"\"ruleErrorDiscarded\""}]' \
  --dry_run true

# Replace a node (tree notation accepted for --node)
grv ast_replace --file ops/checks.go \
  --path 'FuncDecl name=knownRuleNames' \
  --node 'FuncDecl name=knownRuleNames
  body BlockStmt
    list[]
      ReturnStmt
        results[]
          BasicLit tok=STRING value="\"all\""' \
  --dry_run true

# Insert a node
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

# Delete a node
grv ast_delete --file ops/checks.go \
  --path 'FuncDecl name=knownRuleNames' \
  --dry_run true
```

**When constructing nodes**, you don't specify `line` or `end_line` — those are read-only position fields that the formatter assigns after writing.

**Batch writes** — multiple mutations in one atomic write:

```sh
grv ast_replace_many --file ops/checks.go \
  --ops '[
    {"path":[{"kind":"FuncDecl","name":"ruleA"}],"node":{"kind":"FuncDecl","name":"ruleA_new"}},
    {"path":[{"kind":"FuncDecl","name":"ruleB"}],"node":{"kind":"FuncDecl","name":"ruleB_new"}}
  ]' --dry_run true
```

### 5. Check

```sh
# Run enabled checks on a file or directory
grv ast_check --file ops/checks.go
grv ast_check --dir ops/

# Check violations also run automatically on writes when grv.yaml configures rules
```

## Paths

Paths identify a node within a file. Tree notation is preferred:

```sh
'FuncDecl name=foo'                                     # top-level function
'FuncDecl name=foo / BlockStmt'                         # its body
'FuncDecl name=foo / BlockStmt / AssignStmt'            # first statement
'TypeSpec name=Violation / StructType / FieldList'      # struct fields
```

Slash-separated steps, each step is `KindName attr=val`. JSON arrays also work.

## Metadata

`grv ast_meta` returns structural facts about a node:

| Field | Meaning |
|-------|---------|
| `line` / `end_line` | Source file line range |
| `cyclomatic_complexity` | Branch count + 1; >10 is complex |
| `git_churn` | Commits that touched this line range |
| `exported` | Whether the declaration is exported |
| `param_count` / `result_count` | Function signature counts |
| `lth.results[]` | Relevant memories from lth (summary + fetch command) |

Each `lth.results` entry has a `summary` and a `fetch` command. Run the fetch if the summary is relevant:

```sh
lth --json get abc123-...
```

See `grv guide meta` for the complete field reference.

## Check Rules

Write tools enforce configured rules and reject the write on violation. Rules are configured in `grv.yaml`:

```yaml
checks:
  enforce: ["all"]   # or specific: ["error_handled", "nil_dereference"]
```

| Rule | What it catches |
|------|----------------|
| `error_handled` | `val, _ := fn()` — error discarded with `_` |
| `error_not_checked` | `fn()` — error return not captured at all |
| `type_assertion_not_checked` | `v := x.(T)` without comma-ok |
| `mutex_not_embedded` | `sync.Mutex` embedded anonymously in struct |
| `channel_size_not_one_or_zero` | `make(chan T, N)` N>1 without comment |
| `map_without_size_hint` | `make(map[K]V)` without size hint |
| `slice_without_capacity` | `make([]T, n)` without capacity |
| `nil_dereference` | SSA-based nil dereference (transitive across package) |

Suppress a rule with a same-line comment:
```go
n, _ := fmt.Println(msg) // error intentionally ignored: best-effort logging
```

## Common Patterns

### Find and read a symbol before editing

```sh
grv ast_find_symbols --dir ops/ --query ruleErrorHandled
# → copy the path from output, then:
grv ast_query --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled'
```

### Add an import then insert code that uses it

```sh
grv ast_add_import --file ops/checks.go --path '"sync"'
grv ast_insert --file ops/checks.go \
  --path 'TypeSpec name=Runner / StructType / FieldList' \
  --index -1 \
  --node 'Field names=[mu]
  type SelectorExpr sel=Mutex
    x Ident name=sync'
```

### Rename a function everywhere in a file

```sh
grv ast_rename --file ops/checks.go \
  --path 'FuncDecl name=ruleErrorHandled' \
  --to ruleErrorDiscarded
```

### Check if a function has references before deleting

```sh
grv ast_find_refs --file ops/checks.go \
  --path 'FuncDecl name=knownRuleNames' \
  --scope package
# If no refs → safe to delete
grv ast_delete --file ops/checks.go --path 'FuncDecl name=knownRuleNames'
```

### Patch a struct to add a field

```sh
grv ast_patch --file ops/checks.go \
  --path 'TypeSpec name=Violation / StructType / FieldList' \
  --ops '[{"op":"append","field":"list","value":{"kind":"Field","names":["Source"],"type":{"kind":"Ident","name":"string"}}}]'
```

## Reference

```sh
grv guide            # tree notation index
grv guide notation   # format rules (suffixes, scalars, paths)
grv guide fields     # x, fun, sel, tok, op, lhs, rhs, body ...
grv guide nodes      # all node kinds with Go ↔ tree examples
grv guide meta       # all metadata fields
grv example <tool>   # working examples for any tool
grv help <tool>      # args and notes
grv grammar <kind>   # JSON schema for a specific node kind
```
