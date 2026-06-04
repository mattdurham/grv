package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// exampleRegistry maps tool names to their examples.
// Kept separate from ToolRegistry to avoid bloating help.go.
var exampleRegistry = map[string][]ExampleInfo{
	"ast_list": {
		{
			Desc:    "List all declarations in the hooks package",
			Command: `grv ast_list --namespace hooks`,
		},
		{
			Desc:    "List all declarations in the current package",
			Command: `grv ast_list --namespace .`,
		},
		{
			Desc:    "List declarations in a specific file",
			Command: `grv ast_list --file ops/checks.go`,
		},
		{
			Desc:    "List all declarations across a directory",
			Command: `grv ast_list --dir ops/`,
		},
	},
	"ast_query": {
		{
			Desc:    "Read a function — path in tree notation",
			Command: `grv ast_query --namespace 'hooks#RunFile' --path 'FuncDecl name=RunFile'`,
		},
		{
			Desc:    "Read a struct type",
			Command: `grv ast_query --namespace 'hooks#HookConfig' --path 'TypeSpec name=HookConfig'`,
		},
		{
			Desc:    "Read a nested field (slash-separated path)",
			Command: `grv ast_query --file hooks/config.go --path 'TypeSpec name=HookConfig / StructType / FieldList / Field name=Name'`,
		},
		{
			Desc:    "Get file-level metadata (empty path)",
			Command: `grv ast_query --file ops/checks.go --path '[]'`,
		},
	},
	"ast_query_many": {
		{
			Desc: "Read two functions in one call",
			Command: `grv ast_query_many --file ops/checks.go --paths 'FuncDecl name=ruleErrorHandled
---
FuncDecl name=runChecks'`,
		},
	},
	"ast_meta": {
		{
			Desc:    "Get line range, complexity, git_churn and lth context for a function",
			Command: `grv ast_meta --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled'`,
		},
		{
			Desc:    "Get file-level stats (func count, import count, line count)",
			Command: `grv ast_meta --file ops/checks.go --path '[]'`,
		},
		{
			Desc:    "Pull only lth memory for a node",
			Command: `grv ast_meta --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled' --hooks '["lth"]'`,
		},
	},
	"ast_insert": {
		{
			Desc:    "Append a new function to a file using tree notation (dry run first)",
			Command: `grv ast_insert --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled' --index -1 --node 'FuncDecl name=myRule
  type FuncType
    params FieldList
  body BlockStmt' --dry_run true`,
		},
		{
			Desc:    "Insert a statement at the top of a function body",
			Command: `grv ast_insert --file ops/checks.go --path 'FuncDecl name=runChecks / BlockStmt' --index 0 --node 'ExprStmt
  x CallExpr ellipsis=false
    fun SelectorExpr sel=Println
      x Ident name=fmt
    args[]
      BasicLit tok=STRING value="\"debug\""' --dry_run true`,
		},
	},
	"ast_insert_many": {
		{
			Desc: "Add two struct fields in one atomic write",
			Command: `grv ast_insert_many --file ops/checks.go --ops 'path TypeSpec name=Violation / StructType / FieldList
index -1
node
  Field names=[Source]
    type Ident name=string
---
path TypeSpec name=Violation / StructType / FieldList
index -1
node
  Field names=[Severity]
    type Ident name=int' --dry_run true`,
		},
	},
	"ast_replace": {
		{
			Desc:    "Replace a function body using tree notation (dry run first)",
			Command: `grv ast_replace --file ops/checks.go --path 'FuncDecl name=knownRuleNames' --node 'FuncDecl name=knownRuleNames
  body BlockStmt
    list[]
      ReturnStmt
        results[]
          BasicLit tok=STRING value="\"error_handled\""' --dry_run true`,
		},
	},
	"ast_replace_many": {
		{
			Desc: "Replace two nodes in one atomic write",
			Command: `grv ast_replace_many --file ops/checks.go --ops 'path FuncDecl name=ruleErrorHandled / Ident name=ruleErrorHandled
node
  Ident name=ruleErrorDiscarded
---
path FuncDecl name=ruleErrorHandled
node
  FuncDecl name=ruleErrorDiscarded' --dry_run true`,
		},
	},
	"ast_patch": {
		{
			Desc: "Rename a function (patch only the name field)",
			Command: `grv ast_patch --file ops/checks.go --path 'FuncDecl name=ruleErrorHandled' --ops 'set name "ruleErrorDiscarded"' --dry_run true`,
		},
		{
			Desc: "Append a statement to a function body",
			Command: `grv ast_patch --file ops/checks.go --path 'FuncDecl name=runChecks' --ops 'append list
  ReturnStmt
    results[]
      Ident name=out' --dry_run true`,
		},
		{
			Desc: "Remove a field and insert a statement at position 0",
			Command: `grv ast_patch --file ops/checks.go --path 'FuncDecl name=runChecks' --ops 'delete recv
insert list 0
  ExprStmt
    x CallExpr ellipsis=false
      fun Ident name=validate
      args=[]' --dry_run true`,
		},
	},
	"ast_delete": {
		{
			Desc:    "Delete a function from a file",
			Command: `grv ast_delete --file ops/checks.go --path 'FuncDecl name=knownRuleNames' --dry_run true`,
		},
	},
	"ast_delete_many": {
		{
			Desc: "Delete two functions in one atomic write (highest index first)",
			Command: `grv ast_delete_many --file ops/checks.go --ops 'path FuncDecl name=ruleChannelSizeNotOneOrZero
---
path FuncDecl name=ruleMapWithoutSizeHint' --dry_run true`,
		},
	},
	"ast_rename": {
		{
			Desc:    "Rename a top-level function",
			Command: `grv ast_rename --file ops/checks.go --path '[{"kind":"FuncDecl","name":"ruleErrorHandled"}]' --to ruleErrorDiscarded --dry_run true`,
		},
	},
	"ast_node_at": {
		{
			Desc:    "Find the innermost node at line 42, column 5",
			Command: `grv ast_node_at --file ops/checks.go --line 42 --col 5`,
		},
	},
	"ast_find_symbols": {
		{
			Desc:    "Find all functions starting with 'rule' in ops/",
			Command: `grv ast_find_symbols --dir ops/ --query 'rule*' --kinds '["FuncDecl"]'`,
		},
		{
			Desc:    "Find a type by exact name",
			Command: `grv ast_find_symbols --dir ops/ --query Violation --kinds '["TypeSpec"]'`,
		},
	},
	"ast_find": {
		{
			Desc:    "Find all CallExpr nodes in a file",
			Command: `grv ast_find --file ops/checks.go --pattern '{"kind":"CallExpr"}'`,
		},
		{
			Desc:    "Find all ast.Inspect calls across a directory",
			Command: `grv ast_find --dir ops/ --pattern '{"kind":"CallExpr","fun":{"kind":"SelectorExpr","sel":"Inspect"}}'`,
		},
	},
	"ast_find_refs": {
		{
			Desc:    "Find all references to ruleErrorHandled in ops/checks.go",
			Command: `grv ast_find_refs --file ops/checks.go --path '[{"kind":"FuncDecl","name":"ruleErrorHandled"}]'`,
		},
		{
			Desc:    "Find package-wide references to a symbol",
			Command: `grv ast_find_refs --file ops/checks.go --path '[{"kind":"FuncDecl","name":"runChecks"}]' --scope package`,
		},
	},
	"ast_find_def": {
		{
			Desc:    "Find the definition of an identifier used in a function",
			Command: `grv ast_find_def --file ops/checks.go --path '[{"kind":"FuncDecl","name":"ruleErrorHandled"},{"kind":"BlockStmt"},{"kind":"ExprStmt"}]'`,
		},
	},
	"ast_find_impls": {
		{
			Desc:    "Find all types implementing the RunnerInterface",
			Command: `grv ast_find_impls --file ops/hooks.go --path '[{"kind":"TypeSpec","name":"RunnerInterface"}]'`,
		},
	},
	"ast_add_import": {
		{
			Desc:    "Add the sync package import",
			Command: `grv ast_add_import --file ops/checks.go --path '"sync"'`,
		},
		{
			Desc:    "Add an import with an alias",
			Command: `grv ast_add_import --file ops/checks.go --path '"encoding/json"' --alias j`,
		},
	},
	"ast_delete_import": {
		{
			Desc:    "Remove an unused import",
			Command: `grv ast_delete_import --file ops/checks.go --path '"strconv"'`,
		},
	},
	"ast_list_imports": {
		{
			Desc:    "List all imports and whether each is used",
			Command: `grv ast_list_imports --file ops/checks.go`,
		},
	},
	"ast_check": {
		{
			Desc:    "Check a single file with all enabled rules",
			Command: `grv ast_check --file ops/checks.go`,
		},
		{
			Desc:    "Check all files in a directory",
			Command: `grv ast_check --dir ops/`,
		},
	},
	"ast_directory": {
		{
			Desc:    "Inventory all files and symbols in the hooks package",
			Command: `grv ast_directory --dir hooks/`,
		},
		{
			Desc:    "Inventory via namespace (routed automatically)",
			Command: `grv ast_directory --namespace hooks`,
		},
	},
	"gomod_read": {
		{
			Desc:    "Parse and display go.mod",
			Command: `grv gomod_read --file go.mod`,
		},
	},
	"gomod_require": {
		{
			Desc:    "Add or update a dependency",
			Command: `grv gomod_require --file go.mod --path gopkg.in/yaml.v3 --version v3.0.1`,
		},
	},
	"gomod_drop_require": {
		{
			Desc:    "Remove a dependency",
			Command: `grv gomod_drop_require --file go.mod --path gopkg.in/yaml.v3`,
		},
	},
	"file_read": {
		{
			Desc:    "Read a YAML config file",
			Command: `grv file_read --file grv.yaml`,
		},
	},
	"file_write": {
		{
			Desc:    "Write a non-Go file (dry run first)",
			Command: `grv file_write --file grv.yaml --content 'hooks: []' --dry_run true`,
		},
	},
}

