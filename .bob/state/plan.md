# Implementation Plan: goast MCP Server

## Overview

Build a Go MCP server (`github.com/lthiery/goast`) that exposes fully bidirectional Go AST
read/write operations to Claude via structured JSON node trees. The server uses the mark3labs/mcp-go
v0.54.0 stdio transport and exposes tools for querying, inserting, replacing, and deleting Go AST
nodes — no raw text, no line numbers, no snippets.

Build is organized in three tiers. After Tier 1, the server is immediately usable for real code
editing (50 node kinds, full read/write ops, imports, go.mod). Tier 2 adds rename and structural
search. Tier 3 adds cross-file LSP and refactoring.

**Module:** `github.com/lthiery/goast`
**Existing deps:** `github.com/mark3labs/mcp-go v0.54.0`, `golang.org/x/mod v0.36.0`,
`golang.org/x/tools v0.45.0`

---

## Files to Create (complete list)

```
main.go
server.go
kinds/
  node.go
  marshal.go
  expr_ident.go
  expr_basiclit.go
  expr_binary.go
  expr_unary.go
  expr_star.go
  expr_paren.go
  expr_call.go
  expr_selector.go
  expr_index.go
  expr_indexlist.go
  expr_slice.go
  expr_typeassert.go
  expr_funclit.go
  expr_compositelit.go
  expr_keyvalue.go
  expr_ellipsis.go
  type_array.go
  type_struct.go
  type_interface.go
  type_func.go
  type_map.go
  type_chan.go
  field.go
  stmt_block.go
  stmt_if.go
  stmt_for.go
  stmt_range.go
  stmt_switch.go
  stmt_typeswitch.go
  stmt_select.go
  stmt_case.go
  stmt_comm.go
  stmt_assign.go
  stmt_return.go
  stmt_expr.go
  stmt_send.go
  stmt_go.go
  stmt_defer.go
  stmt_incdec.go
  stmt_labeled.go
  stmt_branch.go
  stmt_decl.go
  decl_func.go
  decl_import.go
  decl_const.go
  decl_type.go
  decl_var.go
  spec_import.go
  spec_value.go
  spec_type.go
  kinds_test.go
selector/
  selector.go
  selector_test.go
editor/
  editor.go
  editor_test.go
diff/
  diff.go
  diff_test.go
meta/
  meta.go
  meta_test.go
ops/
  query.go
  insert.go
  replace.go
  delete.go
  imports.go
  gomod.go
  rename.go          (Tier 2)
  lsp.go             (Tier 2)
  refactor.go        (Tier 3)
testdata/
  simple.go          — package with a few funcs, types, structs
  generics.go        — generics test file
  channels.go        — channels and select test file
  imports.go         — import-heavy test file
```

---

## TIER 1 — Working Core

### Task 1.1: `diff/diff.go` — Unified diff wrapper

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/diff/diff.go`
**Test:** `/home/matt/source/goast-worktrees/goast-mcp-server/diff/diff_test.go`
**Blocked by:** nothing
**Complexity:** S

**Types/functions to implement:**

```go
package diff

// Files produces a unified diff between before and after bytes, labelled with path.
// Returns empty string if no changes.
func Files(path string, before, after []byte) (string, error)
```

**Implementation:** Wrap `github.com/pmezard/go-difflib` (already in module cache as transitive dep
— add it to go.mod with `go get`). Use `difflib.UnifiedDiff{A: splitLines(before),
B: splitLines(after), FromFile: path, ToFile: path, Context: 3}`.

**Note:** `go-difflib` is NOT yet in go.mod. First step: add it:
```
go get github.com/pmezard/go-difflib@v1.0.0
```

**Test cases:**
- identical content → empty string
- added line → diff with `+` line
- removed line → diff with `-` line
- changed line → diff with context

---

### Task 1.2: `kinds/node.go` — Node interface and registry

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/kinds/node.go`
**Blocked by:** nothing
**Complexity:** S

**Types/functions:**

```go
package kinds

import (
    "encoding/json"
    "fmt"
    "go/ast"
)

// Node is the JSON representation of a go/ast node.
// Each concrete type implements Kind(), ToAST(), and FromAST().
type Node interface {
    Kind() string
    ToAST() (ast.Node, error)  // constructs a go/ast node; positions are token.NoPos
    FromAST(ast.Node) error    // populates this Node from a go/ast node
}

// registry maps kind strings to zero-value constructors.
var registry = map[string]func() Node{}

// register adds a kind to the registry. Called via init() in each kind file.
func register(kind string, factory func() Node)

// UnmarshalNode decodes a JSON node by peeking at "kind" then dispatching to
// the appropriate concrete type. Returns nil, nil for null/empty input.
func UnmarshalNode(data json.RawMessage) (Node, error)
```

**Key details:**
- `UnmarshalNode` two-phase peek: `var peek struct{ Kind string \`json:"kind"\` }`, then
  look up in registry, construct zero value, `json.Unmarshal(data, node)`, return node.
- Returns `nil, nil` when `len(data) == 0 || string(data) == "null"`.
- Returns `fmt.Errorf("unknown kind %q", peek.Kind)` for unregistered kinds.
- Each kind file calls `register("IfStmt", func() Node { return &IfStmt{} })` in `init()`.

---

### Task 1.3: `kinds/marshal.go` — MarshalNode dispatcher

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/kinds/marshal.go`
**Blocked by:** 1.2 (Node interface), 1.4–1.53 (all kind structs)
**Complexity:** M

**Functions:**

```go
package kinds

import (
    "encoding/json"
    "fmt"
    "go/ast"
)

// MarshalNode converts a go/ast node to its JSON representation.
// This is the inverse of UnmarshalNode — dispatches on concrete ast.Node type.
func MarshalNode(node ast.Node) (json.RawMessage, error)

// MarshalExpr converts ast.Expr to JSON. Returns null for nil input.
func MarshalExpr(expr ast.Expr) (json.RawMessage, error)

// MarshalStmt converts ast.Stmt to JSON. Returns null for nil input.
func MarshalStmt(stmt ast.Stmt) (json.RawMessage, error)

// MarshalDecl converts ast.Decl to JSON.
func MarshalDecl(decl ast.Decl) (json.RawMessage, error)

// marshalField converts *ast.Field to JSON Field node.
func marshalField(f *ast.Field) (json.RawMessage, error)

// marshalFields converts []*ast.Field to []json.RawMessage.
func marshalFields(fields []*ast.Field) ([]json.RawMessage, error)
```

**Implementation strategy:** Large type switch on `node.(type)`:
```go
switch n := node.(type) {
case *ast.Ident:      v := &Ident{}; v.FromAST(n); return json.Marshal(v)
case *ast.BasicLit:   v := &BasicLit{}; v.FromAST(n); return json.Marshal(v)
case *ast.IfStmt:     v := &IfStmt{}; v.FromAST(n); return json.Marshal(v)
// ... all 50 cases
case *ast.GenDecl:    // dispatch on Tok
    switch n.Tok {
    case token.IMPORT: v := &ImportDecl{}; v.FromAST(n); return json.Marshal(v)
    case token.CONST:  v := &ConstDecl{}; v.FromAST(n); return json.Marshal(v)
    case token.TYPE:   v := &TypeDecl{}; v.FromAST(n); return json.Marshal(v)
    case token.VAR:    v := &VarDecl{}; v.FromAST(n); return json.Marshal(v)
    }
}
```

**Note:** This file is written LAST among the kinds files (depends on all structs existing).
The test file `kinds_test.go` tests MarshalNode + UnmarshalNode round-trips.

---

### Tasks 1.4–1.19: Expression kinds (16 files)

All expression files follow the same pattern. Each file:
1. Has namespace comment header
2. Defines one struct with `json:"kind"` field plus child fields as `json.RawMessage`
3. Implements `Kind() string`, `ToAST() (ast.Node, error)`, `FromAST(ast.Node) error`
4. Has an `init()` calling `register(...)`

**Common struct pattern:**
```go
// Namespace: goast/kinds/expr
// Kind: BinaryExpr
// go/ast: *ast.BinaryExpr
package kinds

type BinaryExpr struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`   // Expr
    Op        string          `json:"op"`
    Y         json.RawMessage `json:"y"`   // Expr
}
func (n *BinaryExpr) Kind() string { return "BinaryExpr" }
func init() { register("BinaryExpr", func() Node { return &BinaryExpr{} }) }
```

**ToAST pattern for child nodes:**
```go
func (n *BinaryExpr) ToAST() (ast.Node, error) {
    xNode, err := UnmarshalNode(n.X)
    if err != nil { return nil, fmt.Errorf("BinaryExpr.X: %w", err) }
    xAST, err := xNode.ToAST()
    if err != nil { return nil, fmt.Errorf("BinaryExpr.X: %w", err) }
    // ...
    op := tokenFromString(n.Op) // map "+" → token.ADD, etc.
    return &ast.BinaryExpr{X: xAST.(ast.Expr), Op: op, Y: yAST.(ast.Expr)}, nil
}
```

**FromAST pattern for child nodes:**
```go
func (n *BinaryExpr) FromAST(node ast.Node) error {
    b := node.(*ast.BinaryExpr)
    n.KindField = "BinaryExpr"
    var err error
    n.X, err = MarshalExpr(b.X)
    if err != nil { return err }
    n.Op = b.Op.String()
    n.Y, err = MarshalExpr(b.Y)
    return err
}
```

**Operator token mapping (needed by multiple kinds):**
Place `tokenFromString(s string) token.Token` in `kinds/node.go` or a separate `kinds/tokens.go`.
The mapping covers all binary ops (`+`,`-`,`*`,`/`,`%`,`&`,`|`,`^`,`<<`,`>>`,`&^`,`&&`,`||`,
`==`,`!=`,`<`,`<=`,`>`,`>=`) and unary ops (`+`,`-`,`!`,`^`,`*`,`&`,`<-`).

**File-by-file specifications:**

#### Task 1.4: `kinds/expr_ident.go` — Ident
**Complexity:** S
```go
type Ident struct {
    KindField string `json:"kind"`
    Name      string `json:"name"`
}
// ToAST: return &ast.Ident{Name: n.Name}, nil
// FromAST: n.Name = node.(*ast.Ident).Name
```

