package strategy

import (
	"net/http"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/backend"
)

type LeastConnections struct {
	backends []*backend.Backend
}

func NewLeastConnections(backends []*backend.Backend) *LeastConnections {
	return &LeastConnections{backends: backends}
}

func (l *LeastConnections) Next(_ *http.Request) (*backend.Backend, error) {
	healthy := healthyBackends(l.backends)
	if len(healthy) == 0 {
		return nil, ErrNoHealthyBackend
	}

	best := healthy[0]
	for _, b := range healthy[1:] {
		if b.ActiveConns.Load() < best.ActiveConns.Load() {
			best = b
		}
	}
	return best, nil
}

func (l *LeastConnections) OnRequestComplete(_ *backend.Backend, _ time.Duration) {}
