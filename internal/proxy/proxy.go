// Package proxy implements the HTTP reverse-proxy layer that forwards
// requests to whichever backend a request's path prefix matches.
package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/yourname/distributed-rate-limiter/internal/config"
)

// Proxy forwards requests to backends based on the loaded route config.
// It caches one httputil.ReverseProxy per backend so we're not constructing
// a new director function on every request.
type Proxy struct {
	cfg      *config.Config
	mu       sync.RWMutex
	proxies  map[string]*httputil.ReverseProxy
}

func New(cfg *config.Config) *Proxy {
	return &Proxy{cfg: cfg, proxies: make(map[string]*httputil.ReverseProxy)}
}

// Handler returns an http.Handler that matches the request path against the
// configured routes and forwards to the corresponding backend. If no route
// matches, it responds 404.
func (p *Proxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := p.cfg.MatchRoute(r.URL.Path)
		if route == nil {
			http.Error(w, "no route configured for this path", http.StatusNotFound)
			return
		}
		rp, err := p.reverseProxyFor(route.Backend)
		if err != nil {
			slog.Error("invalid backend URL", "backend", route.Backend, "err", err)
			http.Error(w, "backend misconfigured", http.StatusBadGateway)
			return
		}
		rp.ServeHTTP(w, r)
	})
}

func (p *Proxy) reverseProxyFor(backend string) (*httputil.ReverseProxy, error) {
	p.mu.RLock()
	rp, ok := p.proxies[backend]
	p.mu.RUnlock()
	if ok {
		return rp, nil
	}

	target, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// Re-check in case another goroutine created it while we waited for the lock.
	if rp, ok := p.proxies[backend]; ok {
		return rp, nil
	}
	rp = httputil.NewSingleHostReverseProxy(target)
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("backend request failed", "backend", backend, "path", r.URL.Path, "err", err)
		http.Error(w, "backend unavailable", http.StatusBadGateway)
	}
	p.proxies[backend] = rp
	return rp, nil
}