#### Task 1.5: `kinds/expr_basiclit.go` — BasicLit
**Complexity:** S
```go
type BasicLit struct {
    KindField string `json:"kind"`
    Tok       string `json:"tok"`   // "INT", "FLOAT", "IMAG", "CHAR", "STRING"
    Value     string `json:"value"`
}
// ToAST: tok = tokKindFromString(n.Tok) (token.INT etc.)
// map "INT"→token.INT, "FLOAT"→token.FLOAT, "IMAG"→token.IMAG,
//     "CHAR"→token.CHAR, "STRING"→token.STRING
```

#### Task 1.6: `kinds/expr_binary.go` — BinaryExpr
**Complexity:** S (shown above as example)

#### Task 1.7: `kinds/expr_unary.go` — UnaryExpr
**Complexity:** S
```go
type UnaryExpr struct {
    KindField string          `json:"kind"`
    Op        string          `json:"op"`
    X         json.RawMessage `json:"x"`
}
```

#### Task 1.8: `kinds/expr_star.go` — StarExpr
**Complexity:** S
```go
type StarExpr struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`
}
// ToAST: &ast.StarExpr{Star: token.NoPos, X: xAST.(ast.Expr)}
```

#### Task 1.9: `kinds/expr_paren.go` — ParenExpr
**Complexity:** S
```go
type ParenExpr struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`
}
```

#### Task 1.10: `kinds/expr_call.go` — CallExpr
**Complexity:** S
```go
type CallExpr struct {
    KindField string            `json:"kind"`
    Fun       json.RawMessage   `json:"fun"`
    Args      []json.RawMessage `json:"args"`
    Ellipsis  bool              `json:"ellipsis"`
}
// ToAST: if Ellipsis, set Ellipsis field to token.NoPos (any non-zero = present)
// Actually: ast.CallExpr.Ellipsis is token.Pos; token.NoPos means absent.
// So: if n.Ellipsis { result.Ellipsis = 1 } (1 is a valid non-NoPos position)
// Better: use a sentinel. Brainstorm confirmed token.NoPos=0 is "not set".
// Set Ellipsis = token.Pos(1) if ellipsis=true, token.NoPos if false.
```

#### Task 1.11: `kinds/expr_selector.go` — SelectorExpr
**Complexity:** S
```go
type SelectorExpr struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`
    Sel       string          `json:"sel"`
}
// ToAST: &ast.SelectorExpr{X: xAST.(ast.Expr), Sel: &ast.Ident{Name: n.Sel}}
```

#### Task 1.12: `kinds/expr_index.go` — IndexExpr
**Complexity:** S
```go
type IndexExpr struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`
    Index     json.RawMessage `json:"index"`
}
// ToAST: uses ast.IndexExpr (go 1.18+) from go/ast directly.
// Note: go/ast uses *ast.IndexExpr for single-index; go 1.18 adds IndexListExpr.
// Use typesinternal or direct cast: &ast.IndexExpr{X:..., Index:...}
```

#### Task 1.13: `kinds/expr_indexlist.go` — IndexListExpr
**Complexity:** S
```go
type IndexListExpr struct {
    KindField string            `json:"kind"`
    X         json.RawMessage   `json:"x"`
    Indices   []json.RawMessage `json:"indices"`
}
// Uses go/ast's IndexListExpr (generics, Go 1.18+).
// Import path: go/ast (it's in standard library since 1.18).
```

#### Task 1.14: `kinds/expr_slice.go` — SliceExpr
**Complexity:** S
```go
type SliceExpr struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`
    Low       json.RawMessage `json:"low,omitempty"`
    High      json.RawMessage `json:"high,omitempty"`
    Max       json.RawMessage `json:"max,omitempty"`   // 3-index slice
}
// ToAST: set Slice3 = true if Max is non-null
```

#### Task 1.15: `kinds/expr_typeassert.go` — TypeAssertExpr
**Complexity:** S
```go
type TypeAssertExpr struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`
    Type      json.RawMessage `json:"type,omitempty"` // null for .(type) in type switch
}
```

#### Task 1.16: `kinds/expr_funclit.go` — FuncLit
**Complexity:** S
```go
type FuncLit struct {
    KindField string          `json:"kind"`
    Type      json.RawMessage `json:"type"`  // FuncType
    Body      json.RawMessage `json:"body"`  // BlockStmt
}
```

#### Task 1.17: `kinds/expr_compositelit.go` — CompositeLit
**Complexity:** S
```go
type CompositeLit struct {
    KindField string            `json:"kind"`
    Type      json.RawMessage   `json:"type,omitempty"` // Expr|null
    Elts      []json.RawMessage `json:"elts"`
}
```

#### Task 1.18: `kinds/expr_keyvalue.go` — KeyValueExpr
**Complexity:** S
```go
type KeyValueExpr struct {
    KindField string          `json:"kind"`
    Key       json.RawMessage `json:"key"`
    Value     json.RawMessage `json:"value"`
}
```

#### Task 1.19: `kinds/expr_ellipsis.go` — Ellipsis
**Complexity:** S
```go
type Ellipsis struct {
    KindField string          `json:"kind"`
    Elt       json.RawMessage `json:"elt,omitempty"` // Expr|null
}
// ToAST: &ast.Ellipsis{Ellipsis: token.NoPos, Elt: eltAST}
```

---

### Tasks 1.20–1.25: Type kinds (6 files)

#### Task 1.20: `kinds/type_array.go` — ArrayType
**Complexity:** S
```go
type ArrayType struct {
    KindField string          `json:"kind"`
    Len       json.RawMessage `json:"len,omitempty"` // Expr|null; null = slice type
    Elt       json.RawMessage `json:"elt"`
}
// ToAST: if Len is null, set Len = nil (slice); else unmarshal and set
```

#### Task 1.21: `kinds/type_struct.go` — StructType
**Complexity:** S
```go
type StructType struct {
    KindField string            `json:"kind"`
    Fields    []json.RawMessage `json:"fields"` // [Field]
}
// ToAST: build *ast.FieldList from Fields slice
// Each json.RawMessage unmarshals to *Field (kind="Field")
// result.Fields = &ast.FieldList{List: astFields}
```

#### Task 1.22: `kinds/type_interface.go` — InterfaceType
**Complexity:** S
```go
type InterfaceType struct {
    KindField string            `json:"kind"`
    Methods   []json.RawMessage `json:"methods"` // [Field]
}
// ToAST: &ast.InterfaceType{Methods: &ast.FieldList{List: astFields}}
```

#### Task 1.23: `kinds/type_func.go` — FuncType
**Complexity:** S
```go
type FuncType struct {
    KindField  string            `json:"kind"`
    TypeParams []json.RawMessage `json:"type_params,omitempty"` // [Field]
    Params     []json.RawMessage `json:"params"`                  // [Field]
    Results    []json.RawMessage `json:"results,omitempty"`       // [Field]
}
// ToAST: set TypeParams only if non-empty (nil FieldList for empty)
// Note: ast.FuncType.TypeParams is *ast.FieldList; nil means no type params
```

#### Task 1.24: `kinds/type_map.go` — MapType
**Complexity:** S
```go
type MapType struct {
    KindField string          `json:"kind"`
    Key       json.RawMessage `json:"key"`
    Value     json.RawMessage `json:"value"`
}
```

#### Task 1.25: `kinds/type_chan.go` — ChanType
**Complexity:** S
```go
type ChanType struct {
    KindField string          `json:"kind"`
    Dir       string          `json:"dir"` // "SEND"|"RECV"|"BOTH"
    Value     json.RawMessage `json:"value"`
}
// ToAST: map "SEND"→ast.SEND, "RECV"→ast.RECV, "BOTH"→ast.BOTH
```

---

### Task 1.26: `kinds/field.go` — Field

**Complexity:** S
```go
// Namespace: goast/kinds
// Kind: Field
// go/ast: *ast.Field (reused for struct fields, interface methods, params, results)
type Field struct {
    KindField string          `json:"kind"`
    Names     []string        `json:"names,omitempty"`  // empty for anonymous/embedded
    Type      json.RawMessage `json:"type"`              // Expr
    Tag       *string         `json:"tag,omitempty"`    // nil if no tag
}
func (f *Field) Kind() string { return "Field" }
func (f *Field) ToAST() (ast.Node, error) {
    // Build []*ast.Ident from Names
    // Build type from Type
    // Build *ast.BasicLit for tag if non-nil
    result := &ast.Field{}
    for _, name := range f.Names {
        result.Names = append(result.Names, &ast.Ident{Name: name})
    }
    // ... type and tag
    return result, nil
}
func (f *Field) FromAST(node ast.Node) error {
    field := node.(*ast.Field)
    f.KindField = "Field"
    for _, name := range field.Names {
        f.Names = append(f.Names, name.Name)
    }
    // ... type and tag
    return nil
}
```

**Helper needed:** `unmarshalFieldList(msgs []json.RawMessage) ([]*ast.Field, error)` — converts
`[]json.RawMessage` (each a Field kind) to `[]*ast.Field`. Place in `kinds/field.go` or
`kinds/node.go`.

---

### Tasks 1.27–1.45: Statement kinds (19 files)

#### Task 1.27: `kinds/stmt_block.go` — BlockStmt
**Complexity:** S
```go
type BlockStmt struct {
    KindField string            `json:"kind"`
    List      []json.RawMessage `json:"list"` // [Stmt]
}
// ToAST: &ast.BlockStmt{Lbrace: token.NoPos, List: stmtList, Rbrace: token.NoPos}
// FromAST: iterate block.List, MarshalStmt each
```

#### Task 1.28: `kinds/stmt_if.go` — IfStmt
**Complexity:** S
```go
type IfStmt struct {
    KindField string          `json:"kind"`
    Init      json.RawMessage `json:"init,omitempty"`
    Cond      json.RawMessage `json:"cond"`
    Body      json.RawMessage `json:"body"`
    Else      json.RawMessage `json:"else,omitempty"`
}
// ToAST: all positions token.NoPos
// FromAST: MarshalStmt(init), MarshalExpr(cond), MarshalNode(body), MarshalStmt(else)
```

#### Task 1.29: `kinds/stmt_for.go` — ForStmt
**Complexity:** S
```go
type ForStmt struct {
    KindField string          `json:"kind"`
    Init      json.RawMessage `json:"init,omitempty"`
    Cond      json.RawMessage `json:"cond,omitempty"`
    Post      json.RawMessage `json:"post,omitempty"`
    Body      json.RawMessage `json:"body"`
}
```

