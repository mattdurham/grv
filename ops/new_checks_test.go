package ops_test

import (
	"encoding/json"
	"testing"

	"github.com/mattdurham/grv/hooks"
	"github.com/mattdurham/grv/ops"
)

func checkViolations(t *testing.T, src string, rule string) []ops.Violation {
	t.Helper()
	path := writeTempBatch(t, src)
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{
		Enforce: []string{rule},
	})
	t.Cleanup(func() {
		ops.SetDefaultChecksConfig(hooks.ChecksConfig{})
	})
	raw, err := ops.HandleASTCheck(ops.ASTCheckArgs{File: path})
	if err != nil {
		t.Fatalf("HandleASTCheck: %v", err)
	}
	var violations []ops.Violation
	if err := json.Unmarshal(raw, &violations); err != nil {
		t.Fatalf("unmarshal violations: %v", err)
	}
	return violations
}

func assertFires(t *testing.T, src string, rule string) {
	t.Helper()
	v := checkViolations(t, src, rule)
	if len(v) == 0 {
		t.Errorf("expected %s to fire, got no violations", rule)
	}
}

func assertPasses(t *testing.T, src string, rule string) {
	t.Helper()
	v := checkViolations(t, src, rule)
	for _, vi := range v {
		if vi.Rule == rule {
			t.Errorf("expected %s to pass, but got violation: %+v", rule, vi)
		}
	}
}

// type_assertion_not_checked

func TestASTCheck_TypeAssertionNotChecked_Fires_SingleLHS(t *testing.T) {
	assertFires(t, `package x
func f(i interface{}) {
	t := i.(string)
	_ = t
}
`, "type_assertion_not_checked")
}

func TestASTCheck_TypeAssertionNotChecked_Passes_CommaOk(t *testing.T) {
	assertPasses(t, `package x
func f(i interface{}) {
	t, ok := i.(string)
	_, _ = t, ok
}
`, "type_assertion_not_checked")
}

func TestASTCheck_TypeAssertionNotChecked_Passes_TypeSwitch(t *testing.T) {
	assertPasses(t, `package x
func f(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	}
	return ""
}
`, "type_assertion_not_checked")
}

func TestASTCheck_TypeAssertionNotChecked_Fires_ExprStmt(t *testing.T) {
	assertFires(t, `package x
func f(i interface{}) {
	_ = i.(string)
}
`, "type_assertion_not_checked")
}

// mutex_not_embedded

func TestASTCheck_MutexNotEmbedded_Fires_AnonymousMutex(t *testing.T) {
	assertFires(t, `package x
import "sync"
type S struct {
	sync.Mutex
	val int
}
`, "mutex_not_embedded")
}

func TestASTCheck_MutexNotEmbedded_Fires_AnonymousPointerMutex(t *testing.T) {
	assertFires(t, `package x
import "sync"
type S struct {
	*sync.Mutex
	val int
}
`, "mutex_not_embedded")
}

func TestASTCheck_MutexNotEmbedded_Passes_NamedField(t *testing.T) {
	assertPasses(t, `package x
import "sync"
type S struct {
	mu  sync.Mutex
	val int
}
`, "mutex_not_embedded")
}

// channel_size_not_one_or_zero

func TestASTCheck_ChannelSizeNotOneOrZero_Fires_LargeBuffer(t *testing.T) {
	assertFires(t, `package x
func f() {
	ch := make(chan int, 64)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestASTCheck_ChannelSizeNotOneOrZero_Passes_WithComment(t *testing.T) {
	assertPasses(t, `package x
func f() {
	ch := make(chan int, 64) // worker pool
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestASTCheck_ChannelSizeNotOneOrZero_Passes_SizeOne(t *testing.T) {
	assertPasses(t, `package x
func f() {
	ch := make(chan int, 1)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestASTCheck_ChannelSizeNotOneOrZero_Passes_SizeZero(t *testing.T) {
	assertPasses(t, `package x
func f() {
	ch := make(chan int, 0)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}

func TestASTCheck_ChannelSizeNotOneOrZero_Passes_NonLiteral(t *testing.T) {
	assertPasses(t, `package x
const someConst = 64
func f() {
	ch := make(chan int, someConst)
	_ = ch
}
`, "channel_size_not_one_or_zero")
}
