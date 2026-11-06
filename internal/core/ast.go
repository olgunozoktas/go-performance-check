package core

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// Imports answers the local name of an import path in a file ("" when the file does not import it).
func Imports(f *ast.File, path string) string {
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p != path {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return path[strings.LastIndex(path, "/")+1:]
	}
	return ""
}

// IsCall answers whether call is pkg.Name(...) for the local name pkg and one of names.
func IsCall(call *ast.CallExpr, pkg string, names ...string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || pkg == "" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Name != pkg {
		return false
	}
	for _, n := range names {
		if sel.Sel.Name == n {
			return true
		}
	}
	return false
}

// Method answers the selector name of a method call x.Name(...), or "".
func Method(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	return ""
}

// IsConstant answers whether an expression holds no variable: literals, and + of literals.
func IsConstant(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return IsConstant(v.X)
	case *ast.BinaryExpr:
		return IsConstant(v.X) && IsConstant(v.Y)
	}
	return false
}

// ConstString answers the value of a constant string expression ("a", "a" + "b", ("a" + "b") + "c").
func ConstString(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.ParenExpr:
		return ConstString(v.X)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		a, ok1 := ConstString(v.X)
		b, ok2 := ConstString(v.Y)
		return a + b, ok1 && ok2
	}
	return "", false
}

// Builds answers whether an expression builds a string from a variable: "a" + x, fmt.Sprintf(...), x + y.
func Builds(e ast.Expr, fmtName string) bool {
	switch v := e.(type) {
	case *ast.BinaryExpr:
		return !IsConstant(v)
	case *ast.CallExpr:
		return IsCall(v, fmtName, "Sprintf", "Sprint")
	case *ast.ParenExpr:
		return Builds(v.X, fmtName)
	}
	return false
}

// Field answers a key's value in a composite literal T{Key: value}, or nil.
func Field(lit *ast.CompositeLit, key string) ast.Expr {
	for _, el := range lit.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
				return kv.Value
			}
		}
	}
	return nil
}

// TypeIs answers whether a composite literal's type is pkg.Name.
func TypeIs(lit *ast.CompositeLit, pkg, name string) bool {
	sel, ok := lit.Type.(*ast.SelectorExpr)
	if !ok || pkg == "" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg && sel.Sel.Name == name
}

// IsTrue answers whether an expression is the identifier true.
func IsTrue(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "true"
}

// Text answers the source text of a node.
func (f *File) Text(n ast.Node, m *Module) string {
	start, end := m.Fset.Position(n.Pos()).Offset, m.Fset.Position(n.End()).Offset
	if start < 0 || end > len(f.Src) || start > end {
		return ""
	}
	return string(f.Src[start:end])
}

// InLoop answers whether a node of a function body sits inside a for or range loop of that body (not of a closure
// inside it). The path is the chain of nodes from the function body down to the node.
func InLoop(path []ast.Node) bool {
	for i := len(path) - 1; i >= 0; i-- {
		switch path[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		}
	}
	return false
}

// Walk visits every node with its path from the root (the root is path[0]).
func Walk(root ast.Node, visit func(n ast.Node, path []ast.Node)) {
	var path []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			path = path[:len(path)-1]
			return false
		}
		path = append(path, n)
		visit(n, path)
		return true
	})
}
