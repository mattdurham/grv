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
	// Binary content check: read first 512 bytes, look for null bytes.
	// Files with null bytes are binary and should not be rewritten by grv.
	if isBinaryFile(abs) {
		return true
	}
	return false
}

// isBinaryFile returns true if the file contains null bytes in its first 512 bytes.
func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}
