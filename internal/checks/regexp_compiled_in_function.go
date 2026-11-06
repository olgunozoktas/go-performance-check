package checks

import (
	"go/ast"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// regexp.MustCompile or Compile inside a function: the pattern is compiled again on every call (high inside a loop).
func init() {
	register("regexp-compiled-in-function", func(m *core.Module) []core.Finding {
		var out []core.Finding
		for _, f := range m.Files {
			name := core.Imports(f.AST, "regexp")
			if name == "" {
				continue
			}
			for _, d := range f.AST.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				core.Walk(fd.Body, func(n ast.Node, path []ast.Node) {
					c, ok := n.(*ast.CallExpr)
					if !ok || !core.IsCall(c, name, "MustCompile", "Compile", "MustCompilePOSIX", "CompilePOSIX") || len(c.Args) == 0 || !core.IsConstant(c.Args[0]) {
						return
					}
					s, where := core.Medium, "on every call"
					if core.InLoop(path) {
						s, where = core.High, "on every iteration of a loop"
					}
					out = append(out, m.At("regexp-compiled-in-function", s, f, c, "A constant regexp is compiled "+where+".",
						"Compile it once at package level: var re = regexp.MustCompile(`…`)."))
				})
			}
		}
		return out
	})
}
