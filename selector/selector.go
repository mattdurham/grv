// Namespace: goast/selector
// Path navigation: JSON path → go/ast node traversal.
package selector

import (
	"fmt"
	"go/ast"
)

// PathStep is one step in a selector path.
type PathStep struct {
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"`
	Recv  string `json:"recv,omitempty"`
	Index *int   `json:"index,omitempty"`
}

// ParentContext records how the target node is held by its parent.
type ParentContext struct {
	Parent    ast.Node
	FieldName string
	Index     int // -1 for scalar fields
}

// NavigateError is returned when a path step cannot be resolved.
type NavigateError struct {
	AtStep    int
	Step      PathStep
	Available []string
}

func (e *NavigateError) Error() string {
	return fmt.Sprintf("step %d (%s): not found; available: %v", e.AtStep, e.Step.Kind, e.Available)
}

// Navigate walks the file AST following steps, returning the target node and
// how it is held by its parent.
func Navigate(file *ast.File, steps []PathStep) (ast.Node, ParentContext, error) {
	var current ast.Node = file
	var parent ParentContext

	for i, step := range steps {
		next, ctx, err := applyStep(current, step, i)
		if err != nil {
			return nil, ParentContext{}, err
		}
		parent = ctx
		current = next
	}
	return current, parent, nil
}

func applyStep(current ast.Node, step PathStep, stepIdx int) (ast.Node, ParentContext, error) {
	switch step.Kind {
	case "FuncDecl":
		return stepFuncDecl(current, step, stepIdx)
	case "TypeDecl":
		return stepTypeDecl(current, step, stepIdx)
	case "TypeSpec":
		return stepTypeSpec(current, step, stepIdx)
	case "VarDecl":
		return stepVarDecl(current, step, stepIdx)
	case "ConstDecl":
		return stepConstDecl(current, step, stepIdx)
	case "ImportDecl":
		return stepImportDecl(current, step, stepIdx)
	case "StructType":
		return stepStructType(current, step, stepIdx)
	case "InterfaceType":
		return stepInterfaceType(current, step, stepIdx)
	case "Field":
		return stepField(current, step, stepIdx)
	case "Body":
		return stepBody(current, step, stepIdx)
	case "Params":
		return stepParams(current, step, stepIdx)
	case "Results":
		return stepResults(current, step, stepIdx)
	case "IfStmt":
		return stepIndexedStmtKind[*ast.IfStmt](current, step, stepIdx, "IfStmt")
	case "ForStmt":
		return stepIndexedStmtKind[*ast.ForStmt](current, step, stepIdx, "ForStmt")
	case "RangeStmt":
		return stepIndexedStmtKind[*ast.RangeStmt](current, step, stepIdx, "RangeStmt")
	case "SwitchStmt":
		return stepIndexedStmtKind[*ast.SwitchStmt](current, step, stepIdx, "SwitchStmt")
	case "TypeSwitchStmt":
		return stepIndexedStmtKind[*ast.TypeSwitchStmt](current, step, stepIdx, "TypeSwitchStmt")
	case "SelectStmt":
		return stepIndexedStmtKind[*ast.SelectStmt](current, step, stepIdx, "SelectStmt")
	case "CaseClause":
		return stepCaseClause(current, step, stepIdx)
	case "CommClause":
		return stepCommClause(current, step, stepIdx)
	case "AssignStmt":
		return stepIndexedStmtKind[*ast.AssignStmt](current, step, stepIdx, "AssignStmt")
	case "ReturnStmt":
		return stepIndexedStmtKind[*ast.ReturnStmt](current, step, stepIdx, "ReturnStmt")
	case "ExprStmt":
		return stepIndexedStmtKind[*ast.ExprStmt](current, step, stepIdx, "ExprStmt")
	case "GoStmt":
		return stepIndexedStmtKind[*ast.GoStmt](current, step, stepIdx, "GoStmt")
	case "DeferStmt":
		return stepIndexedStmtKind[*ast.DeferStmt](current, step, stepIdx, "DeferStmt")
	case "Stmt":
		return stepStmtByIndex(current, step, stepIdx)
	case "Cond":
		return stepCond(current, step, stepIdx)
	case "Init":
		return stepInit(current, step, stepIdx)
	case "Post":
		return stepPost(current, step, stepIdx)
	case "Else":
		return stepElse(current, step, stepIdx)
	case "Tag":
		return stepTag(current, step, stepIdx)
	case "Lhs":
		return stepLhsRhs(current, step, stepIdx, true)
	case "Rhs":
		return stepLhsRhs(current, step, stepIdx, false)
	case "Key":
		return stepKey(current, step, stepIdx)
	case "Value":
		return stepValue(current, step, stepIdx)
	case "X":
		return stepX(current, step, stepIdx)
	case "Y":
		return stepY(current, step, stepIdx)
	case "Fun":
		return stepFun(current, step, stepIdx)
	case "Args":
		return stepArgs(current, step, stepIdx)
	case "Sel":
		return stepSel(current, step, stepIdx)
	case "Elts":
		return stepElts(current, step, stepIdx)
	default:
		return nil, ParentContext{}, &NavigateError{AtStep: stepIdx, Step: step}
	}
}

