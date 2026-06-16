package strategy

import (
	"net/http"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/backend"
)

type Strategy interface {
	Next(r *http.Request) (*backend.Backend, error)
	OnRequestComplete(b *backend.Backend, duration time.Duration)
}
