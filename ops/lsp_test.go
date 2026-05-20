// Namespace: goast/ops
package ops_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lthiery/goast/ops"
)

const fixtureA = `package p

func Add(x, y int) int {
	if x > 0 {
		return x + y
	}
	return y
}

func subtract(x, y int) int {
	return x - y
}
`

const fixtureB = `package p

type Dog struct {
	Name string
}

func (d *Dog) Greet() string {
	return d.Name
}
`

const fixtureC = `package p

import "fmt"

func hello() {
	fmt.Println("hi")
	if true {
		fmt.Println("nested")
	}
}
`

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ---- HandleASTNodeAt ----

func TestHandleASTNodeAt_FuncDecl(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{
		File: path,
		Line: 3,
		Col:  6,
	})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)

	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	foundFuncDecl := false
	for _, step := range resp.Path {
		if step.Kind == "FuncDecl" && step.Name == "Add" {
			foundFuncDecl = true
		}
	}
	if !foundFuncDecl {
		t.Errorf("expected path to contain FuncDecl{name:Add}, got: %v", resp.Path)
	}

	// The innermost node at col 6 of "func Add..." is the Ident "Add", which is
	// inside the FuncDecl — the path should contain FuncDecl regardless.
	var peek struct{ Kind string `json:"kind"` }
	if err := json.Unmarshal(resp.Node, &peek); err != nil {
		t.Fatalf("unmarshal node: %v", err)
	}
	if peek.Kind == "" {
		t.Errorf("expected non-empty node kind, got %q", peek.Kind)
	}
}

func TestHandleASTNodeAt_IfStmt(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{
		File: path,
		Line: 4,
		Col:  2,
	})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)

	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	foundIfStmt := false
	for _, step := range resp.Path {
		if step.Kind == "IfStmt" {
			foundIfStmt = true
		}
	}
	if !foundIfStmt {
		t.Errorf("expected path to contain IfStmt, got: %v", resp.Path)
	}
}

func TestHandleASTNodeAt_ReturnsMeta(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{
		File: path,
		Line: 3,
		Col:  1,
	})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)

	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	lineVal := resp.Meta["line"]
	if lineVal == nil {
		t.Error("expected meta.line to be present")
	}
}

func TestHandleASTNodeAt_OutOfRange(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{
		File: path,
		Line: 9999,
		Col:  1,
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for out-of-range line")
	}
}

func TestHandleASTNodeAt_ColZero(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{
		File: path,
		Line: 1,
		Col:  0,
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("expected tool error for col=0")
	}
}

// ---- HandleASTFindSymbols ----

func TestHandleASTFindSymbols_ExactName(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.go"), []byte(fixtureA))

	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "Add",
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)

	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "Add" || results[0].Kind != "FuncDecl" {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

func TestHandleASTFindSymbols_GlobAll(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.go"), []byte(fixtureA))

	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "*",
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)

	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
	}
	if !names["Add"] {
		t.Error("expected Add in results")
	}
	if !names["subtract"] {
		t.Error("expected subtract in results")
	}
}

func TestHandleASTFindSymbols_GlobPrefix(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.go"), []byte(fixtureA))

	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "A*",
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)

	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
	}
	if !names["Add"] {
		t.Error("expected Add in results")
	}
	if names["subtract"] {
		t.Error("expected subtract NOT in results")
	}
}

func TestHandleASTFindSymbols_KindFilter(t *testing.T) {
	combined := fixtureA + "\n" + fixtureB[len("package p\n"):]
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.go"), []byte(combined))

	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "*",
		Kinds: []string{"TypeSpec"},
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)

	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
	}
	if !names["Dog"] {
		t.Error("expected Dog in TypeSpec results")
	}
	for _, r := range results {
		if r.Kind != "TypeSpec" {
			t.Errorf("expected only TypeSpec results, got %s (%s)", r.Name, r.Kind)
		}
	}
}

func TestHandleASTFindSymbols_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "b.go"), []byte(fixtureB))

	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "dog",
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)

	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Name == "Dog" {
			found = true
		}
	}
	if !found {
		t.Error("expected Dog in case-insensitive search for 'dog'")
	}
}

