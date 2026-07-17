# Changelog

All notable changes to this project are documented here.
Format based on [Keep a Changelog](https://keepachangelog.com/).

## [1.0.0] - 2026-07-17
### Added
- Fixed window, sliding window log, sliding window counter, and token
  bucket rate-limiting algorithms, all backed by atomic Redis operations
- Concurrency test suite proving no overcounting under 500–1000 simultaneous
  goroutines against real Redis, with `-race` enabled
- Reverse proxy with per-route backend + algorithm configuration
- API-key auth for rate-limited traffic, JWT auth for the admin API
- Circuit breaker (Closed/Open/HalfOpen) for backend failure isolation
- Admin API: login, view/override per-client limits
- Prometheus-compatible `/metrics`, `/healthz`, `/readyz`
- Dockerfile (multi-stage), docker-compose local stack, Kubernetes manifests
  (3-replica deployment, service, configmap, HPA)
- Full documentation set: architecture, API reference, installation,
  developer guide, user guide
