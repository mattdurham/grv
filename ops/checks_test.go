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

func runRule(t *testing.T, src, rule string) []ops.Violation {
	t.Helper()
	path := writeTemp(t, src)
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

func assertFires(t *testing.T, src, rule string) {
	t.Helper()
	if v := runRule(t, src, rule); len(v) == 0 {
		t.Errorf("expected at least 1 violation for rule %q, got 0", rule)
	}
}

func assertPasses(t *testing.T, src, rule string) {
	t.Helper()
	if v := runRule(t, src, rule); len(v) != 0 {
		t.Errorf("expected 0 violations for rule %q, got %d: %+v", rule, len(v), v)
	}
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

func TestTypeAssertionNotChecked_OkFormPasses(t *testing.T) {
	assertPasses(t, `package x
func f(v interface{}) (string, bool) {
	s, ok := v.(string)
	return s, ok
}
`, "type_assertion_not_checked")
}

func TestTypeAssertionNotChecked_TypeSwitchAssignPasses(t *testing.T) {
	// switch x := v.(type) — TypeAssertExpr with nil Type, must never fire.
	assertPasses(t, `package x
func f(v interface{}) {
	switch x := v.(type) {
	case string:
		_ = x
	}
}
`, "type_assertion_not_checked")
}

func TestTypeAssertionNotChecked_TypeSwitchNoAssignPasses(t *testing.T) {
	assertPasses(t, `package x
func f(v interface{}) {
	switch v.(type) {
	case string:
	}
}
`, "type_assertion_not_checked")
}

func TestTypeAssertionNotChecked_BlankAssignFires(t *testing.T) {
	// _ = v.(string) — single LHS blank, still unchecked, must fire.
	assertFires(t, `package x
func f(v interface{}) {
	_ = v.(string)
}
`, "type_assertion_not_checked")
}

func TestTypeAssertionNotChecked_CommentSuppresses(t *testing.T) {
	assertPasses(t, `package x
func f(v interface{}) string {
	s := v.(string) // safe: caller guarantees type
	return s
}
`, "type_assertion_not_checked")
}

// ---- mutex_not_embedded ----

func TestMutexNotEmbedded_RWMutexFires(t *testing.T) {
	assertFires(t, `package x
import "sync"
type S struct {
	sync.RWMutex
}
`, "mutex_not_embedded")
}

func TestMutexNotEmbedded_OtherEmbedPasses(t *testing.T) {
	// Embedding a non-mutex type must not fire.
	assertPasses(t, `package x
import "sync"
type S struct {
	sync.WaitGroup
}
`, "mutex_not_embedded")
}

func TestMutexNotEmbedded_CommentSuppresses(t *testing.T) {
	assertPasses(t, `package x
import "sync"
type S struct {
	sync.Mutex // intentional: S exposes Lock for external coordination
}
`, "mutex_not_embedded")
}

// ---- channel_size_not_one_or_zero ----

func TestChannelSizeNotOneOrZero_ZeroPasses(t *testing.T) {
	assertPasses(t, `package x
func f() {
	ch := make(chan int, 0)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestChannelSizeNotOneOrZero_OnePasses(t *testing.T) {
	assertPasses(t, `package x
func f() {
	ch := make(chan int, 1)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestChannelSizeNotOneOrZero_TwoFires(t *testing.T) {
	assertFires(t, `package x
func f() {
	ch := make(chan int, 2)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestChannelSizeNotOneOrZero_NamedConstPasses(t *testing.T) {
	// Non-literal size — cannot evaluate statically, must pass silently.
	assertPasses(t, `package x
const bufSize = 10
func f() {
	ch := make(chan int, bufSize)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestChannelSizeNotOneOrZero_UnbufferedPasses(t *testing.T) {
	// make(chan int) with no size arg — must not fire.
	assertPasses(t, `package x
func f() {
	ch := make(chan int)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestChannelSizeNotOneOrZero_NonChanMakePasses(t *testing.T) {
	// make([]int, 10) — not a channel, must not fire.
	assertPasses(t, `package x
func f() {
	s := make([]int, 10)
	_ = s
}
`, "channel_size_not_one_or_zero")
}

// ---- map_without_size_hint ----

func TestMapWithoutSizeHint_Fires(t *testing.T) {
	assertFires(t, `package x
func f() {
	m := make(map[string]int)
	_ = m
}
`, "map_without_size_hint")
}

func TestMapWithoutSizeHint_PassesExplicitZero(t *testing.T) {
	assertPasses(t, `package x
func f() {
	m := make(map[string]int, 0) // unknown size
	_ = m
}
`, "map_without_size_hint")
}

func TestMapWithoutSizeHint_PassesNonZeroHint(t *testing.T) {
	assertPasses(t, `package x
func f() {
	m := make(map[string]int, 100)
	_ = m
}
`, "map_without_size_hint")
}

// ---- slice_without_capacity ----

func TestSliceWithoutCapacity_Fires(t *testing.T) {
	assertFires(t, `package x
func f() {
	s := make([]int, 0)
	_ = s
}
`, "slice_without_capacity")
}

func TestSliceWithoutCapacity_FiresNonZeroLength(t *testing.T) {
	assertFires(t, `package x
func f() {
	s := make([]int, 10)
	_ = s
}
`, "slice_without_capacity")
}

func TestSliceWithoutCapacity_PassesExplicitZeroCap(t *testing.T) {
	assertPasses(t, `package x
func f() {
	s := make([]int, 0, 0) // unknown capacity
	_ = s
}
`, "slice_without_capacity")
}

func TestSliceWithoutCapacity_PassesNonZeroCap(t *testing.T) {
	assertPasses(t, `package x
func f() {
	s := make([]int, 0, 100)
	_ = s
}
`, "slice_without_capacity")
}

func TestSliceWithoutCapacity_PassesVarDecl(t *testing.T) {
	// var s []int is the preferred form and must never fire
	assertPasses(t, `package x
func f() {
	var s []int
	_ = s
}
`, "slice_without_capacity")
}
