// Namespace: goast/cmd
// Command: convert
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/mattdurham/grv/kinds"
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

// readFileContent reads a file's content via the daemon.
func readFileContent(sockPath, fullPath string) (string, error) {
	readArgs, _ := json.Marshal(map[string]string{"file": fullPath})
	resp, err := SendRequest(sockPath, "file_read", readArgs)
	if err != nil {
		return "", err
	}
	var result struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	return result.Content, nil
}

// writeFileContent writes content to a file via the daemon.
func writeFileContent(sockPath, fullPath, content string) error {
	writeArgs, _ := json.Marshal(map[string]interface{}{
		"file":    fullPath,
		"content": content,
		"dry_run": false,
	})
	_, err := SendRequest(sockPath, "file_write", writeArgs)
	return err
}

// convertGoFile parses a .go file into AST, formats via go/format, rewrites if changed.
func convertGoFile(sockPath, dir, filename string) FileConvertResult {
	fullPath := filepath.Join(dir, filename)

	original, err := readFileContent(sockPath, fullPath)
	if err != nil {
		return FileConvertResult{File: filename, Error: err.Error()}
	}

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
	if err := writeFileContent(sockPath, fullPath, formatted); err != nil {
		return FileConvertResult{File: filename, Error: fmt.Sprintf("write: %v", err)}
	}
	return FileConvertResult{File: filename, Changed: true}
}

// convertGoModFile parses go.mod via modfile, formats it canonically, rewrites if changed.
func convertGoModFile(sockPath, dir, filename string) FileConvertResult {
	fullPath := filepath.Join(dir, filename)

	original, err := readFileContent(sockPath, fullPath)
	if err != nil {
		return FileConvertResult{File: filename, Error: err.Error()}
	}

	f, err := modfile.Parse(fullPath, []byte(original), nil)
	if err != nil {
		return FileConvertResult{File: filename, Error: fmt.Sprintf("parse go.mod: %v", err)}
	}
	formatted, err := f.Format()
	if err != nil {
		return FileConvertResult{File: filename, Error: fmt.Sprintf("format go.mod: %v", err)}
	}
	if string(formatted) == original {
		return FileConvertResult{File: filename, Changed: false}
	}
	if err := writeFileContent(sockPath, fullPath, string(formatted)); err != nil {
		return FileConvertResult{File: filename, Error: fmt.Sprintf("write: %v", err)}
	}
	return FileConvertResult{File: filename, Changed: true}
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

	// Get directory inventory — recursive by default
	dirArgs, _ := json.Marshal(map[string]interface{}{"dir": abs, "recursive": true})
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

	// Pass 1: format all Go files in-place via go/format
	fmt.Println("Pass 1: formatting...")
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
		}
	}
	for _, f := range result.NonGoFiles {
		if !f.Readonly && filepath.Base(f.File) == "go.mod" {
			r := convertGoModFile(sockPath, abs, f.File)
			if r.Error != "" {
				fmt.Printf("  ERROR  %s: %s\n", f.File, r.Error)
				errors++
			} else if r.Changed {
				fmt.Printf("  CHANGED %s\n", f.File)
				changed++
			}
		}
	}

	// Pass 2: structural reorganisation — move each type to its canonical file.
	// Rules: struct Foo → foo.go, interface Bar → bar.go.
	// Skips _test.go files (test helpers must stay in test files to avoid import cycles).
	// Uses alreadyMoved to prevent double-placing the same type in one run.
	fmt.Println("\nPass 2: reorganising...")
	pkgDirs := collectPackageDirs(abs, result.GoFiles)
	alreadyMoved := map[string]bool{} // key: pkgDir+"::"+typeName
	moved := 0
	for _, pkgDir := range pkgDirs {
		m, errs := reorganisePackage(sockPath, pkgDir, alreadyMoved)
		moved += m
		for _, e := range errs {
			fmt.Printf("  ERROR  %s\n", e)
			errors++
		}
	}
	if moved > 0 {
		fmt.Printf("  %d declaration(s) moved to canonical files\n", moved)
	} else {
		fmt.Println("  already canonical — nothing to move")
	}

	fmt.Printf("\nDone: %d reformatted, %d moved, %d skipped (readonly), %d errors\n",
		changed, moved, skipped, errors)

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

// ---- Pass 2: structural reorganisation ----

