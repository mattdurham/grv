package ops

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"

	"github.com/mattdurham/grv/editor"
	"github.com/mattdurham/grv/kinds"
	"github.com/mattdurham/grv/selector"
)

type InsertOp struct {
	Path  json.RawMessage `json:"path"`
	Index int             `json:"index"`
	Node  json.RawMessage `json:"node"`
}

type ASTInsertManyArgs struct {
	File   string     `json:"file"`
	Ops    []InsertOp `json:"ops"`
	DryRun bool       `json:"dry_run"`
}

func HandleASTInsertMany(args ASTInsertManyArgs) (json.RawMessage, error) {
	if args.File == "" {
		return errResult("file is required")
	}
	if isReadonly(args.File) {
		return errResult(fmt.Sprintf("file is readonly: %s", args.File))
	}

	type parsedOp struct {
		steps    []selector.PathStep
		index    int
		kindNode kinds.Node
	}

	parsed := make([]parsedOp, len(args.Ops))
	for i, op := range args.Ops {
		var steps []selector.PathStep
		if err := json.Unmarshal(op.Path, &steps); err != nil {
			return errResult(fmt.Sprintf("op[%d]: parse path: %v", i, err))
		}
		kindNode, err := kinds.UnmarshalNode(op.Node)
		if err != nil {
			return errResult(fmt.Sprintf("op[%d]: parse node: %v", i, err))
		}
		if kindNode == nil {
			return errResult(fmt.Sprintf("op[%d]: node is required", i))
		}
		parsed[i] = parsedOp{steps: steps, index: op.Index, kindNode: kindNode}
	}

	original, _ := os.ReadFile(args.File)
	result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, _ *token.FileSet) error {
		for i, op := range parsed {
			target, parentCtx, navErr := selector.Navigate(f, op.steps)
			if navErr != nil {
				return navErr
			}
			newNode, toErr := op.kindNode.ToAST()
			if toErr != nil {
				return fmt.Errorf("op[%d]: ToAST: %w", i, toErr)
			}
			if err := insertIntoNode(target, newNode, op.index); err != nil {
				if err2 := insertIntoList(parentCtx, newNode, op.index); err2 != nil {
					return fmt.Errorf("op[%d]: %w", i, err2)
				}
			}
		}
		return nil
	})
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
