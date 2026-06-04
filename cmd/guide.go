package cmd

import "fmt"

// PrintGuide prints the grv tree notation reference guide.
// Topics: "fields", "nodes", "meta", "notation", or "" for index.
func PrintGuide(topic string) {
	switch topic {
	case "fields":
		printFieldsGuide()
	case "nodes":
		printNodesGuide()
	case "meta":
		printMetaGuide()
	case "notation":
		printNotationGuide()
	default:
		printGuideIndex()
	}
}

func printGuideIndex() {
	fmt.Print(`grv guide — tree notation reference

Topics:
  grv guide notation   How to read and write tree notation
  grv guide fields     Common field names: x, fun, sel, tok, op, lhs, rhs, body ...
  grv guide nodes      Node kinds by category with Go ↔ tree examples
  grv guide meta       Metadata fields: line, git_churn, cyclomatic_complexity, lth.results ...

Quick reference:
  KindName attr=val     — node header with inline scalar attributes
  fieldname             — single-object child block (no suffix)
  fieldname[]           — object-array child block
  fieldname=[a,b]       — scalar array inline
  fieldname=[]          — empty array

`)
}

func printNotationGuide() {
	fmt.Print(`grv guide: tree notation

Every node is one line: "KindName scalar=val scalar2=val2"
Children are indented 2 spaces below their parent.

READING:
  FuncDecl name=Run line=42 end_line=67   ← kind=FuncDecl, name="Run", lines 42-67
    type FuncType                           ← "type" field is a single FuncType object
      params FieldList                      ← "params" field is a FieldList object
        list[]                              ← "list" is an object array ([] suffix)
          Field names=[ctx] type=Context    ← scalar array names=[ctx], scalar type=Context
    body BlockStmt                          ← "body" field is a BlockStmt object
      list[]                                ← statements array
        ReturnStmt                          ← a statement with no scalars

SUFFIXES:
  fieldname     single object child (body, type, fun, x, key, value, cond, init, post ...)
  fieldname[]   object array (list, args, params, results, specs, elts, names ...)
  fieldname=[]  empty array
  fieldname=[a,b,c]  scalar array (names=[x,y], results=[string,error])

SCALARS:
  name=foo         unquoted string (no spaces, no special chars)
  tok=":="         quoted string (contains special chars)
  line=201         integer
  exported=true    boolean
  init=null        null (absent/nil field)

WRITING — --node, --path, --paths, --ops, --pattern all use tree notation:

  --path 'FuncDecl name=foo / BlockStmt / AssignStmt'   slash-separated path steps
  --node 'Ident name=x'                                  simple node (single line)
  --node 'AssignStmt tok=":="                            multiline node
    lhs[]
      Ident name=x
    rhs[]
      BasicLit tok=INT value=42'

  --paths — multiple paths for ast_query_many, separated by ---:
    'FuncDecl name=foo
    ---
    FuncDecl name=bar / BlockStmt'

  --ops for ast_patch — one op per block:
    'set name "newName"
    delete recv
    append list
      Ident name=x
    insert list 0
      Ident name=y
    prepend list
      Ident name=z
    delete list 2'

    Valid ops: set <field> <value>, delete <field> [index],
               append/prepend <field> (node on next indented line),
               insert <field> <index> (node on next indented line)

  --ops for ast_replace_many / ast_insert_many / ast_delete_many:
    'path FuncDecl name=foo
    node
      FuncDecl name=bar
    ---
    path TypeSpec name=Old
    index -1
    node
      TypeSpec name=New'

    Keys: path <tree-path>, node (tree node on next indented line),
          index <int> (insert_many only)

`)

}

