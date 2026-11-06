package perfwatch

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A route is named by its ServeMux pattern, so /users/1 and /users/2 are one route; an unmatched path is "unmatched".
func TestRoutesAreNamedByPattern(t *testing.T) {
	rec := New(Options{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, _ *http.Request) {})
	h := rec.Middleware(mux)
	for _, p := range []string{"/users/1", "/users/2", "/nowhere"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", p, nil))
	}
	got := map[string]int64{}
	for _, s := range rec.Stats() {
		got[s.Route] = s.Count
	}
	if got["GET /users/{id}"] != 2 || got["unmatched"] != 1 || len(got) != 2 {
		t.Errorf("routes: %v, want GET /users/{id}: 2 and unmatched: 1", got)
	}
}

func TestPercentiles(t *testing.T) {
	rec := New(Options{})
	for i := 1; i <= 100; i++ {
		rec.Observe("r", time.Duration(i)*time.Millisecond)
	}
	s := rec.Stats()[0]
	if s.P50 != 50*time.Millisecond || s.P95 != 95*time.Millisecond || s.P99 != 99*time.Millisecond || s.Max != 100*time.Millisecond {
		t.Errorf("percentiles: p50 %v p95 %v p99 %v max %v", s.P50, s.P95, s.P99, s.Max)
	}
}

func TestTheRingKeepsTheNewestSamples(t *testing.T) {
	rec := New(Options{Samples: 3})
	for _, ms := range []int{100, 100, 100, 1, 2, 3} {
		rec.Observe("r", time.Duration(ms)*time.Millisecond)
	}
	s := rec.Stats()[0]
	if s.Count != 6 || s.Max != 3*time.Millisecond {
		t.Errorf("count %d max %v, want 6 and 3ms (the old samples gone)", s.Count, s.Max)
	}
}

func TestSlowRequestsAreCountedAndLogged(t *testing.T) {
	var log bytes.Buffer
	rec := New(Options{Slow: 10 * time.Millisecond, Logger: slog.New(slog.NewTextHandler(&log, nil))})
	rec.Observe("r", 5*time.Millisecond)
	rec.Observe("r", 20*time.Millisecond)
	if s := rec.Stats()[0]; s.Slow != 1 {
		t.Errorf("slow %d, want 1", s.Slow)
	}
	if !strings.Contains(log.String(), "slow request") || strings.Count(log.String(), "slow request") != 1 {
		t.Errorf("log: %q, want one slow request line", log.String())
	}
}

func TestTheStatsHandlerAnswersJSON(t *testing.T) {
	rec := New(Options{})
	rec.Observe("r", time.Millisecond)
	w := httptest.NewRecorder()
	rec.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/debug/perf", nil))
	var got []RouteStats
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got) != 1 || got[0].Route != "r" {
		t.Errorf("stats JSON: %v %s", err, w.Body.String())
	}
}

func TestPprofNeedsTheToken(t *testing.T) {
	ask := func(h http.Handler, auth string) int {
		r := httptest.NewRequest("GET", "/debug/pprof/", nil)
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	h := Pprof("s3cret-token")
	if c := ask(h, ""); c != http.StatusForbidden {
		t.Errorf("no token: %d, want 403", c)
	}
	if c := ask(h, "Bearer wrong"); c != http.StatusForbidden {
		t.Errorf("a wrong token: %d, want 403", c)
	}
	if c := ask(h, "Bearer s3cret-token"); c != http.StatusOK {
		t.Errorf("the token: %d, want 200", c)
	}
	if c := ask(Pprof(""), "Bearer "); c != http.StatusForbidden {
		t.Errorf("an empty token configured: %d, want 403 for everyone", c)
	}
}
