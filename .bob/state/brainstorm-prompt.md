# Tier 3: goast — Type-Aware LSP Operations

## Task
Implement 3 tools using go/types for type-system resolution:
1. ast_find_refs — find all references to an identifier (scope: file or package)
2. ast_find_def — follow an identifier to its declaration
3. ast_find_impls — find all types implementing an interface

## Existing codebase
All Tier 1+2 tools implemented. Relevant:
- ops/lsp.go — ast_node_at, ast_find_symbols, ast_find (use for pattern reference)
- ops/rename.go — ast_rename (file-level, no types)
- ops/query.go — toolError, navError, recvTypeString helpers
- selector/ — Navigate() for path building
- kinds/marshal.go — MarshalNode for node → JSON

## Key design constraints (from design.md)
- ast_find_refs: file scope = AST-only walk (fast); package scope = go/packages (slow, accept it)
- ast_find_def: for identifiers in same file, can use AST scope analysis; for cross-file, go/packages
- ast_find_impls: always needs go/types (interface satisfaction check)
- All 3 accept "scope": "file" | "package"
- "package" scope = use golang.org/x/tools/go/packages.Load with NeedTypesInfo

## go/packages API constraints
- Requires the file to be inside a Go module (go.mod must exist in parent)
- LoadMode: packages.NeedTypesInfo | packages.NeedTypes | packages.NeedSyntax | packages.NeedFiles | packages.NeedImports
- packages.Load returns []*packages.Package; walk Syntax[i] for AST, TypesInfo for type facts
- TypesInfo.Uses: map[*ast.Ident]*types.Object — all identifier uses with their object
- TypesInfo.Defs: map[*ast.Ident]*types.Object — all identifier declarations
- types.Implements(V, T) — checks if type V implements interface T
