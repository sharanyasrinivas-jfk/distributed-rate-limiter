# Installation

## Prerequisites
- Go 1.22+
- Docker (for `docker compose` and building images)
- `kubectl` + `kind` (optional, only for the Kubernetes deployment path)

## Local Development (no Docker)
```bash
redis-server --daemonize yes         # or `docker run -d -p 6379:6379 redis:7`
go run ./cmd/gateway
curl localhost:8080/healthz
```
Defaults to `configs/config.yaml` and `localhost:6379`. Override with the
`CONFIG_PATH`, `REDIS_ADDR`, `PORT`, `JWT_SECRET`, `ADMIN_USERNAME`, and
`ADMIN_PASSWORD` environment variables.

## Full Local Stack (Docker Compose)
```bash
docker compose -f deployments/docker-compose.yaml up --build
curl localhost:8080/healthz
```
Brings up the gateway, Redis, and two mock backend services on one network.

## Running Tests
```bash
go test ./... -race -cover
```
Concurrency tests need a real Redis reachable at `localhost:6379` (or set
`RATELIMIT_TEST_REDIS_ADDR`); they're skipped automatically otherwise.

## Troubleshooting

**"connection refused" talking to Redis** — Redis isn't running, or
`REDIS_ADDR` points at the wrong host/port. Check `redis-cli ping`.

**Port already in use** — another process is bound to 8080 (or 6379 for
Redis). Set `PORT` to something else, or stop the conflicting process.

**All requests return 200 with no rate limiting** — the fail-open default
kicks in when Redis is unreachable; check `/readyz`.
