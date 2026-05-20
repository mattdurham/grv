package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// KindDoc documents one Go AST node kind.
type KindDoc struct {
	Group   string
	Schema  string
	Example string
	Notes   string
}

// GrammarRegistry maps kind name → KindDoc.
var GrammarRegistry = map[string]KindDoc{
	// ---- Expressions ----
	"Ident": {
		Group:   "expressions",
		Schema:  `{"kind":"Ident","name":"string"}`,
		Example: `{"kind":"Ident","name":"x"}`,
		Notes:   "true, false, nil, iota, and blank identifier _ are all Ident nodes, not BasicLit.",
	},
	"BasicLit": {
		Group:   "expressions",
		Schema:  `{"kind":"BasicLit","tok":"INT|FLOAT|IMAG|CHAR|STRING","value":"string"}`,
		Example: `{"kind":"BasicLit","tok":"INT","value":"42"}`,
		Notes:   "tok is the Go token kind. For strings: tok=STRING, value includes quotes (e.g. \"\\\"hello\\\"\"). true/false/nil are Ident nodes.",
	},
	"BinaryExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"BinaryExpr","x":<Expr>,"op":"string","y":<Expr>}`,
		Example: `{"kind":"BinaryExpr","x":{"kind":"Ident","name":"a"},"op":"+","y":{"kind":"Ident","name":"b"}}`,
		Notes:   "op is the operator token string: +, -, *, /, %, ==, !=, <, <=, >, >=, &&, ||, &, |, ^, <<, >>, &^.",
	},
	"UnaryExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"UnaryExpr","op":"string","x":<Expr>}`,
		Example: `{"kind":"UnaryExpr","op":"-","x":{"kind":"Ident","name":"x"}}`,
		Notes:   "op: !, -, +, ^, *, &, <-.",
	},
	"CallExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"CallExpr","fun":<Expr>,"args":[<Expr>],"ellipsis":bool}`,
		Example: `{"kind":"CallExpr","fun":{"kind":"Ident","name":"println"},"args":[{"kind":"BasicLit","tok":"STRING","value":"\"hi\""}]}`,
		Notes:   "ellipsis=true for variadic calls like f(args...).",
	},
	"SelectorExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"SelectorExpr","x":<Expr>,"sel":"string"}`,
		Example: `{"kind":"SelectorExpr","x":{"kind":"Ident","name":"fmt"},"sel":"Println"}`,
		Notes:   "x is the receiver expression; sel is just the field/method name string.",
	},
	"IndexExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"IndexExpr","x":<Expr>,"index":<Expr>}`,
		Example: `{"kind":"IndexExpr","x":{"kind":"Ident","name":"arr"},"index":{"kind":"BasicLit","tok":"INT","value":"0"}}`,
		Notes:   "Used for single-type-parameter indexing as well as slice/map indexing.",
	},
	"IndexListExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"IndexListExpr","x":<Expr>,"indices":[<Expr>]}`,
		Example: `{"kind":"IndexListExpr","x":{"kind":"Ident","name":"F"},"indices":[{"kind":"Ident","name":"int"},{"kind":"Ident","name":"string"}]}`,
		Notes:   "Multi-type-parameter instantiation: F[int, string]. Introduced in Go 1.18.",
	},
	"SliceExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"SliceExpr","x":<Expr>,"low":<Expr|null>,"high":<Expr|null>,"max":<Expr|null>,"slice3":bool}`,
		Example: `{"kind":"SliceExpr","x":{"kind":"Ident","name":"s"},"low":null,"high":{"kind":"BasicLit","tok":"INT","value":"3"}}`,
		Notes:   "slice3=true for 3-index slices: s[1:3:5]. max is the capacity bound.",
	},
	"StarExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"StarExpr","x":<Expr>}`,
		Example: `{"kind":"StarExpr","x":{"kind":"Ident","name":"p"}}`,
		Notes:   "Used for both pointer types (*T) and pointer dereferences (*p).",
	},
	"ParenExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"ParenExpr","x":<Expr>}`,
		Example: `{"kind":"ParenExpr","x":{"kind":"BinaryExpr","x":{"kind":"Ident","name":"a"},"op":"+","y":{"kind":"Ident","name":"b"}}}`,
		Notes:   "Wraps any parenthesized expression.",
	},
	"TypeAssertExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"TypeAssertExpr","x":<Expr>,"type":<Expr|null>}`,
		Example: `{"kind":"TypeAssertExpr","x":{"kind":"Ident","name":"v"},"type":{"kind":"Ident","name":"int"}}`,
		Notes:   "type=null for type switch assertions: v.(type).",
	},
	"CompositeLit": {
		Group:   "expressions",
		Schema:  `{"kind":"CompositeLit","type":<Expr|null>,"elts":[<Expr>]}`,
		Example: `{"kind":"CompositeLit","type":{"kind":"Ident","name":"Dog"},"elts":[{"kind":"KeyValueExpr","key":{"kind":"Ident","name":"Name"},"value":{"kind":"BasicLit","tok":"STRING","value":"\"Rex\""}}]}`,
		Notes:   "type may be null when the type is inferred from context.",
	},
	"KeyValueExpr": {
		Group:   "expressions",
		Schema:  `{"kind":"KeyValueExpr","key":<Expr>,"value":<Expr>}`,
		Example: `{"kind":"KeyValueExpr","key":{"kind":"Ident","name":"X"},"value":{"kind":"BasicLit","tok":"INT","value":"1"}}`,
		Notes:   "Used in composite literals and map literals.",
	},
	"FuncLit": {
		Group:   "expressions",
		Schema:  `{"kind":"FuncLit","type":<FuncType>,"body":<BlockStmt>}`,
		Example: `{"kind":"FuncLit","type":{"kind":"FuncType","params":[],"results":[]},"body":{"kind":"BlockStmt","list":[]}}`,
		Notes:   "Anonymous function literal.",
	},
	"Ellipsis": {
		Group:   "expressions",
		Schema:  `{"kind":"Ellipsis","elt":<Expr|null>}`,
		Example: `{"kind":"Ellipsis","elt":{"kind":"Ident","name":"int"}}`,
		Notes:   "Used in variadic parameter types: ...int. elt=null in variadic calls: f(args...).",
	},

	// ---- Types ----
	"ArrayType": {
		Group:   "types",
		Schema:  `{"kind":"ArrayType","len":<Expr|null>,"elt":<Expr>}`,
		Example: `{"kind":"ArrayType","len":{"kind":"BasicLit","tok":"INT","value":"3"},"elt":{"kind":"Ident","name":"int"}}`,
		Notes:   "len=null for slices ([]int). len present for arrays ([3]int).",
	},
	"MapType": {
		Group:   "types",
		Schema:  `{"kind":"MapType","key":<Expr>,"value":<Expr>}`,
		Example: `{"kind":"MapType","key":{"kind":"Ident","name":"string"},"value":{"kind":"Ident","name":"int"}}`,
		Notes:   "Represents map[K]V.",
	},
	"ChanType": {
		Group:   "types",
		Schema:  `{"kind":"ChanType","dir":"SEND|RECV|BOTH","value":<Expr>}`,
		Example: `{"kind":"ChanType","dir":"BOTH","value":{"kind":"Ident","name":"int"}}`,
		Notes:   "dir: SEND=chan<-, RECV=<-chan, BOTH=chan.",
	},
	"StructType": {
		Group:   "types",
		Schema:  `{"kind":"StructType","fields":[<Field>]}`,
		Example: `{"kind":"StructType","fields":[{"kind":"Field","names":["Name"],"type":{"kind":"Ident","name":"string"}}]}`,
		Notes:   "fields is a list of Field nodes.",
	},
	"InterfaceType": {
		Group:   "types",
		Schema:  `{"kind":"InterfaceType","methods":[<Field>]}`,
		Example: `{"kind":"InterfaceType","methods":[{"kind":"Field","names":["Read"],"type":{"kind":"FuncType","params":[{"kind":"Field","type":{"kind":"SliceType","elt":{"kind":"Ident","name":"byte"}}}],"results":[]}}]}`,
		Notes:   "methods is a FieldList. Embedded interfaces appear as fields with no names.",
	},
	"FuncType": {
		Group:   "types",
		Schema:  `{"kind":"FuncType","params":[<Field>],"results":[<Field>]}`,
		Example: `{"kind":"FuncType","params":[{"kind":"Field","names":["x","y"],"type":{"kind":"Ident","name":"int"}}],"results":[{"kind":"Field","type":{"kind":"Ident","name":"int"}}]}`,
		Notes:   "params and results are lists of Field nodes. Named results have names set.",
	},

	// ---- Field ----
	"Field": {
		Group:   "field",
		Schema:  `{"kind":"Field","names":["string"],"type":<Expr>,"tag":"string"}`,
		Example: `{"kind":"Field","names":["Name","Age"],"type":{"kind":"Ident","name":"string"},"tag":"json:\"name\""}`,
		Notes:   "names may be empty for anonymous (embedded) fields. tag is the raw struct tag string including backticks.",
	},

	// ---- Statements ----
	"BlockStmt": {
		Group:   "statements",
		Schema:  `{"kind":"BlockStmt","list":[<Stmt>]}`,
		Example: `{"kind":"BlockStmt","list":[{"kind":"ReturnStmt","results":[{"kind":"Ident","name":"x"}]}]}`,
		Notes:   "list may be empty for an empty block {}.",
	},
	"ExprStmt": {
		Group:   "statements",
		Schema:  `{"kind":"ExprStmt","x":<Expr>}`,
		Example: `{"kind":"ExprStmt","x":{"kind":"CallExpr","fun":{"kind":"Ident","name":"println"},"args":[]}}`,
		Notes:   "Wraps an expression used as a statement (typically a function call).",
	},
	"AssignStmt": {
		Group:   "statements",
		Schema:  `{"kind":"AssignStmt","lhs":[<Expr>],"tok":"=|:=","rhs":[<Expr>]}`,
		Example: `{"kind":"AssignStmt","lhs":[{"kind":"Ident","name":"x"}],"tok":":=","rhs":[{"kind":"BasicLit","tok":"INT","value":"1"}]}`,
		Notes:   "tok: = for assignment, := for short variable declaration. Other compound assignments (+= etc.) also use this node.",
	},
	"ReturnStmt": {
		Group:   "statements",
		Schema:  `{"kind":"ReturnStmt","results":[<Expr>]}`,
		Example: `{"kind":"ReturnStmt","results":[{"kind":"Ident","name":"x"},{"kind":"Ident","name":"nil"}]}`,
		Notes:   "results may be empty for bare return.",
	},
	"IfStmt": {
		Group:   "statements",
		Schema:  `{"kind":"IfStmt","init":<Stmt|null>,"cond":<Expr>,"body":<BlockStmt>,"else":<Stmt|null>}`,
		Example: `{"kind":"IfStmt","cond":{"kind":"BinaryExpr","x":{"kind":"Ident","name":"x"},"op":">","y":{"kind":"BasicLit","tok":"INT","value":"0"}},"body":{"kind":"BlockStmt","list":[]}}`,
		Notes:   "init and else are omitted when null. true/false/nil are Ident nodes.",
	},
	"ForStmt": {
		Group:   "statements",
		Schema:  `{"kind":"ForStmt","init":<Stmt|null>,"cond":<Expr|null>,"post":<Stmt|null>,"body":<BlockStmt>}`,
		Example: `{"kind":"ForStmt","init":{"kind":"AssignStmt","lhs":[{"kind":"Ident","name":"i"}],"tok":":=","rhs":[{"kind":"BasicLit","tok":"INT","value":"0"}]},"cond":{"kind":"BinaryExpr","x":{"kind":"Ident","name":"i"},"op":"<","y":{"kind":"BasicLit","tok":"INT","value":"10"}},"post":{"kind":"IncDecStmt","x":{"kind":"Ident","name":"i"},"tok":"++"},"body":{"kind":"BlockStmt","list":[]}}`,
		Notes:   "init, cond, post are all optional. Omit all for infinite loop.",
	},
	"RangeStmt": {
		Group:   "statements",
		Schema:  `{"kind":"RangeStmt","key":<Expr|null>,"value":<Expr|null>,"tok":"=|:=","x":<Expr>,"body":<BlockStmt>}`,
		Example: `{"kind":"RangeStmt","key":{"kind":"Ident","name":"i"},"value":{"kind":"Ident","name":"v"},"tok":":=","x":{"kind":"Ident","name":"items"},"body":{"kind":"BlockStmt","list":[]}}`,
		Notes:   "key and value can be blank identifier Ident{name:\"_\"} or omitted (null).",
	},
	"SwitchStmt": {
		Group:   "statements",
		Schema:  `{"kind":"SwitchStmt","init":<Stmt|null>,"tag":<Expr|null>,"body":<BlockStmt>}`,
		Example: `{"kind":"SwitchStmt","tag":{"kind":"Ident","name":"x"},"body":{"kind":"BlockStmt","list":[{"kind":"CaseClause","list":[{"kind":"BasicLit","tok":"INT","value":"1"}],"body":[]}]}}`,
		Notes:   "body contains CaseClause nodes. tag=null for tagless switch.",
	},
	"TypeSwitchStmt": {
		Group:   "statements",
		Schema:  `{"kind":"TypeSwitchStmt","init":<Stmt|null>,"assign":<Stmt>,"body":<BlockStmt>}`,
		Example: `{"kind":"TypeSwitchStmt","assign":{"kind":"ExprStmt","x":{"kind":"TypeAssertExpr","x":{"kind":"Ident","name":"v"},"type":null}},"body":{"kind":"BlockStmt","list":[]}}`,
		Notes:   "assign is usually an AssignStmt or ExprStmt containing a TypeAssertExpr with type=null.",
	},
	"CaseClause": {
		Group:   "statements",
		Schema:  `{"kind":"CaseClause","list":[<Expr>],"body":[<Stmt>]}`,
		Example: `{"kind":"CaseClause","list":[{"kind":"BasicLit","tok":"INT","value":"1"},{"kind":"BasicLit","tok":"INT","value":"2"}],"body":[{"kind":"ReturnStmt","results":[]}]}`,
		Notes:   "list=null (or empty) for the default case.",
	},
	"SelectStmt": {
		Group:   "statements",
		Schema:  `{"kind":"SelectStmt","body":<BlockStmt>}`,
		Example: `{"kind":"SelectStmt","body":{"kind":"BlockStmt","list":[{"kind":"CommClause","comm":{"kind":"AssignStmt","lhs":[{"kind":"Ident","name":"v"}],"tok":":=","rhs":[{"kind":"UnaryExpr","op":"<-","x":{"kind":"Ident","name":"ch"}}]},"body":[]}]}}`,
		Notes:   "body contains CommClause nodes.",
	},
	"CommClause": {
		Group:   "statements",
		Schema:  `{"kind":"CommClause","comm":<Stmt|null>,"body":[<Stmt>]}`,
		Example: `{"kind":"CommClause","comm":null,"body":[{"kind":"ReturnStmt","results":[]}]}`,
		Notes:   "comm=null for the default case. comm is a SendStmt or AssignStmt/ExprStmt wrapping a receive.",
	},
	"SendStmt": {
		Group:   "statements",
		Schema:  `{"kind":"SendStmt","chan":<Expr>,"value":<Expr>}`,
		Example: `{"kind":"SendStmt","chan":{"kind":"Ident","name":"ch"},"value":{"kind":"BasicLit","tok":"INT","value":"1"}}`,
		Notes:   "Represents ch <- value.",
	},
	"IncDecStmt": {
		Group:   "statements",
		Schema:  `{"kind":"IncDecStmt","x":<Expr>,"tok":"++|--"}`,
		Example: `{"kind":"IncDecStmt","x":{"kind":"Ident","name":"i"},"tok":"++"}`,
		Notes:   "tok is ++ or --.",
	},
	"DeclStmt": {
		Group:   "statements",
		Schema:  `{"kind":"DeclStmt","decl":<Decl>}`,
		Example: `{"kind":"DeclStmt","decl":{"kind":"VarDecl","specs":[{"kind":"ValueSpec","names":["x"],"type":{"kind":"Ident","name":"int"}}]}}`,
		Notes:   "Wraps a local variable or constant declaration inside a function body.",
	},
	"DeferStmt": {
		Group:   "statements",
		Schema:  `{"kind":"DeferStmt","call":<CallExpr>}`,
		Example: `{"kind":"DeferStmt","call":{"kind":"CallExpr","fun":{"kind":"SelectorExpr","x":{"kind":"Ident","name":"wg"},"sel":"Done"},"args":[]}}`,
		Notes:   "call must be a CallExpr.",
	},
	"GoStmt": {
		Group:   "statements",
		Schema:  `{"kind":"GoStmt","call":<CallExpr>}`,
		Example: `{"kind":"GoStmt","call":{"kind":"CallExpr","fun":{"kind":"Ident","name":"worker"},"args":[]}}`,
		Notes:   "call must be a CallExpr.",
	},
	"LabeledStmt": {
		Group:   "statements",
		Schema:  `{"kind":"LabeledStmt","label":"string","stmt":<Stmt>}`,
		Example: `{"kind":"LabeledStmt","label":"outer","stmt":{"kind":"ForStmt","body":{"kind":"BlockStmt","list":[]}}}`,
		Notes:   "label is the identifier string of the label.",
	},
	"BranchStmt": {
		Group:   "statements",
		Schema:  `{"kind":"BranchStmt","tok":"break|continue|goto|fallthrough","label":"string"}`,
		Example: `{"kind":"BranchStmt","tok":"break","label":"outer"}`,
		Notes:   "label is empty string for unlabeled branches.",
	},

	// ---- Declarations ----
	"FuncDecl": {
		Group:   "declarations",
		Schema:  `{"kind":"FuncDecl","recv":<FieldList|null>,"name":"string","type":<FuncType>,"body":<BlockStmt|null>}`,
		Example: `{"kind":"FuncDecl","name":"Add","type":{"kind":"FuncType","params":[{"kind":"Field","names":["a","b"],"type":{"kind":"Ident","name":"int"}}],"results":[{"kind":"Field","type":{"kind":"Ident","name":"int"}}]},"body":{"kind":"BlockStmt","list":[]}}`,
		Notes:   "recv is present for methods. body=null for function type declarations without body.",
	},
	"TypeDecl": {
		Group:   "declarations",
		Schema:  `{"kind":"TypeDecl","specs":[<TypeSpec>]}`,
		Example: `{"kind":"TypeDecl","specs":[{"kind":"TypeSpec","name":"Dog","type":{"kind":"StructType","fields":[]}}]}`,
		Notes:   "A type declaration block: type (...). Use TypeSpec for the individual type.",
	},
	"VarDecl": {
		Group:   "declarations",
		Schema:  `{"kind":"VarDecl","specs":[<ValueSpec>]}`,
		Example: `{"kind":"VarDecl","specs":[{"kind":"ValueSpec","names":["x","y"],"type":{"kind":"Ident","name":"int"},"values":[]}]}`,
		Notes:   "A var declaration block: var (...).",
	},
	"ConstDecl": {
		Group:   "declarations",
		Schema:  `{"kind":"ConstDecl","specs":[<ValueSpec>]}`,
		Example: `{"kind":"ConstDecl","specs":[{"kind":"ValueSpec","names":["MaxItems"],"values":[{"kind":"BasicLit","tok":"INT","value":"100"}]}]}`,
		Notes:   "A const declaration block: const (...).",
	},
	"ImportDecl": {
		Group:   "declarations",
		Schema:  `{"kind":"ImportDecl","specs":[<ImportSpec>]}`,
		Example: `{"kind":"ImportDecl","specs":[{"kind":"ImportSpec","path":"fmt"},{"kind":"ImportSpec","alias":"j","path":"encoding/json"}]}`,
		Notes:   "An import declaration block.",
	},

	// ---- Specs ----
	"TypeSpec": {
		Group:   "specs",
		Schema:  `{"kind":"TypeSpec","name":"string","type":<Expr>}`,
		Example: `{"kind":"TypeSpec","name":"Dog","type":{"kind":"StructType","fields":[{"kind":"Field","names":["Name"],"type":{"kind":"Ident","name":"string"}}]}}`,
		Notes:   "Individual type specification inside a type declaration.",
	},
	"ValueSpec": {
		Group:   "specs",
		Schema:  `{"kind":"ValueSpec","names":["string"],"type":<Expr|null>,"values":[<Expr>]}`,
		Example: `{"kind":"ValueSpec","names":["x","y"],"type":{"kind":"Ident","name":"int"},"values":[{"kind":"BasicLit","tok":"INT","value":"0"},{"kind":"BasicLit","tok":"INT","value":"1"}]}`,
		Notes:   "type may be null when inferred. values may be empty for var without initializer.",
	},
	"ImportSpec": {
		Group:   "specs",
		Schema:  `{"kind":"ImportSpec","alias":"string","path":"string"}`,
		Example: `{"kind":"ImportSpec","alias":"j","path":"encoding/json"}`,
		Notes:   "alias is empty for normal imports, \".\" for dot imports, \"_\" for blank imports.",
	},
}