// init merges examples into ToolRegistry entries.
func init() {
	for i, t := range ToolRegistry {
		if ex, ok := exampleRegistry[t.Name]; ok {
			ToolRegistry[i].Examples = ex
		}
	}
}

// PrintExamples prints examples for all tools, or detailed examples for one tool.
func PrintExamples(toolFilter string) {
	if toolFilter != "" {
		for _, t := range ToolRegistry {
			if t.Name == toolFilter {
				printToolExamples(t)
				return
			}
		}
		fmt.Fprintf(os.Stderr, "unknown tool: %q\n", toolFilter)
		os.Exit(1)
	}

	// Summary view: one example per tool.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "grv example <tool>  for more detail on any tool")
	fmt.Fprintln(w)
	for _, t := range ToolRegistry {
		if len(t.Examples) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\n", t.Name, t.Examples[0].Desc)
		fmt.Fprintf(w, "\t  %s\n", t.Examples[0].Command)
	}
	w.Flush()
}

func printToolExamples(t ToolInfo) {
	fmt.Printf("%s — %s\n", t.Name, t.Desc)
	if t.Notes != "" {
		fmt.Printf("\n  Note: %s\n", t.Notes)
	}
	if len(t.Examples) == 0 {
		fmt.Println("\n  No examples available.")
		return
	}
	fmt.Println()
	for i, ex := range t.Examples {
		fmt.Printf("  Example %d: %s\n", i+1, ex.Desc)
		fmt.Printf("    %s\n\n", ex.Command)
	}
}
