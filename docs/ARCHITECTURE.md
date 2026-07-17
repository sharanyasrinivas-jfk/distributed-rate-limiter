# Architecture

## Request Lifecycle

1. Request hits any gateway replica (interchangeable — none holds unique state).
2. **Recovery** middleware catches panics, returns 500 instead of crashing.
3. **Logging** middleware assigns a request ID, logs method/path/status/duration.
4. **Metrics** middleware records the request in the Prometheus-format registry.
5. **Auth** middleware extracts `client_id`/`tier` from `X-API-Key` (Redis lookup),
   falling back to IP-based identity at the strictest tier for unknown keys.
6. **Rate limit** middleware runs the route's configured algorithm as an atomic
   Redis operation. Over limit → `429` with `Retry-After`, request never reaches
   the backend.
7. **Circuit breaker** checks the backend's health state; if open, fails fast
   with `503` without attempting the call.
8. **Proxy** forwards to the backend and streams the response back.

## Why Redis Operations Must Be Atomic

Naive approach (wrong):
```
count = GET key       // e.g. count = 99
if count < 100:
    INCR key           // race: two replicas can both read 99 and both proceed
```
Between `GET` and `INCR`, a concurrent request — possibly on a *different*
gateway replica — can read the same stale value. Both think they're under
the limit; 101 requests get through instead of 100.

This project avoids that in two ways:
1. **`INCR` is atomic by itself** — increment first, then check the returned
   value, never check-then-increment (used by Fixed Window).
2. **Lua scripts via `EVAL`** for anything needing multi-step logic (trim +
   add + count for Sliding Window Log; refill math + consume for Token
   Bucket) — Redis guarantees the whole script runs atomically and
   single-threaded, with no other command interleaving.

Proven with a test that fires 1,000 concurrent goroutines at the same limit
key against real Redis and asserts the allowed count is exactly the limit,
not more (`internal/ratelimit/fixed_window_test.go`).

## Algorithm Comparison

| Algorithm | Accuracy | Memory | Burst Handling | Complexity |
|---|---|---|---|---|
| Fixed Window | Low (up to 2x burst at window edges) | O(1) per client | Poor | Simple |
| Sliding Window Log | Perfect | O(N) requests stored | Good | Medium |
| Sliding Window Counter | Good approximation | O(1) per client | Good | Medium |
| Token Bucket | Good, smooths bursts | O(1) per client | Best (controlled bursts) | Medium |

## Design Decisions & Tradeoffs

**Why Redis over in-memory state?**
Rate limits must be enforced *globally* across replicas. In-memory counters
are per-process and can't agree with each other. Redis gives every replica
a single, fast, shared source of truth.

**Why Lua scripts over multi-command transactions (MULTI/EXEC)?**
`MULTI/EXEC` batches commands but can't branch on intermediate results — you
can't say "read tokens, then conditionally decrement" atomically with
transactions alone. A Lua script runs server-side as one atomic unit and can
contain that conditional logic (see the token bucket script).

**Why is fail-open vs fail-closed configurable?**
This is a CAP-theorem tradeoff in practice. `fail_open` favors availability —
if Redis is down, let requests through rather than take the whole API down
(the right default for most APIs). `fail_closed` favors strict enforcement —
reject everything if the limiter can't verify the limit (appropriate for
security-sensitive endpoints like login, which is why `/admin/login` is
rate-limited even under fail-open elsewhere).

**Why stateless gateway replicas?**
Statelessness is what makes horizontal scaling free. Any replica can handle
any request; a replica dying loses nothing because all durable state (rate
limit counters, client config) lives in Redis, not in gateway memory.

**Why YAML config over a database for routes/limits?**
Route definitions change rarely, benefit from version control and code
review, and don't need ad-hoc querying — a database would be over-engineering
for this access pattern.

## Failure Modes

- **Redis down:** configurable fail-open/fail-closed (see above).
- **Backend down:** circuit breaker trips to `Open` after N consecutive
  failures, gateway returns `503` immediately instead of piling up slow or
  hanging requests — preventing cascading failure into the gateway itself.
