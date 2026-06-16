package strategy

import (
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/backend"
)

const defaultVirtualNodes = 150

type virtualNode struct {
	hash    uint32
	backend *backend.Backend
}

type ConsistentHash struct {
	mu           sync.RWMutex
	ring         []virtualNode // sorted by hash
	virtualNodes int
	backends     []*backend.Backend
}

func NewConsistentHash(backends []*backend.Backend, virtualNodes int) *ConsistentHash {
	if virtualNodes <= 0 {
		virtualNodes = defaultVirtualNodes
	}
	ch := &ConsistentHash{virtualNodes: virtualNodes, backends: backends}
	ch.buildRing()
	return ch
}

func (c *ConsistentHash) buildRing() {
	ring := make([]virtualNode, 0, len(c.backends)*c.virtualNodes)
	for _, b := range c.backends {
		if !b.Healthy.Load() {
			continue
		}
		for i := range c.virtualNodes {
			key := fmt.Sprintf("%s#%d", b.URL.String(), i)
			ring = append(ring, virtualNode{hash: hashKey(key), backend: b})
		}
	}
	sort.Slice(ring, func(i, j int) bool {
		return ring[i].hash < ring[j].hash
	})
	c.ring = ring
}

func (c *ConsistentHash) Next(r *http.Request) (*backend.Backend, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.ring) == 0 {
		return nil, ErrNoHealthyBackend
	}

	key := clientKey(r)
	h := hashKey(key)

	// Binary search for the first virtual node with hash >= h.
	idx := sort.Search(len(c.ring), func(i int) bool {
		return c.ring[i].hash >= h
	})
	if idx == len(c.ring) {
		idx = 0
	}
	return c.ring[idx].backend, nil
}

func (c *ConsistentHash) OnRequestComplete(_ *backend.Backend, _ time.Duration) {}

// RebuildRing rebuilds the ring after health state changes. Called by the
// health checker when a backend's status changes.
func (c *ConsistentHash) RebuildRing() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buildRing()
}

func clientKey(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	// Strip port — ephemeral port changes per connection and would break affinity.
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	return ip
}

func hashKey(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}
