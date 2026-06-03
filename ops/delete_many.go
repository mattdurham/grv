package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/selector"
)

type DeleteOp struct {
	Path json.RawMessage `json:"path"`
}

type ASTDeleteManyArgs struct {
	File   string     `json:"file"`
	Ops    []DeleteOp `json:"ops"`
	DryRun bool       `json:"dry_run"`
}

// HandleASTDeleteMany deletes multiple AST nodes in a single atomic write.
//
// IMPORTANT: ops that target the same parent list must be ordered by descending
// index within that list. Deletions shift subsequent indices down by 1, so a
// higher-index deletion must happen before a lower-index deletion to keep the
// remaining path steps valid. Ops targeting different parent lists may be in
// any order relative to each other.
func HandleASTDeleteMany(args ASTDeleteManyArgs) (json.RawMessage, error) {
	if args.File == "" {
		return errResult("file is required")
	}
	if isReadonly(args.File) {
		return errResult(fmt.Sprintf("file is readonly: %s", args.File))
	}

	type parsedOp struct {
		steps []selector.PathStep
	}

	parsed := make([]parsedOp, len(args.Ops))
	for i, op := range args.Ops {
		var steps []selector.PathStep
		if err := json.Unmarshal(op.Path, &steps); err != nil {
			return errResult(fmt.Sprintf("op[%d]: parse path: %v", i, err))
		}
		parsed[i] = parsedOp{steps: steps}
	}

	original, _ := os.ReadFile(args.File)
	result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, _ *token.FileSet) error {
		for i, op := range parsed {
			_, parentCtx, navErr := selector.Navigate(f, op.steps)
			if navErr != nil {
				return navErr
			}
			if err := deleteFromList(parentCtx); err != nil {
				return fmt.Errorf("op[%d]: %w", i, err)
			}
		}
		return nil
	})
	// enforcePostWrite is intentional: batch deletes run checks; single-op delete.go can be aligned separately.
	if err == nil && !args.DryRun && result.Changed {
		if err2 := enforcePostWrite(args.File, original, DefaultChecksConfig.Enforce); err2 != nil {
			err = err2
		}
	}
	if err != nil {
		if ne, ok := err.(*selector.NavigateError); ok {
			return navErrResult(ne)
		}
		return errResult(err.Error())
	}
	resp := map[string]interface{}{
		"changed": result.Changed,
		"diff":    result.Diff,
	}
	return okResult(resp)
}
