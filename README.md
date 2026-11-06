# go-performance-check

Performance help for Go modules and services, **without AI**, in three parts:

1. **`go-performance-check`**: static checks of a module's source (the standard library's `go/ast`, no dependency, no
   network). Each check is one file; a finding at or above `-fail-on` (default `medium`) exits 1.
2. **`go-performance-check bench`**: runs the module's benchmarks, saves the result, and compares a later run with it;
   a slowdown over `-threshold` percent exits 1.
3. **`perfwatch`**: a small library for a running service: each route's latency (p50, p95, p99, max), its count and its
   slow requests, a JSON stats handler, and a pprof handler behind a token.

## Install and run

```bash
go install github.com/olgunozoktas/go-performance-check/cmd/go-performance-check@latest
go-performance-check                         # the module in this folder (the same as: go-performance-check check)
go-performance-check -fail-on high ./svc     # fail only on high
go-performance-check -json -only defer-in-loop,response-body-not-closed
go-performance-check bench -save base.json                 # on main
go-performance-check bench -compare base.json -count 10    # on the branch; exit 1 on a regression
```

`bench` runs `go test -run '^$' -bench <-bench> -benchmem -count <-count> ./...`, averages each benchmark, and marks a
regression when ns/op or allocs/op grows by more than `-threshold` percent (default 10). No benchmark run is exit 2:
nothing was measured.

## The checks

| Check | Severity | What it finds |
|---|---|---|
| `append-without-capacity` | low | an empty slice filled by an append on EVERY iteration of a range loop (a filter inside an `if` is not reported) |
| `defer-in-loop` | medium | a `defer` directly in a loop: it runs only when the function returns |
| `http-client-per-call` | medium / low | an `http.Client` or `Transport` made for one call (in a loop, or a local outside a constructor); `http.Get` and friends (the default client has no timeout) are low. A client that a constructor stores or returns is not reported |
| `json-in-loop` | low | `json.Marshal` or `Unmarshal` on each iteration of a loop |
| `mutex-by-value` | medium | a value receiver on a struct that holds a `sync.Mutex` or `RWMutex`: the method locks a copy |
| `regexp-compiled-in-function` | medium / high | a constant pattern compiled inside a function (high inside a loop) |
| `response-body-not-closed` | high | `resp, err := client.Do(req)` (or `http.Get` …) whose `resp.Body.Close()` is not in the function and which is not handed on |
| `string-concat-in-loop` | low | `s += "…"` in a loop |

A finding is a **candidate, not a verdict**: measure before you change hot code (`bench`, `perfwatch`, pprof). A line
that the author checked carries a marker in a real comment: `//go-performance-check:ignore <reason>` (or `//nolint:perf`).

## perfwatch

```go
rec := perfwatch.New(perfwatch.Options{Slow: 500 * time.Millisecond})
mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", showUser)
mux.Handle("GET /debug/perf", rec.Handler())                          // JSON: each route's p50, p95, p99, max, count, slow
mux.Handle("/debug/pprof/", perfwatch.Pprof(os.Getenv("PPROF_TOKEN"))) // only with Authorization: Bearer <token>
srv := &http.Server{Addr: ":8080", Handler: rec.Middleware(mux), ReadHeaderTimeout: 10 * time.Second}
```

- A route is named by its `ServeMux` pattern (`GET /users/{id}`), so `/users/1` and `/users/2` are one route.
- Each route keeps its newest `Samples` durations (default 1024); `Observe(name, d)` records a job or a queue worker too.
- `Pprof` compares the token in constant time; an empty token refuses every request, because a profile shows the code
  and the memory.

## Add a check

A file in `internal/checks/` that calls `register(name, func(m *core.Module) []core.Finding)` in its `init`, plus
`testdata/<check>/bad/x.go` and `good/x.go`, and its entry in `checks_test.go`. `go test ./...` fails for a check
without fixtures.

## License

MIT
