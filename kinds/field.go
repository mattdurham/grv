// Namespace: goast/kinds
// Kind: Field
// go/ast: *ast.Field
package kinds

import (
	"encoding/json"
	"go/ast"
	"go/token"
)

type Field struct {
	KindField string          `json:"kind"`
	Names     []string        `json:"names,omitempty"`
	Type      json.RawMessage `json:"type"`
	Tag       *string         `json:"tag,omitempty"`
}

func (f *Field) Kind() string { return "Field" }

func (f *Field) ToAST() (ast.Node, error) {
	result := &ast.Field{}
	for _, name := range f.Names {
		result.Names = append(result.Names, &ast.Ident{Name: name})
	}
	typ, err := unmarshalExpr(f.Type, "Field.Type")
	if err != nil {
		return nil, err
	}
	result.Type = typ
	if f.Tag != nil {
		result.Tag = &ast.BasicLit{Kind: token.STRING, Value: *f.Tag}
	}
	return result, nil
}

func (f *Field) FromAST(node ast.Node) error {
	field := node.(*ast.Field)
	f.KindField = "Field"
	for _, name := range field.Names {
		f.Names = append(f.Names, name.Name)
	}
	var err error
	f.Type, err = MarshalExpr(field.Type)
	if err != nil {
		return err
	}
	if field.Tag != nil {
		tag := field.Tag.Value
		f.Tag = &tag
	}
	return nil
}

// marshalFields converts []*ast.Field to []json.RawMessage.
func marshalFields(fields []*ast.Field) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(fields))
	for _, field := range fields {
		f := &Field{}
		if err := f.FromAST(field); err != nil {
			return nil, err
		}
		m, err := json.Marshal(f)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, nil
}

func init() { register("Field", func() Node { return &Field{} }) }
