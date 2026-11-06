package checks

import (
	"go/ast"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// A defer inside a loop runs only when the function returns: files, locks and bodies pile up until then.
func init() {
	register("defer-in-loop", func(m *core.Module) []core.Finding {
		var out []core.Finding
		for _, f := range m.Files {
			core.Walk(f.AST, func(n ast.Node, path []ast.Node) {
				if d, ok := n.(*ast.DeferStmt); ok && core.InLoop(path) {
					out = append(out, m.At("defer-in-loop", core.Medium, f, d,
						"A defer inside a loop runs only when the function returns: each iteration's resource stays open until then.",
						"Move the loop's body into a function (or a closure that is called), so its defer runs at the end of each iteration."))
				}
			})
		}
		return out
	})
}
