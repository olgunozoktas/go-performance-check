// Package core holds what every performance check shares: a finding, its severity, the parsed source of a module, and the registry.
//
// ADDING A CHECK IS ONE FILE: a file in internal/checks that calls core.Register in its init function. The file being
// there and the check running are one fact. A check must change nothing: it reads the parsed source and answers findings.
package core

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Severity orders findings; -fail-on names the lowest one that makes the command exit 1.
type Severity int

const (
	Info Severity = iota
	Low
	Medium
	High
)

func (s Severity) String() string { return [...]string{"info", "low", "medium", "high"}[s] }

// ParseSeverity reads "high", "medium", "low" or "info".
func ParseSeverity(v string) (Severity, error) {
	for s := Info; s <= High; s++ {
		if s.String() == v {
			return s, nil
		}
	}
	return Info, fmt.Errorf("unknown severity %q: high, medium, low or info", v)
}

// Finding is one problem: what, where, and the exact remedy.
type Finding struct {
	Check    string   `json:"check"`
	Severity Severity `json:"-"`
	Level    string   `json:"severity"`
	Message  string   `json:"message"`
	Remedy   string   `json:"remedy"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
}

// File is one parsed Go file of the module.
type File struct {
	Path string // relative to the module root
	AST  *ast.File
	Src  []byte
}

// Module is what a check may read.
type Module struct {
	Root  string
	Fset  *token.FileSet
	Files []*File
}

// Check is one question about the module.
type Check interface {
	Name() string
	Run(m *Module) []Finding
}

var registry []Check

// Register adds a check; each check file calls it in init.
func Register(c Check) { registry = append(registry, c) }

// Checks answers every registered check, sorted by name.
func Checks() []Check {
	out := append([]Check(nil), registry...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// skipDir: vendored code, test data and dependencies are not the module's own code.
func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || name == "node_modules" || (strings.HasPrefix(name, ".") && name != ".")
}

// Load parses every .go file under root. Test files are left out unless withTests; a file that does not parse is skipped
// and counted (the report says so, so "no findings" never hides "read nothing").
func Load(root string, withTests bool) (*Module, int, error) {
	m := &Module{Root: root, Fset: token.NewFileSet()}
	skipped := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || (!withTests && strings.HasSuffix(path, "_test.go")) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		f, err := parser.ParseFile(m.Fset, rel, src, parser.ParseComments)
		if err != nil {
			skipped++
			return nil
		}
		m.Files = append(m.Files, &File{Path: filepath.ToSlash(rel), AST: f, Src: src})
		return nil
	})
	return m, skipped, err
}

// Ignored answers whether a line carries an ignore marker in a REAL comment of the parsed file: //nolint (all linters),
// //nolint:perf, or //go-performance-check:ignore <reason>. A marker inside a string literal is no comment and counts for
// nothing.
func (f *File) Ignored(fset *token.FileSet, line int) bool {
	for _, group := range f.AST.Comments {
		for _, c := range group.List {
			if fset.Position(c.Slash).Line != line || !strings.HasPrefix(c.Text, "//") {
				continue
			}
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if text == "nolint" || strings.HasPrefix(text, "nolint ") || strings.HasPrefix(text, "nolint:perf") ||
				(strings.HasPrefix(text, "nolint:") && strings.Contains(strings.Fields(text)[0], "perf")) ||
				strings.HasPrefix(text, "go-performance-check:ignore") {
				return true
			}
		}
	}
	return false
}

// Kept drops the findings on lines that carry an ignore marker.
func (m *Module) Kept(found []Finding) []Finding {
	byPath := map[string]*File{}
	for _, f := range m.Files {
		byPath[f.Path] = f
	}
	var out []Finding
	for _, x := range found {
		if f := byPath[x.File]; f != nil && x.Line > 0 && f.Ignored(m.Fset, x.Line) {
			continue
		}
		out = append(out, x)
	}
	return out
}

// At is a finding at a node of a file.
func (m *Module) At(check string, s Severity, f *File, n ast.Node, message, remedy string) Finding {
	return Finding{Check: check, Severity: s, Level: s.String(), Message: message, Remedy: remedy, File: f.Path, Line: m.Fset.Position(n.Pos()).Line}
}