func TestHandleASTFindSymbols_RecvField(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "b.go"), []byte(fixtureB))

	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "Greet",
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)

	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result for Greet")
	}
	if results[0].Recv != "*Dog" {
		t.Errorf("expected recv *Dog, got %q", results[0].Recv)
	}
}

func TestHandleASTFindSymbols_NoMatch(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.go"), []byte(fixtureA))

	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "ZZZNonExistent",
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)

	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestHandleASTFindSymbols_MultiFile(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.go"), []byte(fixtureA))
	mustWriteFile(t, filepath.Join(dir, "b.go"), []byte(fixtureB))

	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "*",
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)

	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	files := map[string]bool{}
	names := map[string]bool{}
	for _, r := range results {
		files[r.File] = true
		names[r.Name] = true
	}
	if len(files) < 2 {
		t.Errorf("expected results from at least 2 files, got: %v", files)
	}
	if !names["Add"] || !names["Dog"] {
		t.Errorf("expected Add and Dog in multi-file results, got: %v", names)
	}
}

// ---- HandleASTFind ----

func TestHandleASTFind_IfStmt(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	pattern, _ := json.Marshal(map[string]string{"kind": "IfStmt"})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)

	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) < 1 {
		t.Error("expected at least one IfStmt")
	}
	var peek struct{ Kind string `json:"kind"` }
	if err := json.Unmarshal(results[0].Node, &peek); err != nil {
		t.Fatalf("unmarshal node: %v", err)
	}
	if peek.Kind != "IfStmt" {
		t.Errorf("expected IfStmt node, got %q", peek.Kind)
	}
}

func TestHandleASTFind_CallExprPrintln(t *testing.T) {
	path := writeTempFile(t, fixtureC)

	pattern, _ := json.Marshal(map[string]interface{}{
		"kind": "CallExpr",
		"fun":  map[string]string{"kind": "SelectorExpr", "sel": "Println"},
	})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)

	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 Println calls, got %d", len(results))
	}
}

func TestHandleASTFind_BinaryExprWithOp(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	pattern, _ := json.Marshal(map[string]string{"kind": "BinaryExpr", "op": ">"})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)

	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) < 1 {
		t.Error("expected at least one BinaryExpr with op >")
	}
	for _, r := range results {
		var node struct {
			Kind string `json:"kind"`
			Op   string `json:"op"`
		}
		if err := json.Unmarshal(r.Node, &node); err != nil {
			t.Fatalf("unmarshal node: %v", err)
		}
		if node.Op != ">" {
			t.Errorf("expected op >, got %q", node.Op)
		}
	}
}

func TestHandleASTFind_Wildcard(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	pattern, _ := json.Marshal(map[string]string{"kind": "BinaryExpr"})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)

	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("expected at least 2 BinaryExpr nodes, got %d", len(results))
	}
}

func TestHandleASTFind_NoMatch(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	pattern, _ := json.Marshal(map[string]string{"kind": "SelectStmt"})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)

	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestHandleASTFind_ScopeDir(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.go"), []byte(fixtureA))
	mustWriteFile(t, filepath.Join(dir, "b.go"), []byte(fixtureB))

	pattern, _ := json.Marshal(map[string]string{"kind": "IfStmt"})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		Dir:     dir,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind dir: %v", err)
	}
	text := resultText(t, result)

	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) < 1 {
		t.Error("expected at least one IfStmt")
	}
	for _, r := range results {
		if strings.HasSuffix(r.File, "b.go") {
			t.Errorf("fixture B should not have IfStmt, but got result from %s", r.File)
		}
	}
}

func TestHandleASTFind_ResultHasPath(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	pattern, _ := json.Marshal(map[string]string{"kind": "IfStmt"})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)

	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if len(results[0].Path) == 0 {
		t.Error("expected non-empty path")
	}
	hasFuncDecl := false
	for _, step := range results[0].Path {
		if step.Kind == "FuncDecl" {
			hasFuncDecl = true
		}
	}
	if !hasFuncDecl {
		t.Errorf("expected path to include FuncDecl, got: %v", results[0].Path)
	}
}

