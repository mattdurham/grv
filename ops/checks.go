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
	"strconv"
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
			if _, ok2 := typeAwareRules[name]; !ok2 {
				fmt.Fprintf(os.Stderr, "grv: unknown check rule %q (known: %s)\n", name, knownRuleNames())
			}
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
	out := runChecks(fset, src, f, absFile, enforce)
	// Run type-aware rules if any are active; degrade gracefully on failure.
	out = append(out, runTypeAwareChecks(absFile, enforce)...)
	return out, nil
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
	"error_handled":              ruleErrorHandled,
	"type_assertion_not_checked": ruleTypeAssertionNotChecked,
	"mutex_not_embedded":         ruleMutexNotEmbedded,
	"channel_size_not_one_or_zero": ruleChannelSizeNotOneOrZero,
	"map_without_size_hint":      ruleMapWithoutSizeHint,
	"slice_without_capacity":     ruleSliceWithoutCapacity,
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
	names := make([]string, 0, len(builtinRules)+len(typeAwareRules))
	for n := range builtinRules {
		names = append(names, n)
	}
	for n := range typeAwareRules {
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

// ruleTypeAssertionNotChecked flags type assertions that discard the boolean ok value.
//
// Two patterns are flagged:
//  1. Single-LHS assign: v := x.(T) — the ok form v, ok := x.(T) is not flagged.
//  2. Bare expression statement: x.(T) used as a statement (always panics on mismatch).
//
// x.(type) switch expressions (TypeAssertExpr with nil Type) are never flagged.
// Suppression: add any comment on the same line.
func ruleTypeAssertionNotChecked(fset *token.FileSet, _ []byte, f *ast.File, absFile string) []Violation {
	commentLines := make(map[int]bool)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			commentLines[fset.Position(c.Slash).Line] = true
		}
	}

	var violations []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Single-LHS assign with a TypeAssertExpr on RHS is unchecked.
			if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
				return true
			}
			ta, ok := node.Rhs[0].(*ast.TypeAssertExpr)
			if !ok || ta.Type == nil {
				return true
			}
			line := fset.Position(node.Pos()).Line
			if commentLines[line] {
				return true
			}
			violations = append(violations, Violation{
				File:    absFile,
				Line:    line,
				Rule:    "type_assertion_not_checked",
				Message: "type assertion result not checked — use v, ok := x.(T) to avoid a panic on mismatch",
			})
		case *ast.ExprStmt:
			ta, ok := node.X.(*ast.TypeAssertExpr)
			if !ok || ta.Type == nil {
				return true
			}
			line := fset.Position(node.Pos()).Line
			if commentLines[line] {
				return true
			}
			violations = append(violations, Violation{
				File:    absFile,
				Line:    line,
				Rule:    "type_assertion_not_checked",
				Message: "type assertion used as statement always panics on mismatch — assign the result instead",
			})
		}
		return true
	})
	return violations
}

// ruleMutexNotEmbedded flags anonymous (embedded) sync.Mutex or sync.RWMutex fields in structs.
//
// Embedding a mutex exposes Lock/Unlock on the outer type, which widens its API
// unintentionally. Prefer a named field: mu sync.Mutex.
// Suppression: add any comment on the same line as the field.
func ruleMutexNotEmbedded(fset *token.FileSet, _ []byte, f *ast.File, absFile string) []Violation {
	commentLines := make(map[int]bool)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			commentLines[fset.Position(c.Slash).Line] = true
		}
	}

	var violations []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok || st.Fields == nil {
			return true
		}
		for _, field := range st.Fields.List {
			// Anonymous field: no names.
			if len(field.Names) != 0 {
				continue
			}
			if !isMutexType(field.Type) {
				continue
			}
			line := fset.Position(field.Pos()).Line
			if commentLines[line] {
				continue
			}
			violations = append(violations, Violation{
				File:    absFile,
				Line:    line,
				Rule:    "mutex_not_embedded",
				Message: "sync.Mutex/sync.RWMutex should not be embedded — use a named field (e.g. mu sync.Mutex) to avoid exposing Lock/Unlock on the outer type",
			})
		}
		return true
	})
	return violations
}

// isMutexType reports whether expr is sync.Mutex, sync.RWMutex, *sync.Mutex, or *sync.RWMutex.
func isMutexType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		return isSyncMutexSel(t)
	case *ast.StarExpr:
		sel, ok := t.X.(*ast.SelectorExpr)
		return ok && isSyncMutexSel(sel)
	}
	return false
}

