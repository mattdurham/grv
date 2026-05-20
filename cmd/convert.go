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

// RunConvert analyzes an existing codebase directory and produces a
// conversion report showing which files can be read/written by grv,
// which are readonly (vendor, stdlib, module cache), and a summary
// of all Go symbols found. Writes non-readonly files using grv write
// tools when --apply is passed.
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

	// Determine apply mode
	apply := false
	for _, arg := range os.Args {
		if arg == "--apply" {
			apply = true
		}
	}

	fmt.Printf("grv convert: analyzing %s\n\n", abs)

	// 1. Get directory inventory via the daemon
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

	// Parse response
	var result struct {
		GoFiles    []GoFileEntry    `json:"go_files"`
		NonGoFiles []NonGoFileEntry `json:"non_go_files"`
		Subdirs    []string         `json:"subdirs"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: parse response: %v\n", err)
		os.Exit(1)
	}

	// 2. Report
	readwrite := 0
	readonly := 0

	fmt.Printf("GO FILES (%d):\n", len(result.GoFiles))
	for _, f := range result.GoFiles {
		status := "rw"
		if f.Readonly {
			status = "ro"
			readonly++
		} else {
			readwrite++
		}
		structs := len(f.Structs)
		funcs := len(f.Functions)
		fmt.Printf("  [%s] %s  (pkg: %s, structs: %d, funcs: %d)\n",
			status, f.File, f.Package, structs, funcs)
	}

	fmt.Printf("\nNON-GO FILES (%d):\n", len(result.NonGoFiles))
	for _, f := range result.NonGoFiles {
		status := "rw"
		if f.Readonly {
			status = "ro"
			readonly++
		} else {
			readwrite++
		}
		fmt.Printf("  [%s] %s (%d bytes)\n", status, f.File, f.Size)
	}

	if len(result.Subdirs) > 0 {
		fmt.Printf("\nSUBDIRS: %s\n", strings.Join(result.Subdirs, ", "))
	}

	fmt.Printf("\nSUMMARY:\n")
	fmt.Printf("  Read-write: %d files (grv can modify these)\n", readwrite)
	fmt.Printf("  Read-only:  %d files (vendor, stdlib, or module cache)\n", readonly)

	if apply {
		fmt.Printf("\nAPPLY MODE: no transforms defined yet.\n")
		fmt.Printf("Future: grv convert will apply pending refactors, renames, and insertions\n")
		fmt.Printf("        that have been queued via grv ast_insert/ast_replace with --dry_run.\n")
	} else {
		fmt.Printf("\nRun `grv convert %s --apply` to apply transforms (when available).\n", abs)
	}
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
