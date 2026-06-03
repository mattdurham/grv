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

// ---- type_assertion_not_checked ----

func checkViolations(t *testing.T, src, rule string, wantCount int) []ops.Violation {
	t.Helper()
	path := writeTemp(t, src)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{rule}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })
	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatal(err)
	}
	var violations []ops.Violation
	if err := json.Unmarshal(raw, &violations); err != nil {
		t.Fatal(err)
	}
	if len(violations) != wantCount {
		t.Errorf("expected %d violation(s), got %d: %+v", wantCount, len(violations), violations)
	}
	return violations
}

func TestTypeAssertionNotChecked_SingleLHSFires(t *testing.T) {
	// v := x.(string) — unchecked, must fire.
	checkViolations(t, `package x
func f(v interface{}) string {
	s := v.(string)
	return s
}
`, "type_assertion_not_checked", 1)
}

func TestTypeAssertionNotChecked_OkFormPasses(t *testing.T) {
	// v, ok := x.(string) — checked, must not fire.
	checkViolations(t, `package x
func f(v interface{}) (string, bool) {
	s, ok := v.(string)
	return s, ok
}
`, "type_assertion_not_checked", 0)
}

func TestTypeAssertionNotChecked_ExprStmtFires(t *testing.T) {
	// x.(string) as a bare statement always panics on mismatch — must fire.
	checkViolations(t, `package x
func f(v interface{}) {
	v.(string)
}
`, "type_assertion_not_checked", 1)
}

func TestTypeAssertionNotChecked_TypeSwitchAssignPasses(t *testing.T) {
	// switch x := v.(type) — TypeAssertExpr with nil Type, must never fire.
	checkViolations(t, `package x
func f(v interface{}) {
	switch x := v.(type) {
	case string:
		_ = x
	}
}
`, "type_assertion_not_checked", 0)
}

func TestTypeAssertionNotChecked_TypeSwitchNoAssignPasses(t *testing.T) {
	// switch v.(type) without assignment — must not fire.
	checkViolations(t, `package x
func f(v interface{}) {
	switch v.(type) {
	case string:
	}
}
`, "type_assertion_not_checked", 0)
}

func TestTypeAssertionNotChecked_BlankAssignFires(t *testing.T) {
	// _ = v.(string) — single LHS blank, still unchecked, must fire.
	checkViolations(t, `package x
func f(v interface{}) {
	_ = v.(string)
}
`, "type_assertion_not_checked", 1)
}

func TestTypeAssertionNotChecked_CommentSuppresses(t *testing.T) {
	checkViolations(t, `package x
func f(v interface{}) string {
	s := v.(string) // safe: caller guarantees type
	return s
}
`, "type_assertion_not_checked", 0)
}

// ---- mutex_not_embedded ----

func TestMutexNotEmbedded_ValueFires(t *testing.T) {
	checkViolations(t, `package x
import "sync"
type S struct {
	sync.Mutex
}
`, "mutex_not_embedded", 1)
}

func TestMutexNotEmbedded_PointerFires(t *testing.T) {
	// *sync.Mutex embedded — same API-leak problem, must fire.
	checkViolations(t, `package x
import "sync"
type S struct {
	*sync.Mutex
}
`, "mutex_not_embedded", 1)
}

func TestMutexNotEmbedded_RWMutexFires(t *testing.T) {
	checkViolations(t, `package x
import "sync"
type S struct {
	sync.RWMutex
}
`, "mutex_not_embedded", 1)
}

func TestMutexNotEmbedded_NamedFieldPasses(t *testing.T) {
	// Named field mu sync.Mutex — not embedded, must not fire.
	checkViolations(t, `package x
import "sync"
type S struct {
	mu sync.Mutex
}
`, "mutex_not_embedded", 0)
}

func TestMutexNotEmbedded_OtherEmbedPasses(t *testing.T) {
	// Embedding a non-mutex type must not fire.
	checkViolations(t, `package x
import "sync"
type S struct {
	sync.WaitGroup
}
`, "mutex_not_embedded", 0)
}

func TestMutexNotEmbedded_CommentSuppresses(t *testing.T) {
	checkViolations(t, `package x
import "sync"
type S struct {
	sync.Mutex // intentional: S exposes Lock for external coordination
}
`, "mutex_not_embedded", 0)
}

// ---- channel_size_not_one_or_zero ----

func TestChannelSizeNotOneOrZero_ZeroPasses(t *testing.T) {
	checkViolations(t, `package x
func f() {
	ch := make(chan int, 0)
	_ = ch
}
`, "channel_size_not_one_or_zero", 0)
}

func TestChannelSizeNotOneOrZero_OnePasses(t *testing.T) {
	checkViolations(t, `package x
func f() {
	ch := make(chan int, 1)
	_ = ch
}
`, "channel_size_not_one_or_zero", 0)
}

func TestChannelSizeNotOneOrZero_TwoFires(t *testing.T) {
	checkViolations(t, `package x
func f() {
	ch := make(chan int, 2)
	_ = ch
}
`, "channel_size_not_one_or_zero", 1)
}

func TestChannelSizeNotOneOrZero_LargeFires(t *testing.T) {
	checkViolations(t, `package x
func f() {
	ch := make(chan int, 100)
	_ = ch
}
`, "channel_size_not_one_or_zero", 1)
}

func TestChannelSizeNotOneOrZero_NamedConstPasses(t *testing.T) {
	// Non-literal size — cannot evaluate statically, must pass silently.
	checkViolations(t, `package x
const bufSize = 10
func f() {
	ch := make(chan int, bufSize)
	_ = ch
}
`, "channel_size_not_one_or_zero", 0)
}

func TestChannelSizeNotOneOrZero_UnbufferedPasses(t *testing.T) {
	// make(chan int) with no size arg — must not fire.
	checkViolations(t, `package x
func f() {
	ch := make(chan int)
	_ = ch
}
`, "channel_size_not_one_or_zero", 0)
}

func TestChannelSizeNotOneOrZero_CommentSuppresses(t *testing.T) {
	checkViolations(t, `package x
func f() {
	ch := make(chan int, 10) // intentional: pre-buffered for batch processing
	_ = ch
}
`, "channel_size_not_one_or_zero", 0)
}

func TestChannelSizeNotOneOrZero_NonChanMakePasses(t *testing.T) {
	// make([]int, 10) — not a channel, must not fire.
	checkViolations(t, `package x
func f() {
	s := make([]int, 10)
	_ = s
}
`, "channel_size_not_one_or_zero", 0)
}
