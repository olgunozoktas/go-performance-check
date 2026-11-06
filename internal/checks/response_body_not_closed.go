package checks

import (
	"go/ast"
	"regexp"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

var responseCalls = map[string]bool{"Do": true, "Get": true, "Post": true, "Head": true, "PostForm": true}

var clientName = regexp.MustCompile(`(?i)client|http`)

// isRequest answers whether resp, err := <call> sends an HTTP request: http.Get/Post/Head/PostForm, or a Do, Get, Post,
// Head or PostForm method on a receiver named like a client. A map-like store's Get(id) is none of these.
func isRequest(call *ast.CallExpr, httpName string, f *core.File, m *core.Module) bool {
	if core.IsCall(call, httpName, "Get", "Post", "Head", "PostForm") {
		return true
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !responseCalls[sel.Sel.Name] {
		return false
	}
	// Do too needs a client-named receiver: chromedp's page.CaptureScreenshot().Do(ctx) is a browser command (2026-11-06).
	return clientName.MatchString(f.Text(sel.X, m))
}

// An HTTP response whose Body is never closed in its function: the connection is not reused, and it leaks.
func init() {
	register("response-body-not-closed", func(m *core.Module) []core.Finding {
		var out []core.Finding
		for _, f := range m.Files {
			httpName := core.Imports(f.AST, "net/http")
			if httpName == "" {
				continue
			}
			for _, d := range f.AST.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				body := f.Text(fd.Body, m)
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					as, ok := n.(*ast.AssignStmt)
					if !ok || len(as.Lhs) != 2 || len(as.Rhs) != 1 {
						return true
					}
					call, ok := as.Rhs[0].(*ast.CallExpr)
					resp, ok2 := as.Lhs[0].(*ast.Ident)
					errName, ok3 := as.Lhs[1].(*ast.Ident)
					if !ok || !ok2 || !ok3 || resp.Name == "_" || errName.Name != "err" || !isRequest(call, httpName, f, m) {
						return true
					}
					// The response is closed here, or handed on (returned, or passed to a function that may close it).
					// "return resp" hands it on; "return resp.StatusCode" does not (a field, not the response).
					handedOn := regexp.MustCompile(`return\s+` + regexp.QuoteMeta(resp.Name) + `\s*(?:[,)}\n]|$)`).MatchString(body)
					if !handedOn && !containsAny(body, resp.Name+".Body.Close()", "("+resp.Name+")", "("+resp.Name+",", ", "+resp.Name+")") {
						out = append(out, m.At("response-body-not-closed", core.High, f, as,
							"The response "+resp.Name+" is never closed in this function: its connection leaks and is not reused.",
							"After the error check: defer "+resp.Name+".Body.Close() (and read the body to the end to reuse the connection)."))
					}
					return true
				})
			}
		}
		return out
	})
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if len(p) > 0 && indexOf(s, p) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
