package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/riorhezaharris/load-balancer/internal/admin"
	"github.com/riorhezaharris/load-balancer/internal/backend"
	"github.com/riorhezaharris/load-balancer/internal/health"
	"github.com/riorhezaharris/load-balancer/internal/proxy"
	"github.com/riorhezaharris/load-balancer/internal/strategy"
)

var (
	strategyFlag   = flag.String("strategy", "round_robin", "initial routing strategy")
	proxyAddr      = flag.String("proxy-addr", ":8080", "proxy listen address")
	adminAddr      = flag.String("admin-addr", ":9090", "admin API listen address")
	healthInterval = flag.Duration("health-interval", 5*time.Second, "health check interval")
	healthTimeout  = flag.Duration("health-timeout", 2*time.Second, "health check timeout")
)

var backendURLs = []struct {
	url    string
	weight int
}{
	{"http://backend1:3000", 5},
	{"http://backend2:3000", 3},
	{"http://backend3:3000", 2},
}

func main() {
	flag.Parse()

	backends := mustLoadBackends()
	factory := strategyFactory(backends)

	initialStrategy, err := factory(*strategyFlag)
	if err != nil {
		log.Fatalf("invalid strategy %q: %v", *strategyFlag, err)
	}

	p := proxy.New(initialStrategy)
	adminHandler := admin.New(p, backends, factory)

	// Health checker notifies the consistent hash ring on status changes.
	var consistentHashStrategy *strategy.ConsistentHash
	if ch, ok := initialStrategy.(*strategy.ConsistentHash); ok {
		consistentHashStrategy = ch
	}

	checker := health.NewChecker(backends, *healthInterval, *healthTimeout, func(b *backend.Backend, _ bool) {
		if consistentHashStrategy != nil {
			consistentHashStrategy.RebuildRing()
		}
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go checker.Run(ctx)

	proxyServer := &http.Server{Addr: *proxyAddr, Handler: p}
	adminServer := &http.Server{Addr: *adminAddr, Handler: adminHandler}

	go func() {
		log.Printf("proxy listening on %s (strategy: %s)", *proxyAddr, *strategyFlag)
		if err := proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("proxy: %v", err)
		}
	}()
	go func() {
		log.Printf("admin API listening on %s", *adminAddr)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("admin: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	proxyServer.Shutdown(shutdownCtx)
	adminServer.Shutdown(shutdownCtx)
}

func mustLoadBackends() []*backend.Backend {
	backends := make([]*backend.Backend, 0, len(backendURLs))
	for _, cfg := range backendURLs {
		b, err := backend.New(cfg.url, cfg.weight)
		if err != nil {
			log.Fatalf("invalid backend URL %q: %v", cfg.url, err)
		}
		backends = append(backends, b)
	}
	return backends
}

func strategyFactory(backends []*backend.Backend) admin.StrategyFactory {
	return func(name string) (strategy.Strategy, error) {
		switch name {
		case "round_robin":
			return strategy.NewRoundRobin(backends), nil
		case "weighted_round_robin":
			return strategy.NewWeightedRoundRobin(backends), nil
		case "least_connections":
			return strategy.NewLeastConnections(backends), nil
		case "least_response_time":
			return strategy.NewLeastResponseTime(backends), nil
		case "consistent_hash":
			return strategy.NewConsistentHash(backends, 150), nil
		default:
			return nil, fmt.Errorf("unknown strategy %q; valid: round_robin, weighted_round_robin, least_connections, least_response_time, consistent_hash", name)
		}
	}
}
