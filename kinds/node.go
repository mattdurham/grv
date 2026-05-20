// Namespace: goast/kinds
// Node interface, registry, and JSON unmarshal dispatch.
package kinds

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
)

// Node is the JSON representation of a go/ast node.
type Node interface {
	Kind() string
	ToAST() (ast.Node, error)
	FromAST(ast.Node) error
}

var registry = map[string]func() Node{}

func register(kind string, factory func() Node) {
	registry[kind] = factory
}

// NewNode creates a zero-value Node for the given kind. Returns nil if unknown.
func NewNode(kind string) Node {
	factory, ok := registry[kind]
	if !ok {
		return nil
	}
	return factory()
}

// UnmarshalNode decodes a JSON node by peeking at "kind" then dispatching.
// Returns nil, nil for null/empty input.
func UnmarshalNode(data json.RawMessage) (Node, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var peek struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("UnmarshalNode peek: %w", err)
	}
	factory, ok := registry[peek.Kind]
	if !ok {
		return nil, fmt.Errorf("unknown kind %q", peek.Kind)
	}
	node := factory()
	if err := json.Unmarshal(data, node); err != nil {
		return nil, fmt.Errorf("UnmarshalNode %s: %w", peek.Kind, err)
	}
	return node, nil
}

// tokenFromString maps operator strings to token.Token values.
func tokenFromString(s string) token.Token {
	switch s {
	case "+":
		return token.ADD
	case "-":
		return token.SUB
	case "*":
		return token.MUL
	case "/":
		return token.QUO
	case "%":
		return token.REM
	case "&":
		return token.AND
	case "|":
		return token.OR
	case "^":
		return token.XOR
	case "<<":
		return token.SHL
	case ">>":
		return token.SHR
	case "&^":
		return token.AND_NOT
	case "&&":
		return token.LAND
	case "||":
		return token.LOR
	case "==":
		return token.EQL
	case "!=":
		return token.NEQ
	case "<":
		return token.LSS
	case "<=":
		return token.LEQ
	case ">":
		return token.GTR
	case ">=":
		return token.GEQ
	// Unary ops
	case "!":
		return token.NOT
	case "<-":
		return token.ARROW
	// Assignment ops
	case "=":
		return token.ASSIGN
	case ":=":
		return token.DEFINE
	case "+=":
		return token.ADD_ASSIGN
	case "-=":
		return token.SUB_ASSIGN
	case "*=":
		return token.MUL_ASSIGN
	case "/=":
		return token.QUO_ASSIGN
	case "%=":
		return token.REM_ASSIGN
	case "&=":
		return token.AND_ASSIGN
	case "|=":
		return token.OR_ASSIGN
	case "^=":
		return token.XOR_ASSIGN
	case "<<=":
		return token.SHL_ASSIGN
	case ">>=":
		return token.SHR_ASSIGN
	case "&^=":
		return token.AND_NOT_ASSIGN
	// Inc/Dec
	case "++":
		return token.INC
	case "--":
		return token.DEC
	// Branch
	case "break":
		return token.BREAK
	case "continue":
		return token.CONTINUE
	case "goto":
		return token.GOTO
	case "fallthrough":
		return token.FALLTHROUGH
	default:
		return token.ILLEGAL
	}
}

// unmarshalFieldList converts []json.RawMessage (each a Field kind) to []*ast.Field.
func unmarshalFieldList(msgs []json.RawMessage) ([]*ast.Field, error) {
	result := make([]*ast.Field, 0, len(msgs))
	for i, msg := range msgs {
		n, err := UnmarshalNode(msg)
		if err != nil {
			return nil, fmt.Errorf("field[%d]: %w", i, err)
		}
		if n == nil {
			continue
		}
		astNode, err := n.ToAST()
		if err != nil {
			return nil, fmt.Errorf("field[%d].ToAST: %w", i, err)
		}
		field, ok := astNode.(*ast.Field)
		if !ok {
			return nil, fmt.Errorf("field[%d]: expected *ast.Field, got %T", i, astNode)
		}
		result = append(result, field)
	}
	return result, nil
}

// unmarshalStmtList converts []json.RawMessage to []ast.Stmt.
func unmarshalStmtList(msgs []json.RawMessage) ([]ast.Stmt, error) {
	result := make([]ast.Stmt, 0, len(msgs))
	for i, msg := range msgs {
		n, err := UnmarshalNode(msg)
		if err != nil {
			return nil, fmt.Errorf("stmt[%d]: %w", i, err)
		}
		if n == nil {
			continue
		}
		astNode, err := n.ToAST()
		if err != nil {
			return nil, fmt.Errorf("stmt[%d].ToAST: %w", i, err)
		}
		stmt, ok := astNode.(ast.Stmt)
		if !ok {
			return nil, fmt.Errorf("stmt[%d]: expected ast.Stmt, got %T", i, astNode)
		}
		result = append(result, stmt)
	}
	return result, nil
}

// unmarshalExprList converts []json.RawMessage to []ast.Expr.
func unmarshalExprList(msgs []json.RawMessage) ([]ast.Expr, error) {
	result := make([]ast.Expr, 0, len(msgs))
	for i, msg := range msgs {
		n, err := UnmarshalNode(msg)
		if err != nil {
			return nil, fmt.Errorf("expr[%d]: %w", i, err)
		}
		if n == nil {
			continue
		}
		astNode, err := n.ToAST()
		if err != nil {
			return nil, fmt.Errorf("expr[%d].ToAST: %w", i, err)
		}
		expr, ok := astNode.(ast.Expr)
		if !ok {
			return nil, fmt.Errorf("expr[%d]: expected ast.Expr, got %T", i, astNode)
		}
		result = append(result, expr)
	}
	return result, nil
}

// unmarshalSpecList converts []json.RawMessage to []ast.Spec.
func unmarshalSpecList(msgs []json.RawMessage) ([]ast.Spec, error) {
	result := make([]ast.Spec, 0, len(msgs))
	for i, msg := range msgs {
		n, err := UnmarshalNode(msg)
		if err != nil {
			return nil, fmt.Errorf("spec[%d]: %w", i, err)
		}
		if n == nil {
			continue
		}
		astNode, err := n.ToAST()
		if err != nil {
			return nil, fmt.Errorf("spec[%d].ToAST: %w", i, err)
		}
		spec, ok := astNode.(ast.Spec)
		if !ok {
			return nil, fmt.Errorf("spec[%d]: expected ast.Spec, got %T", i, astNode)
		}
		result = append(result, spec)
	}
	return result, nil
}

// unmarshalExpr unmarshals a single expression node.
func unmarshalExpr(data json.RawMessage, fieldPath string) (ast.Expr, error) {
	n, err := UnmarshalNode(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fieldPath, err)
	}
	if n == nil {
		return nil, nil
	}
	astNode, err := n.ToAST()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fieldPath, err)
	}
	expr, ok := astNode.(ast.Expr)
	if !ok {
		return nil, fmt.Errorf("%s: expected ast.Expr, got %T", fieldPath, astNode)
	}
	return expr, nil
}

// unmarshalStmt unmarshals a single statement node.
func unmarshalStmt(data json.RawMessage, fieldPath string) (ast.Stmt, error) {
	n, err := UnmarshalNode(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fieldPath, err)
	}
	if n == nil {
		return nil, nil
	}
	astNode, err := n.ToAST()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fieldPath, err)
	}
	stmt, ok := astNode.(ast.Stmt)
	if !ok {
		return nil, fmt.Errorf("%s: expected ast.Stmt, got %T", fieldPath, astNode)
	}
	return stmt, nil
}
