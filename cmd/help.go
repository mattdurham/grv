package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// ArgInfo describes one argument of a tool.
type ArgInfo struct {
	Name     string
	Type     string
	Required bool
	Desc     string
}

// ExampleInfo is a single usage example for a tool.
type ExampleInfo struct {
	Desc    string // one-line description of what the example does
	Command string // the grv command to run
}

// ToolInfo describes one grv tool.
type ToolInfo struct {
	Name     string
	Desc     string
	Args     []ArgInfo
	Notes    string
	Examples []ExampleInfo
}

// ToolRegistry lists all grv tools.
var ToolRegistry = []ToolInfo{
	{
		Name: "ast_list",
		Desc: "List all top-level declarations in a package or file",
		Args: []ArgInfo{
			{Name: "namespace", Type: "string", Required: false, Desc: "Package name (e.g. 'hooks') or 'pkg#Decl' to list one file; use single quotes to protect #"},
			{Name: "file", Type: "string", Required: false, Desc: "Path to Go source file (alternative to namespace)"},
			{Name: "dir", Type: "string", Required: false, Desc: "Directory path to list all declarations (alternative to namespace)"},
		},
		Notes: "Provide namespace, file, or dir. namespace routes to dir or file automatically. Each FuncDecl and TypeDecl includes git_churn in its Meta when running via daemon.",
	},
	{
		Name: "ast_query",
		Desc: "Return the JSON node tree at a selector path",
		Args: []ArgInfo{
			{Name: "namespace", Type: "string", Required: false, Desc: "Package and declaration (e.g. 'hooks#RunFile'); use single quotes to protect #"},
			{Name: "file", Type: "string", Required: false, Desc: "Path to Go source file (alternative to namespace)"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the node (empty for file-level info)"},
		},
	},
	{
		Name: "ast_query_many",
		Desc: "Return multiple node trees in one call",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "paths", Type: "[][]step", Required: true, Desc: "List of selector paths"},
		},
	},
	{
		Name: "ast_check",
		Desc: "Run configured rule checks against a file or directory",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: false, Desc: "Path to Go source file"},
			{Name: "dir", Type: "string", Required: false, Desc: "Directory to check all .go files"},
		},
		Notes: "Returns JSON array of {file,line,rule,message} violations. Rules are configured in grv.toml under [checks] enforce=[\"all\"] or a subset. Available rules: error_handled, type_assertion_not_checked, mutex_not_embedded, channel_size_not_one_or_zero. Default is no rules enforced. Enabled rules also run automatically on ast_insert and ast_replace, rejecting the write on any violation.",
	},
	{
		Name: "ast_meta",
		Desc: "Return metadata (line, col, complexity, git_churn) for a node",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the node"},
			{Name: "hooks", Type: "[]string", Required: false, Desc: "Hook name allowlist (e.g. [\"lth\"]); omit for all hooks"},
		},
		Notes: "Includes git_churn: number of commits that touched the node's line range. Hook results (e.g. lth.results) are merged when the daemon is running.",
	},
	{
		Name: "ast_insert",
		Desc: "Insert a new AST node into a list container",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the target list"},
			{Name: "index", Type: "int", Required: true, Desc: "Insert position (-1 to append)"},
			{Name: "node", Type: "object", Required: true, Desc: "JSON node to insert"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
	},
	{
		Name: "ast_insert_many",
		Desc: "Insert multiple AST nodes in a single atomic write",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "ops", Type: "[]op", Required: true, Desc: "List of {path, index, node} insert operations applied in order"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
		Notes: "All operations are applied to the same in-memory AST and written atomically. Later ops see the AST state produced by earlier ops.",
	},
	{
		Name: "ast_replace",
		Desc: "Replace an AST node at a selector path",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the node to replace"},
			{Name: "node", Type: "object", Required: true, Desc: "Replacement JSON node"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
	},
	{
		Name: "ast_replace_many",
		Desc: "Replace multiple AST nodes in a single atomic write",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "ops", Type: "[]op", Required: true, Desc: "List of {path, node} replace operations applied in order"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
		Notes: "All operations are applied to the same in-memory AST and written atomically. Later ops see the AST state produced by earlier ops.",
	},
	{
		Name: "ast_patch",
		Desc: "Mutate named fields on an AST node without replacing the whole node",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the node to patch"},
			{Name: "ops", Type: "[]op", Required: true, Desc: "List of {op, field, value?, index?} patch operations"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
		Notes: "op values: set (replace field), append/prepend (add to array field), insert (add at index), delete (remove field or array element). All ops are applied to the same in-memory node map before writing.",
	},
	{
		Name: "ast_delete",
		Desc: "Delete an AST node from a list container",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the node to delete"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
	},
	{
		Name: "ast_delete_many",
		Desc: "Delete multiple AST nodes in a single atomic write",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "ops", Type: "[]op", Required: true, Desc: "List of {path} delete operations; ops on the same parent list must be ordered descending by index"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
		Notes: "Ops targeting the same parent list MUST be ordered by descending index (highest first). Each deletion shifts subsequent indices down by 1, so deleting a higher index first keeps lower indices valid. Ops on different parent lists may be in any order.",
	},
	{
		Name: "ast_rename",
		Desc: "Rename an identifier at its declaration site (single-file AST approximation)",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the declaration"},
			{Name: "to", Type: "string", Required: true, Desc: "New identifier name"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
		Notes: "AST-only approximation. Renames all matching identifiers regardless of scope. Accurate for top-level declarations.",
	},
	{
		Name: "ast_node_at",
		Desc: "Find the innermost AST node at a file position (line/col)",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "line", Type: "int", Required: true, Desc: "1-based line number"},
			{Name: "col", Type: "int", Required: true, Desc: "1-based column number"},
		},
	},
	{
		Name: "ast_find_symbols",
		Desc: "Search for symbols (functions, types, vars) across a directory",
		Args: []ArgInfo{
			{Name: "dir", Type: "string", Required: true, Desc: "Directory to search"},
			{Name: "query", Type: "string", Required: true, Desc: "Glob pattern (e.g. 'Add', 'Handle*', '*')"},
			{Name: "kinds", Type: "[]string", Required: false, Desc: "Filter by kind: FuncDecl, TypeSpec, VarSpec, ConstSpec"},
		},
	},
	{
		Name: "ast_find",
		Desc: "Find all AST nodes matching a JSON pattern",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: false, Desc: "Search single file"},
			{Name: "dir", Type: "string", Required: false, Desc: "Search all .go files in directory"},
			{Name: "pattern", Type: "object", Required: true, Desc: "JSON pattern (null fields are wildcards)"},
		},
		Notes: "Provide either file or dir (not both).",
	},
	{
		Name: "ast_find_refs",
		Desc: "Find all references to the identifier at a selector path (type-aware)",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the identifier"},
			{Name: "scope", Type: "string", Required: false, Desc: "Search scope: 'file' (default) or 'package'"},
		},
	},
	{
		Name: "ast_find_def",
		Desc: "Find the definition of the identifier at a selector path (type-aware)",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the identifier"},
		},
	},
	{
		Name: "ast_find_impls",
		Desc: "Find all types implementing an interface (type-aware)",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "[]step", Required: true, Desc: "Selector path to the interface TypeSpec"},
			{Name: "scope", Type: "string", Required: false, Desc: "Search scope: 'package' (default) or 'file'"},
		},
	},
	{
		Name: "ast_add_import",
		Desc: "Add an import to a Go file",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "string", Required: true, Desc: "Import path (e.g. 'encoding/json')"},
			{Name: "alias", Type: "string", Required: false, Desc: "Import alias (e.g. 'j')"},
		},
	},
	{
		Name: "ast_delete_import",
		Desc: "Delete an import from a Go file",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
			{Name: "path", Type: "string", Required: true, Desc: "Import path to remove"},
		},
	},
	{
		Name: "ast_list_imports",
		Desc: "List all imports in a Go file with usage information",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to Go source file"},
		},
	},
	{
		Name: "ast_directory",
		Desc: "Inventory all Go and non-Go files in a directory",
		Args: []ArgInfo{
			{Name: "namespace", Type: "string", Required: false, Desc: "Package name (e.g. 'hooks' or '.'); routed to dir automatically"},
			{Name: "dir", Type: "string", Required: false, Desc: "Directory path (alternative to namespace)"},
		},
	},
	{
		Name: "gomod_read",
		Desc: "Read and parse a go.mod file",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to go.mod file"},
		},
	},
	{
		Name: "gomod_require",
		Desc: "Add or update a require directive in go.mod",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to go.mod file"},
			{Name: "path", Type: "string", Required: true, Desc: "Module path (e.g. 'golang.org/x/tools')"},
			{Name: "version", Type: "string", Required: true, Desc: "Module version (e.g. 'v0.1.0')"},
			{Name: "indirect", Type: "bool", Required: false, Desc: "Mark as indirect dependency"},
		},
	},
	{
		Name: "gomod_drop_require",
		Desc: "Remove a require directive from go.mod",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to go.mod file"},
			{Name: "path", Type: "string", Required: true, Desc: "Module path to remove"},
		},
	},
	{
		Name: "gomod_replace",
		Desc: "Add or update a replace directive in go.mod",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to go.mod file"},
			{Name: "old", Type: "string", Required: true, Desc: "Module path to replace"},
			{Name: "new", Type: "string", Required: true, Desc: "Replacement module path"},
			{Name: "new_version", Type: "string", Required: false, Desc: "Replacement version"},
		},
	},
	{
		Name: "gomod_drop_replace",
		Desc: "Remove a replace directive from go.mod",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to go.mod file"},
			{Name: "old", Type: "string", Required: true, Desc: "Module path whose replace to remove"},
		},
	},
	{
		Name: "file_read",
		Desc: "Read any non-Go file",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to file"},
		},
	},
	{
		Name: "file_write",
		Desc: "Write content to any non-Go file",
		Args: []ArgInfo{
			{Name: "file", Type: "string", Required: true, Desc: "Path to file"},
			{Name: "content", Type: "string", Required: true, Desc: "New file content"},
			{Name: "dry_run", Type: "bool", Required: false, Desc: "Return diff without writing"},
		},
	},
}