// PrintGrammar prints the grammar listing to stdout.
func PrintGrammar(kindFilter string) {
	if kindFilter != "" {
		doc, ok := GrammarRegistry[kindFilter]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown kind: %q\n", kindFilter)
			os.Exit(1)
		}
		printKindDetail(kindFilter, doc)
		return
	}

	groups := []string{"expressions", "types", "field", "statements", "declarations", "specs"}
	groupKinds := map[string][]string{}
	for k, d := range GrammarRegistry {
		groupKinds[d.Group] = append(groupKinds[d.Group], k)
	}
	for _, g := range groups {
		kinds := groupKinds[g]
		sort.Strings(kinds)
		for _, k := range kinds {
			doc := GrammarRegistry[k]
			fmt.Printf("%-20s %s\n", k, doc.Schema)
		}
	}
}

func printKindDetail(name string, doc KindDoc) {
	fmt.Printf("%s\n\n", name)
	fmt.Printf("  Schema:\n    %s\n\n", formatSchema(doc.Schema))
	fmt.Printf("  Example:\n    %s\n", doc.Example)
	if doc.Notes != "" {
		fmt.Printf("\n  Notes: %s\n", strings.ReplaceAll(doc.Notes, "\n", "\n         "))
	}
}

func formatSchema(schema string) string {
	return schema
}
