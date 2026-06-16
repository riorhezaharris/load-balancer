package health

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/backend"
)

type OnStatusChange func(b *backend.Backend, healthy bool)

type Checker struct {
	backends        []*backend.Backend
	interval        time.Duration
	timeout         time.Duration
	client          *http.Client
	onStatusChange  OnStatusChange
}

func NewChecker(backends []*backend.Backend, interval, timeout time.Duration, onChange OnStatusChange) *Checker {
	return &Checker{
		backends:       backends,
		interval:       interval,
		timeout:        timeout,
		client:         &http.Client{Timeout: timeout},
		onStatusChange: onChange,
	}
}

func (c *Checker) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, b := range c.backends {
				go c.probe(b)
			}
		}
	}
}

func (c *Checker) probe(b *backend.Backend) {
	url := b.URL.String() + "/health"
	resp, err := c.client.Get(url)
	healthy := err == nil && resp.StatusCode == http.StatusOK
	if resp != nil {
		resp.Body.Close()
	}

	wasHealthy := b.Healthy.Load()
	b.Healthy.Store(healthy)

	if wasHealthy != healthy {
		status := "unhealthy"
		if healthy {
			status = "healthy"
		}
		log.Printf("backend %s is now %s", b.URL.Host, status)
		if c.onStatusChange != nil {
			c.onStatusChange(b, healthy)
		}
	}
}