#### Task 1.30: `kinds/stmt_range.go` — RangeStmt
**Complexity:** S
```go
type RangeStmt struct {
    KindField string          `json:"kind"`
    Key       json.RawMessage `json:"key,omitempty"`
    Value     json.RawMessage `json:"value,omitempty"`
    Tok       string          `json:"tok"` // ":="|"="|"ILLEGAL"
    X         json.RawMessage `json:"x"`
    Body      json.RawMessage `json:"body"`
}
// ToAST: tok mapping: ":="→token.DEFINE, "="→token.ASSIGN, "ILLEGAL"→token.ILLEGAL
```

#### Task 1.31: `kinds/stmt_switch.go` — SwitchStmt
**Complexity:** S
```go
type SwitchStmt struct {
    KindField string          `json:"kind"`
    Init      json.RawMessage `json:"init,omitempty"`
    Tag       json.RawMessage `json:"tag,omitempty"`
    Body      json.RawMessage `json:"body"` // BlockStmt with CaseClauses
}
```

#### Task 1.32: `kinds/stmt_typeswitch.go` — TypeSwitchStmt
**Complexity:** S
```go
type TypeSwitchStmt struct {
    KindField string          `json:"kind"`
    Init      json.RawMessage `json:"init,omitempty"`
    Assign    json.RawMessage `json:"assign"` // AssignStmt or ExprStmt
    Body      json.RawMessage `json:"body"`
}
```

#### Task 1.33: `kinds/stmt_select.go` — SelectStmt
**Complexity:** S
```go
type SelectStmt struct {
    KindField string          `json:"kind"`
    Body      json.RawMessage `json:"body"` // BlockStmt with CommClauses
}
```

#### Task 1.34: `kinds/stmt_case.go` — CaseClause
**Complexity:** S
```go
type CaseClause struct {
    KindField string            `json:"kind"`
    List      []json.RawMessage `json:"list"` // [Expr]; null = default case
    Body      []json.RawMessage `json:"body"` // [Stmt]
}
// CRITICAL: list: null → default case. Must check if List is nil vs empty.
// In JSON: omitempty won't work here — null is semantically meaningful.
// Use custom marshal or explicit null check.
// In FromAST: if caseClause.List == nil, set List to nil (not []).
// In JSON output: "list": null for default, "list": [...] for case.
```

#### Task 1.35: `kinds/stmt_comm.go` — CommClause
**Complexity:** S
```go
type CommClause struct {
    KindField string            `json:"kind"`
    Comm      json.RawMessage   `json:"comm,omitempty"` // Stmt|null; null = default
    Body      []json.RawMessage `json:"body"`
}
```

#### Task 1.36: `kinds/stmt_assign.go` — AssignStmt
**Complexity:** S
```go
type AssignStmt struct {
    KindField string            `json:"kind"`
    Lhs       []json.RawMessage `json:"lhs"`
    Tok       string            `json:"tok"` // ":="|"="|"+="|"-="|etc.
    Rhs       []json.RawMessage `json:"rhs"`
}
// Tok mapping covers all assignment operators.
// Full list: "=", ":=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
//            "<<=", ">>=", "&^="
```

#### Task 1.37: `kinds/stmt_return.go` — ReturnStmt
**Complexity:** S
```go
type ReturnStmt struct {
    KindField string            `json:"kind"`
    Results   []json.RawMessage `json:"results,omitempty"`
}
```

#### Task 1.38: `kinds/stmt_expr.go` — ExprStmt
**Complexity:** S
```go
type ExprStmt struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`
}
```

#### Task 1.39: `kinds/stmt_send.go` — SendStmt
**Complexity:** S
```go
type SendStmt struct {
    KindField string          `json:"kind"`
    Chan      json.RawMessage `json:"chan"`
    Value     json.RawMessage `json:"value"`
}
```

#### Task 1.40: `kinds/stmt_go.go` — GoStmt
**Complexity:** S
```go
type GoStmt struct {
    KindField string          `json:"kind"`
    Call      json.RawMessage `json:"call"` // CallExpr
}
// ToAST: &ast.GoStmt{Go: token.NoPos, Call: callAST.(*ast.CallExpr)}
```

#### Task 1.41: `kinds/stmt_defer.go` — DeferStmt
**Complexity:** S
```go
type DeferStmt struct {
    KindField string          `json:"kind"`
    Call      json.RawMessage `json:"call"` // CallExpr
}
```

#### Task 1.42: `kinds/stmt_incdec.go` — IncDecStmt
**Complexity:** S
```go
type IncDecStmt struct {
    KindField string          `json:"kind"`
    X         json.RawMessage `json:"x"`
    Tok       string          `json:"tok"` // "++"|"--"
}
// ToAST: "++"→token.INC, "--"→token.DEC
```

#### Task 1.43: `kinds/stmt_labeled.go` — LabeledStmt
**Complexity:** S
```go
type LabeledStmt struct {
    KindField string          `json:"kind"`
    Label     string          `json:"label"`
    Stmt      json.RawMessage `json:"stmt"`
}
```

#### Task 1.44: `kinds/stmt_branch.go` — BranchStmt
**Complexity:** S
```go
type BranchStmt struct {
    KindField string  `json:"kind"`
    Tok       string  `json:"tok"`             // "break"|"continue"|"goto"|"fallthrough"
    Label     *string `json:"label,omitempty"` // nil if no label
}
// ToAST: tok mapping, optional label ident
```

#### Task 1.45: `kinds/stmt_decl.go` — DeclStmt
**Complexity:** S
```go
type DeclStmt struct {
    KindField string          `json:"kind"`
    Decl      json.RawMessage `json:"decl"` // GenDecl (ImportDecl|ConstDecl|TypeDecl|VarDecl)
}
// ToAST: unmarshal Decl, ToAST, cast to ast.Decl
```

---

### Tasks 1.46–1.50: Declaration kinds (5 files)

All GenDecl variants share a ToAST pattern of building `*ast.GenDecl` with appropriate `Tok`.

#### Task 1.46: `kinds/decl_func.go` — FuncDecl
**Complexity:** S
```go
type FuncDecl struct {
    KindField string          `json:"kind"`
    Recv      json.RawMessage `json:"recv,omitempty"` // Field|null (single receiver field)
    Name      string          `json:"name"`
    Type      json.RawMessage `json:"type"`             // FuncType
    Body      json.RawMessage `json:"body,omitempty"`   // BlockStmt|null (nil for interface method decl)
}
// ToAST: recv → *ast.FieldList{List: []*ast.Field{recvField}}
// name → *ast.Ident{Name: n.Name}
```

#### Task 1.47: `kinds/decl_import.go` — ImportDecl
**Complexity:** S
```go
type ImportDecl struct {
    KindField string            `json:"kind"`
    Specs     []json.RawMessage `json:"specs"` // [ImportSpec]
}
// ToAST: &ast.GenDecl{Tok: token.IMPORT, Specs: [...]}
// If len(Specs) > 1, set Lparen = token.NoPos (go/format adds parens automatically)
// Actually: set Lparen = 1 if multiple specs (any non-zero value)
```

#### Task 1.48: `kinds/decl_const.go` — ConstDecl
**Complexity:** S
```go
type ConstDecl struct {
    KindField string            `json:"kind"`
    Specs     []json.RawMessage `json:"specs"` // [ValueSpec]
}
// ToAST: &ast.GenDecl{Tok: token.CONST, Specs: [...]}
```

#### Task 1.49: `kinds/decl_type.go` — TypeDecl
**Complexity:** S
```go
type TypeDecl struct {
    KindField string            `json:"kind"`
    Specs     []json.RawMessage `json:"specs"` // [TypeSpec]
}
// ToAST: &ast.GenDecl{Tok: token.TYPE, Specs: [...]}
```

#### Task 1.50: `kinds/decl_var.go` — VarDecl
**Complexity:** S
```go
type VarDecl struct {
    KindField string            `json:"kind"`
    Specs     []json.RawMessage `json:"specs"` // [ValueSpec]
}
// ToAST: &ast.GenDecl{Tok: token.VAR, Specs: [...]}
```

---

### Tasks 1.51–1.53: Spec kinds (3 files)

#### Task 1.51: `kinds/spec_import.go` — ImportSpec
**Complexity:** S
```go
type ImportSpec struct {
    KindField string  `json:"kind"`
    Name      *string `json:"name,omitempty"` // nil=none, "."=dot, "_"=blank
    Path      string  `json:"path"`
}
// ToAST: &ast.ImportSpec{
//   Name: nameIdent (nil if Name is nil),
//   Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(n.Path)},
// }
// FromAST: spec.Path.Value is already quoted; strip quotes for "path" field
//   → strings.Trim(spec.Path.Value, `"`) or strconv.Unquote
```

#### Task 1.52: `kinds/spec_value.go` — ValueSpec
**Complexity:** S
```go
type ValueSpec struct {
    KindField string            `json:"kind"`
    Names     []string          `json:"names"`
    Type      json.RawMessage   `json:"type,omitempty"`  // Expr|null
    Values    []json.RawMessage `json:"values,omitempty"` // [Expr]
}
```

#### Task 1.53: `kinds/spec_type.go` — TypeSpec
**Complexity:** S
```go
type TypeSpec struct {
    KindField  string            `json:"kind"`
    Name       string            `json:"name"`
    TypeParams []json.RawMessage `json:"type_params,omitempty"` // [Field]
    Type       json.RawMessage   `json:"type"`
}
// TypeParams → TypeParams *ast.FieldList (nil if empty)
```

---

### Task 1.54: `kinds/kinds_test.go` — Per-kind round-trip tests (ALL 50 KINDS)

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/kinds/kinds_test.go`
**Blocked by:** 1.2–1.53 (all kinds), 1.3 (MarshalNode)
**Complexity:** M

**Required: one round-trip test per kind (50 tests total)**

For EACH of the 50 node kinds, implement a test that:
1. Constructs the kind struct directly (or parses from a Go snippet)
2. Calls `ToAST()` to get a `go/ast` node
3. Calls `FromAST()` on the result to get a JSON node back
4. Asserts the round-trip is lossless (JSON output matches input)

The round-trip comparison should normalize by re-marshaling to JSON and comparing, NOT by
comparing positions (which are always zero in constructed nodes but real in parsed nodes).

**Test naming convention:** `TestRoundTrip_<KindName>` for all 50.

