// Namespace: goast/meta
// Node metadata computation — derived, read-only properties from go/ast nodes.
package meta

import (
	"go/ast"
	"go/token"
)

// Meta is a flat map of derived, read-only node properties.
type Meta map[string]interface{}

// Compute returns metadata for node within file.
func Compute(fset *token.FileSet, src []byte, node ast.Node, parent ast.Node, depth int) Meta {
	m := Meta{}

	// Universal fields
	pos := fset.Position(node.Pos())
	endPos := fset.Position(node.End())
	m["line"] = pos.Line
	m["end_line"] = endPos.Line
	m["col"] = pos.Column
	m["byte_offset"] = pos.Offset
	m["byte_end"] = endPos.Offset
	m["depth"] = depth
	if parent != nil {
		m["parent_kind"] = parentKindName(parent)
	}

	// Kind-specific fields
	switch n := node.(type) {
	case *ast.FuncDecl:
		computeFuncDecl(m, n)
	case *ast.TypeSpec:
		computeTypeSpec(m, n)
	case *ast.StructType:
		computeStructType(m, n)
	case *ast.InterfaceType:
		computeInterfaceType(m, n)
	case *ast.IfStmt:
		m["has_init"] = n.Init != nil
		m["has_else"] = n.Else != nil
		if n.Else != nil {
			_, m["else_is_if"] = n.Else.(*ast.IfStmt)
		} else {
			m["else_is_if"] = false
		}
		if n.Body != nil {
			m["body_stmt_count"] = len(n.Body.List)
		}
	case *ast.ForStmt:
		m["has_init"] = n.Init != nil
		if n.Body != nil {
			m["body_stmt_count"] = len(n.Body.List)
		}
	case *ast.RangeStmt:
		if n.Body != nil {
			m["body_stmt_count"] = len(n.Body.List)
		}
	case *ast.SwitchStmt:
		computeSwitchMeta(m, n.Body)
	case *ast.TypeSwitchStmt:
		computeTypeSwitchMeta(m, n.Body)
	case *ast.SelectStmt:
		computeSelectMeta(m, n.Body)
	case *ast.CallExpr:
		m["arg_count"] = len(n.Args)
		m["is_variadic_call"] = n.Ellipsis != token.NoPos
		m["callee"] = calleeString(n.Fun)
	case *ast.Field:
		exported := false
		if len(n.Names) > 0 {
			exported = ast.IsExported(n.Names[0].Name)
		}
		m["exported"] = exported
		m["is_embedded"] = len(n.Names) == 0
		m["has_tag"] = n.Tag != nil
		m["name_count"] = len(n.Names)
	}

	return m
}

// FileInfo returns file-level metadata.
func FileInfo(fset *token.FileSet, src []byte, file *ast.File) Meta {
	m := Meta{}
	m["package"] = file.Name.Name
	if fset != nil {
		f := fset.File(file.Pos())
		if f != nil {
			m["line_count"] = f.LineCount()
		}
	}
	if src != nil {
		lineCount := 1
		for _, b := range src {
			if b == '\n' {
				lineCount++
			}
		}
		m["line_count"] = lineCount
	}
	m["decl_count"] = len(file.Decls)

	funcCount := 0
	typeCount := 0
	importCount := 0
	hasInit := false

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			funcCount++
			if d.Name.Name == "init" && d.Recv == nil {
				hasInit = true
			}
		case *ast.GenDecl:
			switch d.Tok {
			case token.IMPORT:
				importCount += len(d.Specs)
			case token.TYPE:
				typeCount += len(d.Specs)
			}
		}
	}
	m["func_count"] = funcCount
	m["type_count"] = typeCount
	m["import_count"] = importCount
	m["has_init"] = hasInit
	return m
}

func computeFuncDecl(m Meta, n *ast.FuncDecl) {
	m["exported"] = ast.IsExported(n.Name.Name)
	m["is_method"] = n.Recv != nil
	if n.Recv != nil && len(n.Recv.List) > 0 {
		m["recv_type"] = recvTypeName(n.Recv.List[0])
	}
	if n.Type.Params != nil {
		m["param_count"] = len(n.Type.Params.List)
	} else {
		m["param_count"] = 0
	}
	if n.Type.Results != nil {
		m["result_count"] = len(n.Type.Results.List)
		m["has_error_return"] = hasErrorReturn(n.Type.Results)
	} else {
		m["result_count"] = 0
		m["has_error_return"] = false
	}
	if n.Body != nil {
		m["stmt_count"] = len(n.Body.List)
		m["cyclomatic_complexity"] = cyclomaticComplexity(n)
	}
	m["is_variadic"] = isVariadic(n.Type.Params)
}

func computeTypeSpec(m Meta, n *ast.TypeSpec) {
	m["exported"] = ast.IsExported(n.Name.Name)
	m["is_alias"] = n.Assign.IsValid()
	m["has_type_params"] = n.TypeParams != nil && len(n.TypeParams.List) > 0
	m["underlying_kind"] = underlyingKind(n.Type)
}

