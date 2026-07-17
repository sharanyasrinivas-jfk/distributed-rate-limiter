// Package metrics implements a minimal Prometheus text-exposition endpoint
// using only the standard library. A full client library was avoided here
// because its dependency chain reaches hosts unreachable in some restricted
// network environments; this hand-rolled version covers the counter/gauge
// primitives this project actually needs and speaks the same wire format.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type counterKey struct {
	name   string
	labels string
}

// Registry holds all counters and gauges and knows how to render them in
// Prometheus text exposition format for a GET /metrics scrape.
type Registry struct {
	mu       sync.Mutex
	counters map[counterKey]float64
	gauges   map[counterKey]float64
	help     map[string]string
}

func NewRegistry() *Registry {
	return &Registry{
		counters: make(map[counterKey]float64),
		gauges:   make(map[counterKey]float64),
		help:     make(map[string]string),
	}
}

func labelString(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`%s=%q`, k, labels[k]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// IncCounter increments a named, labeled counter (e.g. requests_total by
// route+status).
func (r *Registry) IncCounter(name, help string, labels map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.help[name] = help
	key := counterKey{name: name, labels: labelString(labels)}
	r.counters[key]++
}

// ObserveHistogramLike records a duration-style value as a running sum
// under a "_sum" counter plus a "_count" counter, which is enough to derive
// an average in Grafana/PromQL without pulling in the full histogram bucket
// machinery.
func (r *Registry) ObserveHistogramLike(name, help string, labels map[string]string, value float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.help[name] = help
	sumKey := counterKey{name: name + "_sum", labels: labelString(labels)}
	countKey := counterKey{name: name + "_count", labels: labelString(labels)}
	r.counters[sumKey] += value
	r.counters[countKey]++
}

// SetGauge sets a named, labeled gauge to an absolute value (e.g. circuit
// breaker state per backend: 0=closed, 1=half_open, 2=open).
func (r *Registry) SetGauge(name, help string, labels map[string]string, value float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.help[name] = help
	key := counterKey{name: name, labels: labelString(labels)}
	r.gauges[key] = value
}

// Handler exposes the registry in Prometheus text format at GET /metrics.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		defer r.mu.Unlock()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		names := make([]string, 0, len(r.help))
		for n := range r.help {
			names = append(names, n)
		}
		sort.Strings(names)

		for _, name := range names {
			fmt.Fprintf(w, "# HELP %s %s\n", name, r.help[name])
			metricType := "counter"
			if isGaugeName(r, name) {
				metricType = "gauge"
			}
			fmt.Fprintf(w, "# TYPE %s %s\n", name, metricType)

			for k, v := range r.counters {
				if k.name == name {
					fmt.Fprintf(w, "%s%s %v\n", k.name, k.labels, v)
				}
			}
			for k, v := range r.gauges {
				if k.name == name {
					fmt.Fprintf(w, "%s%s %v\n", k.name, k.labels, v)
				}
			}
		}
	})
}

func isGaugeName(r *Registry, name string) bool {
	for k := range r.gauges {
		if k.name == name {
			return true
		}
	}
	return false
}