**Complete list of required round-trip tests:**

Expressions (16):
- `TestRoundTrip_Ident`
- `TestRoundTrip_BasicLit` — include INT, STRING, FLOAT variants
- `TestRoundTrip_BinaryExpr` — with nested SelectorExpr operands
- `TestRoundTrip_UnaryExpr`
- `TestRoundTrip_StarExpr`
- `TestRoundTrip_ParenExpr`
- `TestRoundTrip_CallExpr` — ellipsis=false variant
- `TestRoundTrip_CallExprEllipsis` — ellipsis=true variant
- `TestRoundTrip_SelectorExpr`
- `TestRoundTrip_IndexExpr`
- `TestRoundTrip_IndexListExpr`
- `TestRoundTrip_SliceExpr` — 2-index and 3-index variants
- `TestRoundTrip_TypeAssertExpr`
- `TestRoundTrip_FuncLit`
- `TestRoundTrip_CompositeLit`
- `TestRoundTrip_KeyValueExpr`
- `TestRoundTrip_Ellipsis`

Types (6):
- `TestRoundTrip_ArrayType` — len=null (slice) and len=N (array) variants
- `TestRoundTrip_StructType`
- `TestRoundTrip_InterfaceType`
- `TestRoundTrip_FuncType` — with type_params for generics
- `TestRoundTrip_MapType`
- `TestRoundTrip_ChanType` — SEND, RECV, BOTH variants

Field (1):
- `TestRoundTrip_Field` — named, unnamed/embedded, with tag variants

Statements (19):
- `TestRoundTrip_BlockStmt`
- `TestRoundTrip_IfStmt` — with init, with else, without both
- `TestRoundTrip_ForStmt`
- `TestRoundTrip_RangeStmt` — := and = tok variants
- `TestRoundTrip_SwitchStmt`
- `TestRoundTrip_TypeSwitchStmt`
- `TestRoundTrip_SelectStmt`
- `TestRoundTrip_CaseClause` — regular case AND default case (list=null)
- `TestRoundTrip_CommClause`
- `TestRoundTrip_AssignStmt` — := and = and += variants
- `TestRoundTrip_ReturnStmt`
- `TestRoundTrip_ExprStmt`
- `TestRoundTrip_SendStmt`
- `TestRoundTrip_GoStmt`
- `TestRoundTrip_DeferStmt`
- `TestRoundTrip_IncDecStmt` — ++ and -- variants
- `TestRoundTrip_LabeledStmt`
- `TestRoundTrip_BranchStmt` — break, continue, goto, fallthrough variants
- `TestRoundTrip_DeclStmt`

Declarations (5):
- `TestRoundTrip_FuncDecl` — with receiver (method), without (function), without body (interface)
- `TestRoundTrip_ImportDecl` — single and multi-spec variants
- `TestRoundTrip_ConstDecl`
- `TestRoundTrip_TypeDecl`
- `TestRoundTrip_VarDecl`

Specs (3):
- `TestRoundTrip_ImportSpec` — verify path is unquoted in JSON
- `TestRoundTrip_ValueSpec`
- `TestRoundTrip_TypeSpec` — with and without type_params

**Round-trip test helper to implement:**
```go
// roundTripAST parses src, extracts the named ast.Node by walking the AST,
// marshals it to JSON via MarshalNode, unmarshals via UnmarshalNode,
// calls ToAST, formats both original and round-tripped node, and asserts equality.
func roundTripAST(t *testing.T, src string, extract func(*ast.File) ast.Node)

// roundTripJSON constructs a node struct directly, calls ToAST, formats result,
// then calls FromAST on the result and verifies the JSON output matches original.
func roundTripJSON(t *testing.T, node kinds.Node, expectedKind string)
```

**Lossless comparison approach:** For round-trip tests, format both the original and round-tripped
AST nodes to source text using `go/format` and compare the formatted strings. This is
position-independent and verifies semantic equivalence.

---

### Task 1.54b: `kinds/golden_test.go` — Golden path integration test

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/kinds/golden_test.go`
**Blocked by:** 1.2–1.53 (all kinds), 1.3 (MarshalNode)
**Complexity:** M
**Priority:** REQUIRED in Tier 1 — this is the highest-value correctness test

**Purpose:** Prove the complete ToAST/format pipeline end-to-end by constructing a real `package main`
program entirely from JSON node structs (no string snippets), running it, and asserting correct output.

**Test: `TestGoldenPathHelloProgram`**

```go
func TestGoldenPathHelloProgram(t *testing.T) {
    // Construct the following program entirely from JSON node structs:
    //
    //   package main
    //
    //   import "fmt"
    //
    //   func main() {
    //       fmt.Println(true)
    //   }
    //
    // Expected output: "true\n"

    // 1. Build the node tree using kinds structs directly.
    //    The tree is:
    //    File {
    //      Name: "main",
    //      Decls: [
    //        ImportDecl { Specs: [ ImportSpec{ Path: "fmt" } ] },
    //        FuncDecl {
    //          Name: "main",
    //          Type: FuncType{ Params: [], Results: [] },
    //          Body: BlockStmt { List: [
    //            ExprStmt { X: CallExpr {
    //              Fun: SelectorExpr {
    //                X: Ident{ Name: "fmt" },
    //                Sel: "Println"
    //              },
    //              Args: [ BasicLit{ Tok: "BOOL", Value: "true" } ],
    //              Ellipsis: false
    //            }}
    //          ]}
    //        }
    //      ]
    //    }

    // Build bottom-up using kinds structs:
    importSpecJSON, _ := json.Marshal(&kinds.ImportSpec{KindField: "ImportSpec", Path: "fmt"})
    importDeclJSON, _ := json.Marshal(&kinds.ImportDecl{
        KindField: "ImportDecl",
        Specs: []json.RawMessage{importSpecJSON},
    })

    fmtIdentJSON, _ := json.Marshal(&kinds.Ident{KindField: "Ident", Name: "fmt"})
    selectorJSON, _ := json.Marshal(&kinds.SelectorExpr{
        KindField: "SelectorExpr",
        X:         fmtIdentJSON,
        Sel:       "Println",
    })
    trueJSON, _ := json.Marshal(&kinds.BasicLit{KindField: "BasicLit", Tok: "IDENT", Value: "true"})
    // Note: "true" is an identifier in Go AST, not a literal. Use Ident instead:
    trueIdentJSON, _ := json.Marshal(&kinds.Ident{KindField: "Ident", Name: "true"})

    callJSON, _ := json.Marshal(&kinds.CallExpr{
        KindField: "CallExpr",
        Fun:       selectorJSON,
        Args:      []json.RawMessage{trueIdentJSON},
        Ellipsis:  false,
    })
    exprStmtJSON, _ := json.Marshal(&kinds.ExprStmt{KindField: "ExprStmt", X: callJSON})
    bodyJSON, _ := json.Marshal(&kinds.BlockStmt{
        KindField: "BlockStmt",
        List:      []json.RawMessage{exprStmtJSON},
    })
    funcTypeJSON, _ := json.Marshal(&kinds.FuncType{
        KindField: "FuncType",
        Params:    []json.RawMessage{},
        Results:   []json.RawMessage{},
    })
    funcDeclJSON, _ := json.Marshal(&kinds.FuncDecl{
        KindField: "FuncDecl",
        Name:      "main",
        Type:      funcTypeJSON,
        Body:      bodyJSON,
    })

    // 2. Call ToAST on each node, building the ast.File manually.
    importDecl, err := kinds.UnmarshalNode(importDeclJSON)
    require.NoError(t, err)
    importDeclAST, err := importDecl.ToAST()
    require.NoError(t, err)

    funcDecl, err := kinds.UnmarshalNode(funcDeclJSON)
    require.NoError(t, err)
    funcDeclAST, err := funcDecl.ToAST()
    require.NoError(t, err)

    // 3. Assemble ast.File.
    fset := token.NewFileSet()
    file := &ast.File{
        Name:  &ast.Ident{Name: "main"},
        Decls: []ast.Decl{importDeclAST.(ast.Decl), funcDeclAST.(ast.Decl)},
    }

    // 4. Format to source using go/format.
    var buf bytes.Buffer
    err = format.Node(&buf, fset, file)
    require.NoError(t, err, "go/format must succeed with NoPos nodes")
    src := buf.String()

    // 5. Write to temp file and run with go run.
    dir := t.TempDir()
    goFile := filepath.Join(dir, "main.go")
    err = os.WriteFile(goFile, []byte(src), 0644)
    require.NoError(t, err)

    cmd := exec.Command("go", "run", goFile)
    out, err := cmd.Output()
    require.NoError(t, err, "go run must succeed; source:\n%s", src)
    assert.Equal(t, "true\n", string(out))
}
```

**Important implementation note on `true` in Go AST:**
In Go's AST, `true` is an `*ast.Ident`, not a `*ast.BasicLit`. The `BasicLit` kinds are only
INT, FLOAT, IMAG, CHAR, STRING. So the correct node for `true` is:
`&ast.Ident{Name: "true"}` → JSON: `{"kind":"Ident","name":"true"}`.

**Why this test is the highest-value test:**
- Exercises the complete pipeline: JSON structs → ToAST → go/format → source → compilation → execution
- If it passes, the ToAST/NoPos approach is confirmed working for the entire tree
- Catches any issue where `go/format` fails on constructed trees
- Verifies the fmt import and SelectorExpr pipeline works correctly
- The test is deterministic: "true\n" is the only correct output

**Note:** This test requires `go` to be on PATH (it invokes `go run`). It is a functional test,
not a unit test, and may be slower (~1-2s for `go run`). It belongs in `kinds/golden_test.go`
with build tag or separate test flag if needed, but should run by default in CI.

**Alternative placement:** If the golden test is better placed in `editor/editor_test.go` (after
the editor package exists), it can be moved there. The key requirement is that it exists and runs
as part of `go test ./...`.

---

### Task 1.55: `testdata/` — Test data files

**Files:**
- `/home/matt/source/goast-worktrees/goast-mcp-server/testdata/simple.go`
- `/home/matt/source/goast-worktrees/goast-mcp-server/testdata/generics.go`
- `/home/matt/source/goast-worktrees/goast-mcp-server/testdata/channels.go`

**Blocked by:** nothing
**Complexity:** S

`simple.go` — covers: function with params/results, struct type, interface type, methods,
if/for/range/switch statements, assignments, returns, defer, go statements.

`generics.go` — covers: generic function, generic type, IndexListExpr.

`channels.go` — covers: channel send/receive, select with CommClauses.

These files must be **valid Go** (parseable) but need not compile (can reference undefined types).

---

### Task 1.56: `selector/selector.go` — Path navigation

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/selector/selector.go`
**Blocked by:** 1.55 (testdata)
**Complexity:** L

