package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Result is one benchmark's mean over its runs.
type Result struct {
	Runs     int     `json:"runs"`
	NsOp     float64 `json:"ns_op"`
	BytesOp  float64 `json:"b_op"`
	AllocsOp float64 `json:"allocs_op"`
}

// A line of `go test -bench -benchmem`: BenchmarkName-8  1000  1234 ns/op  56 B/op  2 allocs/op
var benchLine = regexp.MustCompile(`^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op(?:\s+([\d.]+) B/op)?(?:\s+([\d.]+) allocs/op)?`)

// Parse reads `go test -bench` output: each benchmark's mean ns/op, B/op and allocs/op. Pure.
func Parse(r io.Reader) map[string]Result {
	sums := map[string]*Result{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		m := benchLine.FindStringSubmatch(strings.TrimSpace(sc.Text()))
		if m == nil {
			continue
		}
		s := sums[m[1]]
		if s == nil {
			s = &Result{}
			sums[m[1]] = s
		}
		s.Runs++
		s.NsOp += num(m[2])
		s.BytesOp += num(m[3])
		s.AllocsOp += num(m[4])
	}
	out := map[string]Result{}
	for name, s := range sums {
		n := float64(s.Runs)
		out[name] = Result{Runs: s.Runs, NsOp: s.NsOp / n, BytesOp: s.BytesOp / n, AllocsOp: s.AllocsOp / n}
	}
	return out
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// Change is one benchmark's change from the base, in percent (positive: slower or bigger).
type Change struct {
	Name                      string
	Time, Bytes, Allocs       float64
	Regressed, Missing, Added bool
}

// Compare answers each benchmark's change; a change over threshold percent in ns/op or allocs/op is a regression. Pure.
func Compare(base, now map[string]Result, threshold float64) []Change {
	names := map[string]bool{}
	for n := range base {
		names[n] = true
	}
	for n := range now {
		names[n] = true
	}
	var out []Change
	for n := range names {
		b, inBase := base[n]
		c, inNow := now[n]
		ch := Change{Name: n, Missing: inBase && !inNow, Added: !inBase && inNow}
		if inBase && inNow {
			ch.Time, ch.Bytes, ch.Allocs = pctChange(b.NsOp, c.NsOp), pctChange(b.BytesOp, c.BytesOp), pctChange(b.AllocsOp, c.AllocsOp)
			ch.Regressed = ch.Time > threshold || ch.Allocs > threshold
		}
		out = append(out, ch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func pctChange(a, b float64) float64 {
	if a == 0 {
		if b == 0 {
			return 0
		}
		return 100
	}
	return (b - a) / a * 100
}

// bench runs the benchmarks of a module, then saves the result or compares it with a saved one.
func bench(args []string, stdout, stderr io.Writer, runGo func(dir string, args ...string) ([]byte, error)) int {
	fs := flagSet("go-performance-check bench", stderr)
	save := fs.String("save", "", "write the result to this JSON file (the base)")
	compare := fs.String("compare", "", "compare the result with this saved JSON file")
	pattern := fs.String("bench", ".", "the benchmarks to run (a go test -bench pattern)")
	count := fs.Int("count", 5, "runs of each benchmark")
	threshold := fs.Float64("threshold", 10, "the percent of slowdown (ns/op or allocs/op) that is a regression")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if (*save == "") == (*compare == "") {
		fmt.Fprintln(stderr, "go-performance-check bench: give exactly one of -save or -compare")
		return 2
	}
	raw, err := runGo(dir, "test", "-run", "^$", "-bench", *pattern, "-benchmem", "-count", strconv.Itoa(*count), "./...")
	if err != nil {
		fmt.Fprintf(stderr, "go-performance-check bench: go test failed: %v\n%s", err, raw)
		return 2
	}
	now := Parse(strings.NewReader(string(raw)))
	if len(now) == 0 {
		fmt.Fprintln(stderr, "go-performance-check bench: no benchmark ran (the pattern matched none): nothing was measured")
		return 2
	}
	if *save != "" {
		b, _ := json.MarshalIndent(now, "", "  ")
		if err := os.WriteFile(*save, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintln(stderr, "go-performance-check bench:", err)
			return 2
		}
		fmt.Fprintf(stdout, "saved %d benchmarks (%d runs each) to %s\n", len(now), *count, *save)
		return 0
	}
	b, err := os.ReadFile(*compare)
	var base map[string]Result
	if err == nil {
		err = json.Unmarshal(b, &base)
	}
	if err != nil {
		fmt.Fprintln(stderr, "go-performance-check bench: cannot read the base:", err)
		return 2
	}
	regressed := false
	for _, c := range Compare(base, now, *threshold) {
		switch {
		case c.Missing:
			fmt.Fprintf(stdout, "%-50s gone (in the base only)\n", c.Name)
		case c.Added:
			fmt.Fprintf(stdout, "%-50s new\n", c.Name)
		default:
			mark := ""
			if c.Regressed {
				mark, regressed = "  REGRESSION", true
			}
			fmt.Fprintf(stdout, "%-50s time %+6.1f%%  bytes %+6.1f%%  allocs %+6.1f%%%s\n", c.Name, c.Time, c.Bytes, c.Allocs, mark)
		}
	}
	if regressed {
		return 1
	}
	return 0
}

func goCommand(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
