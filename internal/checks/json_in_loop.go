package checks

import (
	"go/ast"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// json.Marshal or Unmarshal inside a loop: reflection and an allocation for each item.
func init() {
	register("json-in-loop", func(m *core.Module) []core.Finding {
		var out []core.Finding
		for _, f := range m.Files {
			name := core.Imports(f.AST, "encoding/json")
			if name == "" {
				continue
			}
			core.Walk(f.AST, func(n ast.Node, path []ast.Node) {
				if c, ok := n.(*ast.CallExpr); ok && core.IsCall(c, name, "Marshal", "Unmarshal", "MarshalIndent") && core.InLoop(path) {
					out = append(out, m.At("json-in-loop", core.Low, f, c,
						"json."+core.Method(c)+" runs on each iteration of a loop: reflection and an allocation for each item.",
						"Encode the whole slice once, or stream with one json.NewEncoder(w) / json.NewDecoder(r) made before the loop."))
				}
			})
		}
		return out
	})
}
