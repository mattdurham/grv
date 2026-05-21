// Namespace: goast/ops
// LSP-style tools: ast_node_at, ast_find_symbols, ast_find
package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/kinds"
	"github.com/mattdurham/grv/meta"
	"github.com/mattdurham/grv/selector"
	"golang.org/x/tools/go/ast/astutil"
)

// nodeAncestor records how a node is held by its parent.
type nodeAncestor struct {
	node      ast.Node
	parent    ast.Node
	fieldName string
	index     int // -1 for scalar fields
}

// collectAncestors builds a map from every node in the file to its parent context.
func collectAncestors(f *ast.File) map[ast.Node]nodeAncestor {
	result := make(map[ast.Node]nodeAncestor)
	astutil.Apply(f, func(c *astutil.Cursor) bool {
		n := c.Node()
		if n == nil {
			return false
		}
		result[n] = nodeAncestor{
			node:      n,
			parent:    c.Parent(),
			fieldName: c.Name(),
			index:     c.Index(),
		}
		return true
	}, nil)
	return result
}

// buildPath walks from target up to the file root and returns a selector path.
func buildPath(target ast.Node, ancestors map[ast.Node]nodeAncestor) []selector.PathStep {
	var steps []selector.PathStep
	current := target
	for {
		anc, ok := ancestors[current]
		if !ok {
			break
		}
		if _, isFile := anc.parent.(*ast.File); isFile || anc.parent == nil {
			step, ok := nodeToPathStep(current, anc.parent, anc.fieldName, anc.index, ancestors)
			if ok {
				steps = append(steps, step)
			}
			break
		}
		step, ok := nodeToPathStep(current, anc.parent, anc.fieldName, anc.index, ancestors)
		if ok {
			steps = append(steps, step)
		}
		current = anc.parent
	}
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}
	return steps
}

// nodeToPathStep maps a node and its parent relationship to a PathStep.
func nodeToPathStep(node ast.Node, parent ast.Node, fieldName string, sliceIndex int, ancestors map[ast.Node]nodeAncestor) (selector.PathStep, bool) {
	switch n := node.(type) {
	case *ast.FuncDecl:
		step := selector.PathStep{Kind: "FuncDecl", Name: n.Name.Name}
		if n.Recv != nil && len(n.Recv.List) > 0 {
			step.Recv = recvTypeString(n.Recv.List[0])
		}
		return step, true

	case *ast.TypeSpec:
		return selector.PathStep{Kind: "TypeSpec", Name: n.Name.Name}, true

	case *ast.GenDecl:
		switch n.Tok.String() {
		case "type":
			return selector.PathStep{Kind: "TypeDecl"}, true
		case "var":
			return selector.PathStep{Kind: "VarDecl"}, true
		case "const":
			return selector.PathStep{Kind: "ConstDecl"}, true
		case "import":
			return selector.PathStep{Kind: "ImportDecl"}, true
		}

	case *ast.BlockStmt:
		if fieldName == "Body" {
			return selector.PathStep{Kind: "Body"}, true
		}

	case *ast.FieldList:
		switch fieldName {
		case "Params":
			return selector.PathStep{Kind: "Params"}, true
		case "Results":
			return selector.PathStep{Kind: "Results"}, true
		}

	case *ast.StructType:
		return selector.PathStep{Kind: "StructType"}, true

	case *ast.InterfaceType:
		return selector.PathStep{Kind: "InterfaceType"}, true

	case *ast.Field:
		idx := sliceIndex
		return selector.PathStep{Kind: "Field", Index: &idx}, true
	}

	// Scalar field steps.
	switch fieldName {
	case "Cond":
		return selector.PathStep{Kind: "Cond"}, true
	case "Init":
		return selector.PathStep{Kind: "Init"}, true
	case "Post":
		return selector.PathStep{Kind: "Post"}, true
	case "Else":
		return selector.PathStep{Kind: "Else"}, true
	case "Tag":
		return selector.PathStep{Kind: "Tag"}, true
	case "X":
		return selector.PathStep{Kind: "X"}, true
	case "Y":
		return selector.PathStep{Kind: "Y"}, true
	case "Fun":
		return selector.PathStep{Kind: "Fun"}, true
	case "Sel":
		return selector.PathStep{Kind: "Sel"}, true
	case "Key":
		return selector.PathStep{Kind: "Key"}, true
	case "Value":
		return selector.PathStep{Kind: "Value"}, true
	}

	// Indexed stmt slice steps.
	if fieldName == "List" || fieldName == "Body" {
		kindName := stmtKindName(node)
		if kindName != "" {
			nth := nthIndexOfKind(node, parent, sliceIndex)
			return selector.PathStep{Kind: kindName, Index: &nth}, true
		}
	}

	// Indexed expr slice steps.
	switch fieldName {
	case "Args":
		return selector.PathStep{Kind: "Args", Index: &sliceIndex}, true
	case "Elts":
		return selector.PathStep{Kind: "Elts", Index: &sliceIndex}, true
	case "Lhs":
		return selector.PathStep{Kind: "Lhs", Index: &sliceIndex}, true
	case "Rhs":
		return selector.PathStep{Kind: "Rhs", Index: &sliceIndex}, true
	}

	return selector.PathStep{}, false
}

