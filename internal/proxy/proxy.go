package proxy

import (
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/strategy"
)

type Proxy struct {
	mu       sync.RWMutex
	strategy strategy.Strategy
}

func New(s strategy.Strategy) *Proxy {
	return &Proxy{strategy: s}
}

func (p *Proxy) SwapStrategy(s strategy.Strategy) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.strategy = s
}

func (p *Proxy) CurrentStrategyName() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	switch p.strategy.(type) {
	case *strategy.RoundRobin:
		return "round_robin"
	case *strategy.WeightedRoundRobin:
		return "weighted_round_robin"
	case *strategy.LeastConnections:
		return "least_connections"
	case *strategy.LeastResponseTime:
		return "least_response_time"
	case *strategy.ConsistentHash:
		return "consistent_hash"
	default:
		return "unknown"
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	s := p.strategy
	p.mu.RUnlock()

	b, err := s.Next(r)
	if err != nil {
		http.Error(w, "no healthy backend available", http.StatusServiceUnavailable)
		return
	}

	b.ActiveConns.Add(1)
	start := time.Now()

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = b.URL.Scheme
			req.URL.Host = b.URL.Host
			req.Host = b.URL.Host
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "")
			}
			req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		},
	}

	rw := &responseWriter{ResponseWriter: w}
	rp.ServeHTTP(rw, r)

	duration := time.Since(start)
	b.ActiveConns.Add(-1)
	b.UpdateLatency(duration.Nanoseconds())
	s.OnRequestComplete(b, duration)
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Unwrap satisfies the http.ResponseController interface.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

