package daemon_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattdurham/grv/daemon"
)

// ---- parseNamespace tests ----

func TestParseNamespace_PkgOnly(t *testing.T) {
	pkg, decl, err := daemon.ParseNamespace("hooks")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg != "hooks" || decl != "" {
		t.Fatalf("got (%q, %q), want (\"hooks\", \"\")", pkg, decl)
	}
}

func TestParseNamespace_PkgWithDecl(t *testing.T) {
	pkg, decl, err := daemon.ParseNamespace("hooks#RunFile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg != "hooks" || decl != "RunFile" {
		t.Fatalf("got (%q, %q), want (\"hooks\", \"RunFile\")", pkg, decl)
	}
}

func TestParseNamespace_DotPkg(t *testing.T) {
	pkg, decl, err := daemon.ParseNamespace(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg != "." || decl != "" {
		t.Fatalf("got (%q, %q), want (\".\", \"\")", pkg, decl)
	}
}

func TestParseNamespace_EmptyString(t *testing.T) {
	_, _, err := daemon.ParseNamespace("")
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

func TestParseNamespace_HashOnly(t *testing.T) {
	_, _, err := daemon.ParseNamespace("#RunFile")
	if err == nil {
		t.Fatal("expected error for '#RunFile', got nil")
	}
}

func TestParseNamespace_SubPackage(t *testing.T) {
	pkg, decl, err := daemon.ParseNamespace("ops/lsp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg != "ops/lsp" || decl != "" {
		t.Fatalf("got (%q, %q), want (\"ops/lsp\", \"\")", pkg, decl)
	}
}

func TestParseNamespace_SubPackageWithDecl(t *testing.T) {
	pkg, decl, err := daemon.ParseNamespace("ops/lsp#HandleASTFind")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg != "ops/lsp" || decl != "HandleASTFind" {
		t.Fatalf("got (%q, %q), want (\"ops/lsp\", \"HandleASTFind\")", pkg, decl)
	}
}

// ---- findFileForDecl tests ----

func writeTempGoFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestFindFileForDecl_FuncDecl(t *testing.T) {
	dir := t.TempDir()
	want := writeTempGoFile(t, dir, "runner.go", `package hooks

func RunFile() {}
`)
	srv := daemon.NewTestServer(dir)
	got, err := srv.FindFileForDecl(dir, "RunFile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindFileForDecl_TypeDecl(t *testing.T) {
	dir := t.TempDir()
	want := writeTempGoFile(t, dir, "iface.go", `package hooks

type RunnerInterface interface{}
`)
	srv := daemon.NewTestServer(dir)
	got, err := srv.FindFileForDecl(dir, "RunnerInterface")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindFileForDecl_VarDecl(t *testing.T) {
	dir := t.TempDir()
	want := writeTempGoFile(t, dir, "vars.go", `package hooks

type Runner struct{}
var DefaultRunner Runner
`)
	srv := daemon.NewTestServer(dir)
	got, err := srv.FindFileForDecl(dir, "DefaultRunner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindFileForDecl_ConstDecl(t *testing.T) {
	dir := t.TempDir()
	want := writeTempGoFile(t, dir, "consts.go", `package hooks

const MaxRetries = 5
`)
	srv := daemon.NewTestServer(dir)
	got, err := srv.FindFileForDecl(dir, "MaxRetries")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindFileForDecl_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeTempGoFile(t, dir, "a.go", `package hooks

func SomeFunc() {}
`)
	srv := daemon.NewTestServer(dir)
	_, err := srv.FindFileForDecl(dir, "NonExistent")
	if err == nil {
		t.Fatal("expected error for missing decl, got nil")
	}
}

func TestFindFileForDecl_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	srv := daemon.NewTestServer(dir)
	_, err := srv.FindFileForDecl(dir, "Anything")
	if err == nil {
		t.Fatal("expected error for empty dir, got nil")
	}
}

func TestFindFileForDecl_SkipsBrokenFile(t *testing.T) {
	dir := t.TempDir()
	writeTempGoFile(t, dir, "broken.go", `package hooks
this is not valid go code !!!
`)
	want := writeTempGoFile(t, dir, "good.go", `package hooks

func RunFile() {}
`)
	srv := daemon.NewTestServer(dir)
	got, err := srv.FindFileForDecl(dir, "RunFile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// ---- resolveNamespace tests ----

func marshalArg(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestResolveNamespace_NoNamespaceKey(t *testing.T) {
	dir := t.TempDir()
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"file": "/foo.go"})
	got, err := srv.ResolveNamespace("ast_list", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	if _, ok := m["namespace"]; ok {
		t.Fatal("namespace key should not be present")
	}
	if string(m["file"]) != `"/foo.go"` {
		t.Fatalf("file should be unchanged, got %s", m["file"])
	}
}

func TestResolveNamespace_SkipTool(t *testing.T) {
	dir := t.TempDir()
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks"})
	got, err := srv.ResolveNamespace("file_read", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be returned unchanged
	if string(got) != string(raw) {
		t.Fatalf("expected unchanged args, got %s", got)
	}
}

func TestResolveNamespace_DirTool_PkgOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks"})
	got, err := srv.ResolveNamespace("ast_directory", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	if _, ok := m["namespace"]; ok {
		t.Fatal("namespace should be removed")
	}
	var gotDir string
	json.Unmarshal(m["dir"], &gotDir)
	if gotDir != filepath.Join(dir, "hooks") {
		t.Fatalf("got dir %q, want %q", gotDir, filepath.Join(dir, "hooks"))
	}
}

func TestResolveNamespace_DirTool_WithDecl(t *testing.T) {
	dir := t.TempDir()
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks#Foo"})
	_, err := srv.ResolveNamespace("ast_directory", raw)
	if err == nil {
		t.Fatal("expected error for dir tool with decl name, got nil")
	}
}

func TestResolveNamespace_FileTool_WithDecl(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	os.Mkdir(hooksDir, 0755)
	runnerFile := filepath.Join(hooksDir, "runner.go")
	os.WriteFile(runnerFile, []byte("package hooks\n\nfunc RunFile() {}\n"), 0644)
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks#RunFile"})
	got, err := srv.ResolveNamespace("ast_query", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	var gotFile string
	json.Unmarshal(m["file"], &gotFile)
	if gotFile != runnerFile {
		t.Fatalf("got file %q, want %q", gotFile, runnerFile)
	}
}

func TestResolveNamespace_FileTool_WithoutDecl(t *testing.T) {
	dir := t.TempDir()
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks"})
	_, err := srv.ResolveNamespace("ast_query", raw)
	if err == nil {
		t.Fatal("expected error for file tool without decl, got nil")
	}
}

func TestResolveNamespace_ASTList_WithoutDecl(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "hooks"), 0755)
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks"})
	got, err := srv.ResolveNamespace("ast_list", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	var gotDir string
	json.Unmarshal(m["dir"], &gotDir)
	if gotDir != filepath.Join(dir, "hooks") {
		t.Fatalf("got dir %q, want %q", gotDir, filepath.Join(dir, "hooks"))
	}
}

func TestResolveNamespace_ASTList_WithDecl(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	os.Mkdir(hooksDir, 0755)
	runnerFile := filepath.Join(hooksDir, "runner.go")
	os.WriteFile(runnerFile, []byte("package hooks\n\nfunc RunFile() {}\n"), 0644)
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks#RunFile"})
	got, err := srv.ResolveNamespace("ast_list", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	var gotFile string
	json.Unmarshal(m["file"], &gotFile)
	if gotFile != runnerFile {
		t.Fatalf("got file %q, want %q", gotFile, runnerFile)
	}
}

func TestResolveNamespace_HybridTool_WithDecl(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	os.Mkdir(hooksDir, 0755)
	runnerFile := filepath.Join(hooksDir, "runner.go")
	os.WriteFile(runnerFile, []byte("package hooks\n\nfunc RunFile() {}\n"), 0644)
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks#RunFile"})
	got, err := srv.ResolveNamespace("ast_find", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	var gotFile string
	json.Unmarshal(m["file"], &gotFile)
	if gotFile != runnerFile {
		t.Fatalf("got file %q, want %q", gotFile, runnerFile)
	}
}

func TestResolveNamespace_HybridTool_PkgOnly(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "hooks"), 0755)
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "hooks"})
	got, err := srv.ResolveNamespace("ast_find", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	var gotDir string
	json.Unmarshal(m["dir"], &gotDir)
	if gotDir != filepath.Join(dir, "hooks") {
		t.Fatalf("got dir %q, want %q", gotDir, filepath.Join(dir, "hooks"))
	}
}

func TestResolveNamespace_AbsolutePassthrough(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "hooks")
	os.Mkdir(absPath, 0755)
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": absPath})
	got, err := srv.ResolveNamespace("ast_directory", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	var gotDir string
	json.Unmarshal(m["dir"], &gotDir)
	if gotDir != absPath {
		t.Fatalf("got dir %q, want %q", gotDir, absPath)
	}
}

func TestResolveNamespace_DotPkg(t *testing.T) {
	dir := t.TempDir()
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "."})
	got, err := srv.ResolveNamespace("ast_directory", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	var gotDir string
	json.Unmarshal(m["dir"], &gotDir)
	if gotDir != dir {
		t.Fatalf("got dir %q, want %q", gotDir, dir)
	}
}

// ---- daemon integration tests ----

func TestDaemon_NamespaceRouting_ASTList_Dir(t *testing.T) {
	dir := t.TempDir()
	// write a go file at root
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc Hello() {}\n"), 0644)
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "."})
	resolved, err := srv.ResolveNamespace("ast_list", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(resolved, &m)
	var gotDir string
	json.Unmarshal(m["dir"], &gotDir)
	if gotDir != dir {
		t.Fatalf("got dir %q, want %q", gotDir, dir)
	}
}

func TestDaemon_NamespaceRouting_UnknownPkg(t *testing.T) {
	dir := t.TempDir()
	srv := daemon.NewTestServer(dir)
	raw := marshalArg(t, map[string]string{"namespace": "nosuchpkg"})
	// resolveNamespace for a dir tool injects the dir without validating existence
	// the handler will fail with an os error — just verify no panic
	got, err := srv.ResolveNamespace("ast_directory", raw)
	// either returns error or returns resolved args with non-existent dir
	if err != nil {
		return // acceptable
	}
	var m map[string]json.RawMessage
	json.Unmarshal(got, &m)
	var gotDir string
	json.Unmarshal(m["dir"], &gotDir)
	expected := filepath.Join(dir, "nosuchpkg")
	if gotDir != expected {
		t.Fatalf("got dir %q, want %q", gotDir, expected)
	}
	fmt.Println("resolved to non-existent dir; handler will fail gracefully")
}

func TestFindFileForDecl_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	// production file does NOT have the decl
	os.WriteFile(filepath.Join(dir, "prod.go"), []byte("package pkg\n\nfunc ProdFunc() {}\n"), 0644)
	// test file has the decl — should be skipped
	os.WriteFile(filepath.Join(dir, "prod_test.go"), []byte("package pkg\n\nfunc TestOnlyDecl() {}\n"), 0644)
	srv := daemon.NewTestServer(dir)
	_, err := srv.FindFileForDecl(dir, "TestOnlyDecl")
	if err == nil {
		t.Error("expected error when decl exists only in _test.go file, got nil")
	}
}
