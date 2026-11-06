package checks

import (
	"go/ast"
	"go/token"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// s += "..." in a loop copies the whole string on each iteration (quadratic time).
func init() {
	register("string-concat-in-loop", func(m *core.Module) []core.Finding {
		var out []core.Finding
		for _, f := range m.Files {
			fmtName := core.Imports(f.AST, "fmt")
			core.Walk(f.AST, func(n ast.Node, path []ast.Node) {
				as, ok := n.(*ast.AssignStmt)
				if !ok || as.Tok != token.ADD_ASSIGN || len(as.Rhs) != 1 || !core.InLoop(path) {
					return
				}
				stringy := false
				ast.Inspect(as.Rhs[0], func(x ast.Node) bool {
					if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						stringy = true
					}
					if c, ok := x.(*ast.CallExpr); ok && core.IsCall(c, fmtName, "Sprintf", "Sprint", "Sprintln") {
						stringy = true
					}
					return true
				})
				if stringy {
					out = append(out, m.At("string-concat-in-loop", core.Low, f, as,
						"A string grows with += in a loop: each iteration copies the whole string.",
						"Use a strings.Builder (b.WriteString(…)), or collect the parts and strings.Join them once."))
				}
			})
		}
		return out
	})
}
