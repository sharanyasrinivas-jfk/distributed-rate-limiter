# Distributed Rate Limiter & API Gateway

A horizontally-scalable API gateway with distributed rate limiting, built in Go
with Redis-backed atomic counters. Enforces accurate rate limits across
multiple gateway replicas with zero coordination overhead between them —
all coordination happens through Redis.

## Why This Exists

A naive rate limiter (an in-memory counter in one process) breaks the moment
you scale horizontally: 5 replicas each keeping their own counter turns a
"100 requests/minute" limit into 500/minute. This gateway solves that by
making Redis the single source of truth for counters, using atomic `INCR`
and Lua scripts to eliminate race conditions across replicas — proven with
concurrency tests that fire 1,000 simultaneous goroutines at the same limit
key and assert not one request over the limit gets through.

## Features

- **4 rate limiting algorithms**, selectable per route: Fixed Window,
  Sliding Window Log, Sliding Window Counter, Token Bucket
- **Correct behavior under concurrent multi-replica load** — verified with
  `-race` concurrency tests against real Redis, and manually with two
  independent gateway processes sharing one Redis instance (see below)
- **Circuit breaker** for backend failure isolation (Closed → Open → HalfOpen)
- **Admin API** (JWT-protected) to view and override per-client limits
- **Prometheus-compatible `/metrics`** endpoint, Kubernetes-ready `/healthz` / `/readyz`
- **Configurable fail-open / fail-closed** behavior when Redis is unreachable

## Architecture

```
Client → Load Balancer → [Gateway #1, #2, #3 — stateless replicas] → Redis (shared counters) → Backend services
```

Each request passes through: recovery → logging → metrics → auth (API key →
client/tier) → rate limit (atomic Redis op) → circuit breaker → reverse proxy.
See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full request lifecycle,
algorithm comparison table, and design tradeoffs.

## Quick Start

```bash
git clone https://github.com/yourname/distributed-rate-limiter.git
cd distributed-rate-limiter
docker compose -f deployments/docker-compose.yaml up --build
curl localhost:8080/healthz
```

## Proving Distributed Correctness (No Kubernetes Needed)

Run two gateway processes on different ports against the same Redis instance
and fire requests at both — the limit holds globally even though the two
processes share no memory:

```bash
redis-server --daemonize yes
go build -o /tmp/gateway ./cmd/gateway
CONFIG_PATH=configs/config.yaml PORT=8090 REDIS_ADDR=localhost:6379 /tmp/gateway &
CONFIG_PATH=configs/config.yaml PORT=8091 REDIS_ADDR=localhost:6379 /tmp/gateway &

for i in {1..8}; do
  port=$([ $((i % 2)) -eq 0 ] && echo 8090 || echo 8091)
  curl -s -o /dev/null -w "replica:$port -> %{http_code}\n" localhost:$port/api/orders
done
```

With a limit of 5, this correctly prints five `200`s followed by `429`s,
split unpredictably across both "replicas" — proof the counter is shared.

## Tech Stack

Go · Redis · Docker · Kubernetes · Lua (atomicity scripts)

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Installation Guide](docs/INSTALLATION.md)
- [Developer Guide](docs/DEVELOPER_GUIDE.md)
- [Changelog](docs/CHANGELOG.md)

## Testing

```bash
go test ./... -race -cover
```

Unit tests use `miniredis` (in-memory fake, no network). Concurrency tests
run against real Redis and are skipped automatically if none is reachable
(set `RATELIMIT_TEST_REDIS_ADDR` to point at one).

## License

MIT — see [LICENSE](LICENSE)
