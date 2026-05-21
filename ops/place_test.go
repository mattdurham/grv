// Namespace: goast/ops
package ops_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattdurham/grv/ops"
)

// setupPkgDir creates a temp dir with Go files for routing tests.
func setupPkgDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func place(t *testing.T, dir string, node interface{}) ops.ASTPlaceResult {
	t.Helper()
	nodeJSON, _ := json.Marshal(node)
	raw, err := ops.HandleASTPlace(ops.ASTPlaceArgs{Dir: dir, Node: nodeJSON, DryRun: true})
	if err != nil {
		t.Fatalf("ast_place: %v", err)
	}
	var r ops.ASTPlaceResult
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return r
}

func funcDecl(name string) interface{} {
	return map[string]interface{}{
		"kind": "FuncDecl",
		"name": name,
		"type": map[string]interface{}{"kind": "FuncType", "params": []interface{}{}},
		"body": map[string]interface{}{"kind": "BlockStmt", "list": []interface{}{}},
	}
}

func methodDecl(recv, name string) interface{} {
	return map[string]interface{}{
		"kind": "FuncDecl",
		"recv": map[string]interface{}{
			"kind":  "Field",
			"names": []string{"r"},
			"type":  map[string]interface{}{"kind": "StarExpr", "x": map[string]interface{}{"kind": "Ident", "name": recv}},
		},
		"name": name,
		"type": map[string]interface{}{"kind": "FuncType", "params": []interface{}{}},
		"body": map[string]interface{}{"kind": "BlockStmt", "list": []interface{}{}},
	}
}

func typeDecl(name string) interface{} {
	return map[string]interface{}{
		"kind": "TypeDecl",
		"specs": []interface{}{map[string]interface{}{
			"kind": "TypeSpec", "name": name,
			"type": map[string]interface{}{"kind": "StructType", "fields": []interface{}{}},
		}},
	}
}

func typedConstDecl(constName, typeName string) interface{} {
	return map[string]interface{}{
		"kind": "ConstDecl",
		"specs": []interface{}{map[string]interface{}{
			"kind":   "ValueSpec",
			"names":  []string{constName},
			"type":   map[string]interface{}{"kind": "Ident", "name": typeName},
			"values": []interface{}{map[string]interface{}{"kind": "Ident", "name": "iota"}},
		}},
	}
}

func untypedConstDecl(name, value string) interface{} {
	return map[string]interface{}{
		"kind": "ConstDecl",
		"specs": []interface{}{map[string]interface{}{
			"kind":   "ValueSpec",
			"names":  []string{name},
			"values": []interface{}{map[string]interface{}{"kind": "BasicLit", "tok": "INT", "value": value}},
		}},
	}
}

func varDecl(name string) interface{} {
	return map[string]interface{}{
		"kind": "VarDecl",
		"specs": []interface{}{map[string]interface{}{
			"kind":   "ValueSpec",
			"names":  []string{name},
			"values": []interface{}{map[string]interface{}{"kind": "BasicLit", "tok": "INT", "value": "0"}},
		}},
	}
}

// TestASTPlace_StructToOwnFile: type Dog struct → dog.go (new file)
func TestASTPlace_StructToOwnFile(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{"main.go": "package animals\n"})
	r := place(t, dir, typeDecl("Dog"))
	if r.File != "dog.go" {
		t.Errorf("expected dog.go, got %q (reason: %s)", r.File, r.Reason)
	}
	if !r.Created {
		t.Error("expected created=true for new file")
	}
}

// TestASTPlace_MethodToTypeFile: func (d *Dog) Bark → dog.go
func TestASTPlace_MethodToTypeFile(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{
		"dog.go": "package animals\ntype Dog struct{}\n",
	})
	r := place(t, dir, methodDecl("Dog", "Bark"))
	if r.File != "dog.go" {
		t.Errorf("expected dog.go, got %q (reason: %s)", r.File, r.Reason)
	}
	if r.Created {
		t.Error("dog.go already exists, expected created=false")
	}
}

// TestASTPlace_ConstructorToTypeFile: func NewDog → dog.go
func TestASTPlace_ConstructorToTypeFile(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{
		"dog.go": "package animals\ntype Dog struct{}\n",
	})
	r := place(t, dir, funcDecl("NewDog"))
	if r.File != "dog.go" {
		t.Errorf("expected dog.go, got %q (reason: %s)", r.File, r.Reason)
	}
}

// TestASTPlace_TypedConstToEnumFile: const StatusPending Status → status.go
func TestASTPlace_TypedConstToEnumFile(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{
		"status.go": "package pkg\ntype Status int\n",
	})
	r := place(t, dir, typedConstDecl("StatusPending", "Status"))
	if r.File != "status.go" {
		t.Errorf("expected status.go, got %q (reason: %s)", r.File, r.Reason)
	}
}

// TestASTPlace_UntypedConstToConstantsFile: const MaxRetries = 3 → constants.go
func TestASTPlace_UntypedConstToConstantsFile(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{"main.go": "package pkg\n"})
	r := place(t, dir, untypedConstDecl("MaxRetries", "3"))
	if r.File != "constants.go" {
		t.Errorf("expected constants.go, got %q (reason: %s)", r.File, r.Reason)
	}
	if !r.Created {
		t.Error("expected created=true since constants.go doesn't exist")
	}
}