func isSyncMutexSel(sel *ast.SelectorExpr) bool {
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "sync" {
		return false
	}
	return sel.Sel.Name == "Mutex" || sel.Sel.Name == "RWMutex"
}

// ruleChannelSizeNotOneOrZero flags make(chan T, N) calls where N is a literal integer > 1.
//
// Buffered channels with sizes > 1 often hide synchronisation bugs. Size 0 (unbuffered)
// and size 1 (single-slot handoff) are accepted idioms. Non-literal sizes are skipped
// because the value cannot be determined statically.
// Suppression: add any comment on the same line.
func ruleChannelSizeNotOneOrZero(fset *token.FileSet, _ []byte, f *ast.File, absFile string) []Violation {
	commentLines := make(map[int]bool)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			commentLines[fset.Position(c.Slash).Line] = true
		}
	}

	var violations []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// Fun must be the builtin "make".
		fun, ok := call.Fun.(*ast.Ident)
		if !ok || fun.Name != "make" {
			return true
		}
		// First arg must be a channel type.
		if len(call.Args) < 2 {
			return true
		}
		if _, isChan := call.Args[0].(*ast.ChanType); !isChan {
			return true
		}
		// Second arg must be a literal integer.
		lit, ok := call.Args[1].(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		v, err := strconv.Atoi(lit.Value)
		if err != nil || v <= 1 {
			return true
		}
		line := fset.Position(call.Pos()).Line
		if commentLines[line] {
			return true
		}
		violations = append(violations, Violation{
			File:    absFile,
			Line:    line,
			Rule:    "channel_size_not_one_or_zero",
			Message: fmt.Sprintf("channel buffer size %d is greater than 1 — prefer 0 (unbuffered) or 1; larger buffers often hide synchronisation bugs", v),
		})
		return true
	})
	return violations
}

// ruleMapWithoutSizeHint flags make(map[K]V) calls with no size argument.
//
// Providing a capacity hint lets the runtime pre-allocate buckets and avoids
// repeated rehashing as the map grows. Use 0 when the final size is unknown —
// that is semantically equivalent to no hint but makes the intent explicit.
//
// Passes:
//
//	make(map[K]V, 0)   // unknown size, explicit acknowledgement
//	make(map[K]V, 100) // known capacity
//
// Fails:
//
//	make(map[K]V) // no hint at all
func ruleMapWithoutSizeHint(fset *token.FileSet, _ []byte, f *ast.File, absFile string) []Violation {
	var violations []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun, ok := call.Fun.(*ast.Ident)
		if !ok || fun.Name != "make" {
			return true
		}
		if len(call.Args) < 1 {
			return true
		}
		if _, isMap := call.Args[0].(*ast.MapType); !isMap {
			return true
		}
		// Flag only when no size argument is provided at all.
		if len(call.Args) >= 2 {
			return true
		}
		violations = append(violations, Violation{
			File:    absFile,
			Line:    fset.Position(call.Pos()).Line,
			Rule:    "map_without_size_hint",
			Message: "make(map[...]) has no size hint — add a capacity estimate, or use 0 if the final size is unknown",
		})
		return true
	})
	return violations
}

// ruleSliceWithoutCapacity flags make([]T, n) calls with no capacity argument.
//
// Specifying a capacity avoids repeated allocations as the slice grows via append.
// Use 0 as the capacity when the final size is unknown — that is explicit and
// avoids the ambiguity of omitting the argument entirely.
//
// Passes:
//
//	make([]T, 0, 0)   // unknown capacity, explicit acknowledgement
//	make([]T, 0, 100) // known capacity
//	var s []T         // preferred for nil/zero-length slices
//
// Fails:
//
//	make([]T, 0) // length provided but no capacity hint
//	make([]T, n) // length provided but no capacity hint
func ruleSliceWithoutCapacity(fset *token.FileSet, _ []byte, f *ast.File, absFile string) []Violation {
	var violations []Violation
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun, ok := call.Fun.(*ast.Ident)
		if !ok || fun.Name != "make" {
			return true
		}
		if len(call.Args) < 1 {
			return true
		}
		if _, isSlice := call.Args[0].(*ast.ArrayType); !isSlice {
			return true
		}
		// Flag only when exactly one extra argument (length) but no capacity.
		if len(call.Args) != 2 {
			return true
		}
		violations = append(violations, Violation{
			File:    absFile,
			Line:    fset.Position(call.Pos()).Line,
			Rule:    "slice_without_capacity",
			Message: "make([]T, n) has no capacity argument — add a capacity estimate, or use 0 if unknown; use var s []T instead for zero-length nil slices",
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