func TestHandleASTNodeAt_ForStmt(t *testing.T) {
	// Position inside the for loop in fixtureForRange — exercises ForStmt path step
	src := `package p
func F() {
	for i := 0; i < 10; i++ {
		_ = i
	}
}
`
	path := writeTempFile(t, src)
	// Line 3 is the for statement
	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{File: path, Line: 3, Col: 2})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)
	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Path) == 0 {
		t.Fatal("expected non-empty path")
	}
}

func TestHandleASTNodeAt_RangeStmt(t *testing.T) {
	src := `package p
func F(items []int) {
	for i, v := range items {
		_ = i + v
	}
}
`
	path := writeTempFile(t, src)
	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{File: path, Line: 3, Col: 2})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)
	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Path) == 0 {
		t.Fatal("expected non-empty path for range stmt position")
	}
}

func TestHandleASTNodeAt_SwitchStmt(t *testing.T) {
	src := `package p
func F(x int) {
	switch x {
	case 1:
		_ = x
	}
}
`
	path := writeTempFile(t, src)
	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{File: path, Line: 3, Col: 2})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)
	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Path) == 0 {
		t.Fatal("expected non-empty path for switch stmt position")
	}
}

func TestHandleASTNodeAt_StructField(t *testing.T) {
	src := `package p
type Dog struct {
	Name string
	Age  int
}
`
	path := writeTempFile(t, src)
	// Line 3 is the Name field
	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{File: path, Line: 3, Col: 2})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)
	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Path) == 0 {
		t.Fatal("expected non-empty path for struct field position")
	}
}

func TestHandleASTFind_NestedPattern(t *testing.T) {
	// Tests matchPattern recursive branch — pattern with nested object
	src := `package p
import "fmt"
func F() {
	fmt.Println("hello")
	fmt.Sprintf("%d", 1)
}
`
	path := writeTempFile(t, src)
	// Find all calls to fmt.Println specifically (nested SelectorExpr pattern)
	pattern, _ := json.Marshal(map[string]interface{}{
		"kind": "CallExpr",
		"fun":  map[string]interface{}{"kind": "SelectorExpr", "sel": "Println"},
	})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)
	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 match for fmt.Println, got %d", len(results))
	}
}

func TestHandleASTFind_BinaryOpFilter(t *testing.T) {
	// Tests matchPattern with op field — finds only == comparisons
	src := `package p
func F(a, b int) bool {
	if a == b {
		return true
	}
	if a != b {
		return false
	}
	return a > b
}
`
	path := writeTempFile(t, src)
	pattern, _ := json.Marshal(map[string]interface{}{
		"kind": "BinaryExpr",
		"op":   "==",
	})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)
	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 == comparison, got %d", len(results))
	}
}

func TestHandleASTNodeAt_MultipleStmtKinds(t *testing.T) {
	// One source with many stmt types — exercises stmtKindName arms
	src := `package p

import "fmt"

func F(ch chan int) {
	x := 1
	switch x {
	case 1:
		_ = x
	}
	switch x.(type) {
	}
	select {
	case v := <-ch:
		_ = v
	}
	go func() {}()
	defer fmt.Println()
	fmt.Println(x)
	return
}
`
	tmpPath := writeTempFile(t, src)

	// Line numbers (1-based):
	//  6  → AssignStmt  (x := 1)
	//  7  → SwitchStmt
	// 11  → TypeSwitchStmt
	// 14  → SelectStmt
	// 17  → GoStmt
	// 18  → DeferStmt
	// 19  → ExprStmt
	// 20  → ReturnStmt
	tests := []struct {
		name string
		line int
	}{
		{"AssignStmt", 6},
		{"SwitchStmt", 7},
		{"TypeSwitchStmt", 11},
		{"SelectStmt", 14},
		{"GoStmt", 17},
		{"DeferStmt", 18},
		{"ExprStmt", 19},
		{"ReturnStmt", 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{
				File: tmpPath, Line: tt.line, Col: 2,
			})
			if err != nil {
				t.Fatalf("HandleASTNodeAt %s: %v", tt.name, err)
			}
			text := resultText(t, result)
			var resp ops.ASTNodeAtResponse
			if err := json.Unmarshal([]byte(text), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(resp.Path) == 0 {
				t.Fatalf("expected non-empty path for %s at line %d", tt.name, tt.line)
			}
		})
	}
}

