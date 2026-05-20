// Namespace: goast/ops
// Directory inventory tool: ast_directory
package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ASTDirectoryArgs is the argument struct for ast_directory.
type ASTDirectoryArgs struct {
	Dir string `json:"dir"`
}

// GoFileEntry describes a parsed Go file in the directory.
type GoFileEntry struct {
	File       string        `json:"file"`
	Readonly   bool          `json:"readonly"`
	Package    string        `json:"package"`
	Structs    []StructEntry `json:"structs"`
	Interfaces []IfaceEntry  `json:"interfaces"`
	Functions  []FuncEntry   `json:"functions"`
	Globals    []GlobalEntry `json:"globals"`
}

// StructEntry is a struct type found in a Go file.
type StructEntry struct {
	Name       string              `json:"name"`
	Path       []map[string]string `json:"path"`
	FieldCount int                 `json:"field_count"`
}

// IfaceEntry is an interface type found in a Go file.
type IfaceEntry struct {
	Name        string              `json:"name"`
	Path        []map[string]string `json:"path"`
	MethodCount int                 `json:"method_count"`
}

// FuncEntry is a function or method found in a Go file.
type FuncEntry struct {
	Name string              `json:"name"`
	Recv string              `json:"recv,omitempty"`
	Path []map[string]string `json:"path"`
}

// GlobalEntry is a top-level var or const declaration.
type GlobalEntry struct {
	Kind  string   `json:"kind"`
	Names []string `json:"names"`
}

// NonGoFileEntry describes a non-Go file in the directory.
type NonGoFileEntry struct {
	File     string `json:"file"`
	Size     int64  `json:"size"`
	Readonly bool   `json:"readonly"`
}

// ASTDirectoryResult is the response for ast_directory.
type ASTDirectoryResult struct {
	GoFiles    []GoFileEntry    `json:"go_files"`
	NonGoFiles []NonGoFileEntry `json:"non_go_files"`
	Subdirs    []string         `json:"subdirs"`
}

// HandleASTDirectory implements the ast_directory tool.
func HandleASTDirectory(ctx context.Context, req mcp.CallToolRequest, args ASTDirectoryArgs) (*mcp.CallToolResult, error) {
	if args.Dir == "" {
		return toolError("dir is required"), nil
	}

	entries, err := os.ReadDir(args.Dir)
	if err != nil {
		return toolError(fmt.Sprintf("read dir: %v", err)), nil
	}

	result := ASTDirectoryResult{
		GoFiles:    []GoFileEntry{},
		NonGoFiles: []NonGoFileEntry{},
		Subdirs:    []string{},
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(args.Dir, name)

		if entry.IsDir() {
			result.Subdirs = append(result.Subdirs, name)
			continue
		}

		if strings.HasSuffix(name, ".go") {
			gf, err := parseGoFile(fullPath)
			if err != nil {
				// Include file with empty symbols on parse error.
				result.GoFiles = append(result.GoFiles, GoFileEntry{
					File:       name,
					Readonly:   isReadonly(fullPath),
					Structs:    []StructEntry{},
					Interfaces: []IfaceEntry{},
					Functions:  []FuncEntry{},
					Globals:    []GlobalEntry{},
				})
				continue
			}
			gf.File = name
			result.GoFiles = append(result.GoFiles, *gf)
		} else {
			info, err := entry.Info()
			var size int64
			if err == nil {
				size = info.Size()
			}
			result.NonGoFiles = append(result.NonGoFiles, NonGoFileEntry{
				File:     name,
				Size:     size,
				Readonly: isReadonly(fullPath),
			})
		}
	}

	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b)), nil
}

func parseGoFile(path string) (*GoFileEntry, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	entry := &GoFileEntry{
		Readonly:   isReadonly(path),
		Package:    f.Name.Name,
		Structs:    []StructEntry{},
		Interfaces: []IfaceEntry{},
		Functions:  []FuncEntry{},
		Globals:    []GlobalEntry{},
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			fe := FuncEntry{
				Name: d.Name.Name,
				Path: []map[string]string{{"kind": "FuncDecl", "name": d.Name.Name}},
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				fe.Recv = recvTypeString(d.Recv.List[0])
				if fe.Recv != "" {
					fe.Path[0]["recv"] = fe.Recv
				}
			}
			entry.Functions = append(entry.Functions, fe)

		case *ast.GenDecl:
			switch d.Tok {
			case token.TYPE:
				for _, spec := range d.Specs {
					ts := spec.(*ast.TypeSpec)
					switch tt := ts.Type.(type) {
					case *ast.StructType:
						fieldCount := 0
						if tt.Fields != nil {
							fieldCount = len(tt.Fields.List)
						}
						entry.Structs = append(entry.Structs, StructEntry{
							Name:       ts.Name.Name,
							Path:       []map[string]string{{"kind": "TypeSpec", "name": ts.Name.Name}},
							FieldCount: fieldCount,
						})
					case *ast.InterfaceType:
						methodCount := 0
						if tt.Methods != nil {
							methodCount = len(tt.Methods.List)
						}
						entry.Interfaces = append(entry.Interfaces, IfaceEntry{
							Name:        ts.Name.Name,
							Path:        []map[string]string{{"kind": "TypeSpec", "name": ts.Name.Name}},
							MethodCount: methodCount,
						})
					}
				}
			case token.VAR:
				var names []string
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					for _, n := range vs.Names {
						names = append(names, n.Name)
					}
				}
				if len(names) > 0 {
					entry.Globals = append(entry.Globals, GlobalEntry{Kind: "VarDecl", Names: names})
				}
			case token.CONST:
				var names []string
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					for _, n := range vs.Names {
						names = append(names, n.Name)
					}
				}
				if len(names) > 0 {
					entry.Globals = append(entry.Globals, GlobalEntry{Kind: "ConstDecl", Names: names})
				}
			}
		}
	}

	return entry, nil
}
