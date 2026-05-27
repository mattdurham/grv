package daemon

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattdurham/grv/editor"
)

var dirTools = map[string]bool{
	"ast_directory":    true,
	"ast_find_symbols": true,
}

var hybridTools = map[string]bool{
	"ast_find":   true,
	"ast_insert": true,
}

var skipTools = map[string]bool{
	"file_read":          true,
	"file_write":         true,
	"gomod_read":         true,
	"gomod_require":      true,
	"gomod_drop_require": true,
	"gomod_replace":      true,
	"gomod_drop_replace": true,
}

// ParseNamespace splits a namespace string like "hooks#RunFile" into
// pkg ("hooks") and decl ("RunFile"). An empty string or a leading "#"
// is an error.
func parseNamespace(ns string) (pkgRel, declName string, err error) {
	if ns == "" {
		return "", "", fmt.Errorf("namespace must not be empty")
	}
	parts := strings.SplitN(ns, "#", 2)
	if parts[0] == "" {
		return "", "", fmt.Errorf("namespace %q has empty package part", ns)
	}
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return parts[0], "", nil
}

// ResolveNamespace rewrites the args JSON for tool by replacing the
// "namespace" key with either "file" or "dir" based on tool category.
// Returns args unchanged when there is no "namespace" key or when tool
// is in skipTools.
func (s *Server) resolveNamespace(tool string, raw json.RawMessage) (json.RawMessage, error) {
	if skipTools[tool] {
		return raw, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m["namespace"] == nil {
		return raw, nil
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil {
		return raw, nil
	}
	pkgRel, declName, err := parseNamespace(ns)
	if err != nil {
		return nil, err
	}

	var absDir string
	if strings.HasPrefix(ns, "/") {
		// absolute path: strip any #DeclName suffix to get the dir part
		absDir = strings.SplitN(ns, "#", 2)[0]
	} else if pkgRel == "." {
		absDir = s.Dir
	} else {
		absDir = filepath.Join(s.Dir, pkgRel)
	}

	delete(m, "namespace")

	switch {
	case dirTools[tool]:
		if declName != "" {
			return nil, fmt.Errorf("%s does not accept a declaration name; use %s without #%s", tool, pkgRel, declName)
		}
		return injectDir(m, absDir)

	case hybridTools[tool] || tool == "ast_list":
		if declName != "" {
			path, ferr := s.findFileForDecl(absDir, declName)
			if ferr != nil {
				return nil, ferr
			}
			return injectFile(m, path)
		}
		return injectDir(m, absDir)

	default:
		// file-scoped tools require a declaration name
		if declName == "" {
			return nil, fmt.Errorf("%s requires #DeclName to identify a specific file (e.g. '%s#MyFunc')", tool, pkgRel)
		}
		path, ferr := s.findFileForDecl(absDir, declName)
		if ferr != nil {
			return nil, ferr
		}
		return injectFile(m, path)
	}
}

func injectDir(m map[string]json.RawMessage, dir string) (json.RawMessage, error) {
	b, err := json.Marshal(dir)
	if err != nil {
		return nil, err
	}
	m["dir"] = b
	return json.Marshal(m)
}

func injectFile(m map[string]json.RawMessage, file string) (json.RawMessage, error) {
	b, err := json.Marshal(file)
	if err != nil {
		return nil, err
	}
	m["file"] = b
	return json.Marshal(m)
}

// FindFileForDecl scans .go files in dir and returns the path of the file
// that contains a top-level declaration named declName.
func (s *Server) findFileForDecl(dir, declName string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		absPath := filepath.Join(dir, e.Name())
		f, _, _, err := editor.ParseFile(absPath)
		if err != nil {
			continue
		}
		if found := findDeclInFile(f, declName); found {
			return absPath, nil
		}
	}
	return "", fmt.Errorf("declaration %q not found in %s", declName, dir)
}

func findDeclInFile(f *ast.File, declName string) bool {
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			if decl.Name.Name == declName {
				return true
			}
		case *ast.GenDecl:
			switch decl.Tok {
			case token.TYPE:
				for _, spec := range decl.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == declName {
						return true
					}
				}
			case token.VAR, token.CONST:
				for _, spec := range decl.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Names) > 0 && vs.Names[0].Name == declName {
						return true
					}
				}
			}
		}
	}
	return false
}

