// Namespace: goast
// MCP tool registration — Tier 1 tools.
package main

import (
	"github.com/lthiery/goast/ops"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTools registers all Tier 1 MCP tools with the server.
func RegisterTools(s *server.MCPServer) {
	// ast_list — list top-level declarations in a file
	s.AddTool(
		mcp.NewTool("ast_list",
			mcp.WithDescription("List all top-level declarations in a Go source file. Returns an array of {kind, name, recv, line} summaries."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
		),
		mcp.NewTypedToolHandler(ops.HandleASTList),
	)

	// ast_query — return the JSON node tree at a path
	s.AddTool(
		mcp.NewTool("ast_query",
			mcp.WithDescription("Return the JSON node tree at a selector path within a Go source file. Empty path returns file-level metadata."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
			mcp.WithArray("path", mcp.Description("Selector path as array of step objects (e.g. [{kind:FuncDecl,name:main},{kind:Body}])")),
		),
		mcp.NewTypedToolHandler(ops.HandleASTQuery),
	)

	// ast_query_many — run multiple queries in one call
	s.AddTool(
		mcp.NewTool("ast_query_many",
			mcp.WithDescription("Run multiple ast_query calls in one round-trip. Returns an array of node trees."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
			mcp.WithArray("paths", mcp.Required(), mcp.Description("Array of selector paths, each an array of step objects")),
		),
		mcp.NewTypedToolHandler(ops.HandleASTQueryMany),
	)

	// ast_meta — return only metadata for a node
	s.AddTool(
		mcp.NewTool("ast_meta",
			mcp.WithDescription("Return metadata for a node at a selector path. Includes line, column, complexity, etc. Empty path returns file-level metadata."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
			mcp.WithArray("path", mcp.Description("Selector path as array of step objects")),
		),
		mcp.NewTypedToolHandler(ops.HandleASTMeta),
	)

	// ast_insert — insert a node into a list
	s.AddTool(
		mcp.NewTool("ast_insert",
			mcp.WithDescription("Insert a node into a list (block statements, struct fields, function args, etc.) at a given index. index=-1 appends."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
			mcp.WithArray("path", mcp.Required(), mcp.Description("Selector path to the list container")),
			mcp.WithInteger("index", mcp.Description("Position to insert at. -1 = append to end.")),
			mcp.WithObject("node", mcp.Required(), mcp.Description("Node to insert as JSON")),
			mcp.WithBoolean("dry_run", mcp.Description("If true, return diff without writing to disk")),
		),
		mcp.NewTypedToolHandler(ops.HandleASTInsert),
	)

	// ast_replace — replace the node at a path
	s.AddTool(
		mcp.NewTool("ast_replace",
			mcp.WithDescription("Replace the node at a selector path with a new node tree."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
			mcp.WithArray("path", mcp.Required(), mcp.Description("Selector path to the node to replace")),
			mcp.WithObject("node", mcp.Required(), mcp.Description("Replacement node as JSON")),
			mcp.WithBoolean("dry_run", mcp.Description("If true, return diff without writing to disk")),
		),
		mcp.NewTypedToolHandler(ops.HandleASTReplace),
	)

	// ast_delete — remove a node from its parent list
	s.AddTool(
		mcp.NewTool("ast_delete",
			mcp.WithDescription("Remove the node at a selector path from its parent list."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
			mcp.WithArray("path", mcp.Required(), mcp.Description("Selector path to the node to delete")),
			mcp.WithBoolean("dry_run", mcp.Description("If true, return diff without writing to disk")),
		),
		mcp.NewTypedToolHandler(ops.HandleASTDelete),
	)

	// ast_add_import — add an import to a file
	s.AddTool(
		mcp.NewTool("ast_add_import",
			mcp.WithDescription("Add an import to a Go source file. Merges into existing import block; no-ops if already present."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
			mcp.WithString("path", mcp.Required(), mcp.Description("Import path to add, e.g. \"net/http\"")),
			mcp.WithString("alias", mcp.Description("Import alias: empty=none, \".\"=dot, \"_\"=blank, or identifier")),
		),
		mcp.NewTypedToolHandler(ops.HandleAddImport),
	)

	// ast_delete_import — remove an import by path
	s.AddTool(
		mcp.NewTool("ast_delete_import",
			mcp.WithDescription("Remove an import from a Go source file by import path."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
			mcp.WithString("path", mcp.Required(), mcp.Description("Import path to remove, e.g. \"net/http\"")),
		),
		mcp.NewTypedToolHandler(ops.HandleDeleteImport),
	)

	// ast_list_imports — list all imports in a file
	s.AddTool(
		mcp.NewTool("ast_list_imports",
			mcp.WithDescription("Return all imports in a Go source file with their aliases and usage status."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the Go source file")),
		),
		mcp.NewTypedToolHandler(ops.HandleListImports),
	)

	// gomod_read — read go.mod as structured JSON
	s.AddTool(
		mcp.NewTool("gomod_read",
			mcp.WithDescription("Read and parse a go.mod file, returning its contents as structured JSON."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the go.mod file")),
		),
		mcp.NewTypedToolHandler(ops.HandleGoModRead),
	)

	// gomod_require — add or update a require directive
	s.AddTool(
		mcp.NewTool("gomod_require",
			mcp.WithDescription("Add or update a require directive in go.mod."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the go.mod file")),
			mcp.WithString("path", mcp.Required(), mcp.Description("Module path, e.g. \"golang.org/x/tools\"")),
			mcp.WithString("version", mcp.Required(), mcp.Description("Version, e.g. \"v0.21.0\"")),
			mcp.WithBoolean("indirect", mcp.Description("Whether to mark as indirect")),
		),
		mcp.NewTypedToolHandler(ops.HandleGoModRequire),
	)

	// gomod_drop_require — remove a require directive
	s.AddTool(
		mcp.NewTool("gomod_drop_require",
			mcp.WithDescription("Remove a require directive from go.mod."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the go.mod file")),
			mcp.WithString("path", mcp.Required(), mcp.Description("Module path to remove")),
		),
		mcp.NewTypedToolHandler(ops.HandleGoModDropRequire),
	)

	// gomod_replace — add or update a replace directive
	s.AddTool(
		mcp.NewTool("gomod_replace",
			mcp.WithDescription("Add or update a replace directive in go.mod."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the go.mod file")),
			mcp.WithString("old", mcp.Required(), mcp.Description("Module path to replace")),
			mcp.WithString("new", mcp.Required(), mcp.Description("Replacement module path or local path")),
			mcp.WithString("new_version", mcp.Description("Version for the replacement (empty for local paths)")),
		),
		mcp.NewTypedToolHandler(ops.HandleGoModReplace),
	)

	// gomod_drop_replace — remove a replace directive
	s.AddTool(
		mcp.NewTool("gomod_drop_replace",
			mcp.WithDescription("Remove a replace directive from go.mod."),
			mcp.WithString("file", mcp.Required(), mcp.Description("Absolute path to the go.mod file")),
			mcp.WithString("old", mcp.Required(), mcp.Description("Module path whose replace directive to remove")),
		),
		mcp.NewTypedToolHandler(ops.HandleGoModDropReplace),
	)
}
