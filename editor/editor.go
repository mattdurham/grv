// Namespace: goast/editor
// Parse → edit → format → write cycle.
package editor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	"github.com/mattdurham/grv/diff"
)

// Result is returned by Edit and DryRun.
type Result struct {
	Diff    string
	Changed bool
}

// Edit parses the file, calls fn to mutate the AST, formats, computes diff,
// and writes atomically. If dryRun is true, skips the write.
func Edit(path string, dryRun bool, fn func(*ast.File, *token.FileSet) error) (Result, error) {
	f, fset, original, err := ParseFile(path)
	if err != nil {
		return Result{}, err
	}

	if err := fn(f, fset); err != nil {
		return Result{}, err
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return Result{}, fmt.Errorf("format: %w", err)
	}
	formatted := buf.Bytes()

	diffStr, err := diff.Files(path, original, formatted)
	if err != nil {
		return Result{}, err
	}

	changed := diffStr != ""
	if changed && !dryRun {
		if err := WriteAtomic(path, formatted); err != nil {
			return Result{}, err
		}
	}
	return Result{Diff: diffStr, Changed: changed}, nil
}

// ParseFile parses a file and returns the AST, FileSet, and original source bytes.
func ParseFile(path string) (*ast.File, *token.FileSet, []byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, fset, src, nil
}

// WriteAtomic writes content to path via a temp file + rename.
func WriteAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".goast-*.go")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