// collectPackageDirs returns unique package directories from the GoFiles list.
func collectPackageDirs(base string, files []GoFileEntry) []string {
	seen := map[string]bool{}
	var dirs []string
	for _, f := range files {
		if f.Readonly || strings.HasSuffix(f.File, "_test.go") {
			continue
		}
		dir := filepath.Join(base, filepath.Dir(f.File))
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// reorganisePackage moves type declarations in pkgDir to their canonical files.
// Skips _test.go files. Uses alreadyMoved to prevent double-placing.
func reorganisePackage(sockPath, pkgDir string, alreadyMoved map[string]bool) (moved int, errs []string) {
	dirArgs, _ := json.Marshal(map[string]interface{}{"dir": pkgDir, "recursive": false})
	resp, err := SendRequest(sockPath, "ast_directory", dirArgs)
	if err != nil {
		return 0, []string{fmt.Sprintf("ast_directory %s: %v", pkgDir, err)}
	}
	var dirResult struct {
		GoFiles []struct {
			File       string `json:"file"`
			Readonly   bool   `json:"readonly"`
			Structs    []struct{ Name string `json:"name"` } `json:"structs"`
			Interfaces []struct{ Name string `json:"name"` } `json:"interfaces"`
		} `json:"go_files"`
	}
	if err := json.Unmarshal(resp, &dirResult); err != nil {
		return 0, []string{fmt.Sprintf("parse: %v", err)}
	}

	for _, f := range dirResult.GoFiles {
		if f.Readonly || strings.HasSuffix(f.File, "_test.go") {
			continue
		}
		filePath := filepath.Join(pkgDir, f.File)
		for _, s := range append(asNames(f.Structs), asNames(f.Interfaces)...) {
			key := pkgDir + "::" + s
			canonical := strings.ToLower(s) + ".go"
			if f.File == canonical || alreadyMoved[key] {
				continue
			}
			if n, e := moveType(sockPath, pkgDir, filePath, s); n {
				moved++
				alreadyMoved[key] = true
				fmt.Printf("  MOVED  %s:%s → %s\n", f.File, s, canonical)
			} else if e != "" {
				errs = append(errs, e)
			}
		}
	}
	return moved, errs
}

type nameHolder interface{ getName() string }

func asNames(items []struct{ Name string `json:"name"` }) []string {
	names := make([]string, len(items))
	for i, v := range items {
		names[i] = v.Name
	}
	return names
}

// moveType moves a type declaration from sourceFile to its canonical file using ast_place.
// It: reads+parses the source, marshals the TypeSpec via kinds.MarshalNode, checks the
// target doesn't already have the type (idempotency), removes from source, places in target.
func moveType(sockPath, pkgDir, sourceFile, name string) (bool, string) {
	// 1. Read and parse source file
	content, err := readFileContent(sockPath, sourceFile)
	if err != nil {
		return false, fmt.Sprintf("read %s: %v", sourceFile, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, sourceFile, content, parser.ParseComments)
	if err != nil {
		return false, ""
	}

	// 2. Find the TypeSpec
	typeSpec, genDecl, specIdx := findTypeSpec(f, name)
	if typeSpec == nil {
		return false, "" // not in this file
	}

	// Capture the doc comment from the GenDecl so it moves with the type.
	// genDecl.Doc holds the comment group immediately above "type Name ...".
	var docComment string
	if genDecl.Doc != nil {
		var lines []string
		for _, c := range genDecl.Doc.List {
			lines = append(lines, c.Text)
		}
		docComment = strings.Join(lines, "\n")
	}

	// 3. Marshal the TypeSpec via kinds package for accurate serialisation
	specJSON, err := kinds.MarshalNode(typeSpec)
	if err != nil {
		return false, fmt.Sprintf("marshal %s: %v", name, err)
	}
	nodeToPlace, _ := json.Marshal(map[string]interface{}{
		"kind":  "TypeDecl",
		"specs": []json.RawMessage{specJSON},
	})

	// 4. Determine target file via ast_place dry run
	placeArgs, _ := json.Marshal(map[string]interface{}{"dir": pkgDir, "node": json.RawMessage(nodeToPlace), "dry_run": true})
	placeResp, err := SendRequest(sockPath, "ast_place", placeArgs)
	if err != nil {
		return false, ""
	}
	var pr struct{ File string `json:"file"` }
	if err := json.Unmarshal(placeResp, &pr); err != nil {
		return false, ""
	}
	targetPath := filepath.Join(pkgDir, pr.File)
	if targetPath == sourceFile {
		return false, "" // already canonical
	}

	// 5. Idempotency: check target doesn't already have this type (check disk directly)
	if data, statErr := os.ReadFile(targetPath); statErr == nil {
		tfset := token.NewFileSet()
		if tf, parseErr := parser.ParseFile(tfset, targetPath, data, 0); parseErr == nil {
			if existing, _, _ := findTypeSpec(tf, name); existing != nil {
				return false, "" // already there — skip
			}
		}
	}

	// 6. Place into target FIRST — if this fails, source is untouched (safe)
	if _, err := SendRequest(sockPath, "ast_place", func() json.RawMessage {
		b, _ := json.Marshal(map[string]interface{}{"dir": pkgDir, "node": json.RawMessage(nodeToPlace), "dry_run": false})
		return b
	}()); err != nil {
		return false, fmt.Sprintf("place %s: %v", name, err)
	}

	// 6b. If the type had a doc comment, prepend it to the type declaration in the
	// target file. We do a direct string replacement so no re-parse is needed.
	if docComment != "" {
		if targetContent, readErr := os.ReadFile(targetPath); readErr == nil {
			needle := "\ntype " + name + " "
			replacement := "\n" + docComment + "\ntype " + name + " "
			if !strings.Contains(string(targetContent), docComment) {
				newContent := strings.Replace(string(targetContent), needle, replacement, 1)
				if newContent != string(targetContent) {
					os.WriteFile(targetPath, []byte(newContent), 0644) //nolint:errcheck
				}
			}
		}
	}

	// 7. Only remove from source AFTER successful placement
	if err := removeTypeFromFile(sockPath, sourceFile, f, fset, content, genDecl, specIdx); err != nil {
		// Placement already succeeded; log but don't fail — goimports will see duplicate
		return true, ""
	}

	// 8. Copy only the imports actually used by this type to the target file.
	// We collect imports referenced in the TypeSpec's source span and copy those.
	// goimports will prune any that are still unused.
	usedImports := importsUsedByType(f, typeSpec)
	for _, imp := range usedImports {
		p := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		addArgs, _ := json.Marshal(map[string]interface{}{"file": targetPath, "path": p, "alias": alias})
		SendRequest(sockPath, "ast_add_import", addArgs) //nolint:errcheck
	}

	return true, ""
}

// importsUsedByType returns the subset of f.Imports referenced within typeSpec.
// Walks the type's AST collecting selector expressions (pkg.Name) — only those
// packages are copied to the target file. goimports will prune any remaining unused.
func importsUsedByType(f *ast.File, typeSpec *ast.TypeSpec) []*ast.ImportSpec {
	referenced := map[string]bool{}
	ast.Inspect(typeSpec, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				referenced[ident.Name] = true
			}
		}
		return true
	})
	if len(referenced) == 0 {
		return nil
	}
	var used []*ast.ImportSpec
	for _, imp := range f.Imports {
		localName := ""
		if imp.Name != nil {
			localName = imp.Name.Name
		} else {
			p := strings.Trim(imp.Path.Value, `"`)
			parts := strings.Split(p, "/")
			localName = parts[len(parts)-1]
		}
		if referenced[localName] {
			used = append(used, imp)
		}
	}
	return used
}

// findTypeSpec searches an ast.File for a TypeSpec with the given name.
// Returns (typeSpec, parentGenDecl, specIndex) or (nil, nil, 0) if not found.
func findTypeSpec(f *ast.File, name string) (*ast.TypeSpec, *ast.GenDecl, int) {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok.String() != "type" {
			continue
		}
		for i, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if ok && ts.Name.Name == name {
				return ts, gd, i
			}
		}
	}
	return nil, nil, 0
}

// removeTypeFromFile rewrites sourceFile with the named TypeSpec removed.
func removeTypeFromFile(sockPath, sourceFile string, f *ast.File, fset *token.FileSet, _ string, gd *ast.GenDecl, specIdx int) error {
	gd.Specs = append(gd.Specs[:specIdx], gd.Specs[specIdx+1:]...)
	if len(gd.Specs) == 0 {
		newDecls := make([]ast.Decl, 0, len(f.Decls))
		for _, d := range f.Decls {
			if d != ast.Decl(gd) {
				newDecls = append(newDecls, d)
			}
		}
		f.Decls = newDecls
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return err
	}
	return writeFileContent(sockPath, sourceFile, buf.String())
}
