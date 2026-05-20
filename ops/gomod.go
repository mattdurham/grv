// Namespace: goast/ops
// go.mod tools: gomod_read, gomod_require, gomod_drop_require, gomod_replace, gomod_drop_replace
package ops

import (
	"encoding/json"
	"fmt"
	"os"

	godiff "github.com/mattdurham/grv/diff"
	"github.com/mattdurham/grv/editor"
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
func HandleGoModRead(args GoModReadArgs) (json.RawMessage, error) {
	f, _, err := readGoMod(args.File)
	if err != nil {
		return errResult(err.Error())
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

	return okResult(summary)
}

// HandleGoModRequire implements the gomod_require tool.
func HandleGoModRequire(args GoModRequireArgs) (json.RawMessage, error) {
	f, origData, err := readGoMod(args.File)
	if err != nil {
		return errResult(err.Error())
	}

	if err := f.AddRequire(args.Path, args.Version); err != nil {
		return errResult(fmt.Sprintf("add require: %v", err))
	}
	if args.Indirect {
		f.SetRequireSeparateIndirect(f.Require)
	}

	return writeGoMod(args.File, f, origData)
}

// HandleGoModDropRequire implements the gomod_drop_require tool.
func HandleGoModDropRequire(args GoModDropRequireArgs) (json.RawMessage, error) {
	f, origData, err := readGoMod(args.File)
	if err != nil {
		return errResult(err.Error())
	}

	if err := f.DropRequire(args.Path); err != nil {
		return errResult(fmt.Sprintf("drop require: %v", err))
	}

	return writeGoMod(args.File, f, origData)
}

// HandleGoModReplace implements the gomod_replace tool.
func HandleGoModReplace(args GoModReplaceArgs) (json.RawMessage, error) {
	f, origData, err := readGoMod(args.File)
	if err != nil {
		return errResult(err.Error())
	}

	if err := f.AddReplace(args.Old, "", args.New, args.NewVersion); err != nil {
		return errResult(fmt.Sprintf("add replace: %v", err))
	}

	return writeGoMod(args.File, f, origData)
}

// HandleGoModDropReplace implements the gomod_drop_replace tool.
func HandleGoModDropReplace(args GoModDropReplaceArgs) (json.RawMessage, error) {
	f, origData, err := readGoMod(args.File)
	if err != nil {
		return errResult(err.Error())
	}

	if err := f.DropReplace(args.Old, ""); err != nil {
		return errResult(fmt.Sprintf("drop replace: %v", err))
	}

	return writeGoMod(args.File, f, origData)
}

func writeGoMod(file string, f *modfile.File, origData []byte) (json.RawMessage, error) {
	f.Cleanup()
	newData, err := f.Format()
	if err != nil {
		return errResult(fmt.Sprintf("format go.mod: %v", err))
	}

	diffStr := ""
	if string(origData) != string(newData) {
		d, _ := godiff.Files(file, origData, newData)
		diffStr = d
		if err := editor.WriteAtomic(file, newData); err != nil {
			return errResult(fmt.Sprintf("write go.mod: %v", err))
		}
	}

	resp := map[string]interface{}{
		"changed": diffStr != "",
		"diff":    diffStr,
	}
	return okResult(resp)
}
