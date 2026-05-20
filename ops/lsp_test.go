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