func printFieldsGuide() {
	fmt.Print(`grv guide: common field names

STRUCTURAL FIELDS (appear in many node kinds):

  x          The primary sub-expression. In SelectorExpr (pkg.Name), x is pkg.
             In UnaryExpr (*p, !b, -n), x is the operand.
             In IndexExpr (arr[i]), x is arr.
             In TypeAssertExpr (v.(T)), x is v.
             In StarExpr (*T), x is T.

  fun        The function being called in a CallExpr. Often a SelectorExpr or Ident.
               fmt.Println("hi")  →  fun=SelectorExpr{x=Ident(fmt), sel=Println}

  sel        The selected name in a SelectorExpr (the part after the dot).
               fmt.Println  →  SelectorExpr sel=Println

  op         Operator. In BinaryExpr: +, -, *, /, %, ==, !=, <, >, &&, ||, ...
             In UnaryExpr: !, -, *, &, ^
               a + b  →  BinaryExpr op=+
               !ok    →  UnaryExpr  op=!

  tok        Token kind. In AssignStmt: =, :=, +=, -=, ...
             In BasicLit: INT, STRING, FLOAT, CHAR, IMAG
             In RangeStmt / SendStmt: := or =
               err := foo()  →  AssignStmt tok=":="
               "hello"       →  BasicLit   tok=STRING value="\"hello\""
               42            →  BasicLit   tok=INT    value=42

  body       The BlockStmt body of a FuncDecl, IfStmt, ForStmt, RangeStmt etc.

  lhs / rhs  Left-hand side / right-hand side of an AssignStmt.
               x, err := foo()  →  lhs[]=[Ident(x), Ident(err)]  rhs[]=[CallExpr]

  cond       The condition expression in IfStmt, ForStmt.
  init       Optional init statement in IfStmt, ForStmt, SwitchStmt.
  post       The post statement in ForStmt (the i++ part).

  key / value  In RangeStmt: loop variables (for key, value := range x).
               In KeyValueExpr: the key and value of a composite literal field.

  list       Statement list inside a BlockStmt; case list in CaseClause.
  args       Arguments to a CallExpr.
  elts       Elements of a CompositeLit (struct/slice/map literal).
  names      Identifier names in a Field or ValueSpec (e.g. names=[x,y] for "x, y int").
  specs      Specs inside a GenDecl (import/const/type/var declarations).
  results    Return values list in FuncType, or results of a ReturnStmt.
  params     Parameter list in FuncType.

DECLARATION FIELDS:
  name       Declared name: FuncDecl.name, TypeSpec.name, ImportSpec.path etc.
  recv       Method receiver FieldList on a FuncDecl.
  type       Type expression: Field.type, ValueSpec.type, TypeSpec.type.
  tag        Struct field tag (BasicLit string).
  decl       The inner GenDecl inside a DeclStmt.

LITERAL / SCALAR FIELDS:
  value      The literal value string: BasicLit.value, e.g. value=42 or value="\"hello\""
  ellipsis   true if a CallExpr uses ... (variadic spread)

`)
}

func printNodesGuide() {
	fmt.Print(`grv guide: node kinds by category

Each example shows Go source code and its tree notation equivalent.

━━━ DECLARATIONS ━━━

FuncDecl — function or method declaration
  Go:   func (r *Runner) Run(ctx context.Context) error { ... }
  Tree: FuncDecl name=Run
          recv FieldList
            list[]
              Field names=[r] type=*Runner
          type FuncType
            params FieldList
              list[]
                Field names=[ctx] type=context.Context
            results FieldList
              list[]
                Field type=error
          body BlockStmt
            list[]

TypeSpec — type declaration (inside a GenDecl)
  Go:   type Violation struct { File string; Line int }
  Tree: TypeSpec name=Violation
          type StructType
            fields FieldList
              list[]
                Field names=[File] type=string
                Field names=[Line] type=int

ValueSpec — var or const declaration (inside a GenDecl)
  Go:   var x int = 42
  Tree: ValueSpec names=[x]
          type Ident name=int
          values[]
            BasicLit tok=INT value=42

GenDecl / VarDecl / ConstDecl / ImportDecl — declaration groups
  Go:   import "fmt"
  Tree: ImportDecl
          specs[]
            ImportSpec path="fmt"

━━━ STATEMENTS ━━━

AssignStmt — assignment or short variable declaration
  Go:   x, err := foo()
  Tree: AssignStmt tok=":="
          lhs[]
            Ident name=x
            Ident name=err
          rhs[]
            CallExpr fun=Ident(foo) args=[]

ExprStmt — expression used as a statement
  Go:   fmt.Println("hello")
  Tree: ExprStmt
          x CallExpr ellipsis=false
            fun SelectorExpr sel=Println
              x Ident name=fmt
            args[]
              BasicLit tok=STRING value="\"hello\""

ReturnStmt — return statement
  Go:   return violations
  Tree: ReturnStmt
          results[]
            Ident name=violations

IfStmt — if/else statement
  Go:   if p == nil { return "" }
  Tree: IfStmt
          cond BinaryExpr op="=="
            x Ident name=p
            y Ident name=nil
          body BlockStmt
            list[]
              ReturnStmt
                results[]
                  BasicLit tok=STRING value="\"\""

RangeStmt — for-range loop
  Go:   for _, cg := range f.Comments { ... }
  Tree: RangeStmt tok=":="
          key Ident name=_
          value Ident name=cg
          x SelectorExpr sel=Comments
            x Ident name=f
          body BlockStmt
            list[]

ForStmt — traditional for loop
  Go:   for i := 0; i < n; i++ { ... }
  Tree: ForStmt
          init AssignStmt tok=":="
            lhs[] [Ident name=i]
            rhs[] [BasicLit tok=INT value=0]
          cond BinaryExpr op=<
            x Ident name=i
            y Ident name=n
          post IncDecStmt tok=++
            x Ident name=i
          body BlockStmt

DeclStmt — var/const declaration inside a function body
  Go:   var violations []Violation
  Tree: DeclStmt
          decl VarDecl
            specs[]
              ValueSpec names=[violations]
                type ArrayType
                  elt Ident name=Violation

━━━ EXPRESSIONS ━━━

CallExpr — function call
  Go:   json.Marshal(v)
  Tree: CallExpr ellipsis=false
          fun SelectorExpr sel=Marshal
            x Ident name=json
          args[]
            Ident name=v

SelectorExpr — field or method access (pkg.Name or x.Field)
  Go:   fset.Position
  Tree: SelectorExpr sel=Position
          x Ident name=fset

BinaryExpr — binary operation
  Go:   len(s) > 0
  Tree: BinaryExpr op=>
          x CallExpr fun=Ident(len) args=[Ident(s)]
          y BasicLit tok=INT value=0

UnaryExpr — unary operation
  Go:   !ok
  Tree: UnaryExpr op=!
          x Ident name=ok

TypeAssertExpr — type assertion
  Go:   v.(string)   or   v, ok := v.(string)
  Tree: TypeAssertExpr
          x Ident name=v
          type Ident name=string

IndexExpr — index expression
  Go:   arr[i]   or   m[key]
  Tree: IndexExpr
          x Ident name=arr
          index Ident name=i

CompositeLit — composite literal (struct, slice, map)
  Go:   Violation{File: f, Line: 42}
  Tree: CompositeLit
          type Ident name=Violation
          elts[]
            KeyValueExpr
              key Ident name=File
              value Ident name=f
            KeyValueExpr
              key Ident name=Line
              value BasicLit tok=INT value=42

FuncLit — anonymous function
  Go:   func(n ast.Node) bool { return true }
  Tree: FuncLit
          type FuncType
            params FieldList
              list[]
                Field names=[n] type=ast.Node
            results FieldList
              list[]
                Field type=bool
          body BlockStmt
            list[]
              ReturnStmt
                results[] [Ident name=true]

━━━ TYPES ━━━

StarExpr — pointer type or pointer dereference
  Go:   *ast.File (type)   or   *p (expr)
  Tree: StarExpr
          x SelectorExpr sel=File
            x Ident name=ast

ArrayType — slice or array type
  Go:   []Violation
  Tree: ArrayType
          elt Ident name=Violation

MapType — map type
  Go:   map[int]bool
  Tree: MapType
          key Ident name=int
          value Ident name=bool

ChanType — channel type
  Go:   chan int
  Tree: ChanType
          value Ident name=int

━━━ LITERALS ━━━

BasicLit — literal value (int, string, float, char)
  Go:    42        →  BasicLit tok=INT    value=42
         "hello"   →  BasicLit tok=STRING value="\"hello\""
         3.14      →  BasicLit tok=FLOAT  value=3.14
         'x'       →  BasicLit tok=CHAR   value="'x'"

Ident — identifier
  Go:   violations
  Tree: Ident name=violations

`)
}