// PrintHelp prints tool help to stdout.
func PrintHelp(toolFilter string) {
	if toolFilter != "" {
		for _, t := range ToolRegistry {
			if t.Name == toolFilter {
				printToolDetail(t)
				return
			}
		}
		fmt.Fprintf(os.Stderr, "unknown tool: %q\n", toolFilter)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, t := range ToolRegistry {
		fmt.Fprintf(w, "%s\t%s\n", t.Name, t.Desc)
		argSummary := ""
		for i, a := range t.Args {
			req := ""
			if a.Required {
				req = "*"
			}
			if i > 0 {
				argSummary += ", "
			}
			argSummary += a.Name + req
		}
		fmt.Fprintf(w, "\t  Args: %s\n", argSummary)
	}
	w.Flush()
}

func printToolDetail(t ToolInfo) {
	fmt.Printf("%s — %s\n\n", t.Name, t.Desc)
	fmt.Printf("  Args:\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, a := range t.Args {
		req := "optional"
		if a.Required {
			req = "required"
		}
		fmt.Fprintf(w, "    %s\t%s\t%s\t%s\n", a.Name, a.Type, req, a.Desc)
	}
	w.Flush()
	if t.Notes != "" {
		fmt.Printf("\n  Notes: %s\n", t.Notes)
	}
}
