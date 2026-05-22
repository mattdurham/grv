// Namespace: goast/ops
// Hook runner injection — package-level runner set by daemon at startup.
package ops

import (
	"strings"

	"github.com/mattdurham/grv/meta"
)

// RunnerInterface is the dependency injection interface for the hooks package.
// Defined here (not in hooks/) to avoid the ops↔hooks import cycle.
// hooks.Runner satisfies this interface via Go's structural typing.
type RunnerInterface interface {
	RunFile(absFile string) map[string]interface{}
	Invalidate(absFile string)
}

// DefaultHookRunner is set once by the daemon at startup. Nil = no hooks.
var DefaultHookRunner RunnerInterface

// SetDefaultHookRunner sets the global hook runner. Called by daemon on startup.
func SetDefaultHookRunner(r RunnerInterface) {
	DefaultHookRunner = r
}

// mergeHookMeta merges hook results for absFile into m.
// If allowlist is non-empty, only hooks whose name prefix matches an entry are included.
// Returns m unchanged if DefaultHookRunner is nil or RunFile returns empty.
func mergeHookMeta(m meta.Meta, absFile string, allowlist []string) meta.Meta {
	if DefaultHookRunner == nil {
		return m
	}
	hookResult := DefaultHookRunner.RunFile(absFile)
	for k, v := range hookResult {
		if len(allowlist) > 0 && !inAllowlist(k, allowlist) {
			continue
		}
		m[k] = v
	}
	return m
}

// inAllowlist checks whether the key's hook-name prefix (part before first ".") is in the list.
func inAllowlist(key string, list []string) bool {
	dot := strings.IndexByte(key, '.')
	prefix := key
	if dot >= 0 {
		prefix = key[:dot]
	}
	for _, name := range list {
		if name == prefix {
			return true
		}
	}
	return false
}