**Types and functions:**

```go
package selector

import (
    "fmt"
    "go/ast"
)

// PathStep is one step in a selector path.
type PathStep struct {
    Kind  string `json:"kind"`
    Name  string `json:"name,omitempty"`
    Recv  string `json:"recv,omitempty"`
    Index *int   `json:"index,omitempty"` // pointer to distinguish 0 from absent
}

// ParentContext records how the target node is held by its parent.
// Used by insert/replace/delete to know how to mutate the parent.
type ParentContext struct {
    Parent    ast.Node // the parent node
    FieldName string   // field name in parent: "List", "Specs", "Args", etc.
    Index     int      // position in slice (-1 if not in a slice, e.g. scalar field)
}

// NavigateError is returned when a path step cannot be resolved.
type NavigateError struct {
    AtStep    int      // which step (0-based) failed
    Step      PathStep // the step that failed
    Available []string // human-readable list of what was available, e.g. "IfStmt[0]"
}

func (e *NavigateError) Error() string {
    return fmt.Sprintf("step %d (%s): not found; available: %v", e.AtStep, e.Step.Kind, e.Available)
}

// Navigate walks the file AST following steps, returning the target node and
// how it is held by its parent. The ParentContext is needed for insert/delete.
func Navigate(file *ast.File, steps []PathStep) (ast.Node, ParentContext, error)
```

**Implementation structure — Navigate function:**

```go
func Navigate(file *ast.File, steps []PathStep) (ast.Node, ParentContext, error) {
    var current ast.Node = file
    var parent ParentContext

    for i, step := range steps {
        next, ctx, err := applyStep(current, step, i)
        if err != nil {
            return nil, ParentContext{}, err
        }
        parent = ctx
        current = next
    }
    return current, parent, nil
}

func applyStep(current ast.Node, step PathStep, stepIdx int) (ast.Node, ParentContext, error) {
    switch step.Kind {
    case "FuncDecl":     return stepFuncDecl(current, step, stepIdx)
    case "TypeDecl":     return stepTypeDecl(current, step, stepIdx)
    case "TypeSpec":     return stepTypeSpec(current, step, stepIdx)
    case "VarDecl":      return stepVarDecl(current, step, stepIdx)
    case "ConstDecl":    return stepConstDecl(current, step, stepIdx)
    case "ImportDecl":   return stepImportDecl(current, step, stepIdx)
    case "StructType":   return stepStructType(current, step, stepIdx)
    case "InterfaceType":return stepInterfaceType(current, step, stepIdx)
    case "Field":        return stepField(current, step, stepIdx)
    case "Body":         return stepBody(current, step, stepIdx)
    case "Params":       return stepParams(current, step, stepIdx)
    case "Results":      return stepResults(current, step, stepIdx)
    case "IfStmt":       return stepIndexedStmt[*ast.IfStmt](current, step, stepIdx, "IfStmt")
    case "ForStmt":      return stepIndexedStmt[*ast.ForStmt](current, step, stepIdx, "ForStmt")
    case "RangeStmt":    return stepIndexedStmt[*ast.RangeStmt](current, step, stepIdx, "RangeStmt")
    case "SwitchStmt":   return stepIndexedStmt[*ast.SwitchStmt](current, step, stepIdx, "SwitchStmt")
    case "TypeSwitchStmt":return stepIndexedStmt[*ast.TypeSwitchStmt](current, step, stepIdx, "TypeSwitchStmt")
    case "SelectStmt":   return stepIndexedStmt[*ast.SelectStmt](current, step, stepIdx, "SelectStmt")
    case "CaseClause":   return stepCaseClause(current, step, stepIdx)
    case "CommClause":   return stepCommClause(current, step, stepIdx)
    case "AssignStmt":   return stepIndexedStmt[*ast.AssignStmt](current, step, stepIdx, "AssignStmt")
    case "ReturnStmt":   return stepIndexedStmt[*ast.ReturnStmt](current, step, stepIdx, "ReturnStmt")
    case "ExprStmt":     return stepIndexedStmt[*ast.ExprStmt](current, step, stepIdx, "ExprStmt")
    case "GoStmt":       return stepIndexedStmt[*ast.GoStmt](current, step, stepIdx, "GoStmt")
    case "DeferStmt":    return stepIndexedStmt[*ast.DeferStmt](current, step, stepIdx, "DeferStmt")
    case "Stmt":         return stepStmtByIndex(current, step, stepIdx)
    case "Cond":         return stepCond(current, step, stepIdx)
    case "Init":         return stepInit(current, step, stepIdx)
    case "Post":         return stepPost(current, step, stepIdx)
    case "Else":         return stepElse(current, step, stepIdx)
    case "Tag":          return stepTag(current, step, stepIdx)
    case "Lhs":          return stepLhsRhs(current, step, stepIdx, true)
    case "Rhs":          return stepLhsRhs(current, step, stepIdx, false)
    case "Key":          return stepKey(current, step, stepIdx)
    case "Value":        return stepValue(current, step, stepIdx)
    case "X":            return stepX(current, step, stepIdx)
    case "Y":            return stepY(current, step, stepIdx)
    case "Fun":          return stepFun(current, step, stepIdx)
    case "Args":         return stepArgs(current, step, stepIdx)
    case "Sel":          return stepSel(current, step, stepIdx)
    case "Elts":         return stepElts(current, step, stepIdx)
    default:
        return nil, ParentContext{}, &NavigateError{AtStep: stepIdx, Step: step}
    }
}
```

**Key step implementations:**

```go
// stepFuncDecl: find FuncDecl in file.Decls by name and optional recv
func stepFuncDecl(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
    file, ok := current.(*ast.File)
    if !ok {
        return nil, ParentContext{}, &NavigateError{...}
    }
    var available []string
    for i, decl := range file.Decls {
        fd, ok := decl.(*ast.FuncDecl)
        if !ok { continue }
        if fd.Name.Name == step.Name {
            if step.Recv == "" || recvMatches(fd, step.Recv) {
                return fd, ParentContext{Parent: file, FieldName: "Decls", Index: i}, nil
            }
        }
        available = append(available, fmt.Sprintf("FuncDecl(%s)", fd.Name.Name))
    }
    return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

// stepBody: get BlockStmt from FuncDecl or FuncLit
func stepBody(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
    switch n := current.(type) {
    case *ast.FuncDecl:
        if n.Body == nil {
            return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: []string{"(no body)"}}
        }
        return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
    case *ast.FuncLit:
        return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
    default:
        return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
    }
}

// stepIndexedStmt: finds the Nth statement of a specific ast type in a block list
// Uses generics (Go 1.18+) to avoid repetition.
func stepIndexedStmt[T ast.Stmt](current ast.Node, step PathStep, idx int, kindName string) (ast.Node, ParentContext, error) {
    list, parent, err := getStmtList(current, idx, step)
    if err != nil { return nil, ParentContext{}, err }

    target := 0
    if step.Index != nil { target = *step.Index }

    count := 0
    var available []string
    for i, stmt := range list {
        if _, ok := stmt.(T); ok {
            if count == target {
                return stmt, ParentContext{Parent: parent, FieldName: "List", Index: i}, nil
            }
            available = append(available, fmt.Sprintf("%s[%d]", kindName, count))
            count++
        }
    }
    return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

// getStmtList extracts the []ast.Stmt from a BlockStmt, CaseClause, or CommClause
func getStmtList(current ast.Node, idx int, step PathStep) ([]ast.Stmt, ast.Node, error) {
    switch n := current.(type) {
    case *ast.BlockStmt:
        return n.List, n, nil
    case *ast.CaseClause:
        return n.Body, n, nil
    case *ast.CommClause:
        return n.Body, n, nil
    default:
        return nil, nil, &NavigateError{AtStep: idx, Step: step}
    }
}

// stepStmtByIndex: Stmt[N] — finds Nth statement of any kind
func stepStmtByIndex(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
    list, parent, err := getStmtList(current, idx, step)
    if err != nil { return nil, ParentContext{}, err }
    target := 0
    if step.Index != nil { target = *step.Index }
    if target < 0 { target = len(list) + target } // negative indexing
    if target < 0 || target >= len(list) {
        return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step,
            Available: []string{fmt.Sprintf("0..%d", len(list)-1)}}
    }
    return list[target], ParentContext{Parent: parent, FieldName: "List", Index: target}, nil
}
```

**All 35 field accessor steps** (Cond, Init, Post, Else, Tag, X, Y, Fun, Key, Value, Sel, Args,
Lhs, Rhs, Elts, etc.) follow the same pattern: type-switch current, extract field, return with
ParentContext{Index: -1} for scalar fields or ParentContext{Index: i} for slice elements.

**Available list generation:** For navigation errors, build a human-readable list of what was
actually at the current position. For a BlockStmt with 3 stmts, return something like
`["IfStmt[0]", "AssignStmt[1]", "ReturnStmt[2]"]`. This is used in error responses.

---

### Task 1.57: `selector/selector_test.go` — Selector tests

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/selector/selector_test.go`
**Blocked by:** 1.55 (testdata), 1.56 (selector)
**Complexity:** M

**Test cases:**
- `TestNavigateFuncDecl` — navigate to named func in testdata/simple.go
- `TestNavigateFuncDeclWithRecv` — navigate to method with receiver
- `TestNavigateBody` — navigate to FuncDecl.Body
- `TestNavigateIfStmtByIndex` — navigate to IfStmt[0], IfStmt[1]
- `TestNavigateCond` — navigate to IfStmt.Cond
- `TestNavigateElse` — navigate to IfStmt.Else
- `TestNavigateStmtByIndex` — navigate to Stmt[N] of any kind
- `TestNavigateTypeSpec` — navigate to TypeSpec by name
- `TestNavigateField` — navigate to struct field by name and index
- `TestNavigateArgs` — navigate to CallExpr argument by index
- `TestNavigateNestedPath` — multi-step path into deeply nested node
- `TestNavigateErrorMissing` — error when name not found, Available list populated
- `TestNavigateErrorWrongContext` — Body on non-FuncDecl returns error
- `TestParentContextForDelete` — verify ParentContext for block list items
- `TestParentContextForScalar` — verify Index=-1 for scalar fields

---

### Task 1.58: `editor/editor.go` — Parse → edit → format → write cycle

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/editor/editor.go`
**Blocked by:** 1.1 (diff), 1.55 (testdata)
**Complexity:** M

