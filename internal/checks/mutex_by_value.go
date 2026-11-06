package checks

import (
	"go/ast"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// A method with a value receiver on a struct that holds a sync.Mutex or RWMutex: each call locks a COPY, so the lock
// protects nothing (go vet's copylocks finds some of these; this names the type and the method).
func init() {
	register("mutex-by-value", func(m *core.Module) []core.Finding {
		var out []core.Finding
		for _, f := range m.Files {
			syncName := core.Imports(f.AST, "sync")
			if syncName == "" {
				continue
			}
			locked := map[string]bool{}
			for _, d := range f.AST.Decls {
				g, ok := d.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, s := range g.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, fld := range st.Fields.List {
						if sel, ok := fld.Type.(*ast.SelectorExpr); ok {
							if id, ok := sel.X.(*ast.Ident); ok && id.Name == syncName && (sel.Sel.Name == "Mutex" || sel.Sel.Name == "RWMutex") {
								locked[ts.Name.Name] = true
							}
						}
					}
				}
			}
			for _, d := range f.AST.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
					continue
				}
				if id, ok := fd.Recv.List[0].Type.(*ast.Ident); ok && locked[id.Name] {
					out = append(out, m.At("mutex-by-value", core.Medium, f, fd.Recv,
						id.Name+"."+fd.Name.Name+" has a value receiver, and "+id.Name+" holds a mutex: the method locks a copy.",
						"Use a pointer receiver: func (x *"+id.Name+") "+fd.Name.Name+"(…)."))
				}
			}
		}
		return out
	})
}
