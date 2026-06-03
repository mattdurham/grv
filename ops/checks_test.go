package ops_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattdurham/grv/hooks"
	"github.com/mattdurham/grv/ops"
)

// writeTemp writes content to a temp .go file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestASTCheck_ErrorHandled_Fires(t *testing.T) {
	path := writeTemp(t, `package x

import "fmt"

func f() {
	n, _ := fmt.Println("hi")
	_ = n
}
`)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{"error_handled"}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })

	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatal(err)
	}
	var violations []ops.Violation
	if err := json.Unmarshal(raw, &violations); err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Rule != "error_handled" {
		t.Errorf("rule: want error_handled, got %q", violations[0].Rule)
	}
}

func TestASTCheck_ErrorHandled_PassesWithComment(t *testing.T) {
	path := writeTemp(t, `package x

import "fmt"

func f() {
	n, _ := fmt.Println("hi") // intentional: stderr is best-effort
	_ = n
}
`)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{"error_handled"}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })

	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatal(err)
	}
	var violations []ops.Violation
	if err := json.Unmarshal(raw, &violations); err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestASTCheck_ErrorHandled_PassesWhenHandled(t *testing.T) {
	path := writeTemp(t, `package x

import "fmt"

func f() error {
	n, err := fmt.Println("hi")
	_ = n
	return err
}
`)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{"error_handled"}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })

	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatal(err)
	}
	var violations []ops.Violation
	if err := json.Unmarshal(raw, &violations); err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations (err is used), got %d", len(violations))
	}
}

func TestASTCheck_ErrorHandled_NoFalsePositiveMapIndex(t *testing.T) {
	// Map index uses IndexExpr on RHS — not a CallExpr, must never fire.
	path := writeTemp(t, `package x

func f() {
	m := map[string]int{"a": 1}
	_, ok := m["a"]
	_ = ok
}
`)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{"error_handled"}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })

	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatal(err)
	}
	var violations []ops.Violation
	if err := json.Unmarshal(raw, &violations); err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("map index must not trigger error_handled, got %d violations", len(violations))
	}
}

func TestASTCheck_AllEnablesErrorHandled(t *testing.T) {
	path := writeTemp(t, `package x

import "fmt"

func f() {
	n, _ := fmt.Println("hi")
	_ = n
}
`)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{"all"}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })

	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatal(err)
	}
	var violations []ops.Violation
	if err := json.Unmarshal(raw, &violations); err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		t.Error("expected violations with enforce=[all]")
	}
}

func TestASTCheck_EmptyEnforce_NoViolations(t *testing.T) {
	path := writeTemp(t, `package x

import "fmt"

func f() {
	n, _ := fmt.Println("hi")
	_ = n
}
`)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })

	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatal(err)
	}
	var violations []ops.Violation
	if err := json.Unmarshal(raw, &violations); err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("empty enforce must return no violations, got %d", len(violations))
	}
}
