package strategy

import (
	"net/http"
	"sync"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/backend"
)

// weightedPeer tracks the smooth-weighted state for one backend.
type weightedPeer struct {
	backend       *backend.Backend
	currentWeight int
}

// WeightedRoundRobin implements the Nginx smooth weighted round-robin algorithm.
// Each round: pick the peer with the highest currentWeight, subtract total weight
// from it, then add each peer's configured weight to currentWeight.
type WeightedRoundRobin struct {
	mu    sync.Mutex
	peers []*weightedPeer
}

func NewWeightedRoundRobin(backends []*backend.Backend) *WeightedRoundRobin {
	peers := make([]*weightedPeer, len(backends))
	for i, b := range backends {
		peers[i] = &weightedPeer{backend: b}
	}
	return &WeightedRoundRobin{peers: peers}
}

func (w *WeightedRoundRobin) Next(_ *http.Request) (*backend.Backend, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var active []*weightedPeer
	total := 0
	for _, p := range w.peers {
		if p.backend.Healthy.Load() {
			active = append(active, p)
			total += p.backend.Weight
		}
	}
	if len(active) == 0 {
		return nil, ErrNoHealthyBackend
	}

	for _, p := range active {
		p.currentWeight += p.backend.Weight
	}

	best := active[0]
	for _, p := range active[1:] {
		if p.currentWeight > best.currentWeight {
			best = p
		}
	}
	best.currentWeight -= total

	return best.backend, nil
}

func (w *WeightedRoundRobin) OnRequestComplete(_ *backend.Backend, _ time.Duration) {}
