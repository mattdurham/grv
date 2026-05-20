// Namespace: goast/ops
// Raw file read/write tools for non-Go files.
package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lthiery/goast/diff"
	"github.com/lthiery/goast/editor"
	"github.com/mark3labs/mcp-go/mcp"
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
func HandleFileRead(ctx context.Context, req mcp.CallToolRequest, args FileReadArgs) (*mcp.CallToolResult, error) {
	if args.File == "" {
		return toolError("file is required"), nil
	}
	content, err := os.ReadFile(args.File)
	if err != nil {
		return toolError(fmt.Sprintf("read: %v", err)), nil
	}
	result := FileReadResult{
		Content:  string(content),
		Size:     len(content),
		Readonly: isReadonly(args.File),
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b)), nil
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
func HandleFileWrite(ctx context.Context, req mcp.CallToolRequest, args FileWriteArgs) (*mcp.CallToolResult, error) {
	if args.File == "" {
		return toolError("file is required"), nil
	}
	if isReadonly(args.File) {
		return toolError(fmt.Sprintf("file is readonly: %s", args.File)), nil
	}

	newContent := []byte(args.Content)

	// Read existing content for diff (may not exist yet).
	existing, _ := os.ReadFile(args.File)

	diffStr, err := diff.Files(args.File, existing, newContent)
	if err != nil {
		return toolError(fmt.Sprintf("diff: %v", err)), nil
	}

	changed := diffStr != ""
	if changed && !args.DryRun {
		if err := editor.WriteAtomic(args.File, newContent); err != nil {
			return toolError(fmt.Sprintf("write: %v", err)), nil
		}
	}

	result := FileWriteResult{
		Diff:    diffStr,
		Changed: changed,
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b)), nil
}