func stmtKindName(n ast.Node) string {
	switch n.(type) {
	case *ast.IfStmt:
		return "IfStmt"
	case *ast.ForStmt:
		return "ForStmt"
	case *ast.RangeStmt:
		return "RangeStmt"
	case *ast.SwitchStmt:
		return "SwitchStmt"
	case *ast.TypeSwitchStmt:
		return "TypeSwitchStmt"
	case *ast.SelectStmt:
		return "SelectStmt"
	case *ast.AssignStmt:
		return "AssignStmt"
	case *ast.ReturnStmt:
		return "ReturnStmt"
	case *ast.ExprStmt:
		return "ExprStmt"
	case *ast.GoStmt:
		return "GoStmt"
	case *ast.DeferStmt:
		return "DeferStmt"
	case *ast.CaseClause:
		return "CaseClause"
	case *ast.CommClause:
		return "CommClause"
	}
	return ""
}

func nthIndexOfKind(node ast.Node, parent ast.Node, sliceIndex int) int {
	list := stmtListOf(parent)
	if list == nil {
		return 0
	}
	targetType := fmt.Sprintf("%T", node)
	count := 0
	for i := 0; i < sliceIndex && i < len(list); i++ {
		if fmt.Sprintf("%T", list[i]) == targetType {
			count++
		}
	}
	return count
}

func stmtListOf(n ast.Node) []ast.Stmt {
	switch v := n.(type) {
	case *ast.BlockStmt:
		return v.List
	case *ast.CaseClause:
		return v.Body
	case *ast.CommClause:
		return v.Body
	}
	return nil
}

// ASTNodeAtArgs is the argument struct for ast_node_at.
type ASTNodeAtArgs struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// ASTNodeAtResponse is the response for ast_node_at.
type ASTNodeAtResponse struct {
	Path   []selector.PathStep `json:"path"`
	Node   json.RawMessage     `json:"node"`
	Source string              `json:"source,omitempty"`
	Meta   meta.Meta           `json:"meta,omitempty"`
}

// HandleASTNodeAt implements the ast_node_at tool.
func HandleASTNodeAt(args ASTNodeAtArgs) (json.RawMessage, error) {
	f, fset, src, err := editor.ParseFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("parse: %v", err))
	}

	tokenFile := fset.File(f.Pos())
	if args.Line < 1 || args.Line > tokenFile.LineCount() {
		return errResult(fmt.Sprintf("line %d out of range (file has %d lines)", args.Line, tokenFile.LineCount()))
	}
	if args.Col < 1 {
		return errResult(fmt.Sprintf("col %d out of range (must be >= 1)", args.Col))
	}

	lineStart := tokenFile.LineStart(args.Line)
	lineStartOffset := fset.Position(lineStart).Offset
	targetOffset := lineStartOffset + (args.Col - 1)
	if targetOffset >= len(src) {
		return errResult(fmt.Sprintf("col %d out of range for line %d", args.Col, args.Line))
	}

	ancestors := collectAncestors(f)

	var best ast.Node
	bestSpan := -1
	for node := range ancestors {
		pos := fset.Position(node.Pos())
		end := fset.Position(node.End())
		if !pos.IsValid() || !end.IsValid() {
			continue
		}
		if pos.Offset <= targetOffset && targetOffset < end.Offset {
			span := end.Offset - pos.Offset
			if bestSpan < 0 || span < bestSpan {
				best = node
				bestSpan = span
			}
		}
	}
	if best == nil {
		return errResult(fmt.Sprintf("no node found at line %d col %d", args.Line, args.Col))
	}

	nodePath := buildPath(best, ancestors)

	nodeJSON, err := kinds.MarshalNode(best)
	if err != nil {
		return errResult(fmt.Sprintf("marshal node: %v", err))
	}

	var sourceFrag string
	pos := fset.Position(best.Pos())
	end := fset.Position(best.End())
	if pos.IsValid() && end.IsValid() && end.Offset <= len(src) {
		sourceFrag = string(src[pos.Offset:end.Offset])
	}

	m := meta.Compute(fset, src, best, nil, len(nodePath))
	resp := ASTNodeAtResponse{Path: nodePath, Node: nodeJSON, Source: sourceFrag, Meta: m}
	return okResult(resp)
}

