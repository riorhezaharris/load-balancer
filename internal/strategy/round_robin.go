package strategy

import (
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/backend"
)

var ErrNoHealthyBackend = errors.New("no healthy backend available")

type RoundRobin struct {
	backends []*backend.Backend
	counter  atomic.Uint64
}

func NewRoundRobin(backends []*backend.Backend) *RoundRobin {
	return &RoundRobin{backends: backends}
}

func (r *RoundRobin) Next(_ *http.Request) (*backend.Backend, error) {
	healthy := healthyBackends(r.backends)
	if len(healthy) == 0 {
		return nil, ErrNoHealthyBackend
	}
	idx := r.counter.Add(1) - 1
	return healthy[idx%uint64(len(healthy))], nil
}

func (r *RoundRobin) OnRequestComplete(_ *backend.Backend, _ time.Duration) {}

func healthyBackends(all []*backend.Backend) []*backend.Backend {
	out := make([]*backend.Backend, 0, len(all))
	for _, b := range all {
		if b.Healthy.Load() {
			out = append(out, b)
		}
	}
	return out
}
