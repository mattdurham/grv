package ops_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattdurham/grv/hooks"
	"github.com/mattdurham/grv/ops"
)

// writeTempPkg writes a Go source file into a temp directory with a valid
// go.mod so packages.Load can resolve it. Returns the file path.
func writeTempPkg(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	gomod := "module testpkg\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "x.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runTypeRule(t *testing.T, src, rule string) []ops.Violation {
	t.Helper()
	path := writeTempPkg(t, src)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{rule}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })
	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatal(err)
	}
	var v []ops.Violation
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestErrorNotChecked_Fires(t *testing.T) {
	// os.Remove returns error — discarded entirely with no assignment.
	v := runTypeRule(t, `package testpkg

import "os"

func f() {
	os.Remove("/tmp/x")
}
`, "error_not_checked")
	if len(v) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(v), v)
	}
	if v[0].Rule != "error_not_checked" {
		t.Errorf("wrong rule: %q", v[0].Rule)
	}
}

func TestErrorNotChecked_PassesWhenAssigned(t *testing.T) {
	// Error assigned to named var — not flagged.
	v := runTypeRule(t, `package testpkg

import "os"

func f() error {
	err := os.Remove("/tmp/x")
	return err
}
`, "error_not_checked")
	if len(v) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(v), v)
	}
}

func TestErrorNotChecked_PassesWhenBlankWithComment(t *testing.T) {
	// Blank assignment with comment — suppressed.
	v := runTypeRule(t, `package testpkg

import "os"

func f() {
	_ = os.Remove("/tmp/x") // best-effort cleanup
}
`, "error_not_checked")
	if len(v) != 0 {
		t.Errorf("expected 0 violations (comment suppresses), got %d", len(v))
	}
}

func TestErrorNotChecked_PassesNoErrorReturn(t *testing.T) {
	// fmt.Println returns (int, error) — but this tests a non-error-returning call.
	v := runTypeRule(t, `package testpkg

func noErr() int { return 1 }

func f() {
	noErr()
}
`, "error_not_checked")
	if len(v) != 0 {
		t.Errorf("expected 0 violations for non-error return, got %d", len(v))
	}
}

func TestErrorNotChecked_PassesWithComment(t *testing.T) {
	// Same-line comment suppresses the rule.
	v := runTypeRule(t, `package testpkg

import "os"

func f() {
	os.Remove("/tmp/x") // intentional: best-effort cleanup
}
`, "error_not_checked")
	if len(v) != 0 {
		t.Errorf("expected 0 violations (comment suppresses), got %d", len(v))
	}
}
