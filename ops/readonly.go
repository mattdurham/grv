// Namespace: goast/ops
// Shared readonly detection helper.
package ops

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// isReadonly reports whether filePath is in a readonly location
// (vendor/, stdlib, module cache, or has no write permission).
func isReadonly(filePath string) bool {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	if strings.Contains(abs, "/vendor/") {
		return true
	}
	if strings.HasPrefix(abs, runtime.GOROOT()) {
		return true
	}
	gomod := os.Getenv("GOMODCACHE")
	if gomod == "" {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			// Default GOPATH when unset: ~/go
			if home, err := os.UserHomeDir(); err == nil {
				gopath = filepath.Join(home, "go")
			}
		}
		if gopath != "" {
			gomod = filepath.Join(gopath, "pkg", "mod")
		}
	}
	if gomod != "" && strings.HasPrefix(abs, gomod) {
		return true
	}
	info, err := os.Stat(abs)
	if err != nil {
		return false
	}
	// Filesystem read-only
	if info.Mode()&0200 == 0 {
		return true
	}
	// Executable bit set — treat as binary (compiled artifact, shell script, etc.)
	if info.Mode()&0111 != 0 {
		return true
	}
	return false
}

// IsConvertTarget returns true if grv convert should process this file.
// grv convert only understands Go source (.go) and go.mod.
// Everything else — including go.sum, YAML, JSON, binaries — is skipped.
func IsConvertTarget(path string) bool {
	base := filepath.Base(path)
	if base == "go.mod" {
		return true
	}
	return strings.ToLower(filepath.Ext(path)) == ".go"
}
