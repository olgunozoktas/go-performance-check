// Package perfwatch measures a Go HTTP service where it runs: each route's latency (p50, p95, p99, max), its count,
// and its slow requests, with a small JSON stats handler and a pprof handler behind a token.
//
//	rec := perfwatch.New(perfwatch.Options{Slow: 500 * time.Millisecond})
//	mux := http.NewServeMux()
//	mux.Handle("GET /debug/perf", rec.Handler())
//	mux.Handle("/debug/pprof/", perfwatch.Pprof(os.Getenv("PPROF_TOKEN")))
//	http.ListenAndServe(addr, rec.Middleware(mux))
//
// A route is named by its ServeMux pattern ("GET /users/{id}"), which ServeMux writes on the request (net/http
// server.go, Go 1.22 and later), so /users/1 and /users/2 are one route. A request that no pattern matched is "unmatched".
package perfwatch

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"slices"
	"strings"
	"sync"
	"time"
)

// Options: Slow logs each request that takes longer (0: none); Samples is how many durations each route keeps (the
// newest; default 1024); Logger takes the slow-request lines (default slog.Default()).
type Options struct {
	Slow    time.Duration
	Samples int
	Logger  *slog.Logger
}

// RouteStats is one route's numbers, from its kept samples.
type RouteStats struct {
	Route string        `json:"route"`
	Count int64         `json:"count"`
	Slow  int64         `json:"slow"`
	P50   time.Duration `json:"p50_ns"`
	P95   time.Duration `json:"p95_ns"`
	P99   time.Duration `json:"p99_ns"`
	Max   time.Duration `json:"max_ns"`
}

type route struct {
	count, slow int64
	samples     []time.Duration // a ring of the newest Samples durations
	next        int
}

// Recorder keeps the numbers of each route; it is safe for concurrent use.
type Recorder struct {
	mu     sync.Mutex
	routes map[string]*route
	opt    Options
}

// New makes a Recorder.
func New(opt Options) *Recorder {
	if opt.Samples <= 0 {
		opt.Samples = 1024
	}
	if opt.Logger == nil {
		opt.Logger = slog.Default()
	}
	return &Recorder{routes: map[string]*route{}, opt: opt}
}

// Middleware measures each request that next serves.
func (r *Recorder) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, req)
		name := req.Pattern
		if name == "" {
			name = "unmatched"
		}
		r.Observe(name, time.Since(start))
	})
}

// Observe records one duration for a route (the middleware calls it; a job or a queue worker can too).
func (r *Recorder) Observe(name string, d time.Duration) {
	r.mu.Lock()
	rt := r.routes[name]
	if rt == nil {
		rt = &route{samples: make([]time.Duration, 0, r.opt.Samples)}
		r.routes[name] = rt
	}
	rt.count++
	if len(rt.samples) < r.opt.Samples {
		rt.samples = append(rt.samples, d)
	} else {
		rt.samples[rt.next] = d
		rt.next = (rt.next + 1) % r.opt.Samples
	}
	slow := r.opt.Slow > 0 && d > r.opt.Slow
	if slow {
		rt.slow++
	}
	r.mu.Unlock()
	if slow {
		r.opt.Logger.Warn("slow request", "route", name, "duration", d)
	}
}

// Stats answers each route's numbers, the slowest p95 first.
func (r *Recorder) Stats() []RouteStats {
	r.mu.Lock()
	out := make([]RouteStats, 0, len(r.routes))
	for name, rt := range r.routes {
		s := slices.Clone(rt.samples)
		slices.Sort(s)
		out = append(out, RouteStats{Route: name, Count: rt.count, Slow: rt.slow, P50: pct(s, 50), P95: pct(s, 95), P99: pct(s, 99), Max: pct(s, 100)})
	}
	r.mu.Unlock()
	slices.SortFunc(out, func(a, b RouteStats) int {
		if a.P95 != b.P95 {
			return int(b.P95 - a.P95)
		}
		return strings.Compare(a.Route, b.Route)
	})
	return out
}

// pct answers the p-th percentile of sorted durations (the nearest rank), 0 for none.
func pct(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := (p*len(sorted)+99)/100 - 1
	return sorted[max(0, min(i, len(sorted)-1))]
}

// Handler answers the stats as JSON.
func (r *Recorder) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(r.Stats())
	})
}

// Pprof serves net/http/pprof under /debug/pprof/ only to a request with "Authorization: Bearer <token>" (compared in
// constant time). An empty token refuses every request: a profile shows the code and the memory, so it is never open.
func Pprof(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		given := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
		if token == "" || subtle.ConstantTimeCompare([]byte(given), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, req)
	})
}
