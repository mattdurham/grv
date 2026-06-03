// Namespace: goast/ops
// SSA-based check rules — interprocedural nil dereference analysis.
package ops

import (
	"go/token"
	"go/types"
	"path/filepath"
	"sync"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// ssaRuleFunc receives the built SSA package for the whole package being checked.
// Only instructions whose Pos() maps to absFile should be reported.
type ssaRuleFunc func(pkg *ssa.Package, absFile string) []Violation

// ssaRules is the registry of rules that need SSA form.
var ssaRules = map[string]ssaRuleFunc{
	"nil_dereference": ruleNilDereference,
}

// ssaCacheEntry holds the loaded and SSA-built package, shared across rules.
type ssaCacheEntry struct {
	ssaPkg *ssa.Package
	fset   *token.FileSet
}

type ssaCacheKey struct {
	dir     string
	gitHash string
}

var ssaCache sync.Map // ssaCacheKey → *ssaCacheEntry

// loadSSAForFile returns a cached or freshly built SSA package for the package
// containing absFile. Results are cached by (dir, git HEAD hash) so repeated
// calls within the same commit are instant.
func loadSSAForFile(absFile string) (*ssaCacheEntry, error) {
	dir := filepath.Dir(absFile)
	hash := gitHeadHash(DefaultRepoRoot)
	key := ssaCacheKey{dir: dir, gitHash: hash}

	if v, ok := ssaCache.Load(key); ok {
		return v.(*ssaCacheEntry), nil
	}

	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps,
		Dir:  dir,
		Fset: fset,
	}
	loaded, err := packages.Load(cfg, ".")
	if err != nil || len(loaded) == 0 {
		return nil, err
	}

	prog, pkgs := ssautil.AllPackages(loaded, ssa.InstantiateGenerics)
	prog.Build()

	var ssaPkg *ssa.Package
	for _, p := range pkgs {
		if p != nil {
			ssaPkg = p
			break
		}
	}
	if ssaPkg == nil {
		return nil, nil
	}

	entry := &ssaCacheEntry{ssaPkg: ssaPkg, fset: fset}
	ssaCache.Store(key, entry)
	return entry, nil
}

// InvalidateSSACache drops all cached SSA entries for a directory.
// Called after a write tool modifies a file.
func InvalidateSSACache(absFile string) {
	dir := filepath.Dir(absFile)
	ssaCache.Range(func(k, _ any) bool {
		if k.(ssaCacheKey).dir == dir {
			ssaCache.Delete(k)
		}
		return true
	})
}

// resolveSSARules returns active SSA rule funcs for the enforce list.
func resolveSSARules(enforce []string) []ssaRuleFunc {
	var out []ssaRuleFunc
	seen := make(map[string]bool)
	for _, name := range enforce {
		if name == "all" {
			for n, fn := range ssaRules {
				if !seen[n] {
					seen[n] = true
					out = append(out, fn)
				}
			}
			continue
		}
		if fn, ok := ssaRules[name]; ok && !seen[name] {
			seen[name] = true
			out = append(out, fn)
		}
	}
	return out
}

// runSSAChecks builds (or retrieves cached) SSA for the package and runs active SSA rules.
// Degrades gracefully if SSA cannot be built.
func runSSAChecks(absFile string, enforce []string) []Violation {
	active := resolveSSARules(enforce)
	if len(active) == 0 {
		return nil
	}

	entry, err := loadSSAForFile(absFile)
	if err != nil || entry == nil {
		return nil
	}

	var out []Violation
	for _, rule := range active {
		out = append(out, rule(entry.ssaPkg, absFile)...)
	}
	return out
}

// --- nil_dereference rule ---

