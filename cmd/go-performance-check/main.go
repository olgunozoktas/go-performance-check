// Command go-performance-check checks a Go module for performance problems, without AI and without changing anything.
//
//	go-performance-check [check] [-fail-on medium] [-only check,check] [-json] [-tests] [-list] [module folder]
//
// Exit 0: no finding at or above -fail-on; 1: a finding; 2: a wrong flag or a folder with no go.mod.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	_ "github.com/olgunozoktas/go-performance-check/internal/checks"
	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// go-performance-check bench -save base.json [-bench .] [-count 5] [module folder]
// go-performance-check bench -compare base.json [-threshold 10] [module folder]   (exit 1 on a regression)
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func flagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "bench" {
		return bench(args[1:], stdout, stderr, goCommand)
	}
	if len(args) > 0 && args[0] == "check" {
		args = args[1:]
	}
	fs := flagSet("go-performance-check", stderr)
	failOn := fs.String("fail-on", "medium", "the lowest severity that exits 1: high, medium, low or info")
	only := fs.String("only", "", "comma-separated check names")
	asJSON := fs.Bool("json", false, "the report as JSON")
	withTests := fs.Bool("tests", false, "also read _test.go files")
	list := fs.Bool("list", false, "list the checks and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	floor, err := core.ParseSeverity(*failOn)
	if err != nil {
		fmt.Fprintln(stderr, "go-performance-check:", err)
		return 2
	}
	checks := core.Checks()
	if *list {
		for _, c := range checks {
			fmt.Fprintln(stdout, c.Name())
		}
		return 0
	}
	if *only != "" {
		names := strings.Split(*only, ",")
		checks = slices.DeleteFunc(checks, func(c core.Check) bool { return !slices.Contains(names, c.Name()) })
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		fmt.Fprintln(stderr, "go-performance-check: no go.mod in", root)
		return 2
	}
	m, skipped, err := core.Load(root, *withTests)
	if err != nil {
		fmt.Fprintln(stderr, "go-performance-check:", err)
		return 2
	}
	var findings []core.Finding
	for _, c := range checks {
		findings = append(findings, m.Kept(c.Run(m))...)
	}
	slices.SortStableFunc(findings, func(a, b core.Finding) int {
		if a.Severity != b.Severity {
			return int(b.Severity - a.Severity)
		}
		return strings.Compare(fmt.Sprintf("%s:%06d", a.File, a.Line), fmt.Sprintf("%s:%06d", b.File, b.Line))
	})
	counts := map[string]int{"high": 0, "medium": 0, "low": 0, "info": 0}
	fail := false
	for _, f := range findings {
		counts[f.Level]++
		fail = fail || f.Severity >= floor
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(map[string]any{"checks": len(checks), "files_read": len(m.Files), "files_not_parsed": skipped, "counts": counts, "findings": findings})
	} else {
		for _, f := range findings {
			where := f.File
			if f.Line > 0 {
				where = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			fmt.Fprintf(stdout, "%-6s %s  [%s] %s\n       Fix: %s\n\n", strings.ToUpper(f.Level), where, f.Check, f.Message, f.Remedy)
		}
		fmt.Fprintf(stdout, "%d checks, %d files read (%d did not parse). Findings: %d high, %d medium, %d low, %d info.\n",
			len(checks), len(m.Files), skipped, counts["high"], counts["medium"], counts["low"], counts["info"])
	}
	if fail {
		return 1
	}
	return 0
}
