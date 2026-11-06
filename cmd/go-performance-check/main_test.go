package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fixture = filepath.Join("..", "..", "internal", "checks", "testdata", "cmd")

func exit(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String() + errOut.String()
}

// The check command: exit 1 at or above -fail-on, 0 below, 2 for a wrong flag; "check" is the default subcommand.
func TestCheckExitCodes(t *testing.T) {
	if code, out := exit(t, "-only", "mutex-by-value", fixture); code != 1 {
		t.Errorf("a medium finding: exit %d, want 1\n%s", code, out)
	}
	if code, _ := exit(t, "check", "-only", "mutex-by-value", "-fail-on", "high", fixture); code != 0 {
		t.Errorf("-fail-on high: exit %d, want 0", code)
	}
	if code, _ := exit(t, "-fail-on", "severe", fixture); code != 2 {
		t.Errorf("a wrong -fail-on: exit %d, want 2", code)
	}
	if _, out := exit(t, "-list"); len(strings.Fields(out)) != 8 {
		t.Errorf("-list: %d checks, want 8", len(strings.Fields(out)))
	}
}

const benchOut = `goos: darwin
BenchmarkParse-8   	  1000	      1000 ns/op	     100 B/op	       2 allocs/op
BenchmarkParse-8   	  1000	      3000 ns/op	     300 B/op	       2 allocs/op
BenchmarkJoin-8    	  5000	       200 ns/op	      16 B/op	       1 allocs/op
PASS`

func TestParseAveragesEachBenchmark(t *testing.T) {
	got := Parse(strings.NewReader(benchOut))
	if p := got["BenchmarkParse"]; p.Runs != 2 || p.NsOp != 2000 || p.BytesOp != 200 || p.AllocsOp != 2 {
		t.Errorf("BenchmarkParse: %+v", p)
	}
	if j := got["BenchmarkJoin"]; j.Runs != 1 || j.NsOp != 200 {
		t.Errorf("BenchmarkJoin: %+v", j)
	}
}

func TestCompareFindsARegressionOverTheThreshold(t *testing.T) {
	base := map[string]Result{"A": {NsOp: 100, AllocsOp: 1}, "B": {NsOp: 100, AllocsOp: 1}, "Gone": {NsOp: 1}}
	now := map[string]Result{"A": {NsOp: 109, AllocsOp: 1}, "B": {NsOp: 100, AllocsOp: 2}, "New": {NsOp: 1}}
	got := map[string]Change{}
	for _, c := range Compare(base, now, 10) {
		got[c.Name] = c
	}
	if got["A"].Regressed || !got["B"].Regressed || !got["Gone"].Missing || !got["New"].Added {
		t.Errorf("changes: %+v", got)
	}
}

func TestBenchSavesThenComparesAndExitsOneOnARegression(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "checks", "testdata", "cmd")
	base := filepath.Join(dir, "base.json")
	t.Cleanup(func() { os.Remove(base) })
	fake := func(out string) func(string, ...string) ([]byte, error) {
		return func(string, ...string) ([]byte, error) { return []byte(out), nil }
	}
	var o, e bytes.Buffer
	if c := bench([]string{"-save", base, dir}, &o, &e, fake(benchOut)); c != 0 {
		t.Fatalf("save: exit %d %s", c, e.String())
	}
	slower := strings.ReplaceAll(benchOut, "200 ns/op", "400 ns/op")
	if c := bench([]string{"-compare", base, dir}, &o, &e, fake(slower)); c != 1 {
		t.Errorf("a 100%% slower Join: exit %d, want 1\n%s", c, o.String())
	}
	if c := bench([]string{"-compare", base, dir}, &o, &e, fake(benchOut)); c != 0 {
		t.Errorf("the same numbers: exit %d, want 0", c)
	}
	if c := bench([]string{"-compare", base, dir}, &o, &e, fake("PASS")); c != 2 {
		t.Errorf("no benchmark ran: exit %d, want 2", c)
	}
	if c := bench([]string{dir}, &o, &e, fake(benchOut)); c != 2 {
		t.Errorf("neither -save nor -compare: exit %d, want 2", c)
	}
}