**Functions:**

```go
package editor

import (
    "bytes"
    "go/format"
    "go/parser"
    "go/token"
    "os"
    "path/filepath"
)

// Result is returned by Edit and DryRun.
type Result struct {
    Diff    string // unified diff; empty if no changes
    Changed bool   // true if source changed
}

// Edit parses the file, calls fn to mutate the AST, formats, computes diff,
// and writes atomically. If dry_run is true, skips the write.
func Edit(path string, dryRun bool, fn func(*ast.File, *token.FileSet) error) (Result, error)

// ParseFile parses a file and returns the AST and FileSet.
// Exported for ops that need to read without editing.
func ParseFile(path string) (*ast.File, *token.FileSet, []byte, error)

// WriteAtomic writes content to path via a temp file + rename.
// Exported so ops/gomod.go can use it for go.mod writes.
func WriteAtomic(path string, content []byte) error
```

**Edit implementation:**
```go
func Edit(path string, dryRun bool, fn func(*ast.File, *token.FileSet) error) (Result, error) {
    f, fset, original, err := ParseFile(path)
    if err != nil { return Result{}, err }

    if err := fn(f, fset); err != nil {
        return Result{}, err
    }

    var buf bytes.Buffer
    if err := format.Node(&buf, fset, f); err != nil {
        return Result{}, fmt.Errorf("format: %w", err)
    }
    formatted := buf.Bytes()

    diffStr, err := diff.Files(path, original, formatted)
    if err != nil { return Result{}, err }

    changed := diffStr != ""
    if changed && !dryRun {
        if err := WriteAtomic(path, formatted); err != nil {
            return Result{}, err
        }
    }
    return Result{Diff: diffStr, Changed: changed}, nil
}
```

**ParseFile implementation:**
```go
func ParseFile(path string) (*ast.File, *token.FileSet, []byte, error) {
    src, err := os.ReadFile(path)
    if err != nil { return nil, nil, nil, err }
    fset := token.NewFileSet()
    f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
    if err != nil { return nil, nil, nil, fmt.Errorf("parse %s: %w", path, err) }
    return f, fset, src, nil
}
```

**WriteAtomic:**
```go
func WriteAtomic(path string, content []byte) error {
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, ".goast-*.go")
    if err != nil { return err }
    tmpName := tmp.Name()
    defer func() { os.Remove(tmpName) }() // no-op if rename succeeded
    if _, err := tmp.Write(content); err != nil {
        tmp.Close()
        return err
    }
    if err := tmp.Close(); err != nil { return err }
    return os.Rename(tmpName, path)
}
```

---

### Task 1.59: `editor/editor_test.go` — Editor tests

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/editor/editor_test.go`
**Blocked by:** 1.58 (editor)
**Complexity:** S

**Test cases:**
- `TestEditNoChange` — fn makes no changes → Result.Changed = false, diff empty
- `TestEditWithChange` — fn renames an ident → diff shows change, file updated
- `TestEditDryRun` — fn makes change, dryRun=true → diff returned, file NOT updated
- `TestEditAtomicWrite` — after error in fn, original file unchanged
- `TestParseFile` — verify ParseFile returns correct AST

Use `t.TempDir()` + copying testdata files for all write tests.

---

### Task 1.60: `meta/meta.go` — Node metadata computation

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/meta/meta.go`
**Test:** `/home/matt/source/goast-worktrees/goast-mcp-server/meta/meta_test.go`
**Blocked by:** nothing (pure go/ast, no kinds dependency)
**Complexity:** M

**Types and functions:**

```go
package meta

import (
    "go/ast"
    "go/token"
)

// Meta is a flat map of derived, read-only node properties.
type Meta map[string]interface{}

// Compute returns metadata for node within file, given the FileSet, source bytes,
// the parent node, and the nesting depth.
// Returns only fields applicable to the node kind.
func Compute(fset *token.FileSet, src []byte, node ast.Node, parent ast.Node, depth int) Meta

// FileInfo returns file-level metadata (for ast_list and empty-path ast_query).
func FileInfo(fset *token.FileSet, src []byte, file *ast.File) Meta
```

**Compute implementation pattern:**
```go
func Compute(fset *token.FileSet, src []byte, node ast.Node, parent ast.Node, depth int) Meta {
    m := Meta{}
    // Universal fields
    pos := fset.Position(node.Pos())
    endPos := fset.Position(node.End())
    m["line"] = pos.Line
    m["end_line"] = endPos.Line
    m["col"] = pos.Column
    m["byte_offset"] = pos.Offset
    m["byte_end"] = endPos.Offset
    m["depth"] = depth
    if parent != nil {
        m["parent_kind"] = parentKindName(parent)
    }

    // Kind-specific fields
    switch n := node.(type) {
    case *ast.FuncDecl:
        m["exported"] = ast.IsExported(n.Name.Name)
        m["is_method"] = n.Recv != nil
        if n.Recv != nil && len(n.Recv.List) > 0 {
            m["recv_type"] = recvTypeName(n.Recv.List[0])
        }
        if n.Type.Params != nil {
            m["param_count"] = len(n.Type.Params.List)
        }
        if n.Type.Results != nil {
            m["result_count"] = len(n.Type.Results.List)
            m["has_error_return"] = hasErrorReturn(n.Type.Results)
        }
        if n.Body != nil {
            m["stmt_count"] = len(n.Body.List)
            m["cyclomatic_complexity"] = cyclomaticComplexity(n)
        }
        m["is_variadic"] = isVariadic(n.Type.Params)
    case *ast.TypeSpec:
        // ...
    case *ast.StructType:
        // ...
    // etc.
    }
    return m
}
```

**Cyclomatic complexity:** `1 + count of (IfStmt, ForStmt, RangeStmt, CaseClause not default,
CommClause not default, BinaryExpr with && or ||, TypeSwitchStmt)` within a FuncDecl body.
Walk with `ast.Inspect`.

**parentKindName:** Returns the kind string for a parent node (type switch to get name).

**Meta fields to implement** (from design.md):
- Universal: line, end_line, col, byte_offset, byte_end, parent_kind, depth
- File: package, line_count, decl_count, func_count, type_count, import_count, has_init
- FuncDecl: exported, is_method, recv_type, param_count, result_count, has_error_return, stmt_count, cyclomatic_complexity, is_variadic
- TypeSpec: exported, underlying_kind, is_alias, has_type_params
- StructType: field_count, has_embedded, exported_field_count
- InterfaceType: method_count, embed_count, is_empty
- IfStmt/ForStmt/RangeStmt: has_init, has_else, else_is_if, body_stmt_count
- SwitchStmt/TypeSwitchStmt: case_count, has_default
- SelectStmt: case_count, has_default
- CallExpr: arg_count, is_variadic_call, callee
- Field: exported, is_embedded, has_tag, name_count

---

### Task 1.61: `ops/query.go` — Read tools

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/ops/query.go`
**Blocked by:** 1.56 (selector), 1.58 (editor), 1.60 (meta), 1.2–1.53 (kinds)
**Complexity:** M

**Functions:**

```go
package ops

// ASTListArgs is the argument struct for ast_list.
type ASTListArgs struct {
    File string `json:"file"`
}

// ASTListItem is one entry in the ast_list response.
type ASTListItem struct {
    Kind string `json:"kind"`
    Name string `json:"name,omitempty"`
    Recv string `json:"recv,omitempty"`
    Line int    `json:"line"`
}

// HandleASTList implements the ast_list tool.
func HandleASTList(ctx context.Context, req mcp.CallToolRequest, args ASTListArgs) (*mcp.CallToolResult, error)

// ASTQueryArgs is the argument struct for ast_query.
type ASTQueryArgs struct {
    File string          `json:"file"`
    Path json.RawMessage `json:"path"` // []selector.PathStep
}

// ASTQueryResponse is the response for ast_query.
type ASTQueryResponse struct {
    Node   json.RawMessage `json:"node"`
    Source string          `json:"source,omitempty"` // source text of node
    Meta   meta.Meta       `json:"meta,omitempty"`
}

// HandleASTQuery implements the ast_query tool.
func HandleASTQuery(ctx context.Context, req mcp.CallToolRequest, args ASTQueryArgs) (*mcp.CallToolResult, error)

// ASTQueryManyArgs is the argument struct for ast_query_many.
type ASTQueryManyArgs struct {
    File  string            `json:"file"`
    Paths []json.RawMessage `json:"paths"` // [][]selector.PathStep
}

// HandleASTQueryMany implements the ast_query_many tool.
func HandleASTQueryMany(ctx context.Context, req mcp.CallToolRequest, args ASTQueryManyArgs) (*mcp.CallToolResult, error)

// ASTMetaArgs is the argument struct for ast_meta.
type ASTMetaArgs struct {
    File string          `json:"file"`
    Path json.RawMessage `json:"path"` // []selector.PathStep; empty=file level
}

// HandleASTMeta implements the ast_meta tool (AST-derived meta only, no hooks in Tier 1).
func HandleASTMeta(ctx context.Context, req mcp.CallToolRequest, args ASTMetaArgs) (*mcp.CallToolResult, error)
```

**ast_list response:** For each top-level `ast.Decl` in `file.Decls`, produce an `ASTListItem`.
For `*ast.FuncDecl`: kind="FuncDecl", name=fd.Name.Name, recv=recvTypeName (if method).
For `*ast.GenDecl` with `Tok==token.IMPORT`: kind="ImportDecl", list all specs.
For `*ast.GenDecl` with `Tok==token.TYPE`: kind="TypeDecl", name=first spec name.
For `*ast.GenDecl` with `Tok==token.VAR`: kind="VarDecl".
For `*ast.GenDecl` with `Tok==token.CONST`: kind="ConstDecl".

**Source extraction:** In ast_query, after navigating to the node, extract the source text:
```go
pos := fset.Position(node.Pos())
end := fset.Position(node.End())
if pos.IsValid() && end.IsValid() && end.Offset <= len(src) {
    response.Source = string(src[pos.Offset:end.Offset])
}
```

**Error response format** (for path failures):
```go
type ErrorResponse struct {
    Error     string             `json:"error"`
    AtStep    int                `json:"at_step,omitempty"`
    Step      *selector.PathStep `json:"step,omitempty"`
    Available []string           `json:"available,omitempty"`
}
```

Return as `mcp.NewToolResultError(...)` with JSON-marshalled error for NavigateError cases.

---

### Task 1.62: `ops/insert.go` — ast_insert

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/ops/insert.go`
**Blocked by:** 1.56 (selector), 1.58 (editor), 1.61 (query for response pattern)
**Complexity:** M

