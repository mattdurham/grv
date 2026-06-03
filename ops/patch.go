// Namespace: goast/ops
// Write tool: ast_patch
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

// PatchOp is one mutation applied to a node's JSON representation.
//
// Op values:
//   - "set"     — replace the named field with Value (scalar or array)
//   - "append"  — append Value to the named array field
//   - "prepend" — prepend Value to the named array field
//   - "insert"  — insert Value into the named array field at Index
//   - "delete"  — remove the named field entirely (or, for arrays, remove element at Index)
type PatchOp struct {
	Op    string          `json:"op"`
	Field string          `json:"field"`
	Value json.RawMessage `json:"value,omitempty"`
	Index int             `json:"index,omitempty"`
}

// ASTPatchArgs is the argument struct for ast_patch.
type ASTPatchArgs struct {
	File   string          `json:"file"`
	Path   json.RawMessage `json:"path"`
	Ops    []PatchOp       `json:"ops"`
	DryRun bool            `json:"dry_run"`
}

// HandleASTPatch implements the ast_patch tool.
func HandleASTPatch(args ASTPatchArgs) (json.RawMessage, error) {
	if isReadonly(args.File) {
		return errResult(fmt.Sprintf("file is readonly: %s", args.File))
	}

	var steps []selector.PathStep
	if err := json.Unmarshal(args.Path, &steps); err != nil {
		return errResult(fmt.Sprintf("parse path: %v", err))
	}

	if len(args.Ops) == 0 {
		return errResult("ops must not be empty")
	}
	for i, op := range args.Ops {
		if op.Field == "" {
			return errResult(fmt.Sprintf("op[%d]: field is required", i))
		}
		switch op.Op {
		case "set", "append", "prepend", "insert", "delete":
		default:
			return errResult(fmt.Sprintf("op[%d]: unknown op %q (want set, append, prepend, insert, delete)", i, op.Op))
		}
	}

	original, _ := os.ReadFile(args.File)
	result, err := editor.Edit(args.File, args.DryRun, func(f *ast.File, _ *token.FileSet) error {
		target, parentCtx, navErr := selector.Navigate(f, steps)
		if navErr != nil {
			return navErr
		}

		// Marshal the target node to its JSON map representation.
		raw, marshalErr := kinds.MarshalNode(target)
		if marshalErr != nil {
			return fmt.Errorf("marshal node: %w", marshalErr)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return fmt.Errorf("unmarshal node to map: %w", err)
		}

		// Apply each patch operation.
		for i, op := range args.Ops {
			if err := applyPatchOp(fields, op); err != nil {
				return fmt.Errorf("op[%d]: %w", i, err)
			}
		}

		// Marshal back and reconstruct the AST node.
		patched, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("marshal patched node: %w", err)
		}
		kindNode, err := kinds.UnmarshalNode(json.RawMessage(patched))
		if err != nil {
			return fmt.Errorf("parse patched node: %w", err)
		}
		if kindNode == nil {
			return fmt.Errorf("patched node is nil")
		}
		newNode, err := kindNode.ToAST()
		if err != nil {
			return fmt.Errorf("ToAST: %w", err)
		}
		return replaceInParent(parentCtx, newNode)
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

// applyPatchOp mutates the fields map according to one PatchOp.
func applyPatchOp(fields map[string]json.RawMessage, op PatchOp) error {
	switch op.Op {
	case "set":
		if len(op.Value) == 0 {
			return fmt.Errorf("value is required for op=set")
		}
		fields[op.Field] = op.Value

	case "append":
		if len(op.Value) == 0 {
			return fmt.Errorf("value is required for op=append")
		}
		arr, err := fieldAsArray(fields, op.Field)
		if err != nil {
			return err
		}
		arr = append(arr, op.Value)
		fields[op.Field] = marshalArray(arr)

	case "prepend":
		if len(op.Value) == 0 {
			return fmt.Errorf("value is required for op=prepend")
		}
		arr, err := fieldAsArray(fields, op.Field)
		if err != nil {
			return err
		}
		arr = append([]json.RawMessage{op.Value}, arr...)
		fields[op.Field] = marshalArray(arr)

	case "insert":
		if len(op.Value) == 0 {
			return fmt.Errorf("value is required for op=insert")
		}
		arr, err := fieldAsArray(fields, op.Field)
		if err != nil {
			return err
		}
		idx := op.Index
		if idx < 0 || idx >= len(arr) {
			arr = append(arr, op.Value)
		} else {
			arr = append(arr, nil)
			copy(arr[idx+1:], arr[idx:])
			arr[idx] = op.Value
		}
		fields[op.Field] = marshalArray(arr)

	case "delete":
		existing, exists := fields[op.Field]
		if !exists {
			return nil // idempotent — field already absent
		}
		// If the field is an array, delete the element at Index; otherwise delete the field.
		var probe []json.RawMessage
		if json.Unmarshal(existing, &probe) == nil {
			// It's an array — remove element at Index.
			idx := op.Index
			if idx < 0 || idx >= len(probe) {
				return fmt.Errorf("index %d out of range for field %q (len %d)", idx, op.Field, len(probe))
			}
			probe = append(probe[:idx], probe[idx+1:]...)
			fields[op.Field] = marshalArray(probe)
		} else {
			delete(fields, op.Field)
		}
	}
	return nil
}

// fieldAsArray returns the named field parsed as a JSON array.
// If the field is absent, returns an empty slice.
func fieldAsArray(fields map[string]json.RawMessage, name string) ([]json.RawMessage, error) {
	raw, ok := fields[name]
	if !ok || string(raw) == "null" {
		return []json.RawMessage{}, nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("field %q is not an array: %w", name, err)
	}
	return arr, nil
}

// marshalArray serialises a slice of json.RawMessage back to a JSON array.
func marshalArray(arr []json.RawMessage) json.RawMessage {
	out, _ := json.Marshal(arr)
	return out
}
