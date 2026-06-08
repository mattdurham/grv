// Namespace: goast/ops
// Directory inventory tool: ast_directory
package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ASTDirectoryArgs is the argument struct for ast_directory.
type ASTDirectoryArgs struct {
	Dir       string `json:"dir"`
	Recursive *bool  `json:"recursive,omitempty"` // default true; pass false for top-level only
}

// NonGoFileEntry describes a non-Go file in the directory.
type NonGoFileEntry struct {
	File     string `json:"file"`
	Size     int64  `json:"size"`
	Readonly bool   `json:"readonly"`
}

// ASTDirectoryResult is the response for ast_directory.
// Go files are intentionally excluded — use ast_list or ast_find_symbols for those.
type ASTDirectoryResult struct {
	NonGoFiles []NonGoFileEntry `json:"non_go_files"`
	Subdirs    []string         `json:"subdirs"`
}

// HandleASTDirectory implements the ast_directory tool.
func HandleASTDirectory(args ASTDirectoryArgs) (json.RawMessage, error) {
	if args.Dir == "" {
		return errResult("dir is required")
	}

	result := ASTDirectoryResult{
		NonGoFiles: []NonGoFileEntry{},
		Subdirs:    []string{},
	}

	recursive := args.Recursive == nil || *args.Recursive // default true
	if err := walkDir(args.Dir, args.Dir, recursive, &result); err != nil {
		return errResult(fmt.Sprintf("read dir: %v", err))
	}

	return okResult(result)
}

// walkDir recursively (or not) processes a directory, populating result.
// relBase is the original root dir; all file paths are relative to it.
func walkDir(root, dir string, recursive bool, result *ASTDirectoryResult) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden directories (e.g. .git, .bob)
		if entry.IsDir() && strings.HasPrefix(name, ".") {
			continue
		}
		fullPath := filepath.Join(dir, name)
		relPath, _ := filepath.Rel(root, fullPath)

		if entry.IsDir() {
			if recursive {
				if err := walkDir(root, fullPath, recursive, result); err != nil {
					return err
				}
			} else {
				result.Subdirs = append(result.Subdirs, name)
			}
			continue
		}

		// Go files are never returned — use ast_list or ast_find_symbols for those.
		if strings.HasSuffix(name, ".go") {
			continue
		}
		info, err := entry.Info()
		var size int64
		if err == nil {
			size = info.Size()
		}
		result.NonGoFiles = append(result.NonGoFiles, NonGoFileEntry{
			File:     relPath,
			Size:     size,
			Readonly: isReadonly(fullPath),
		})
	}
	return nil
}

