// Namespace: goast/kinds
// MarshalNode dispatcher: converts go/ast nodes to JSON representations.
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
)

// MarshalNode converts a go/ast node to its JSON representation.
func MarshalNode(node ast.Node) (json.RawMessage, error) {
	if node == nil {
		return json.RawMessage("null"), nil
	}
	switch n := node.(type) {
	// Expressions
	case *ast.Ident:
		v := &Ident{}
		_ = v.FromAST(n) // Ident.FromAST never errors
		return json.Marshal(v)
	case *ast.BasicLit:
		v := &BasicLit{}
		_ = v.FromAST(n) // BasicLit.FromAST never errors
		return json.Marshal(v)
	case *ast.Ellipsis:
		v := &Ellipsis{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.FuncLit:
		v := &FuncLit{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.CompositeLit:
		v := &CompositeLit{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.ParenExpr:
		v := &ParenExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.SelectorExpr:
		v := &SelectorExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.IndexExpr:
		v := &IndexExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.IndexListExpr:
		v := &IndexListExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.SliceExpr:
		v := &SliceExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.TypeAssertExpr:
		v := &TypeAssertExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.CallExpr:
		v := &CallExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.StarExpr:
		v := &StarExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.UnaryExpr:
		v := &UnaryExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.BinaryExpr:
		v := &BinaryExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.KeyValueExpr:
		v := &KeyValueExpr{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	// Type expressions
	case *ast.ArrayType:
		v := &ArrayType{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.StructType:
		v := &StructType{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.FuncType:
		v := &FuncType{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.InterfaceType:
		v := &InterfaceType{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.MapType:
		v := &MapType{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.ChanType:
		v := &ChanType{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	// Field
	case *ast.Field:
		v := &Field{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	// Statements
	case *ast.BlockStmt:
		v := &BlockStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.ExprStmt:
		v := &ExprStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.SendStmt:
		v := &SendStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.IncDecStmt:
		v := &IncDecStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.AssignStmt:
		v := &AssignStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.GoStmt:
		v := &GoStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.DeferStmt:
		v := &DeferStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.ReturnStmt:
		v := &ReturnStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.BranchStmt:
		v := &BranchStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.LabeledStmt:
		v := &LabeledStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.IfStmt:
		v := &IfStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.CaseClause:
		v := &CaseClause{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.SwitchStmt:
		v := &SwitchStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.TypeSwitchStmt:
		v := &TypeSwitchStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.CommClause:
		v := &CommClause{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.SelectStmt:
		v := &SelectStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.ForStmt:
		v := &ForStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.RangeStmt:
		v := &RangeStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.DeclStmt:
		v := &DeclStmt{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	// Declarations
	case *ast.FuncDecl:
		v := &FuncDecl{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.GenDecl:
		switch n.Tok {
		case token.IMPORT:
			v := &ImportDecl{}
			if err := v.FromAST(n); err != nil {
				return nil, err
			}
			return json.Marshal(v)
		case token.CONST:
			v := &ConstDecl{}
			if err := v.FromAST(n); err != nil {
				return nil, err
			}
			return json.Marshal(v)
		case token.TYPE:
			v := &TypeDecl{}
			if err := v.FromAST(n); err != nil {
				return nil, err
			}
			return json.Marshal(v)
		case token.VAR:
			v := &VarDecl{}
			if err := v.FromAST(n); err != nil {
				return nil, err
			}
			return json.Marshal(v)
		default:
			return nil, fmt.Errorf("MarshalNode: unknown GenDecl token %v", n.Tok)
		}
	// Specs
	case *ast.ImportSpec:
		v := &ImportSpec{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.ValueSpec:
		v := &ValueSpec{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	case *ast.TypeSpec:
		v := &TypeSpec{}
		if err := v.FromAST(n); err != nil {
			return nil, err
		}
		return json.Marshal(v)
	default:
		return nil, fmt.Errorf("MarshalNode: unsupported node type %T", node)
	}
}

// MarshalExpr converts ast.Expr to JSON. Returns null for nil input.
func MarshalExpr(expr ast.Expr) (json.RawMessage, error) {
	if expr == nil {
		return json.RawMessage("null"), nil
	}
	return MarshalNode(expr)
}

// MarshalStmt converts ast.Stmt to JSON. Returns null for nil input.
func MarshalStmt(stmt ast.Stmt) (json.RawMessage, error) {
	if stmt == nil {
		return json.RawMessage("null"), nil
	}
	return MarshalNode(stmt)
}

// MarshalDecl converts ast.Decl to JSON.
func MarshalDecl(decl ast.Decl) (json.RawMessage, error) {
	if decl == nil {
		return json.RawMessage("null"), nil
	}
	return MarshalNode(decl)
}