func computeStructType(m Meta, n *ast.StructType) {
	if n.Fields == nil {
		m["field_count"] = 0
		m["has_embedded"] = false
		m["exported_field_count"] = 0
		return
	}
	fieldCount := len(n.Fields.List)
	m["field_count"] = fieldCount
	hasEmbedded := false
	exportedCount := 0
	for _, f := range n.Fields.List {
		if len(f.Names) == 0 {
			hasEmbedded = true
		} else if len(f.Names) > 0 && ast.IsExported(f.Names[0].Name) {
			exportedCount++
		}
	}
	m["has_embedded"] = hasEmbedded
	m["exported_field_count"] = exportedCount
}

func computeInterfaceType(m Meta, n *ast.InterfaceType) {
	if n.Methods == nil {
		m["method_count"] = 0
		m["embed_count"] = 0
		m["is_empty"] = true
		return
	}
	methodCount := 0
	embedCount := 0
	for _, f := range n.Methods.List {
		if len(f.Names) == 0 {
			embedCount++
		} else {
			methodCount++
		}
	}
	m["method_count"] = methodCount
	m["embed_count"] = embedCount
	m["is_empty"] = methodCount == 0 && embedCount == 0
}

func computeSwitchMeta(m Meta, body *ast.BlockStmt) {
	if body == nil {
		m["case_count"] = 0
		m["has_default"] = false
		return
	}
	caseCount := 0
	hasDefault := false
	for _, stmt := range body.List {
		if cc, ok := stmt.(*ast.CaseClause); ok {
			caseCount++
			if cc.List == nil {
				hasDefault = true
			}
		}
	}
	m["case_count"] = caseCount
	m["has_default"] = hasDefault
}

func computeTypeSwitchMeta(m Meta, body *ast.BlockStmt) {
	computeSwitchMeta(m, body)
}

func computeSelectMeta(m Meta, body *ast.BlockStmt) {
	if body == nil {
		m["case_count"] = 0
		m["has_default"] = false
		return
	}
	caseCount := 0
	hasDefault := false
	for _, stmt := range body.List {
		if cc, ok := stmt.(*ast.CommClause); ok {
			caseCount++
			if cc.Comm == nil {
				hasDefault = true
			}
		}
	}
	m["case_count"] = caseCount
	m["has_default"] = hasDefault
}

func parentKindName(node ast.Node) string {
	switch node.(type) {
	case *ast.File:
		return "File"
	case *ast.FuncDecl:
		return "FuncDecl"
	case *ast.GenDecl:
		return "GenDecl"
	case *ast.BlockStmt:
		return "BlockStmt"
	case *ast.IfStmt:
		return "IfStmt"
	case *ast.ForStmt:
		return "ForStmt"
	case *ast.RangeStmt:
		return "RangeStmt"
	case *ast.SwitchStmt:
		return "SwitchStmt"
	case *ast.TypeSwitchStmt:
		return "TypeSwitchStmt"
	case *ast.SelectStmt:
		return "SelectStmt"
	case *ast.CaseClause:
		return "CaseClause"
	case *ast.CommClause:
		return "CommClause"
	case *ast.FuncLit:
		return "FuncLit"
	case *ast.CompositeLit:
		return "CompositeLit"
	case *ast.CallExpr:
		return "CallExpr"
	case *ast.StructType:
		return "StructType"
	case *ast.InterfaceType:
		return "InterfaceType"
	case *ast.FuncType:
		return "FuncType"
	case *ast.FieldList:
		return "FieldList"
	case *ast.Field:
		return "Field"
	default:
		return "Unknown"
	}
}

func recvTypeName(field *ast.Field) string {
	return exprString(field.Type)
}

func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.IndexExpr:
		return exprString(e.X) + "[...]"
	default:
		return ""
	}
}

func hasErrorReturn(results *ast.FieldList) bool {
	if results == nil || len(results.List) == 0 {
		return false
	}
	last := results.List[len(results.List)-1]
	if ident, ok := last.Type.(*ast.Ident); ok {
		return ident.Name == "error"
	}
	return false
}

func isVariadic(params *ast.FieldList) bool {
	if params == nil || len(params.List) == 0 {
		return false
	}
	last := params.List[len(params.List)-1]
	_, ok := last.Type.(*ast.Ellipsis)
	return ok
}

func cyclomaticComplexity(fn *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			if node.List != nil { // non-default
				complexity++
			}
		case *ast.CommClause:
			if node.Comm != nil { // non-default
				complexity++
			}
		case *ast.TypeSwitchStmt:
			complexity++
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}

func underlyingKind(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return "ident"
	case *ast.StarExpr:
		_ = e
		return "pointer"
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	case *ast.ArrayType:
		if e.Len == nil {
			return "slice"
		}
		return "array"
	case *ast.MapType:
		return "map"
	case *ast.ChanType:
		return "chan"
	case *ast.FuncType:
		return "func"
	default:
		return "unknown"
	}
}

func calleeString(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return exprString(f.X) + "." + f.Sel.Name
	default:
		return ""
	}
}
