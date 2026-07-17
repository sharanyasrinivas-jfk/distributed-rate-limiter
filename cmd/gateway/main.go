// Command gateway starts the distributed rate limiter / API gateway HTTP server.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/yourname/distributed-rate-limiter/internal/admin"
	"github.com/yourname/distributed-rate-limiter/internal/config"
	"github.com/yourname/distributed-rate-limiter/internal/health"
	"github.com/yourname/distributed-rate-limiter/internal/metrics"
	"github.com/yourname/distributed-rate-limiter/internal/middleware"
	"github.com/yourname/distributed-rate-limiter/internal/proxy"
	"github.com/yourname/distributed-rate-limiter/internal/ratelimit"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	configPath := envOr("CONFIG_PATH", "configs/config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// REDIS_ADDR env var overrides the config file — this is how Docker
	// Compose / Kubernetes inject the right address per environment.
	redisAddr := envOr("REDIS_ADDR", cfg.Redis.Addr)
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	store := ratelimit.NewStore(rdb)
	clientStore := admin.NewRedisClientStore(rdb)
	reg := metrics.NewRegistry()

	jwtSecret := []byte(envOr("JWT_SECRET", "dev-only-insecure-secret-change-me"))
	adminUser := envOr("ADMIN_USERNAME", "admin")
	adminPassword := envOr("ADMIN_PASSWORD", "admin")
	adminHandlers := admin.NewHandlers(clientStore, jwtSecret, adminUser, adminPassword)

	mux := http.NewServeMux()

	// --- Proxy (rate-limited, authenticated traffic) ---
	p := proxy.New(cfg)
	proxyChain := middleware.Logging(
		middleware.Metrics(reg)(
			middleware.Auth(clientStore)(
				middleware.RateLimit(cfg, store)(
					middleware.CircuitBreaker(cfg, 5)(
						p.Handler(),
					),
				),
			),
		),
	)

	// --- Health & metrics ---
	mux.HandleFunc("/healthz", health.Liveness)
	mux.HandleFunc("/readyz", health.Readiness(store))
	mux.Handle("/metrics", reg.Handler())

	// --- Admin API (JWT protected, login itself rate-limited) ---
	loginLimiter := ratelimit.NewFixedWindowLimiter(store, 5, time.Minute, ratelimit.FailOpen)
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		decision, _ := loginLimiter.Allow(r.Context(), "ip:"+ip, "/admin/login")
		if !decision.Allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(decision.RetryAfter.Seconds())))
			http.Error(w, "too many login attempts", http.StatusTooManyRequests)
			return
		}
		adminHandlers.Login(w, r)
	})
	adminMux.HandleFunc("/admin/limits/", func(w http.ResponseWriter, r *http.Request) {
		clientID := strings.TrimPrefix(r.URL.Path, "/admin/limits/")
		if clientID == "" {
			http.Error(w, "client_id required", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			adminHandlers.GetLimits(w, r, clientID)
		case http.MethodPut:
			adminHandlers.PutLimits(w, r, clientID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.Handle("/admin/", middleware.Logging(admin.RequireJWT(jwtSecret)(adminMux)))

	// --- Everything else goes through the full rate-limited proxy chain ---
	mux.Handle("/", middleware.Recovery(proxyChain))

	addr := ":" + envOr("PORT", "8080")
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting gateway", "addr", addr, "redis_addr", redisAddr, "config", configPath)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