// TestASTPlace_FreeFuncMainPackage: free func in main package → main.go
func TestASTPlace_FreeFuncMainPackage(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{
		"main.go": "package main\nfunc main() {}\n",
	})
	r := place(t, dir, funcDecl("setupServer"))
	if r.File != "main.go" {
		t.Errorf("expected main.go, got %q (reason: %s)", r.File, r.Reason)
	}
}

// TestASTPlace_VarToConstantsFile: var defaultPort = 8080 → constants.go
func TestASTPlace_VarToConstantsFile(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{"server.go": "package pkg\n"})
	r := place(t, dir, varDecl("defaultPort"))
	if r.File != "constants.go" {
		t.Errorf("expected constants.go, got %q (reason: %s)", r.File, r.Reason)
	}
}

// TestASTPlace_MultipleMainPackages: cmd/tool1/main.go and cmd/tool2/main.go are separate
// Free functions in each go to their own main.go
func TestASTPlace_MultipleMainPackages(t *testing.T) {
	tool1 := setupPkgDir(t, map[string]string{
		"main.go": "package main\nfunc main() {}\n",
	})
	tool2 := setupPkgDir(t, map[string]string{
		"main.go": "package main\nfunc main() {}\n",
	})
	r1 := place(t, tool1, funcDecl("run"))
	r2 := place(t, tool2, funcDecl("run"))
	if r1.File != "main.go" {
		t.Errorf("tool1: expected main.go, got %q", r1.File)
	}
	if r2.File != "main.go" {
		t.Errorf("tool2: expected main.go, got %q", r2.File)
	}
	// Each routes to main.go independently — routing is per-dir
	// (r.File is a basename, so we just verify both chose main.go)
	_ = filepath.Dir // suppress import if needed
}

// TestASTPlace_ReasonNonEmpty: reason field explains the decision
func TestASTPlace_ReasonNonEmpty(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{
		"dog.go": "package pkg\ntype Dog struct{}\n",
	})
	r := place(t, dir, funcDecl("NewDog"))
	if r.Reason == "" {
		t.Error("expected non-empty reason")
	}
	if !strings.Contains(strings.ToLower(r.Reason), "dog") {
		t.Errorf("reason should mention Dog, got %q", r.Reason)
	}
}

// ---- packageImportPath ----

func TestPackageImportPath_WithGoMod(t *testing.T) {
	// Create a temp module with go.mod
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/myapp\n\ngo 1.21\n"), 0644)
	subpkg := filepath.Join(dir, "internal", "util")
	os.MkdirAll(subpkg, 0755)

	// Root package
	got := ops.PackageImportPath(dir)
	if got != "github.com/example/myapp" {
		t.Errorf("root: expected github.com/example/myapp, got %q", got)
	}

	// Sub-package
	got = ops.PackageImportPath(subpkg)
	if got != "github.com/example/myapp/internal/util" {
		t.Errorf("subpkg: expected github.com/example/myapp/internal/util, got %q", got)
	}
}

func TestPackageImportPath_NoGoMod(t *testing.T) {
	// No go.mod → falls back to directory basename
	dir := t.TempDir()
	got := ops.PackageImportPath(dir)
	if got == "" {
		t.Error("expected non-empty fallback, got empty string")
	}
	// Should not panic
}

// ---- namespace in ast_list ----

func TestASTList_HasNamespace(t *testing.T) {
	dir := setupPkgDir(t, map[string]string{
		"go.mod": "module github.com/example/animals\n\ngo 1.21\n",
		"dog.go": "package animals\ntype Dog struct{}\nfunc (d *Dog) Bark() {}\n",
	})

	nodeJSON, _ := json.Marshal(map[string]interface{}{"kind": "TypeDecl"}) // unused but needed
	_ = nodeJSON

	raw, err := ops.HandleASTList(ops.ASTListArgs{File: filepath.Join(dir, "dog.go")})
	if err != nil {
		t.Fatal(err)
	}
	var items []ops.ASTListItem
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	foundDog := false
	for _, item := range items {
		if item.Name == "Dog" {
			foundDog = true
			if item.Namespace != "github.com/example/animals#Dog" {
				t.Errorf("Dog namespace: expected github.com/example/animals#Dog, got %q", item.Namespace)
			}
		}
	}
	if !foundDog {
		t.Errorf("Dog not found in items: %v", items)
	}
}

// ---- namespace in ast_place result ----

func TestASTPlace_HasNamespace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/pets\n\ngo 1.21\n"), 0644)
	os.WriteFile(filepath.Join(dir, "pets.go"), []byte("package pets\n"), 0644)

	catNode, _ := json.Marshal(typeDecl("Cat"))
	result, err := ops.HandleASTPlace(ops.ASTPlaceArgs{Dir: dir, Node: catNode, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	var r ops.ASTPlaceResult
	if err := json.Unmarshal(result, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Namespace != "github.com/example/pets#Cat" {
		t.Errorf("expected github.com/example/pets#Cat, got %q", r.Namespace)
	}
}
