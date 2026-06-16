package backend

import (
	"net/url"
	"sync/atomic"
)

type Backend struct {
	URL         *url.URL
	Weight      int
	Healthy     atomic.Bool
	ActiveConns atomic.Int64
	AvgLatency  atomic.Int64 // EWMA nanoseconds as fixed-point (x1000)
}

func New(rawURL string, weight int) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	b := &Backend{URL: u, Weight: weight}
	b.Healthy.Store(true)
	return b, nil
}

func (b *Backend) UpdateLatency(sampleNs int64) {
	const alpha = 100 // α = 0.1 scaled by 1000
	for {
		old := b.AvgLatency.Load()
		// EWMA: new = α*sample + (1-α)*old, all scaled by 1000
		updated := (alpha*sampleNs + (1000-alpha)*old) / 1000
		if b.AvgLatency.CompareAndSwap(old, updated) {
			return
		}
	}
}
