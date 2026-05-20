// Namespace: goast/cmd
// Command: convert
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
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

// FileConvertResult records what happened to a single file during conversion.
type FileConvertResult struct {
	File    string
	Changed bool
	Error   string
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

// FormatConvertReport renders an inventory summary without conversion results.
func FormatConvertReport(dir string, r *ConvertResult, _ bool) string {
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
	fmt.Fprintf(&sb, "  Read-write: %d files (will be converted)\n", r.ReadWrite)
	fmt.Fprintf(&sb, "  Read-only:  %d files (skipped)\n", r.ReadOnly)
	return sb.String()
}

// convertGoFile parses a Go file into AST, formats it via go/format, deletes
// the original, and rewrites it. Returns whether content changed.
func convertGoFile(sockPath, dir, filename string) FileConvertResult {
	fullPath := filepath.Join(dir, filename)

	// Read current content via daemon
	readArgs, _ := json.Marshal(map[string]string{"file": fullPath})
	resp, err := SendRequest(sockPath, "file_read", readArgs)
	if err != nil {
		return FileConvertResult{File: filename, Error: err.Error()}
	}
	var readResult struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(resp, &readResult); err != nil {
		return FileConvertResult{File: filename, Error: err.Error()}
	}
	original := readResult.Content

	// Parse through go/ast
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fullPath, original, parser.ParseComments)
	if err != nil {
		return FileConvertResult{File: filename, Error: fmt.Sprintf("parse: %v", err)}
	}

	// Format via go/format (AST → canonical source)
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return FileConvertResult{File: filename, Error: fmt.Sprintf("format: %v", err)}
	}
	formatted := buf.String()

	if formatted == original {
		return FileConvertResult{File: filename, Changed: false}
	}

	// Delete original and rewrite via daemon
	writeArgs, _ := json.Marshal(map[string]interface{}{
		"file":    fullPath,
		"content": formatted,
		"dry_run": false,
	})
	if _, err := SendRequest(sockPath, "file_write", writeArgs); err != nil {
		return FileConvertResult{File: filename, Error: fmt.Sprintf("write: %v", err)}
	}
	return FileConvertResult{File: filename, Changed: true}
}

// convertNonGoFile reads and rewrites a non-Go file via the daemon.
// This validates daemon access and normalises any platform line-ending issues.
func convertNonGoFile(sockPath, dir, filename string) FileConvertResult {
	fullPath := filepath.Join(dir, filename)

	readArgs, _ := json.Marshal(map[string]string{"file": fullPath})
	resp, err := SendRequest(sockPath, "file_read", readArgs)
	if err != nil {
		return FileConvertResult{File: filename, Error: err.Error()}
	}
	var readResult struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(resp, &readResult); err != nil {
		return FileConvertResult{File: filename, Error: err.Error()}
	}

	// Rewrite identical content — validates write access, no content change
	writeArgs, _ := json.Marshal(map[string]interface{}{
		"file":    fullPath,
		"content": readResult.Content,
		"dry_run": false,
	})
	if _, err := SendRequest(sockPath, "file_write", writeArgs); err != nil {
		return FileConvertResult{File: filename, Error: fmt.Sprintf("write: %v", err)}
	}
	return FileConvertResult{File: filename, Changed: false}
}

// RunConvert processes every non-readonly file in dir:
//   - Go files: parse into AST via go/parser, delete, rewrite via go/format
//   - Non-Go files: read and rewrite (validates access, normalises line endings)
//
// Read-only files (vendor/, stdlib, module cache) are skipped.
func RunConvert(dir string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: %v\n", err)
		os.Exit(1)
	}

	sockPath, err := resolveSock(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: cannot connect to daemon: %v\n", err)
		os.Exit(1)
	}

	// Get directory inventory
	dirArgs, _ := json.Marshal(map[string]interface{}{"dir": abs, "recursive": false})
	resp, err := SendRequest(sockPath, "ast_directory", dirArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: ast_directory: %v\n", err)
		os.Exit(1)
	}
	result, err := BuildConvertResult(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grv convert: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("grv convert: %s\n", abs)
	fmt.Printf("  %d read-write files to convert, %d read-only skipped\n\n",
		result.ReadWrite, result.ReadOnly)

	changed, skipped, errors := 0, 0, 0

	// Convert Go files
	for _, f := range result.GoFiles {
		if f.Readonly {
			skipped++
			continue
		}
		r := convertGoFile(sockPath, abs, f.File)
		if r.Error != "" {
			fmt.Printf("  ERROR  %s: %s\n", f.File, r.Error)
			errors++
		} else if r.Changed {
			fmt.Printf("  CHANGED %s\n", f.File)
			changed++
		} else {
			fmt.Printf("  OK      %s\n", f.File)
		}
	}

	// Convert non-Go files
	for _, f := range result.NonGoFiles {
		if f.Readonly {
			skipped++
			continue
		}
		r := convertNonGoFile(sockPath, abs, f.File)
		if r.Error != "" {
			fmt.Printf("  ERROR  %s: %s\n", f.File, r.Error)
			errors++
		} else {
			fmt.Printf("  OK      %s\n", f.File)
		}
	}

	fmt.Printf("\nDone: %d changed, %d unchanged, %d skipped (readonly), %d errors\n",
		changed, result.ReadWrite-changed-errors, skipped, errors)

	if errors > 0 {
		os.Exit(1)
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
	if err := StartDaemon(dir); err != nil {
		return "", fmt.Errorf("start daemon: %w", err)
	}
	return sock, nil
}
