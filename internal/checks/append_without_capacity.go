package checks

import (
	"go/ast"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// out := []T{} (or var out []T) filled by append in a range loop: the slice is copied each time it grows, though the
// loop's length is known.
func init() {
	register("append-without-capacity", func(m *core.Module) []core.Finding {
		var out []core.Finding
		for _, f := range m.Files {
			for _, d := range f.AST.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				empty := map[string]bool{}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					switch v := n.(type) {
					case *ast.AssignStmt: // out := []T{}
						for i, l := range v.Lhs {
							if id, ok := l.(*ast.Ident); ok && i < len(v.Rhs) {
								if lit, ok := v.Rhs[i].(*ast.CompositeLit); ok && len(lit.Elts) == 0 {
									if _, ok := lit.Type.(*ast.ArrayType); ok {
										empty[id.Name] = true
									}
								}
							}
						}
					case *ast.DeclStmt: // var out []T
						if g, ok := v.Decl.(*ast.GenDecl); ok {
							for _, s := range g.Specs {
								if vs, ok := s.(*ast.ValueSpec); ok && len(vs.Values) == 0 {
									if _, ok := vs.Type.(*ast.ArrayType); ok {
										for _, n := range vs.Names {
											empty[n.Name] = true
										}
									}
								}
							}
						}
					case *ast.RangeStmt:
						// Only an append that runs on EVERY iteration (a statement of the loop body itself): one inside an
						// if is a filter, whose final length is not known (287 such findings on two real modules, 2026-11-06).
						for _, stmt := range v.Body.List {
							as, ok := stmt.(*ast.AssignStmt)
							if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
								continue
							}
							id, ok := as.Lhs[0].(*ast.Ident)
							call, ok2 := as.Rhs[0].(*ast.CallExpr)
							if ok && ok2 && empty[id.Name] {
								if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "append" {
									out = append(out, m.At("append-without-capacity", core.Low, f, as,
										id.Name+" grows by append in a range loop, from an empty slice: it is copied each time it grows.",
										id.Name+" := make([]T, 0, len(<the ranged value>)) before the loop."))
									delete(empty, id.Name) // one finding for each slice
								}
							}
						}
					}
					return true
				})
			}
		}
		return out
	})
}