// ruleNilDereference detects pointer dereferences where the pointer is provably nil.
//
// Two patterns are caught:
//  1. Direct nil dereference: the pointer value is a nil SSA constant.
//  2. Unchecked nil return: a call within the same package returns nil on some path
//     and the result is dereferenced without a nil guard.
//
// Only instructions whose source position maps to absFile are reported.
func ruleNilDereference(pkg *ssa.Package, absFile string) []Violation {
	// Pre-compute which functions in this package can return nil on their last result.
	nilReturnFuncs := findNilReturnFuncs(pkg)

	var violations []Violation
	for _, mem := range pkg.Members {
		fn, ok := mem.(*ssa.Function)
		if !ok {
			continue
		}
		violations = append(violations, checkFuncForNilDeref(fn, pkg, nilReturnFuncs, absFile)...)
		for _, anon := range fn.AnonFuncs {
			violations = append(violations, checkFuncForNilDeref(anon, pkg, nilReturnFuncs, absFile)...)
		}
	}
	return violations
}

// findNilReturnFuncs returns the set of functions in pkg that provably return nil
// on at least one path (for the first pointer/interface return value).
func findNilReturnFuncs(pkg *ssa.Package) map[*ssa.Function]bool {
	result := make(map[*ssa.Function]bool)
	for _, mem := range pkg.Members {
		fn, ok := mem.(*ssa.Function)
		if !ok || fn.Blocks == nil {
			continue
		}
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				ret, ok := instr.(*ssa.Return)
				if !ok {
					continue
				}
				for _, res := range ret.Results {
					if isNilSSAValue(res) && isPointerOrInterface(res.Type()) {
						result[fn] = true
					}
				}
			}
		}
	}
	return result
}

// checkFuncForNilDeref walks a single SSA function body looking for unguarded
// nil dereferences.
func checkFuncForNilDeref(fn *ssa.Function, pkg *ssa.Package, nilReturnFuncs map[*ssa.Function]bool, absFile string) []Violation {
	if fn.Blocks == nil {
		return nil
	}

	// Collect values in this function that may be nil.
	maybeNil := collectMaybeNilValues(fn, nilReturnFuncs)

	var violations []Violation
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			ptr, instrPos := derefPointer(instr)
			if ptr == nil {
				continue
			}
			// nil Const values are not instructions (they float as referenced values)
			// so they won't appear in maybeNil — check them directly here.
			if !isNilSSAValue(ptr) && !maybeNil[ptr] {
				continue
			}
			// Check that a nil guard doesn't dominate this block.
			if isGuardedByNilCheck(b, ptr, fn) {
				continue
			}
			pos := fn.Prog.Fset.Position(instrPos)
			if filepath.Clean(pos.Filename) != filepath.Clean(absFile) {
				continue
			}
			violations = append(violations, Violation{
				File:    absFile,
				Line:    pos.Line,
				Rule:    "nil_dereference",
				Message: "potential nil pointer dereference — value may be nil on some execution path",
			})
		}
	}
	return violations
}

// collectMaybeNilValues returns the set of SSA values in fn that may be nil:
//   - nil SSA constants
//   - Phi nodes all of whose operands are nil
//   - Results of calls to functions that can return nil (same package)
//   - Loads from local allocas that are never stored a non-nil value
//     (i.e. `var p *T` with no assignment → load of the alloca is nil)
func collectMaybeNilValues(fn *ssa.Function, nilReturnFuncs map[*ssa.Function]bool) map[ssa.Value]bool {
	result := make(map[ssa.Value]bool)

	// Step 1: find which allocas are written with a non-nil value anywhere in the function.
	// An alloca never written to holds its zero value (nil for pointer types).
	writtenAllocas := make(map[*ssa.Alloc]bool)
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			store, ok := instr.(*ssa.Store)
			if !ok {
				continue
			}
			alloc, ok := store.Addr.(*ssa.Alloc)
			if !ok {
				continue
			}
			if !isNilSSAValue(store.Val) {
				writtenAllocas[alloc] = true
			}
		}
	}

	// Step 2: walk instructions and collect maybe-nil values.
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			val, ok := instr.(ssa.Value)
			if !ok {
				continue
			}
			switch v := val.(type) {
			case *ssa.Const:
				if v.IsNil() && isPointerOrInterface(v.Type()) {
					result[v] = true
				}
			case *ssa.Phi:
				if isPointerOrInterface(v.Type()) && allEdgesNil(v.Edges, result) {
					result[v] = true
				}
			case *ssa.Call:
				if callee, ok := v.Common().Value.(*ssa.Function); ok {
					if nilReturnFuncs[callee] && isPointerOrInterface(v.Type()) {
						result[v] = true
					}
				}
			case *ssa.UnOp:
				// Load from an unwritten alloca: `var p *T` with no assignment.
				if v.Op == token.MUL {
					if alloc, ok := v.X.(*ssa.Alloc); ok && !writtenAllocas[alloc] {
						if isPointerOrInterface(v.Type()) {
							result[v] = true
						}
					}
				}
			}
		}
	}
	return result
}

