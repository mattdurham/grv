// Namespace: goast/ops
// go.mod tools: gomod_read, gomod_require, gomod_drop_require, gomod_replace, gomod_drop_replace
package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	godiff "github.com/lthiery/goast/diff"
	"github.com/lthiery/goast/editor"
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/mod/modfile"
)

// GoModReadArgs is the argument struct for gomod_read.
type GoModReadArgs struct {
	File string `json:"file"`
}

// GoModRequireArgs is the argument struct for gomod_require.
type GoModRequireArgs struct {
	File     string `json:"file"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	Indirect bool   `json:"indirect"`
}

// GoModDropRequireArgs is the argument struct for gomod_drop_require.
type GoModDropRequireArgs struct {
	File string `json:"file"`
	Path string `json:"path"`
}

// GoModReplaceArgs is the argument struct for gomod_replace.
type GoModReplaceArgs struct {
	File       string `json:"file"`
	Old        string `json:"old"`
	New        string `json:"new"`
	NewVersion string `json:"new_version"`
}

// GoModDropReplaceArgs is the argument struct for gomod_drop_replace.
type GoModDropReplaceArgs struct {
	File string `json:"file"`
	Old  string `json:"old"`
}

type goModRequire struct {
	Path     string `json:"path"`
	Version  string `json:"version"`
	Indirect bool   `json:"indirect"`
}

type goModReplace struct {
	Old        string `json:"old"`
	New        string `json:"new"`
	NewVersion string `json:"new_version"`
}

type goModExclude struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type goModSummary struct {
	Module  string         `json:"module"`
	Go      string         `json:"go"`
	Require []goModRequire `json:"require"`
	Replace []goModReplace `json:"replace"`
	Exclude []goModExclude `json:"exclude"`
}

func readGoMod(file string) (*modfile.File, []byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	f, err := modfile.Parse(file, data, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("parse go.mod: %w", err)
	}
	return f, data, nil
}

// HandleGoModRead implements the gomod_read tool.
func HandleGoModRead(ctx context.Context, req mcp.CallToolRequest, args GoModReadArgs) (*mcp.CallToolResult, error) {
	f, _, err := readGoMod(args.File)
	if err != nil {
		return toolError(err.Error()), nil
	}

	summary := goModSummary{
		Module:  f.Module.Mod.Path,
		Require: []goModRequire{},
		Replace: []goModReplace{},
		Exclude: []goModExclude{},
	}
	if f.Go != nil {
		summary.Go = f.Go.Version
	}
	for _, r := range f.Require {
		summary.Require = append(summary.Require, goModRequire{
			Path:     r.Mod.Path,
			Version:  r.Mod.Version,
			Indirect: r.Indirect,
		})
	}
	for _, r := range f.Replace {
		newVer := r.New.Version
		summary.Replace = append(summary.Replace, goModReplace{
			Old:        r.Old.Path,
			New:        r.New.Path,
			NewVersion: newVer,
		})
	}
	for _, e := range f.Exclude {
		summary.Exclude = append(summary.Exclude, goModExclude{
			Path:    e.Mod.Path,
			Version: e.Mod.Version,
		})
	}

	b, _ := json.Marshal(summary)
	return mcp.NewToolResultText(string(b)), nil
}

// HandleGoModRequire implements the gomod_require tool.
func HandleGoModRequire(ctx context.Context, req mcp.CallToolRequest, args GoModRequireArgs) (*mcp.CallToolResult, error) {
	f, origData, err := readGoMod(args.File)
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := f.AddRequire(args.Path, args.Version); err != nil {
		return toolError(fmt.Sprintf("add require: %v", err)), nil
	}
	if args.Indirect {
		f.SetRequireSeparateIndirect(f.Require)
	}

	return writeGoMod(args.File, f, origData)
}

// HandleGoModDropRequire implements the gomod_drop_require tool.
func HandleGoModDropRequire(ctx context.Context, req mcp.CallToolRequest, args GoModDropRequireArgs) (*mcp.CallToolResult, error) {
	f, origData, err := readGoMod(args.File)
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := f.DropRequire(args.Path); err != nil {
		return toolError(fmt.Sprintf("drop require: %v", err)), nil
	}

	return writeGoMod(args.File, f, origData)
}

// HandleGoModReplace implements the gomod_replace tool.
func HandleGoModReplace(ctx context.Context, req mcp.CallToolRequest, args GoModReplaceArgs) (*mcp.CallToolResult, error) {
	f, origData, err := readGoMod(args.File)
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := f.AddReplace(args.Old, "", args.New, args.NewVersion); err != nil {
		return toolError(fmt.Sprintf("add replace: %v", err)), nil
	}

	return writeGoMod(args.File, f, origData)
}

// HandleGoModDropReplace implements the gomod_drop_replace tool.
func HandleGoModDropReplace(ctx context.Context, req mcp.CallToolRequest, args GoModDropReplaceArgs) (*mcp.CallToolResult, error) {
	f, origData, err := readGoMod(args.File)
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := f.DropReplace(args.Old, ""); err != nil {
		return toolError(fmt.Sprintf("drop replace: %v", err)), nil
	}

	return writeGoMod(args.File, f, origData)
}

func writeGoMod(file string, f *modfile.File, origData []byte) (*mcp.CallToolResult, error) {
	f.Cleanup()
	newData, err := f.Format()
	if err != nil {
		return toolError(fmt.Sprintf("format go.mod: %v", err)), nil
	}

	diffStr := ""
	if string(origData) != string(newData) {
		d, _ := godiff.Files(file, origData, newData)
		diffStr = d
		if err := editor.WriteAtomic(file, newData); err != nil {
			return toolError(fmt.Sprintf("write go.mod: %v", err)), nil
		}
	}

	resp := map[string]interface{}{
		"changed": diffStr != "",
		"diff":    diffStr,
	}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}
