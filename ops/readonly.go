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
	if err == nil && info.Mode()&0200 == 0 {
		return true
	}
	return false
}
