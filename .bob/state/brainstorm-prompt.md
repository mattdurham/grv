# Brainstorm: goast MCP Server

## Task
Build the goast MCP server as designed in design.md — a Go MCP server that exposes Go AST read/write operations to Claude via structured JSON node trees. No raw text editing, no line numbers, no snippets. Fully bidirectional JSON ↔ go/ast.

## Requirements
- MCP server in Go, stdio transport
- 50 node kinds (expressions, statements, declarations, specs, types) each with ToAST/FromAST
- JSON path selector system for navigating to nodes within files
- Read tools: ast_list, ast_query, ast_query_many, ast_meta
- Write tools: ast_insert, ast_replace, ast_delete, ast_rename
- Import tools: ast_add_import, ast_delete_import, ast_list_imports
- go.mod tools: gomod_read, gomod_require, gomod_drop_require, gomod_replace, gomod_drop_replace
- LSP-style tools: ast_node_at, ast_find_refs, ast_find_def, ast_find_impls, ast_find_symbols, ast_find
- Refactoring tools: ast_extract_func, ast_inline_func
- Metadata system: derived meta per node kind (position, complexity, exported, etc)
- Metadata hooks: subprocess hooks that contribute key-value pairs to meta (git, blame, bugginess)
- One struct per file under kinds/ with namespace comment headers
- Atomic writes (temp + rename), parse-on-every-call, dry_run support

## Key Design File
/home/matt/source/goast-worktrees/goast-mcp-server/design.md

## Constraints
- Pure Go, no CGO
- stdlib go/ast + go/parser + go/format + go/types
- golang.org/x/mod/modfile for go.mod
- golang.org/x/tools/go/ast/astutil for node replacement
- MCP via stdio (mark3labs/mcp-go or similar)
- No in-memory state between calls
- Atomic file writes

## Spec-driven modules
None yet — this is a greenfield project.
