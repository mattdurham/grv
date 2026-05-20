// Namespace: goast/cmd
// Command: convert
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConvertResult holds the parsed output of an ast_directory call.
type ConvertResult struct {
	GoFiles    []GoFileEntry
	NonGoFiles []NonGoFileEntry
	Subdirs    []string
	ReadWrite  int
	ReadOnly   int
}

// BuildConvertResult parses an ast_directory JSON response into a ConvertResult.
func BuildConvertResult(data []byte) (*ConvertResult, error) {
	var raw struct {
		GoFiles    []GoFileEntry    `json:"go_files"`
		NonGoFiles []NonGoFileEntry `json:"non_go_files"`
		Subdirs    []string         `json:"subdirs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	r := &ConvertResult{GoFiles: raw.GoFiles, NonGoFiles: raw.NonGoFiles, Subdirs: raw.Subdirs}
	for _, f := range raw.GoFiles {
		if f.Readonly {
			r.ReadOnly++
		} else {
			r.ReadWrite++
		}
	}
	for _, f := range raw.NonGoFiles {
		if f.Readonly {
			r.ReadOnly++
		} else {
			r.ReadWrite++
		}
	}
	return r, nil
}

// FormatConvertReport renders a ConvertResult as a human-readable string.
func FormatConvertReport(dir string, r *ConvertResult, apply bool) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "grv convert: analyzing %s\n\n", dir)

	fmt.Fprintf(&sb, "GO FILES (%d):\n", len(r.GoFiles))
	for _, f := range r.GoFiles {
		status := "rw"
		if f.Readonly {
			status = "ro"
		}
		fmt.Fprintf(&sb, "  [%s] %s  (pkg: %s, structs: %d, funcs: %d)\n",
			status, f.File, f.Package, len(f.Structs), len(f.Functions))
	}

	fmt.Fprintf(&sb, "\nNON-GO FILES (%d):\n", len(r.NonGoFiles))
	for _, f := range r.NonGoFiles {
		status := "rw"
		if f.Readonly {
			status = "ro"
		}
		fmt.Fprintf(&sb, "  [%s] %s (%d bytes)\n", status, f.File, f.Size)
	}

	if len(r.Subdirs) > 0 {
		fmt.Fprintf(&sb, "\nSUBDIRS: %s\n", strings.Join(r.Subdirs, ", "))
	}

	fmt.Fprintf(&sb, "\nSUMMARY:\n")
	fmt.Fprintf(&sb, "  Read-write: %d files (grv can modify these)\n", r.ReadWrite)
	fmt.Fprintf(&sb, "  Read-only:  %d files (vendor, stdlib, or module cache)\n", r.ReadOnly)

	if apply {
		fmt.Fprintf(&sb, "\nAPPLY MODE: no transforms defined yet.\n")
		fmt.Fprintf(&sb, "Future: transforms queued via grv ast_insert/ast_replace --dry_run will be applied here.\n")
	} else {
		fmt.Fprintf(&sb, "\nRun `grv convert %s --apply` to apply transforms (when available).\n", dir)
	}
	return sb.String()
}

// RunConvert analyzes an existing codebase directory and produces a
// conversion report showing which files can be read/written by grv.
//
// Usage:
//
//	grv convert [dir]           — report mode: show what grv sees
//	grv convert [dir] --apply  — apply mode: (future) apply pending transforms
func RunConvert(dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: %v\n", err)
		os.Exit(1)
	}

	apply := false
	for _, arg := range os.Args {
		if arg == "--apply" {
			apply = true
		}
	}

	sockPath, err := resolveSock(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: cannot connect to daemon: %v\n", err)
		os.Exit(1)
	}

	argsBytes, _ := json.Marshal(map[string]interface{}{
		"dir":       abs,
		"recursive": false,
	})

	resp, err := SendRequest(sockPath, "ast_directory", argsBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: ast_directory: %v\n", err)
		os.Exit(1)
	}

	result, err := BuildConvertResult(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: parse response: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(FormatConvertReport(abs, result, apply))
}

// GoFileEntry mirrors the ops.GoFileEntry JSON shape for unmarshalling.
type GoFileEntry struct {
	File      string        `json:"file"`
	Readonly  bool          `json:"readonly"`
	Package   string        `json:"package"`
	Structs   []interface{} `json:"structs"`
	Functions []interface{} `json:"functions"`
}

// NonGoFileEntry mirrors ops.NonGoFileEntry for unmarshalling.
type NonGoFileEntry struct {
	File     string `json:"file"`
	Size     int64  `json:"size"`
	Readonly bool   `json:"readonly"`
}

// resolveSock finds the daemon socket for the given directory.
func resolveSock(dir string) (string, error) {
	grvDir, err := GRVDir()
	if err != nil {
		return "", err
	}
	hash := HashDir(dir)
	sock := SockPath(grvDir, hash)
	// Auto-start daemon if not running
	if err := StartDaemon(dir); err != nil {
		return "", fmt.Errorf("start daemon: %w", err)
	}
	return sock, nil
}
