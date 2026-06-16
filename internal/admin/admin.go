package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/riorhezaharris/load-balancer/internal/backend"
	"github.com/riorhezaharris/load-balancer/internal/proxy"
	"github.com/riorhezaharris/load-balancer/internal/strategy"
)

type StrategyFactory func(name string) (strategy.Strategy, error)

type Handler struct {
	proxy    *proxy.Proxy
	backends []*backend.Backend
	factory  StrategyFactory
	mux      *http.ServeMux
}

func New(p *proxy.Proxy, backends []*backend.Backend, factory StrategyFactory) *Handler {
	h := &Handler{proxy: p, backends: backends, factory: factory, mux: http.NewServeMux()}
	h.mux.HandleFunc("POST /admin/strategy", h.swapStrategy)
	h.mux.HandleFunc("GET /admin/status", h.status)
	h.mux.HandleFunc("POST /admin/backends/{id}/degrade", h.degradeBackend)
	h.mux.HandleFunc("POST /admin/backends/{id}/recover", h.recoverBackend)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

type swapRequest struct {
	Strategy string `json:"strategy"`
}

func (h *Handler) swapStrategy(w http.ResponseWriter, r *http.Request) {
	var req swapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	s, err := h.factory(req.Strategy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.proxy.SwapStrategy(s)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"strategy": req.Strategy})
}

type backendStatus struct {
	URL         string `json:"url"`
	Healthy     bool   `json:"healthy"`
	ActiveConns int64  `json:"active_conns"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

type statusResponse struct {
	Strategy string           `json:"strategy"`
	Backends []backendStatus  `json:"backends"`
}

func (h *Handler) status(w http.ResponseWriter, _ *http.Request) {
	resp := statusResponse{
		Strategy: h.proxy.CurrentStrategyName(),
		Backends: make([]backendStatus, len(h.backends)),
	}
	for i, b := range h.backends {
		resp.Backends[i] = backendStatus{
			URL:          b.URL.String(),
			Healthy:      b.Healthy.Load(),
			ActiveConns:  b.ActiveConns.Load(),
			AvgLatencyMs: float64(b.AvgLatency.Load()) / 1_000_000,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) degradeBackend(w http.ResponseWriter, r *http.Request) {
	h.forwardToBackend(w, r, "degrade")
}

func (h *Handler) recoverBackend(w http.ResponseWriter, r *http.Request) {
	h.forwardToBackend(w, r, "recover")
}

func (h *Handler) forwardToBackend(w http.ResponseWriter, r *http.Request, action string) {
	id := r.PathValue("id")
	idx := 0
	fmt.Sscanf(id, "%d", &idx)
	if idx < 0 || idx >= len(h.backends) {
		http.Error(w, "backend not found", http.StatusNotFound)
		return
	}
	b := h.backends[idx]
	targetURL := b.URL.String() + "/" + action
	resp, err := http.Post(targetURL, "application/json", strings.NewReader("{}"))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to contact backend: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
