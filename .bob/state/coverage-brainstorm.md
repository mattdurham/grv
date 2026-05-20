# Coverage Push: goast — Target 80%+

## Current State (from go test -coverprofile)

| Package | Current | Gap | Primary uncovered areas |
|---|---|---|---|
| diff | 85.7% | ✓ near target | Error path in Files() |
| editor | 73.7% | ~7pp needed | WriteAtomic failure, format error |
| kinds | 72.4% | ~8pp needed | init() funcs (0%), error branches in ToAST, tokenFromString dead arms |
| meta | 48.0% | ~32pp needed | ForStmt, RangeStmt, SwitchStmt, TypeSwitchStmt, SelectStmt, InterfaceType, CommClause branches |
| ops | 25.0% | ~55pp needed | ALL gomod handlers (0%), replaceInParent (3.4%), insertIntoList (0%), ast_query_many, ast_meta |
| selector | 27.5% | ~52pp needed | All field accessors: Cond, Init, Post, Else, Tag, X, Y, Fun, Args, Lhs, Rhs, Key, Value, Sel, Params, Results, Elts, Indices, CaseClause, CommClause, StructType/InterfaceType, VarDecl/ConstDecl/ImportDecl navigation |

## What needs tests

### selector (worst gap — 27.5%)
The applyStep dispatcher has ~45 cases. Only ~8 are exercised.
Missing: ALL scalar field accessors. Need tests for:
- Cond (IfStmt.Cond, ForStmt.Cond, SwitchStmt.Tag)  
- Init (IfStmt.Init, ForStmt.Init, SwitchStmt.Init)
- Post (ForStmt.Post)
- Else (IfStmt.Else)
- X, Y (BinaryExpr operands)
- Fun (CallExpr.Fun)
- Args[index] (CallExpr arguments)
- Lhs[index], Rhs[index] (AssignStmt)
- Key, Value (RangeStmt)
- Sel (SelectorExpr.Sel)
- Params, Results (FuncType/FuncDecl)
- Elts[index] (CompositeLit)
- StructType, InterfaceType (from TypeSpec)
- VarDecl, ConstDecl, ImportDecl (from File)
- CaseClause[index], CommClause[index]
- ForStmt[index], RangeStmt[index], SwitchStmt[index], SelectStmt[index], TypeSwitchStmt[index]
- AssignStmt[index], ExprStmt[index], GoStmt[index], DeferStmt[index]

Strategy: Expand selector_test.go with inline Go source strings parsed per test.

### ops (25% — needs 55pp)
- gomod.go: ALL handlers at 0%. Need test go.mod file + tests for read, require, drop_require, replace, drop_replace
- replaceInParent: only 3.4%. Need tests replacing struct fields, interface methods, if conditions, return values, assign lhs/rhs, etc.
- insertIntoList fallback: 0%. Need test inserting into FieldList, File.Decls, CallExpr.Args
- ast_query_many: 0%. Add test.
- ast_meta: 0%. Add test.
- HandleASTList type coverage: needs ImportDecl, VarDecl, ConstDecl in test file

### meta (48% — needs 32pp)
Compute() has branches for 15+ node kinds, only 4 exercised.
Need tests for:
- ForStmt (has_init, body_stmt_count)
- RangeStmt (body_stmt_count)
- SwitchStmt (case_count, has_default)
- TypeSwitchStmt (case_count, has_default)
- SelectStmt (case_count, has_default)
- InterfaceType (method_count, embed_count, is_empty)
- Field (exported, is_embedded, has_tag, name_count)
- BranchStmt, ReturnStmt, ExprStmt (universal fields only — just need to hit them)

### kinds (72.4% — needs 8pp)
- init() funcs: 0% (auto-called, not directly testable — these are already covered by UnmarshalNode tests)
- tokenFromString: only 28.6%. Many arms untested. Add test covering all operators.
- Error branches: ToAST when child unmarshal fails
- FromAST error branches (wrong type assertions)

### editor (73.7% — needs 7pp)
- WriteAtomic when write fails (readonly dir)
- Format failure (malformed AST)
- ParseFile on nonexistent file

## Approach

Split work by package across two coders in parallel:
- Coder-1: selector + ops (the two biggest gaps)
- Coder-2: meta + kinds + editor (three medium gaps)

Each coder writes tests only — no implementation changes.
Target: reach 80%+ per package with `go test -coverprofile`.