func printMetaGuide() {
	fmt.Print(`grv guide: metadata fields

Metadata appears above the node in ast_query output and is returned by ast_meta.
It comes from three sources: AST position info, git history, and lth memory hooks.

━━━ POSITION (from go/token FileSet) ━━━

  line          First line of the node in the source file (1-based)
  end_line      Last line of the node
  col           Column of the node's start position (1-based)
  byte_offset   Byte offset from start of file to node start
  byte_end      Byte offset of node end
  depth         How many AST levels deep this node is (1 = top-level declaration)
  parent_kind   Kind of the immediate parent node

━━━ FUNCTION METRICS (FuncDecl only) ━━━

  param_count           Number of parameters
  result_count          Number of return values
  has_error_return      true if last return type is error
  stmt_count            Number of top-level statements in the body
  cyclomatic_complexity Cyclomatic complexity (branches + 1; higher = harder to test)
  is_method             true if the function has a receiver (method vs free function)
  is_variadic           true if the last parameter is variadic (...)
  recv_type             Receiver type name (e.g. "*Runner")
  exported              true if the name starts with an uppercase letter

━━━ TYPE METRICS (TypeSpec only) ━━━

  exported          true if exported
  is_alias          true if this is a type alias (type X = Y)
  has_type_params   true if generic (type Foo[T any])
  underlying_kind   "struct", "interface", "slice", "map", "chan", "func", "pointer", etc.

━━━ GIT CHURN ━━━

  git_churn   Number of commits that touched this node's exact line range.
              Computed via: git log --oneline --no-patch -L start,end:file
              Cached by (file, git HEAD hash) — free on repeated calls.
              High churn = frequently modified = higher maintenance surface.

━━━ LTH MEMORY (from grv.yaml hook) ━━━

  lth.results[]   Memories from lth that match this file's namespace.
                  Each result has:
                    score    Relevance score (0.0–1.0, higher = more relevant)
                    layer    Memory layer (3 = techniques, 4 = project context)
                    summary  3-sentence Haiku-generated summary of the memory
                    fetch    CLI command to retrieve the full memory:
                             lth --json get <id>

  Run "lth --json get <id>" to expand any summary into the full stored memory.
  Add a same-line comment to your grv.yaml hook to customize which memories appear.

━━━ EXAMPLE OUTPUT ━━━

  grv ast_meta --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled'

  line=201
  end_line=246
  col=1
  cyclomatic_complexity=10
  git_churn=1
  exported=false
  is_method=false
  param_count=4
  result_count=1
  stmt_count=5
  lth.results[]
    score=0.63 layer=3
    summary="..."
    fetch="lth --json get abc123"

`)
}
