# Tier 2: goast LSP-style Operations

## Task
Add 4 Tier 2 tools to the goast MCP server:
1. ast_rename — rename identifier at declaration site + all references in same file
2. ast_node_at — given file + line + col, return innermost node + its structural path
3. ast_find_symbols — find declarations matching a name glob across a directory
4. ast_find — structural search: find all nodes matching a partial node tree (absent fields = wildcards)

## Existing codebase
- kinds/ — 50 bidirectional JSON ↔ go/ast node types
- selector/ — Navigate(file, path) → (ast.Node, ParentContext, error)
- editor/ — parse/edit/format/write cycle
- meta/ — derived node metadata
- ops/ — 15 Tier 1 tools (query, insert, replace, delete, imports, gomod)
- server.go — RegisterTools()

## Design spec
See design.md "LSP-style Operations" section for full specs.

## Key constraints
- ast_rename: AST-only (no go/types). Walk file with astutil.Apply, rename all *ast.Ident with matching name. Document approximation (may rename unrelated idents of same name in different scopes).
- ast_node_at: Parse file, compute byte offset from line+col (using token.FileSet), walk AST to find innermost node at that position, reverse-build path from file root to that node.
- ast_find_symbols: Walk .go files in a dir, parse each, scan top-level decls, match name against glob pattern. Return {file, path, kind, name, recv, meta} per match.
- ast_find: Structural search — walk AST with astutil.Apply, MarshalNode each node, compare against pattern using recursive field matching where absent pattern fields are wildcards. Return array of {path, node, meta}.

## All tools: scope parameter
"file" (default), "dir" (all .go files in dir, non-recursive), "package" (via go list — Tier 3, skip for now)

## Register in server.go
4 new tools: ast_rename, ast_node_at, ast_find_symbols, ast_find
