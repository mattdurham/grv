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

// binaryExtensions is the set of file extensions that are always treated as binary.
// These formats may not contain null bytes in their headers but must not be rewritten.
var binaryExtensions = map[string]bool{
	// Images
	".gif": true, ".png": true, ".jpg": true, ".jpeg": true,
	".webp": true, ".ico": true, ".bmp": true, ".tiff": true, ".svg": false, // svg is XML text
	// Video / audio
	".mp4": true, ".webm": true, ".mkv": true, ".avi": true,
	".mp3": true, ".ogg": true, ".wav": true, ".flac": true,
	// Archives / compressed
	".gz": true, ".zst": true, ".zstd": true, ".bz2": true, ".xz": true,
	".zip": true, ".tar": true, ".tgz": true, ".7z": true, ".rar": true,
	// Data / serialised formats
	".parquet": true, ".avro": true, ".orc": true, ".arrow": true,
	".pb": true, ".proto": false, // .proto is text; .pb is binary protobuf
	".wasm": true,
	// Compiled / object
	".o": true, ".a": true, ".so": true, ".dylib": true, ".dll": true, ".exe": true,
	// Documents / fonts
	".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
	// Misc binary
	".cast": true, // asciinema cast files are actually JSON — treat as text
}

// isBinaryFile returns true if the file is binary: known binary extension,
// null bytes in the first 8KB, or a recognised binary magic signature.
func isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if binary, ok := binaryExtensions[ext]; ok {
		return binary
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Read up to 8KB — large enough to catch formats with non-null headers
	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	buf = buf[:n]

	// Null byte = binary
	for _, b := range buf {
		if b == 0 {
			return true
		}
	}

	// Check for high proportion of non-UTF8 / non-printable bytes (>30% → binary)
	nonText := 0
	for _, b := range buf {
		if b < 9 || (b > 13 && b < 32) || b == 127 {
			nonText++
		}
	}
	if nonText*100/n > 30 {
		return true
	}

	return false
}