// ASTFindSymbolsArgs is the argument struct for ast_find_symbols.
type ASTFindSymbolsArgs struct {
	Dir   string   `json:"dir"`
	Query string   `json:"query"`
	Kinds []string `json:"kinds,omitempty"`
}

// SymbolResult is one entry in the ast_find_symbols response.
type SymbolResult struct {
	File      string              `json:"file"`
	Path      []selector.PathStep `json:"path"`
	Kind      string              `json:"kind"`
	Name      string              `json:"name"`
	Recv      string              `json:"recv,omitempty"`
	Line      int                 `json:"line"`
	Namespace string              `json:"namespace,omitempty"` // <import-path>#<Name>
	Readonly  bool                `json:"readonly"`
	Meta      meta.Meta           `json:"meta,omitempty"`
}

// HandleASTFindSymbols implements the ast_find_symbols tool.
func HandleASTFindSymbols(args ASTFindSymbolsArgs) (json.RawMessage, error) {
	if args.Dir == "" {
		return errResult("dir is required")
	}
	if args.Query == "" {
		args.Query = "*"
	}

	entries, err := os.ReadDir(args.Dir)
	if err != nil {
		return errResult(fmt.Sprintf("readdir: %v", err))
	}

	kindsSet := make(map[string]bool, len(args.Kinds))
	for _, k := range args.Kinds {
		kindsSet[k] = true
	}

	var results []SymbolResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		filePath := filepath.Join(args.Dir, entry.Name())
		f, fset, src, parseErr := editor.ParseFile(filePath)
		if parseErr != nil {
			continue
		}
		results = append(results, scanSymbols(f, fset, src, filePath, args.Query, kindsSet)...)
	}

	if results == nil {
		results = []SymbolResult{}
	}
	return okResult(results)
}

func scanSymbols(f *ast.File, fset *token.FileSet, src []byte, filePath, query string, kindsFilter map[string]bool) []SymbolResult {
	queryLower := strings.ToLower(query)
	var results []SymbolResult

	matchName := func(name string) bool {
		ok, _ := path.Match(queryLower, strings.ToLower(name))
		return ok
	}
	kindAllowed := func(k string) bool {
		return len(kindsFilter) == 0 || kindsFilter[k]
	}

	pkgPath := packageImportPath(filepath.Dir(filePath))
	ro := isReadonly(filePath)
	nsFor := func(name string) string { return pkgPath + "#" + name }

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !kindAllowed("FuncDecl") || !matchName(d.Name.Name) {
				continue
			}
			step := selector.PathStep{Kind: "FuncDecl", Name: d.Name.Name}
			recv := ""
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recv = recvTypeString(d.Recv.List[0])
				step.Recv = recv
			}
			results = append(results, SymbolResult{
				File:      filePath,
				Path:      []selector.PathStep{step},
				Kind:      "FuncDecl",
				Name:      d.Name.Name,
				Recv:      recv,
				Line:      fset.Position(d.Pos()).Line,
				Namespace: nsFor(d.Name.Name),
				Readonly:  ro,
				Meta:      meta.Compute(fset, src, d, nil, 1),
			})

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !kindAllowed("TypeSpec") || !matchName(s.Name.Name) {
						continue
					}
					results = append(results, SymbolResult{
						File:      filePath,
						Path:      []selector.PathStep{{Kind: "TypeSpec", Name: s.Name.Name}},
						Kind:      "TypeSpec",
						Name:      s.Name.Name,
						Line:      fset.Position(s.Pos()).Line,
						Namespace: nsFor(s.Name.Name),
						Readonly:  ro,
						Meta:      meta.Compute(fset, src, s, nil, 1),
					})
				case *ast.ValueSpec:
					specKind := "VarSpec"
					if d.Tok.String() == "const" {
						specKind = "ConstSpec"
					}
					if !kindAllowed(specKind) {
						continue
					}
					for _, nameIdent := range s.Names {
						if !matchName(nameIdent.Name) {
							continue
						}
						results = append(results, SymbolResult{
							File:      filePath,
							Path:      []selector.PathStep{{Kind: specKind, Name: nameIdent.Name}},
							Kind:      specKind,
							Name:      nameIdent.Name,
							Line:      fset.Position(s.Pos()).Line,
							Namespace: nsFor(nameIdent.Name),
							Readonly:  ro,
						})
					}
				}
			}
		}
	}
	return results
}