```go
type ASTInsertArgs struct {
    File   string          `json:"file"`
    Path   json.RawMessage `json:"path"`  // []selector.PathStep; must point to a list
    Index  int             `json:"index"` // -1 = append
    Node   json.RawMessage `json:"node"`  // Node to insert
    DryRun bool            `json:"dry_run"`
}

func HandleASTInsert(ctx context.Context, req mcp.CallToolRequest, args ASTInsertArgs) (*mcp.CallToolResult, error)
```

**insertIntoList performs splice:**
```go
func insertIntoList(ctx selector.ParentContext, newNode ast.Node, index int) error {
    switch parent := ctx.Parent.(type) {
    case *ast.BlockStmt:
        stmt := newNode.(ast.Stmt)
        parent.List = insertAt(parent.List, stmt, index)
    case *ast.FieldList:
        field := newNode.(*ast.Field)
        parent.List = insertAt(parent.List, field, index)
    // etc. for all list-bearing nodes
    }
}

func insertAt[T any](list []T, item T, index int) []T {
    if index < 0 || index >= len(list) { return append(list, item) }
    list = append(list, item) // grow
    copy(list[index+1:], list[index:])
    list[index] = item
    return list
}
```

**Insert targets (all list-type fields):**
- `*ast.BlockStmt.List` — statements
- `*ast.FieldList.List` — fields/params/results
- `*ast.File.Decls` — top-level declarations
- `*ast.CallExpr.Args` — function arguments
- `*ast.CompositeLit.Elts` — composite literal elements
- `*ast.CaseClause.List` — case expressions
- `*ast.CaseClause.Body` — case body statements
- `*ast.CommClause.Body` — comm clause body

---

### Task 1.63: `ops/replace.go` — ast_replace

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/ops/replace.go`
**Blocked by:** 1.56 (selector), 1.58 (editor)
**Complexity:** M

```go
type ASTReplaceArgs struct {
    File   string          `json:"file"`
    Path   json.RawMessage `json:"path"` // []selector.PathStep; points to node to replace
    Node   json.RawMessage `json:"node"` // replacement node
    DryRun bool            `json:"dry_run"`
}
```

**replaceInParent** type-switches on `ctx.Parent` and assigns replacement to the correct field.
Covers all ~50 parent types and their list/scalar fields using the same exhaustive
type switch pattern as MarshalNode. Default case returns a detailed error.

---

### Task 1.64: `ops/delete.go` — ast_delete

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/ops/delete.go`
**Blocked by:** 1.56 (selector), 1.58 (editor)
**Complexity:** M

```go
type ASTDeleteArgs struct {
    File   string          `json:"file"`
    Path   json.RawMessage `json:"path"` // points to node to delete
    DryRun bool            `json:"dry_run"`
}
```

**deleteFromList** checks `ctx.Index < 0` → error "cannot delete scalar field". Otherwise
slices the element out of the parent's list.

---

### Task 1.65: `ops/imports.go` — Import tools

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/ops/imports.go`
**Blocked by:** 1.58 (editor)
**Complexity:** S

```go
type AddImportArgs struct {
    File  string `json:"file"`
    Path  string `json:"path"`
    Alias string `json:"alias"` // ""=none, "."=dot, "_"=blank, or identifier
}

type DeleteImportArgs struct {
    File string `json:"file"`
    Path string `json:"path"`
}

type ListImportsArgs struct {
    File string `json:"file"`
}

type ImportInfo struct {
    Path  string `json:"path"`
    Alias string `json:"alias,omitempty"`
    Used  bool   `json:"used"`
}
```

**HandleAddImport:** Uses `editor.Edit` + `astutil.AddNamedImport(fset, f, alias, path)`.
**HandleDeleteImport:** Uses `editor.Edit` + `astutil.DeleteImport(fset, f, path)`.
**HandleListImports:** Parses file, walks `f.Imports`, returns alias + used detection.

---

### Task 1.66: `ops/gomod.go` — go.mod tools

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/ops/gomod.go`
**Blocked by:** 1.58 (editor, for WriteAtomic export)
**Complexity:** S

All mutating handlers: Read → parse → mutate → `f.Format()` → diff → `editor.WriteAtomic`.

---

### Task 1.67: `server.go` — Tool registration

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/server.go`
**Blocked by:** 1.61–1.66 (all ops)
**Complexity:** M

Registers all Tier 1 tools with `mcp.NewTypedToolHandler`. All tool descriptions taken from design.md verbatim.

---

### Task 1.68: `main.go` — Entrypoint

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/main.go`
**Blocked by:** 1.67 (server.go)
**Complexity:** S

```go
package main

import (
    "log"
    "github.com/mark3labs/mcp-go/server"
)

func main() {
    log.SetFlags(0)
    log.SetPrefix("goast: ")
    s := server.NewMCPServer("goast", "0.1.0", server.WithToolCapabilities(false))
    RegisterTools(s)
    if err := server.ServeStdio(s); err != nil {
        log.Fatalf("server error: %v", err)
    }
}
```

---

## TIER 2 — Full Single-File Capability

