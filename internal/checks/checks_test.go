package checks

import (
	"path/filepath"
	"testing"

	"github.com/olgunozoktas/go-performance-check/internal/core"
)

// Each AST check: its bad shapes are ALL found at the right severity, and its good shapes (the safe form, an ignore
// marker in a real comment, a stored client, a filter …) are silent. testdata/<check>/bad/ and good/ hold
// the shapes; the fixtures are read where they are, and no file is written.
var want = map[string]core.Severity{
	"regexp-compiled-in-function": core.Medium, "http-client-per-call": core.Medium, "defer-in-loop": core.Medium,
	"string-concat-in-loop": core.Low, "response-body-not-closed": core.High, "append-without-capacity": core.Low,
	"mutex-by-value": core.Medium, "json-in-loop": core.Low,
}

// How many shapes each bad fixture holds: every one of them must be found, not only the first.
var shapes = map[string]int{"regexp-compiled-in-function": 2, "http-client-per-call": 2}

func run(t *testing.T, check, dir string) []core.Finding {
	t.Helper()
	m, skipped, err := core.Load(dir, false)
	if err != nil || skipped != 0 {
		t.Fatalf("%s: load: %v, %d not parsed", dir, err, skipped)
	}
	for _, c := range core.Checks() {
		if c.Name() == check {
			return m.Kept(c.Run(m))
		}
	}
	t.Fatalf("no check named %s", check)
	return nil
}

func TestEachCheckFindsTheBadShape(t *testing.T) {
	for check, severity := range want {
		got := run(t, check, filepath.Join("testdata", check, "bad"))
		n := shapes[check]
		if n == 0 {
			n = 1
		}
		if len(got) != n {
			t.Errorf("%s: %d finding(s) on bad/, want %d: %+v", check, len(got), n, got)
			continue
		}
		if got[0].Severity != severity {
			t.Errorf("%s: severity %s, want %s", check, got[0].Severity, severity)
		}
	}
}

func TestEachCheckIsSilentOnTheGoodShape(t *testing.T) {
	for check := range want {
		if got := run(t, check, filepath.Join("testdata", check, "good")); len(got) != 0 {
			t.Errorf("%s: %d finding(s) on good/: %+v", check, len(got), got)
		}
	}
}

func TestEveryCheckHasFixtures(t *testing.T) {
	for _, c := range core.Checks() {
		if _, ok := want[c.Name()]; !ok {
			t.Errorf("%s has no entry in want (and no testdata)", c.Name())
		}
	}
	if len(want) != len(core.Checks()) {
		t.Errorf("want has %d checks, the registry %d", len(want), len(core.Checks()))
	}
}

func TestTheLoopShapeIsHigh(t *testing.T) {
	got := run(t, "regexp-compiled-in-function", filepath.Join("testdata", "regexp-compiled-in-function", "bad"))
	levels := map[core.Severity]int{}
	for _, f := range got {
		levels[f.Severity]++
	}
	if levels[core.High] != 1 || levels[core.Medium] != 1 {
		t.Errorf("regexp: %v, want one high (in a loop) and one medium", levels)
	}
}

func TestAnIgnoreMarkerInAStringCountsForNothing(t *testing.T) {
	// testdata/marker-in-a-string: _ = "//nolint:perf"; regexp.MustCompile("a") on one line.
	got := run(t, "regexp-compiled-in-function", filepath.Join("testdata", "marker-in-a-string"))
	if len(got) != 1 {
		t.Errorf("a //nolint:perf inside a string hid the finding: %+v", got)
	}
}