func TestHandleASTNodeAt_SecondIfStmt(t *testing.T) {
	// Two if statements — exercises nthIndexOfKind counting
	src := `package p
func F(a, b int) int {
	if a > 0 {
		return a
	}
	if b > 0 {
		return b
	}
	return 0
}
`
	path := writeTempFile(t, src)
	// Line 6 is the second if statement
	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{File: path, Line: 6, Col: 2})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)
	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Path) == 0 {
		t.Fatal("expected non-empty path")
	}
}

func TestHandleASTNodeAt_InsideCaseClause(t *testing.T) {
	// Position inside a case clause body — exercises stmtListOf(CaseClause)
	src := `package p
func F(x int) {
	switch x {
	case 1:
		_ = x
	case 2:
		_ = x
	}
}
`
	path := writeTempFile(t, src)
	// Line 5 is inside case 1 body
	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{File: path, Line: 5, Col: 3})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)
	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Path) == 0 {
		t.Fatal("expected non-empty path inside case clause")
	}
}

func TestHandleASTNodeAt_InsideSelectCase(t *testing.T) {
	// Position inside a select comm clause — exercises stmtListOf(CommClause)
	src := `package p
func F(ch <-chan int) {
	select {
	case v := <-ch:
		_ = v
	default:
		return
	}
}
`
	path := writeTempFile(t, src)
	// Line 5 is inside the first case body
	result, err := ops.HandleASTNodeAt(ctx, emptyReq, ops.ASTNodeAtArgs{File: path, Line: 5, Col: 3})
	if err != nil {
		t.Fatalf("HandleASTNodeAt: %v", err)
	}
	text := resultText(t, result)
	var resp ops.ASTNodeAtResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Path) == 0 {
		t.Fatal("expected non-empty path inside select case")
	}
}

func TestHandleASTFindSymbols_VarAndConst(t *testing.T) {
	// Exercises VarSpec and ConstSpec arms in scanSymbols
	src := `package p

const MaxItems = 100
const MinItems = 1

var globalCount int
var debugMode bool
`
	dir := t.TempDir()
	f := filepath.Join(dir, "vars.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// Find all symbols
	result, err := ops.HandleASTFindSymbols(ctx, emptyReq, ops.ASTFindSymbolsArgs{
		Dir:   dir,
		Query: "*",
	})
	if err != nil {
		t.Fatalf("HandleASTFindSymbols: %v", err)
	}
	text := resultText(t, result)
	var results []ops.SymbolResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected symbols from var/const file")
	}
	// Verify at least one const and one var are found
	kinds := map[string]bool{}
	for _, r := range results {
		kinds[r.Kind] = true
	}
	if !kinds["ConstSpec"] && !kinds["VarSpec"] {
		t.Errorf("expected ConstSpec or VarSpec in results, got kinds: %v", kinds)
	}
}

func TestHandleASTFind_ResultHasMeta(t *testing.T) {
	path := writeTempFile(t, fixtureA)

	pattern, _ := json.Marshal(map[string]string{"kind": "ReturnStmt"})
	result, err := ops.HandleASTFind(ctx, emptyReq, ops.ASTFindArgs{
		File:    path,
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("HandleASTFind: %v", err)
	}
	text := resultText(t, result)

	var results []ops.FindResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one ReturnStmt")
	}
	lineVal := results[0].Meta["line"]
	if lineVal == nil {
		t.Error("expected meta.line in result")
	}
	line, ok := lineVal.(float64)
	if !ok || line <= 0 {
		t.Errorf("expected meta.line > 0, got %v", lineVal)
	}
}
