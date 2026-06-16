# Load Balancer

A reverse proxy load balancer built in Go that demonstrates mastery of proxy mechanics, stateful traffic distribution, and backend routing tradeoffs. Routing strategies are hot-swappable at runtime via an HTTP admin API — no restart required.

## Features

- **5 routing strategies** selectable at startup and hot-swappable at runtime
- **Active health checking** — backends are probed every 5 seconds and excluded from rotation when unhealthy
- **Live admin API** — inspect backend state, swap strategies, and trigger backend degradation/recovery
- **Consistent Hashing with virtual nodes** — minimal session reshuffling when backends are added or removed
- **EWMA latency tracking** — per-backend exponential moving average, updated on every request regardless of active strategy

---

## Architecture

```
                        ┌─────────────────────────────────┐
                        │          Load Balancer          │
                        │                                 │
  Client ──────────────▶│  :8080 Proxy                    │──────────▶ backend1:3000
                        │    └─ Strategy (hot-swappable)  │──────────▶ backend2:3000
  Admin  ──────────────▶│  :9090 Admin API                │──────────▶ backend3:3000
                        │    └─ Health Checker (5s)       │
                        └─────────────────────────────────┘
```

**Strategy interface:**
```go
type Strategy interface {
    Next(r *http.Request) (*backend.Backend, error)
    OnRequestComplete(b *backend.Backend, duration time.Duration)
}
```

Every strategy implements `Next` to select a backend and `OnRequestComplete` as a post-request hook. Stateless strategies (Round Robin, Consistent Hash) implement the hook as a no-op. Hot-swap is protected by a `sync.RWMutex` — in-flight requests complete uninterrupted, new requests immediately use the new strategy.

---

## Routing Strategies

| Strategy | Flag | Description |
|---|---|---|
| Round Robin | `round_robin` | Sequential distribution across healthy backends |
| Weighted Round Robin | `weighted_round_robin` | Nginx smooth weighted algorithm — capacity-proportional, interleaved |
| Least Connections | `least_connections` | Routes to the backend with fewest active concurrent connections |
| Least Response Time | `least_response_time` | Routes to the backend with the lowest EWMA latency |
| Consistent Hash | `consistent_hash` | IP-affinity via a virtual node ring — minimal reshuffling on topology changes |

### Weighted Round Robin