// derefPointer returns the pointer value being dereferenced and the instruction
// position. Returns nil, 0 if the instruction is not a dereference.
func derefPointer(instr ssa.Instruction) (ssa.Value, token.Pos) {
	switch v := instr.(type) {
	case *ssa.FieldAddr:
		return v.X, v.Pos()
	case *ssa.UnOp:
		if v.Op == token.MUL {
			return v.X, v.Pos()
		}
	case *ssa.IndexAddr:
		if _, isSlice := v.X.Type().Underlying().(*types.Slice); isSlice {
			return v.X, v.Pos()
		}
	case *ssa.MapUpdate:
		return v.Map, v.Pos()
	case *ssa.Lookup:
		if _, isMap := v.X.Type().Underlying().(*types.Map); isMap {
			return v.X, v.Pos()
		}
	}
	return nil, 0
}

// isGuardedByNilCheck returns true if block b is dominated by a block that
// checks ptr != nil (or nil != ptr) and b is on the non-nil branch.
func isGuardedByNilCheck(b *ssa.BasicBlock, ptr ssa.Value, fn *ssa.Function) bool {
	for _, dom := range fn.DomPreorder() {
		if dom == b {
			break
		}
		// Look for an If instruction whose condition checks ptr against nil.
		if len(dom.Instrs) == 0 {
			continue
		}
		ifInstr, ok := dom.Instrs[len(dom.Instrs)-1].(*ssa.If)
		if !ok {
			continue
		}
		binop, ok := ifInstr.Cond.(*ssa.BinOp)
		if !ok {
			continue
		}
		if binop.Op != token.NEQ && binop.Op != token.EQL {
			continue
		}
		xIsPtr := binop.X == ptr && isNilSSAValue(binop.Y)
		yIsPtr := binop.Y == ptr && isNilSSAValue(binop.X)
		if !xIsPtr && !yIsPtr {
			continue
		}
		// b must be reachable only through the non-nil branch.
		trueSucc := dom.Succs[0]
		nilBranch := trueSucc
		if binop.Op == token.NEQ {
			nilBranch = dom.Succs[1] // NEQ true means ptr != nil, false branch is nil
		}
		if blockDominates(dom.Succs[0], b) && nilBranch != dom.Succs[0] {
			return true
		}
		if blockDominates(dom.Succs[1], b) && nilBranch != dom.Succs[1] {
			return true
		}
	}
	return false
}

// blockDominates reports whether a dominates or equals b.
func blockDominates(a, b *ssa.BasicBlock) bool {
	for cur := b; cur != nil; cur = cur.Idom() {
		if cur == a {
			return true
		}
	}
	return false
}

func isNilSSAValue(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	return ok && c.IsNil()
}

func isPointerOrInterface(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Pointer, *types.Interface, *types.Map, *types.Slice, *types.Chan:
		return true
	}
	return false
}

func allEdgesNil(edges []ssa.Value, maybeNil map[ssa.Value]bool) bool {
	if len(edges) == 0 {
		return false
	}
	for _, e := range edges {
		if !isNilSSAValue(e) && !maybeNil[e] {
			return false
		}
	}
	return true
}