### Task 2.1: `ops/rename.go` — ast_rename (single-file)

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/ops/rename.go`
**Blocked by:** All Tier 1 (selector, editor)
**Complexity:** M

```go
type ASTRenameArgs struct {
    File   string          `json:"file"`
    Path   json.RawMessage `json:"path"` // path to declaration site
    To     string          `json:"to"`   // new name
    DryRun bool            `json:"dry_run"`
}
```

Walk AST with `astutil.Apply`, rename all `*ast.Ident` with matching name. AST-only (no type
resolution). Document this limitation clearly in the tool description.

---

### Task 2.2: `ops/lsp.go` — ast_node_at, ast_find_symbols, ast_find

**File:** `/home/matt/source/goast-worktrees/goast-mcp-server/ops/lsp.go`
**Blocked by:** All Tier 1
**Complexity:** M/L

**ast_node_at:** Parse, compute byte offset from line+col, walk AST to find innermost node,
reverse-walk to build path.

**ast_find_symbols:** Walk .go files in dir, parse each, scan top-level decls, match name
against glob pattern.

**ast_find:** Walk AST with `astutil.Apply`, convert each node via `MarshalNode`, pattern-match
using recursive field comparison where absent pattern fields are wildcards.

---

## TIER 3 — Advanced (Defer If Time-Limited)

### Task 3.1: Cross-file LSP — ast_find_refs, ast_find_def, ast_find_impls

**Blocked by:** Tier 2 / **Complexity:** L/XL

Requires `golang.org/x/tools/go/packages.Load` with full type-checking modes. Expect 1-5s cold
latency. Accept it; document it.

---

### Task 3.2: `ops/refactor.go` — ast_extract_func, ast_inline_func

**Blocked by:** Tier 2 / **Complexity:** XL

Scope analysis without `go/types` is unreliable. Acceptable to fail on complex cases with a
descriptive error.

---

## Implementation Steps (In Order)

### Phase 1: Tests First (TDD)

Write `_test.go` files first for each package. Verify they fail to compile before implementation.

Order:
1. `diff/diff_test.go` → implement `diff/diff.go`
2. `testdata/` files (no deps)
3. `selector/selector_test.go` → implement `selector/selector.go`
4. `editor/editor_test.go` → implement `editor/editor.go`
5. `kinds/kinds_test.go` (with all 50 round-trip tests) + `kinds/golden_test.go` → implement kinds
6. `meta/meta_test.go` → implement `meta/meta.go`

### Phase 2: Tier 1 Implementation Order

```
1.  go get github.com/pmezard/go-difflib@v1.0.0
2.  diff/diff.go + diff_test.go         (no deps)
3.  testdata/*.go                        (no deps)
4.  kinds/node.go                        (no deps — defines interface)
5.  kinds/expr_*.go (16 files)           (depends on kinds/node.go)
6.  kinds/type_*.go (6 files)            (depends on kinds/node.go)
7.  kinds/field.go                       (depends on kinds/node.go)
8.  kinds/stmt_*.go (19 files)           (depends on kinds/node.go)
9.  kinds/decl_*.go (5 files)            (depends on kinds/node.go)
10. kinds/spec_*.go (3 files)            (depends on kinds/node.go)
11. kinds/marshal.go                     (depends on all kind structs)
12. kinds/kinds_test.go + golden_test.go (depends on marshal.go)
13. meta/meta.go + meta_test.go          (depends on go/ast only)
14. selector/selector.go + test          (depends on testdata)
15. editor/editor.go + test              (depends on diff/)
16. ops/query.go                         (depends on selector, editor, kinds, meta)
17. ops/insert.go                        (depends on selector, editor, kinds)
18. ops/replace.go                       (depends on selector, editor, kinds)
19. ops/delete.go                        (depends on selector, editor, kinds)
20. ops/imports.go                       (depends on editor)
21. ops/gomod.go                         (depends on editor.WriteAtomic)
22. server.go                            (depends on all ops)
23. main.go                              (depends on server.go)
```

### Phase 3: Verification

```bash
cd /home/matt/source/goast-worktrees/goast-mcp-server
go test ./...
go test -race ./...
go test -cover ./...
go build ./...
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./goast
go vet ./...
gofmt -l .
```

### Phase 4: Tier 2 Implementation

```
T2-1: ops/rename.go + tests
T2-2: ops/lsp.go (ast_node_at + ast_find_symbols + ast_find) + tests
T2-3: Register Tier 2 tools in server.go
```

### Phase 5: Tier 3 (after Tier 2, if time permits)

```
T3-1: ops/lsp.go (ast_find_refs package scope, ast_find_def, ast_find_impls)
T3-2: ops/refactor.go (ast_extract_func, ast_inline_func)
T3-3: Register Tier 3 tools in server.go
```

---

## Edge Cases to Handle

### Edge Case 1: CaseClause list=null (default case)
**Scenario:** JSON `"list": null` must produce `CaseClause.List = nil` (not `[]ast.Expr{}`).
**Implementation:** In `FromAST`, if `caseClause.List == nil`, set JSON field to `null` (not empty array).
**Test:** `TestRoundTrip_CaseClause` includes default case variant.

### Edge Case 2: ImportSpec path quoting
**Scenario:** `ast.ImportSpec.Path.Value` is `"\"fmt\""`. JSON should have `"fmt"`.
**Implementation:** `strconv.Unquote(spec.Path.Value)` in `FromAST`. `strconv.Quote(n.Path)` in `ToAST`.
**Test:** `TestRoundTrip_ImportSpec`.

### Edge Case 3: GenDecl with multiple specs and parentheses
**Scenario:** `import ("fmt"; "os")` has `GenDecl.Lparen != token.NoPos`.
**Implementation:** Set `Lparen = token.Pos(1)` when `len(specs) > 1` in ToAST for ImportDecl/ConstDecl/VarDecl.
**Test:** `TestRoundTrip_ImportDecl` with multi-spec.

### Edge Case 4: Navigation to File root (empty path)
**Scenario:** `ast_query` with `path: []` returns file-level info.
**Implementation:** In `HandleASTQuery`, empty path → return ast_list-equivalent + file meta.
**Test:** `TestQueryEmptyPath`.

### Edge Case 5: Negative index in ast_insert
**Scenario:** `index: -1` means append to end.
**Implementation:** `insertAt` appends when index < 0.
**Test:** `TestInsertAppend`.

### Edge Case 6: FuncDecl with nil body
**Scenario:** Interface method declarations have `Body: nil`.
**Implementation:** `FromAST` omits body field when nil; `ToAST` leaves body nil when absent.
**Test:** `TestRoundTrip_FuncDecl` with interface method variant.

### Edge Case 7: ast_delete on scalar field
**Implementation:** `deleteFromList` returns error "cannot delete scalar field".
**Test:** `TestDeleteScalarError`.

### Edge Case 8: Token.NoPos in go/format
**Validation:** Confirmed by brainstorm prototype. Verified by `TestGoldenPathHelloProgram`.

### Edge Case 9: `true` is an Ident, not a BasicLit
**Scenario:** In Go AST, `true`, `false`, `nil` are `*ast.Ident`, not `*ast.BasicLit`.
**Implementation:** These map to `{"kind":"Ident","name":"true"}` in JSON.
**Test:** `TestGoldenPathHelloProgram` uses `Ident{Name:"true"}`.

### Edge Case 10: Empty file / package-only file
**Implementation:** `ast_list` returns empty array. `ast_query` with any path returns NavigateError.
**Test:** `TestListEmptyFile` and `TestQueryEmptyFile`.

---

## Risks and Mitigations

### Risk 1: Selector completeness (missing step kinds)
**Mitigation:** Write `selector_test.go` covering every step kind before implementing. The test acts as checklist.

### Risk 2: GenDecl split mapping in MarshalNode
**Mitigation:** All 4 GenDecl variants tested in `TestRoundTrip_ImportDecl/ConstDecl/TypeDecl/VarDecl`.

### Risk 3: FieldList construction in ToAST
**Mitigation:** Extract `unmarshalFieldList([]json.RawMessage) (*ast.FieldList, error)` shared helper in `kinds/field.go`.

### Risk 4: replaceInParent coverage gaps
**Mitigation:** Default case returns detailed error. Test all common replace targets.

### Risk 5: go-difflib not in go.mod yet
**Mitigation:** First implementation action: `go get github.com/pmezard/go-difflib@v1.0.0`.

### Risk 6: Cyclomatic complexity of Navigate and MarshalNode
**Mitigation:** Document as mechanical dispatchers. These are tables, not logic.

### Risk 7: mcp.NewTypedToolHandler with json.RawMessage fields
**Mitigation:** Confirmed working in brainstorm. Fallback: manual `json.Unmarshal`.

### Risk 8: Golden test requires go on PATH
**Mitigation:** `go run` is a standard tool; CI environments always have it. If needed, add
`//go:build integration` tag and separate test target.

---

## Dependencies

### New dependency to add
```bash
go get github.com/pmezard/go-difflib@v1.0.0
```

### Existing dependencies (already in go.mod)
- `github.com/mark3labs/mcp-go v0.54.0` — MCP server, tool registration, stdio transport
- `golang.org/x/mod v0.36.0` — modfile.Parse, f.AddRequire, f.Format etc.
- `golang.org/x/tools v0.45.0` — astutil.Apply, astutil.AddNamedImport, astutil.DeleteImport

### Standard library
- `go/ast`, `go/parser`, `go/token`, `go/format` — core AST manipulation
- `encoding/json` — JSON marshaling/unmarshaling
- `os`, `os/exec`, `path/filepath` — file I/O, golden test execution, atomic writes
- `fmt`, `strings`, `strconv` — utilities
- `go/types` (Tier 3 only) — type checking for cross-file LSP ops

---

## Complexity Analysis

### High-complexity functions (by design — dispatch tables)

| Function | Location | Est. Complexity | Notes |
|---|---|---|---|
| `MarshalNode` | kinds/marshal.go | ~55 | 50 type-switch arms; mechanical |
| `applyStep` | selector/selector.go | ~90 | 45+ case dispatch; mechanical |
| `replaceInParent` | ops/replace.go | ~60 | 50 parent types × fields; mechanical |
| `insertIntoList` | ops/insert.go | ~20 | ~10 list-bearing node types |
| `deleteFromList` | ops/delete.go | ~15 | ~10 list-bearing node types |
| `Compute` (meta) | meta/meta.go | ~40 | kind-specific fields dispatch |

All high-complexity functions are mechanical dispatchers. Add a comment on each.

### Normal-complexity functions

| Function | Location | Est. Complexity | Notes |
|---|---|---|---|
| `Navigate` | selector/selector.go | ~5 | Delegates to applyStep |
| `Edit` | editor/editor.go | ~8 | Linear flow |
| `HandleASTInsert` | ops/insert.go | ~10 | Linear with error checks |
| `matchNode` (Tier 2) | ops/lsp.go | ~15 | Recursive field comparison |
| `cyclomaticComplexity` | meta/meta.go | ~8 | ast.Inspect walk |

---

## Test Coverage Goals

- **kinds/ package:** >85% (50 round-trip tests + golden path test + error cases)
- **selector/ package:** >90% (all 45+ step kinds tested, all error paths)
- **editor/ package:** >90% (all branches: edit/dryrun/nochange/error)
- **meta/ package:** >80% (all node kinds with applicable fields)
- **ops/ package:** >70% (each tool handler tested with real .go files)
- **diff/ package:** 100% (trivial, 4 cases)

---

## Success Criteria

### Tier 1 Complete When:
- [ ] `go test ./...` passes with no failures
- [ ] `go build ./...` succeeds
- [ ] All 50 node kinds have working `ToAST` + `FromAST` (verified by 50 round-trip tests)
- [ ] `TestGoldenPathHelloProgram` passes — constructs, formats, runs a real Go program from JSON nodes
- [ ] `selector.Navigate` handles all step kinds from design.md path vocabulary table
- [ ] `ast_list`, `ast_query`, `ast_query_many`, `ast_meta` return correct JSON
- [ ] `ast_insert`, `ast_replace`, `ast_delete` mutate, format, and write correctly
- [ ] `ast_add_import`, `ast_delete_import`, `ast_list_imports` work correctly
- [ ] `gomod_read`, `gomod_require`, `gomod_drop_require`, `gomod_replace`, `gomod_drop_replace` work
- [ ] All write tools return unified diff; dry_run=true skips write
- [ ] MCP server starts, registers all Tier 1 tools, responds to `tools/list`

### Tier 2 Complete When:
- [ ] `ast_rename` renames identifiers within a single file
- [ ] `ast_node_at` returns correct path for a line/col position
- [ ] `ast_find_symbols` returns matching decls across a directory
- [ ] `ast_find` performs structural pattern matching with wildcard fields

### Quality Gates (all tiers):
- [ ] `go vet ./...` passes cleanly
- [ ] `gofmt -l .` shows no files need formatting
- [ ] No unhandled error returns (every error is wrapped and propagated)
- [ ] All logging to stderr (stdout is MCP protocol only)

---

## Notes

1. **Coders should implement kinds files in parallel** — the 50 kind files are independent
   (only depend on `kinds/node.go`). Two coders can split them: one does expressions+types,
   one does statements+decls+specs.

2. **marshal.go is written last** among the kinds files, after all structs exist.

3. **The selector test file** should be written BEFORE implementing selector.go. Use the
   complete path vocabulary table from design.md as the checklist. Every step kind in the
   table needs at least one test.

4. **Token string mappings** (operator → token.Token) should be in a shared helper in
   `kinds/node.go` or `kinds/tokens.go`. They're needed by BinaryExpr, UnaryExpr, AssignStmt,
   IncDecStmt, RangeStmt, BranchStmt, BasicLit. Don't duplicate this logic across files.

5. **The `source` field** in ast_query responses requires threading the original source bytes
   through the edit call. `editor.ParseFile` already returns `[]byte` for this purpose.

6. **All errors from NavigateError** should be returned as structured JSON via `mcp.NewToolResultError`.
   The design specifies: `{"error":"...", "at_step":N, "step":{...}, "available":[...]}`.

7. **Index=-1 is append** in ast_insert. This is specified in design.md and must be consistent
   across all list-type insertions.

8. **Tier 2 and Tier 3 tools** should NOT be registered in server.go until they are implemented
   and tested. Register only what works.

9. **`true`, `false`, `nil` are Ident nodes**, not BasicLit. In Go's AST these are
   predeclared identifiers, not literals. Use `{"kind":"Ident","name":"true"}` etc.
   The golden test verifies this implicitly.

10. **`editor.WriteAtomic` must be exported** (capital W) so `ops/gomod.go` can use it for
    go.mod writes without re-implementing the same pattern.

11. **The golden test `TestGoldenPathHelloProgram`** is a required Tier 1 deliverable.
    It must pass before Tier 1 is considered complete. It is the single most important test
    in the entire test suite because it proves the complete ToAST → format → compile → run
    pipeline works end-to-end.
