package strategy

import (
	"net/http"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/backend"
)

type LeastResponseTime struct {
	backends []*backend.Backend
}

func NewLeastResponseTime(backends []*backend.Backend) *LeastResponseTime {
	return &LeastResponseTime{backends: backends}
}

func (l *LeastResponseTime) Next(_ *http.Request) (*backend.Backend, error) {
	healthy := healthyBackends(l.backends)
	if len(healthy) == 0 {
		return nil, ErrNoHealthyBackend
	}

	best := healthy[0]
	for _, b := range healthy[1:] {
		// Prefer backend with no latency sample yet (0) to seed it quickly.
		if b.AvgLatency.Load() < best.AvgLatency.Load() {
			best = b
		}
	}
	return best, nil
}

func (l *LeastResponseTime) OnRequestComplete(b *backend.Backend, duration time.Duration) {
	b.UpdateLatency(duration.Nanoseconds())
}
