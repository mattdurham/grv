// Namespace: goast/ops
// Raw file read/write tools for non-Go files.
package ops

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mattdurham/grv/diff"
	"github.com/mattdurham/grv/editor"
)

// FileReadArgs is the argument struct for file_read.
type FileReadArgs struct {
	File string `json:"file"`
}

// FileReadResult is the response for file_read.
type FileReadResult struct {
	Content  string `json:"content"`
	Size     int    `json:"size"`
	Readonly bool   `json:"readonly"`
}

// HandleFileRead implements the file_read tool.
func HandleFileRead(args FileReadArgs) (json.RawMessage, error) {
	if args.File == "" {
		return errResult("file is required")
	}
	content, err := os.ReadFile(args.File)
	if err != nil {
		return errResult(fmt.Sprintf("read: %v", err))
	}
	result := FileReadResult{
		Content:  string(content),
		Size:     len(content),
		Readonly: isReadonly(args.File),
	}
	return okResult(result)
}

// FileWriteArgs is the argument struct for file_write.
type FileWriteArgs struct {
	File    string `json:"file"`
	Content string `json:"content"`
	DryRun  bool   `json:"dry_run"`
}

// FileWriteResult is the response for file_write.
type FileWriteResult struct {
	Diff    string `json:"diff"`
	Changed bool   `json:"changed"`
}

// HandleFileWrite implements the file_write tool.
func HandleFileWrite(args FileWriteArgs) (json.RawMessage, error) {
	if args.File == "" {
		return errResult("file is required")
	}
	if isReadonly(args.File) {
		return errResult(fmt.Sprintf("file is readonly: %s", args.File))
	}

	newContent := []byte(args.Content)

	// Read existing content for diff (may not exist yet).
	existing, _ := os.ReadFile(args.File)

	diffStr, err := diff.Files(args.File, existing, newContent)
	if err != nil {
		return errResult(fmt.Sprintf("diff: %v", err))
	}

	changed := diffStr != ""
	if changed && !args.DryRun {
		if err := editor.WriteAtomic(args.File, newContent); err != nil {
			return errResult(fmt.Sprintf("write: %v", err))
		}
	}

	result := FileWriteResult{
		Diff:    diffStr,
		Changed: changed,
	}
	return okResult(result)
}