// ASTFindArgs is the argument struct for ast_find.
type ASTFindArgs struct {
	File    string          `json:"file,omitempty"`
	Dir     string          `json:"dir,omitempty"`
	Pattern json.RawMessage `json:"pattern"`
}

// FindResult is one entry in the ast_find response.
type FindResult struct {
	File   string              `json:"file"`
	Path   []selector.PathStep `json:"path"`
	Node   json.RawMessage     `json:"node"`
	Source string              `json:"source,omitempty"`
	Meta   meta.Meta           `json:"meta,omitempty"`
}

// HandleASTFind implements the ast_find tool.
func HandleASTFind(args ASTFindArgs) (json.RawMessage, error) {
	if args.File == "" && args.Dir == "" {
		return errResult("file or dir is required")
	}
	if len(args.Pattern) == 0 || string(args.Pattern) == "null" {
		return errResult("pattern is required")
	}

	var patternMap map[string]json.RawMessage
	if err := json.Unmarshal(args.Pattern, &patternMap); err != nil {
		return errResult(fmt.Sprintf("parse pattern: %v", err))
	}

	var files []string
	if args.File != "" {
		files = []string{args.File}
	} else {
		entries, err := os.ReadDir(args.Dir)
		if err != nil {
			return errResult(fmt.Sprintf("readdir: %v", err))
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				files = append(files, filepath.Join(args.Dir, e.Name()))
			}
		}
	}

	var allResults []FindResult
	for _, filePath := range files {
		f, fset, src, err := editor.ParseFile(filePath)
		if err != nil {
			continue
		}
		allResults = append(allResults, findInFile(f, fset, src, filePath, patternMap)...)
	}

	if allResults == nil {
		allResults = []FindResult{}
	}
	return okResult(allResults)
}

func findInFile(f *ast.File, fset *token.FileSet, src []byte, filePath string, patternMap map[string]json.RawMessage) []FindResult {
	ancestors := collectAncestors(f)
	var results []FindResult

	astutil.Apply(f, func(c *astutil.Cursor) bool {
		n := c.Node()
		if n == nil {
			return false
		}
		nodeJSON, err := kinds.MarshalNode(n)
		if err != nil {
			return true
		}
		var actualMap map[string]json.RawMessage
		if err := json.Unmarshal(nodeJSON, &actualMap); err != nil {
			return true
		}
		if !matchPattern(patternMap, actualMap) {
			return true
		}
		nodePath := buildPath(n, ancestors)
		var sourceFrag string
		pos := fset.Position(n.Pos())
		end := fset.Position(n.End())
		if pos.IsValid() && end.IsValid() && end.Offset <= len(src) {
			sourceFrag = string(src[pos.Offset:end.Offset])
		}
		m := meta.Compute(fset, src, n, nil, len(nodePath))
		results = append(results, FindResult{
			File:   filePath,
			Path:   nodePath,
			Node:   nodeJSON,
			Source: sourceFrag,
			Meta:   m,
		})
		return true
	}, nil)

	return results
}

// matchPattern returns true if all non-null fields in patternMap match actualMap.
// Absent or null pattern fields are wildcards. Array fields require exact-length match.
func matchPattern(patternMap, actualMap map[string]json.RawMessage) bool {
	for k, pv := range patternMap {
		if len(pv) == 0 || string(pv) == "null" {
			continue
		}
		av, ok := actualMap[k]
		if !ok {
			return false
		}
		if len(pv) > 0 && pv[0] == '{' && len(av) > 0 && av[0] == '{' {
			var pm, am map[string]json.RawMessage
			if json.Unmarshal(pv, &pm) == nil && json.Unmarshal(av, &am) == nil {
				if !matchPattern(pm, am) {
					return false
				}
				continue
			}
		}
		if len(pv) > 0 && pv[0] == '[' && len(av) > 0 && av[0] == '[' {
			var pa, aa []json.RawMessage
			if json.Unmarshal(pv, &pa) == nil && json.Unmarshal(av, &aa) == nil {
				if len(pa) != len(aa) {
					return false
				}
				for i := range pa {
					var pm2, am2 map[string]json.RawMessage
					if json.Unmarshal(pa[i], &pm2) == nil && json.Unmarshal(aa[i], &am2) == nil {
						if !matchPattern(pm2, am2) {
							return false
						}
					} else if string(pa[i]) != string(aa[i]) {
						return false
					}
				}
				continue
			}
		}
		if string(pv) != string(av) {
			return false
		}
	}
	return true
}
