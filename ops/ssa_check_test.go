package ops_test

import (
	"testing"

	"github.com/mattdurham/grv/hooks"
	"github.com/mattdurham/grv/ops"
)

func TestNilDereference_DirectNilFires(t *testing.T) {
	// Directly dereferencing a nil pointer — statically provable.
	v := runTypeRule(t, `package testpkg

func f() string {
	var p *string
	return *p
}
`, "nil_dereference")
	if len(v) == 0 {
		t.Error("expected violation for direct nil dereference, got 0")
	}
}

func TestNilDereference_GuardedPasses(t *testing.T) {
	// Nil check before dereference — must not fire.
	v := runTypeRule(t, `package testpkg

func f(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
`, "nil_dereference")
	if len(v) != 0 {
		t.Errorf("expected 0 violations for guarded deref, got %d: %+v", len(v), v)
	}
}

func TestNilDereference_NilReturnUsedFires(t *testing.T) {
	// getPtr() can return nil; caller dereferences without nil check.
	v := runTypeRule(t, `package testpkg

func getPtr() *string { return nil }

func f() string {
	p := getPtr()
	return *p
}
`, "nil_dereference")
	if len(v) == 0 {
		t.Error("expected violation: callee returns nil and result is dereferenced unchecked")
	}
}

func TestNilDereference_NilReturnGuardedPasses(t *testing.T) {
	// getPtr() can return nil but caller checks before dereferencing.
	v := runTypeRule(t, `package testpkg

func getPtr() *string { return nil }

func f() string {
	p := getPtr()
	if p == nil {
		return ""
	}
	return *p
}
`, "nil_dereference")
	if len(v) != 0 {
		t.Errorf("expected 0 violations (guarded), got %d: %+v", len(v), v)
	}
}

func TestNilDereference_NonNilReturnPasses(t *testing.T) {
	// getPtr() never returns nil — must not fire.
	v := runTypeRule(t, `package testpkg

func getPtr() *string {
	s := "hello"
	return &s
}

func f() string {
	p := getPtr()
	return *p
}
`, "nil_dereference")
	if len(v) != 0 {
		t.Errorf("expected 0 violations (non-nil return), got %d: %+v", len(v), v)
	}
}

func TestNilDereference_FieldAccessFires(t *testing.T) {
	// Field access on a nil pointer — must fire.
	v := runTypeRule(t, `package testpkg

type S struct{ X int }

func f() int {
	var s *S
	return s.X
}
`, "nil_dereference")
	if len(v) == 0 {
		t.Error("expected violation for field access on nil pointer")
	}
}

func TestNilDereference_AllRuleEnabledByAll(t *testing.T) {
	ops.SetDefaultChecksConfig(hooks.ChecksConfig{Enforce: []string{"all"}})
	t.Cleanup(func() { ops.SetDefaultChecksConfig(hooks.ChecksConfig{}) })

	// "all" should include nil_dereference — same fire case.
	v := runTypeRule(t, `package testpkg

func f() string {
	var p *string
	return *p
}
`, "nil_dereference")
	if len(v) == 0 {
		t.Error("expected nil_dereference to be included in 'all'")
	}
}