Uses the [Nginx smooth weighted algorithm](https://github.com/nginx/nginx/commit/52327e0627f49dbda1e8d5441b7e8e3ed6a63d9e). With weights `[5, 3, 2]` the sequence is interleaved (`A A B A C A B A B C`) rather than bursty (`A A A A A B B B C C`), distributing load evenly even at low request volumes.

### Least Response Time

Latency is tracked per backend using EWMA (α=0.1). Each new sample pulls the average 10% toward the real value, so recent measurements are weighted more heavily than old ones. The value is stored as a fixed-point `atomic.Int64` (nanoseconds) to avoid the need for locks or `atomic.Float64`.

### Consistent Hashing

Each backend is mapped to 150 virtual nodes on a hash ring (configurable). Client IP is hashed onto the ring; the request goes to the nearest node clockwise. When a backend is removed, only clients that mapped to its virtual nodes reshuffle — all other sessions are unaffected.

---

## Getting Started

**Prerequisites:** Docker, Docker Compose

```bash
git clone https://github.com/riorhezaharris/load-balancer
cd load-balancer
docker compose up --build
```

The proxy starts on `:8080` with `round_robin` as the default strategy. The admin API starts on `:9090`.

To start with a different strategy:

```yaml
# docker-compose.yml
command: ["./load-balancer", "-strategy", "least_response_time"]
```

---

## Mock Backends

Three mock backends run in Docker with configurable baseline latency:

| Backend | Base Delay |
|---|---|
| backend1 | 10ms |
| backend2 | 50ms |
| backend3 | 20ms |

Each backend exposes:
- `GET /health` — health check endpoint
- `POST /degrade` — adds 500ms artificial delay
- `POST /recover` — removes the artificial delay

---

## Admin API

### Get status
```bash
curl -s localhost:9090/admin/status | jq
```
```json
{
  "strategy": "round_robin",
  "backends": [
    {
      "url": "http://backend1:3000",
      "healthy": true,
      "active_conns": 0,
      "avg_latency_ms": 12.5
    }
  ]
}
```

### Hot-swap strategy
```bash
curl -X POST localhost:9090/admin/strategy \
  -H "Content-Type: application/json" \
  -d '{"strategy": "least_response_time"}'
```

Valid values: `round_robin`, `weighted_round_robin`, `least_connections`, `least_response_time`, `consistent_hash`

### Degrade / recover a backend
```bash
# Degrade backend2 (+500ms delay)
curl -X POST localhost:9090/admin/backends/1/degrade

# Recover backend2
curl -X POST localhost:9090/admin/backends/1/recover
```

Backend index: `0` = backend1, `1` = backend2, `2` = backend3

---

## Demo Walkthrough

### Round Robin
```bash
for i in {1..9}; do curl -s localhost:8080/ | jq .backend; done
# backend1 → backend2 → backend3 → backend1 → ...
```

### Weighted Round Robin
```bash
curl -X POST localhost:9090/admin/strategy \
  -H "Content-Type: application/json" \
  -d '{"strategy": "weighted_round_robin"}'

for i in {1..20}; do curl -s localhost:8080/ | jq .backend; done
# ~10x backend1, ~6x backend2, ~4x backend3
```

### Least Response Time — live traffic shifting
```bash
curl -X POST localhost:9090/admin/strategy \
  -H "Content-Type: application/json" \
  -d '{"strategy": "least_response_time"}'

# Degrade backend2
curl -X POST localhost:9090/admin/backends/1/degrade

# Watch traffic shift away from backend2 in real time
while true; do
  curl -s localhost:9090/admin/status | jq '.backends[] | {url, avg_latency_ms}'
  echo "---"
  sleep 1
done
```

Traffic will avoid backend2 as its EWMA climbs. Recover it and traffic redistributes:
```bash
curl -X POST localhost:9090/admin/backends/1/recover
```

### Consistent Hashing — session affinity
```bash
curl -X POST localhost:9090/admin/strategy \
  -H "Content-Type: application/json" \
  -d '{"strategy": "consistent_hash"}'

# Same client IP always hits the same backend
for i in {1..10}; do curl -s localhost:8080/ | jq .backend; done
# backend2 backend2 backend2 backend2 ...

# Stop a backend — clients NOT mapped to it are unaffected
docker compose stop backend3
for i in {1..10}; do curl -s localhost:8080/ | jq .backend; done
# Still backend2 — no reshuffle for this client

docker compose start backend3
```

---

## Project Structure

```
load-balancer/
├── main.go                          # Wiring, CLI flags, graceful shutdown
├── Dockerfile
├── docker-compose.yml
├── internal/
│   ├── backend/
│   │   └── backend.go               # Backend struct, EWMA latency update
│   ├── strategy/
│   │   ├── strategy.go              # Strategy interface
│   │   ├── round_robin.go
│   │   ├── weighted_round_robin.go  # Nginx smooth weighted algorithm
│   │   ├── least_connections.go
│   │   ├── least_response_time.go
│   │   └── consistent_hash.go       # Virtual node ring, fnv hash
│   ├── proxy/
│   │   └── proxy.go                 # RWMutex hot-swap, connection tracking
│   ├── health/
│   │   └── health.go                # Per-backend goroutine health checker
│   └── admin/
│       └── admin.go                 # Admin HTTP handler
└── docker/
    └── mock-backend/
        └── main.go                  # Configurable delay, /degrade, /recover
```

---

## Configuration

| Flag | Default | Description |
|---|---|---|
| `-strategy` | `round_robin` | Initial routing strategy |
| `-proxy-addr` | `:8080` | Proxy listen address |
| `-admin-addr` | `:9090` | Admin API listen address |
| `-health-interval` | `5s` | Health check polling interval |
| `-health-timeout` | `2s` | Health check request timeout |

---

## Design Decisions

See [`docs/adr/`](docs/adr/) for architectural decision records covering:

- [ADR-0001](docs/adr/0001-strategy-lifecycle-hook.md) — Why the Strategy interface includes an `OnRequestComplete` lifecycle hook
- [ADR-0002](docs/adr/0002-smooth-weighted-round-robin.md) — Why Weighted Round Robin uses the Nginx smooth algorithm over slot expansion
- [ADR-0003](docs/adr/0003-ewma-latency-tracking.md) — Why Least Response Time uses EWMA over simple average or sliding window
