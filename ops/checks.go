// Namespace: goast/ops
// ast_check tool — run configured rules against a file or directory.
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/hooks"
)

// DefaultChecksConfig holds the checks configuration loaded from grv.toml at daemon startup.
var DefaultChecksConfig hooks.ChecksConfig

// SetDefaultChecksConfig sets the checks configuration. Called by daemon on startup.
// Logs a warning for any unrecognized rule names so misconfiguration is visible.
func SetDefaultChecksConfig(c hooks.ChecksConfig) {
	for _, name := range c.Enforce {
		if name == "all" {
			continue
		}
		if _, ok := builtinRules[name]; !ok {
			fmt.Fprintf(os.Stderr, "grv: unknown check rule %q (known: %s)\n", name, knownRuleNames())
		}
	}
	DefaultChecksConfig = c
}

// Violation is a single rule finding returned by ast_check.
type Violation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// ASTCheckArgs is the argument struct for ast_check.
type ASTCheckArgs struct {
	File string `json:"file,omitempty"`
	Dir  string `json:"dir,omitempty"`
}

// HandleASTCheck implements the ast_check tool.
// Returns a JSON array of Violation objects (empty array if no violations).
func HandleASTCheck(args ASTCheckArgs) (json.RawMessage, error) {
	if args.Dir != "" && args.File == "" {
		return handleASTCheckDir(args.Dir)
	}
	if args.File == "" {
		return errResult("ast_check requires file or dir")
	}
	violations, err := checkFile(args.File, DefaultChecksConfig.Enforce)
	if err != nil {
		return errResult(fmt.Sprintf("check: %v", err))
	}
	return okResult(violations)
}

func handleASTCheckDir(dir string) (json.RawMessage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errResult(fmt.Sprintf("read dir: %v", err))
	}
	var all []Violation
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		violations, err := checkFile(filepath.Join(dir, e.Name()), DefaultChecksConfig.Enforce)
		if err != nil {
			continue
		}
		all = append(all, violations...)
	}
	if all == nil {
		all = []Violation{}
	}
	return okResult(all)
}

func checkFile(absFile string, enforce []string) ([]Violation, error) {
	if len(enforce) == 0 {
		return []Violation{}, nil
	}
	f, fset, src, err := editor.ParseFile(absFile)
	if err != nil {
		return nil, err
	}
	return runChecks(fset, src, f, absFile, enforce), nil
}

// enforcePostWrite runs checks on absFile after a successful write.
// If violations are found, it restores the original content and returns an error.
// originalContent must be the file bytes read before the write.
// Returns nil if no violations or checks are disabled.
func enforcePostWrite(absFile string, originalContent []byte, enforce []string) error {
	if len(enforce) == 0 {
		return nil
	}
	violations, err := checkFile(absFile, enforce)
	if err != nil || len(violations) == 0 {
		return nil
	}
	// Restore original — the write is rejected.
	_ = editor.WriteAtomic(absFile, originalContent)
	return violationsToError(violations)
}

// runChecks runs the enabled rules against an already-parsed file.
// enforce may contain rule names or "all" to run every built-in rule.
func runChecks(fset *token.FileSet, src []byte, f *ast.File, absFile string, enforce []string) []Violation {
	active := resolveRules(enforce)
	var out []Violation
	for _, rule := range active {
		out = append(out, rule(fset, src, f, absFile)...)
	}
	return out
}

// ruleFunc is a single check: given a parsed file, return any violations.
type ruleFunc func(fset *token.FileSet, src []byte, f *ast.File, absFile string) []Violation

// builtinRules is the registry of all built-in rules by name.
var builtinRules = map[string]ruleFunc{
	"error_handled": ruleErrorHandled,
}

// resolveRules expands the enforce list into concrete ruleFuncs.
// "all" includes every registered built-in rule.
func resolveRules(enforce []string) []ruleFunc {
	var out []ruleFunc
	seen := make(map[string]bool)
	for _, name := range enforce {
		if name == "all" {
			for n, fn := range builtinRules {
				if !seen[n] {
					seen[n] = true
					out = append(out, fn)
				}
			}
			continue
		}
		if fn, ok := builtinRules[name]; ok && !seen[name] {
			seen[name] = true
			out = append(out, fn)
		}
	}
	return out
}

func knownRuleNames() string {
	names := make([]string, 0, len(builtinRules))
	for n := range builtinRules {
		names = append(names, n)
	}
	return strings.Join(names, ", ")
}

// ruleErrorHandled flags AssignStmt nodes where a CallExpr result is discarded
// with a blank identifier in the last (error) position and no comment on that line.
//
// Heuristic: only fires when there are exactly 2 LHS identifiers (the canonical
// Go (value, error) pattern) and the last is "_". This avoids false-positives on
// multi-return functions where the discarded value is not an error.
//
// Map/type-assert patterns (IndexExpr, TypeAssertExpr) are excluded because the
// RHS gate requires a CallExpr.
//
// Suppression: add any comment on the same line to silence the rule, e.g.:
//
//	n, _ := fmt.Fprintln(w, msg) // error intentionally ignored: stderr write
func ruleErrorHandled(fset *token.FileSet, _ []byte, f *ast.File, absFile string) []Violation {
	// Pre-build set of lines that have a comment.
	commentLines := make(map[int]bool)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			commentLines[fset.Position(c.Slash).Line] = true
		}
	}

	var violations []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		// Only the canonical 2-return (value, error) pattern: exactly 2 LHS.
		if len(assign.Lhs) != 2 {
			return true
		}
		// Must be a single-expression RHS that is a function call.
		if len(assign.Rhs) != 1 {
			return true
		}
		if _, isCall := assign.Rhs[0].(*ast.CallExpr); !isCall {
			return true
		}
		// Last LHS must be a blank identifier.
		last, isIdent := assign.Lhs[1].(*ast.Ident)
		if !isIdent || last.Name != "_" {
			return true
		}
		// Pass if there is a comment on the same line.
		line := fset.Position(assign.Pos()).Line
		if commentLines[line] {
			return true
		}
		violations = append(violations, Violation{
			File:    absFile,
			Line:    line,
			Rule:    "error_handled",
			Message: "error return discarded with _ — handle the error or add a comment explaining why",
		})
		return true
	})
	return violations
}

// violationsToError converts a slice of violations to an error string for write-time rejection.
func violationsToError(violations []Violation) error {
	b, _ := json.Marshal(violations)
	return fmt.Errorf("check violations: %s", b)
}