// stepFuncDecl finds FuncDecl in file.Decls by name and optional recv.
func stepFuncDecl(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	file, ok := current.(*ast.File)
	if !ok {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	var available []string
	for i, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Name.Name == step.Name {
			if step.Recv == "" || recvMatches(fd, step.Recv) {
				return fd, ParentContext{Parent: file, FieldName: "Decls", Index: i}, nil
			}
		}
		name := fd.Name.Name
		if fd.Recv != nil && len(fd.Recv.List) > 0 {
			name = recvTypeName(fd.Recv.List[0]) + "." + name
		}
		available = append(available, fmt.Sprintf("FuncDecl(%s)", name))
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

func recvMatches(fd *ast.FuncDecl, recv string) bool {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return false
	}
	return recvTypeName(fd.Recv.List[0]) == recv
}

func recvTypeName(field *ast.Field) string {
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return "*" + id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func stepTypeDecl(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	file, ok := current.(*ast.File)
	if !ok {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	var available []string
	for i, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if gd.Tok.String() == "type" {
			return gd, ParentContext{Parent: file, FieldName: "Decls", Index: i}, nil
		}
		available = append(available, fmt.Sprintf("GenDecl(%s)", gd.Tok))
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

func stepTypeSpec(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	// When navigating from a File directly, search all type GenDecls.
	if file, ok := current.(*ast.File); ok {
		var available []string
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok.String() != "type" {
				continue
			}
			for i, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if step.Name == "" || ts.Name.Name == step.Name {
					return ts, ParentContext{Parent: gd, FieldName: "Specs", Index: i}, nil
				}
				available = append(available, fmt.Sprintf("TypeSpec(%s)", ts.Name.Name))
			}
		}
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
	}

	var specs []ast.Spec
	var parent ast.Node
	switch n := current.(type) {
	case *ast.GenDecl:
		specs = n.Specs
		parent = n
	default:
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	var available []string
	for i, spec := range specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if step.Name == "" || ts.Name.Name == step.Name {
			return ts, ParentContext{Parent: parent, FieldName: "Specs", Index: i}, nil
		}
		available = append(available, fmt.Sprintf("TypeSpec(%s)", ts.Name.Name))
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

func stepVarDecl(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	return stepGenDeclByTok(current, step, idx, "var")
}

func stepConstDecl(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	return stepGenDeclByTok(current, step, idx, "const")
}

func stepImportDecl(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	return stepGenDeclByTok(current, step, idx, "import")
}

func stepGenDeclByTok(current ast.Node, step PathStep, idx int, tok string) (ast.Node, ParentContext, error) {
	file, ok := current.(*ast.File)
	if !ok {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	var available []string
	for i, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		if gd.Tok.String() == tok {
			return gd, ParentContext{Parent: file, FieldName: "Decls", Index: i}, nil
		}
		available = append(available, fmt.Sprintf("GenDecl(%s)", gd.Tok))
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

func stepStructType(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.TypeSpec:
		if st, ok := n.Type.(*ast.StructType); ok {
			return st, ParentContext{Parent: n, FieldName: "Type", Index: -1}, nil
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepInterfaceType(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.TypeSpec:
		if it, ok := n.Type.(*ast.InterfaceType); ok {
			return it, ParentContext{Parent: n, FieldName: "Type", Index: -1}, nil
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepField(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	var fields []*ast.Field
	var parent ast.Node
	switch n := current.(type) {
	case *ast.StructType:
		if n.Fields != nil {
			fields = n.Fields.List
			parent = n.Fields
		}
	case *ast.InterfaceType:
		if n.Methods != nil {
			fields = n.Methods.List
			parent = n.Methods
		}
	case *ast.FieldList:
		fields = n.List
		parent = n
	default:
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}

	target := 0
	if step.Index != nil {
		target = *step.Index
	}
	var available []string
	fieldIdx := 0
	for i, field := range fields {
		if step.Name != "" {
			for _, name := range field.Names {
				if name.Name == step.Name {
					return field, ParentContext{Parent: parent, FieldName: "List", Index: i}, nil
				}
			}
			if len(field.Names) > 0 {
				available = append(available, fmt.Sprintf("Field(%s)", field.Names[0].Name))
			}
		} else {
			if fieldIdx == target {
				return field, ParentContext{Parent: parent, FieldName: "List", Index: i}, nil
			}
			available = append(available, fmt.Sprintf("Field[%d]", fieldIdx))
			fieldIdx++
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

func stepBody(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.FuncDecl:
		if n.Body == nil {
			return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: []string{"(no body)"}}
		}
		return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
	case *ast.FuncLit:
		return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
	case *ast.SwitchStmt:
		return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
	case *ast.TypeSwitchStmt:
		return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
	case *ast.SelectStmt:
		return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
	case *ast.ForStmt:
		return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
	case *ast.RangeStmt:
		return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
	case *ast.IfStmt:
		return n.Body, ParentContext{Parent: n, FieldName: "Body", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepParams(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.FuncDecl:
		if n.Type.Params != nil {
			return n.Type.Params, ParentContext{Parent: n.Type, FieldName: "Params", Index: -1}, nil
		}
	case *ast.FuncType:
		if n.Params != nil {
			return n.Params, ParentContext{Parent: n, FieldName: "Params", Index: -1}, nil
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepResults(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.FuncDecl:
		if n.Type.Results != nil {
			return n.Type.Results, ParentContext{Parent: n.Type, FieldName: "Results", Index: -1}, nil
		}
	case *ast.FuncType:
		if n.Results != nil {
			return n.Results, ParentContext{Parent: n, FieldName: "Results", Index: -1}, nil
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

// stepIndexedStmtKind finds the Nth statement of a specific ast type in a block list.
func stepIndexedStmtKind[T ast.Stmt](current ast.Node, step PathStep, idx int, kindName string) (ast.Node, ParentContext, error) {
	list, listParent, err := getStmtList(current, idx, step)
	if err != nil {
		return nil, ParentContext{}, err
	}

	target := 0
	if step.Index != nil {
		target = *step.Index
	}

	count := 0
	var available []string
	for i, stmt := range list {
		if _, ok := stmt.(T); ok {
			if count == target {
				return stmt, ParentContext{Parent: listParent, FieldName: "List", Index: i}, nil
			}
			available = append(available, fmt.Sprintf("%s[%d]", kindName, count))
			count++
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

func getStmtList(current ast.Node, idx int, step PathStep) ([]ast.Stmt, ast.Node, error) {
	switch n := current.(type) {
	case *ast.BlockStmt:
		return n.List, n, nil
	case *ast.CaseClause:
		return n.Body, n, nil
	case *ast.CommClause:
		return n.Body, n, nil
	default:
		return nil, nil, &NavigateError{AtStep: idx, Step: step}
	}
}

func stepStmtByIndex(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	list, listParent, err := getStmtList(current, idx, step)
	if err != nil {
		return nil, ParentContext{}, err
	}
	target := 0
	if step.Index != nil {
		target = *step.Index
	}
	if target < 0 {
		target = len(list) + target
	}
	if target < 0 || target >= len(list) {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step,
			Available: []string{fmt.Sprintf("0..%d", len(list)-1)}}
	}
	return list[target], ParentContext{Parent: listParent, FieldName: "List", Index: target}, nil
}

func stepCaseClause(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	body, ok := current.(*ast.BlockStmt)
	if !ok {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	target := 0
	if step.Index != nil {
		target = *step.Index
	}
	count := 0
	var available []string
	for i, stmt := range body.List {
		if cc, ok := stmt.(*ast.CaseClause); ok {
			if count == target {
				return cc, ParentContext{Parent: body, FieldName: "List", Index: i}, nil
			}
			available = append(available, fmt.Sprintf("CaseClause[%d]", count))
			count++
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

func stepCommClause(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	body, ok := current.(*ast.BlockStmt)
	if !ok {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	target := 0
	if step.Index != nil {
		target = *step.Index
	}
	count := 0
	var available []string
	for i, stmt := range body.List {
		if cc, ok := stmt.(*ast.CommClause); ok {
			if count == target {
				return cc, ParentContext{Parent: body, FieldName: "List", Index: i}, nil
			}
			available = append(available, fmt.Sprintf("CommClause[%d]", count))
			count++
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step, Available: available}
}

func stepCond(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.IfStmt:
		return n.Cond, ParentContext{Parent: n, FieldName: "Cond", Index: -1}, nil
	case *ast.ForStmt:
		if n.Cond != nil {
			return n.Cond, ParentContext{Parent: n, FieldName: "Cond", Index: -1}, nil
		}
	case *ast.SwitchStmt:
		if n.Tag != nil {
			return n.Tag, ParentContext{Parent: n, FieldName: "Tag", Index: -1}, nil
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepInit(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.IfStmt:
		if n.Init != nil {
			return n.Init, ParentContext{Parent: n, FieldName: "Init", Index: -1}, nil
		}
	case *ast.ForStmt:
		if n.Init != nil {
			return n.Init, ParentContext{Parent: n, FieldName: "Init", Index: -1}, nil
		}
	case *ast.SwitchStmt:
		if n.Init != nil {
			return n.Init, ParentContext{Parent: n, FieldName: "Init", Index: -1}, nil
		}
	case *ast.TypeSwitchStmt:
		if n.Init != nil {
			return n.Init, ParentContext{Parent: n, FieldName: "Init", Index: -1}, nil
		}
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepPost(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	if n, ok := current.(*ast.ForStmt); ok && n.Post != nil {
		return n.Post, ParentContext{Parent: n, FieldName: "Post", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepElse(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	if n, ok := current.(*ast.IfStmt); ok && n.Else != nil {
		return n.Else, ParentContext{Parent: n, FieldName: "Else", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepTag(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	if n, ok := current.(*ast.SwitchStmt); ok && n.Tag != nil {
		return n.Tag, ParentContext{Parent: n, FieldName: "Tag", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepLhsRhs(current ast.Node, step PathStep, idx int, lhs bool) (ast.Node, ParentContext, error) {
	assign, ok := current.(*ast.AssignStmt)
	if !ok {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	var list []ast.Expr
	var fieldName string
	if lhs {
		list = assign.Lhs
		fieldName = "Lhs"
	} else {
		list = assign.Rhs
		fieldName = "Rhs"
	}
	target := 0
	if step.Index != nil {
		target = *step.Index
	}
	if target < 0 || target >= len(list) {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step,
			Available: []string{fmt.Sprintf("0..%d", len(list)-1)}}
	}
	return list[target], ParentContext{Parent: assign, FieldName: fieldName, Index: target}, nil
}

func stepKey(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.RangeStmt:
		if n.Key != nil {
			return n.Key, ParentContext{Parent: n, FieldName: "Key", Index: -1}, nil
		}
	case *ast.KeyValueExpr:
		return n.Key, ParentContext{Parent: n, FieldName: "Key", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepValue(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.RangeStmt:
		if n.Value != nil {
			return n.Value, ParentContext{Parent: n, FieldName: "Value", Index: -1}, nil
		}
	case *ast.KeyValueExpr:
		return n.Value, ParentContext{Parent: n, FieldName: "Value", Index: -1}, nil
	case *ast.SendStmt:
		return n.Value, ParentContext{Parent: n, FieldName: "Value", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepX(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	switch n := current.(type) {
	case *ast.BinaryExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.UnaryExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.StarExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.ParenExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.SelectorExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.IndexExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.IndexListExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.SliceExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.TypeAssertExpr:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.ExprStmt:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.IncDecStmt:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	case *ast.RangeStmt:
		return n.X, ParentContext{Parent: n, FieldName: "X", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepY(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	if n, ok := current.(*ast.BinaryExpr); ok {
		return n.Y, ParentContext{Parent: n, FieldName: "Y", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepFun(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	if n, ok := current.(*ast.CallExpr); ok {
		return n.Fun, ParentContext{Parent: n, FieldName: "Fun", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepArgs(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	call, ok := current.(*ast.CallExpr)
	if !ok {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	target := 0
	if step.Index != nil {
		target = *step.Index
	}
	if target < 0 || target >= len(call.Args) {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step,
			Available: []string{fmt.Sprintf("0..%d", len(call.Args)-1)}}
	}
	return call.Args[target], ParentContext{Parent: call, FieldName: "Args", Index: target}, nil
}

func stepSel(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	if n, ok := current.(*ast.SelectorExpr); ok {
		return n.Sel, ParentContext{Parent: n, FieldName: "Sel", Index: -1}, nil
	}
	return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
}

func stepElts(current ast.Node, step PathStep, idx int) (ast.Node, ParentContext, error) {
	cl, ok := current.(*ast.CompositeLit)
	if !ok {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step}
	}
	target := 0
	if step.Index != nil {
		target = *step.Index
	}
	if target < 0 || target >= len(cl.Elts) {
		return nil, ParentContext{}, &NavigateError{AtStep: idx, Step: step,
			Available: []string{fmt.Sprintf("0..%d", len(cl.Elts)-1)}}
	}
	return cl.Elts[target], ParentContext{Parent: cl, FieldName: "Elts", Index: target}, nil
}
