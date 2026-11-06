package checks

import (
	"go/ast"
	"strings"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// An http.Client made for one call loses its connection pool each time; http.Get and friends use the default client,
// which has no timeout, so a slow server holds the goroutine forever. A client that a constructor makes and STORES (a
// field, a struct literal, a return value) is the right way, and not reported (measured on two real modules, 2026-11-06).

// perCall answers whether a client literal at the end of path is made for one call: in a loop, or put in a local variable
// of a function that is not a constructor (New…, new…, init, main). Stored in a field, a struct literal or returned: no.
func perCall(path []ast.Node, builder bool) bool {
	if core.InLoop(path) {
		return true
	}
	i := len(path) - 2
	if i >= 0 {
		if u, ok := path[i].(*ast.UnaryExpr); ok && u.Op.String() == "&" {
			i--
		}
	}
	if i < 0 {
		return false
	}
	switch p := path[i].(type) {
	case *ast.KeyValueExpr, *ast.CompositeLit, *ast.ReturnStmt:
		return false
	case *ast.AssignStmt:
		for _, l := range p.Lhs {
			if _, field := l.(*ast.SelectorExpr); field {
				return false
			}
		}
		return !builder
	case *ast.ValueSpec:
		return !builder
	}
	return false
}
func init() {
	register("http-client-per-call", func(m *core.Module) []core.Finding {
		var out []core.Finding
		for _, f := range m.Files {
			name := core.Imports(f.AST, "net/http")
			if name == "" {
				continue
			}
			for _, d := range f.AST.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				builder := strings.HasPrefix(fd.Name.Name, "New") || strings.HasPrefix(fd.Name.Name, "new") ||
					fd.Name.Name == "init" || fd.Name.Name == "main"
				core.Walk(fd.Body, func(n ast.Node, path []ast.Node) {
					switch v := n.(type) {
					case *ast.CompositeLit:
						if (core.TypeIs(v, name, "Client") || core.TypeIs(v, name, "Transport")) && perCall(path, builder) {
							out = append(out, m.At("http-client-per-call", core.Medium, f, v,
								"An http."+v.Type.(*ast.SelectorExpr).Sel.Name+" is made inside a function: each call starts without the connection pool.",
								"Make one client (with a Timeout) at package level or in the service's struct, and reuse it."))
						}
					case *ast.CallExpr:
						if core.IsCall(v, name, "Get", "Post", "Head", "PostForm") {
							out = append(out, m.At("http-client-per-call", core.Low, f, v,
								"http."+core.Method(v)+" uses the default client, which has no timeout.",
								"Use a client with a Timeout: client := &http.Client{Timeout: 10 * time.Second}, made once."))
						}
					}
				})
			}
		}
		return out
	})
}
