# API Reference

## Proxied Routes

Any path matching a configured `path_prefix` in `configs/config.yaml` is
rate-limited and forwarded to that route's backend. Every response includes:

| Header | Meaning |
|---|---|
| `X-RateLimit-Limit` | The limit in effect for this client/tier |
| `X-RateLimit-Remaining` | Requests remaining in the current window/bucket |
| `X-RateLimit-Reset` | Unix timestamp when the window resets (window-based algorithms only) |
| `Retry-After` | Seconds to wait before retrying (only present on `429`) |

Identify yourself with `X-API-Key: <key>`. Unknown or missing keys fall back
to IP-based identification at the strictest configured tier.

### Example: `GET /api/orders`
Response `200`: proxied backend response, with rate-limit headers set.
Response `429`:
```json
{"error": "rate limit exceeded"}
```

## Admin API

All `/admin/*` routes except `/admin/login` require `Authorization: Bearer <jwt>`.

### `POST /admin/login`
Rate-limited (5/min) to prevent brute force.

Request:
```json
{ "username": "admin", "password": "..." }
```
Response `200`:
```json
{ "token": "<jwt>", "expires_in": 3600 }
```
Response `401`:
```json
{ "error": "invalid credentials" }
```

### `GET /admin/limits/:client_id`
Headers: `Authorization: Bearer <jwt>`

Response `200`:
```json
{
  "client_id": "acme-corp",
  "tier": "pro",
  "current_usage": 340,
  "limit": 1000
}
```
Response `404`: unknown client.

### `PUT /admin/limits/:client_id`
Headers: `Authorization: Bearer <jwt>`

Request:
```json
{ "limit": 5000 }
```
Response `200`:
```json
{ "client_id": "acme-corp", "limit": 5000 }
```

## Operational Endpoints

| Endpoint | Purpose |
|---|---|
| `GET /healthz` | Liveness — 200 if the process is up |
| `GET /readyz` | Readiness — 200 only if Redis is reachable |
| `GET /metrics` | Prometheus text-exposition format |
