# Developer Guide

## Folder Structure
```
cmd/gateway/          entry point, wires everything together
internal/config/       YAML config loading
internal/proxy/         reverse proxy
internal/ratelimit/     the 4 algorithms + Redis store (Limiter interface)
internal/middleware/    recovery, logging, metrics, auth, rate limit, circuit breaker
internal/circuitbreaker/ breaker state machine
internal/admin/         JWT login + limit view/override handlers
internal/metrics/       stdlib-only Prometheus text-exposition registry
internal/health/        /healthz, /readyz
configs/                config.yaml (routes, backends, limits)
deployments/            Dockerfile, docker-compose, k8s manifests
docs/                   this documentation
```

## Adding a New Rate-Limiting Algorithm
1. Implement the `Limiter` interface in `internal/ratelimit/`:
   ```go
   type Limiter interface {
       Allow(ctx context.Context, clientID, route string) (Decision, error)
   }
   ```
2. If it needs multi-step atomicity, write it as a Lua script (see
   `sliding_window_log.go` or `token_bucket.go` for examples) rather than
   multiple round trips.
3. Add a case for it in `internal/middleware/ratelimit_middleware.go`'s
   `limiterFor` switch, and a new `config.Algorithm` constant.
4. Write unit tests with `miniredis` and a concurrency test against real
   Redis proving no overcounting under concurrent load (copy the pattern in
   `fixed_window_test.go`).

## Running Tests Locally
```bash
go test ./...                    # everything
go test ./internal/ratelimit/... -race -v   # just the algorithms, verbose
go test ./... -cover             # with coverage
```

## Coding Style
- Run `gofmt -l .` before committing; CI enforces it via `golangci-lint`.
- Prefer table-driven tests where it doesn't sacrifice readability.
- Every exported type/function gets a doc comment.

## Middleware Chain
Composed in `cmd/gateway/main.go` as nested function calls, outermost first:
```go
Logging(Metrics(Auth(RateLimit(CircuitBreaker(proxyHandler)))))
```
Metrics sits inside Logging so it still sees the final status code from
everything downstream, including 429s and 503s.
