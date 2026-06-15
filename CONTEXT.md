# Load Balancer

A reverse proxy gateway that distributes incoming HTTP requests across a pool of backends using a pluggable, hot-swappable routing strategy.

## Language

**Load Balancer**:
The single gateway process that accepts all incoming requests and forwards them to a backend.
_Avoid_: proxy, gateway, router

**Backend**:
An upstream HTTP server that receives forwarded requests from the load balancer.
_Avoid_: server, upstream, node, instance

**Strategy**:
The algorithm that selects which backend receives each incoming request.
_Avoid_: algorithm, policy, mode, balancer

**Hot-swap**:
Replacing the active Strategy at runtime without restarting the load balancer process.
_Avoid_: reload, restart, switch

**Round Robin**:
A Strategy that distributes requests sequentially across all healthy backends.

**Weighted Round Robin**:
A Round Robin variant where each backend receives traffic proportional to its configured weight.

**Least Connections**:
A Strategy that routes each request to the backend with the fewest active concurrent connections.

**Least Response Time**:
A Strategy that routes each request to the backend with the lowest measured average response latency.

**Consistent Hashing**:
A Strategy that maps a request attribute (e.g. client IP) onto a ring, ensuring the same source consistently reaches the same backend.

**Virtual Node**:
A phantom ring position representing a real backend in Consistent Hashing, used to achieve even keyspace distribution.
_Avoid_: replica, token

**Health Check**:
A periodic probe sent to a backend to determine whether it is available to receive traffic.

**Admin API**:
The HTTP control plane for managing and observing the load balancer at runtime.

## Relationships

- A **Strategy** selects one **Backend** per request
- A **Backend** may be represented by multiple **Virtual Nodes** when the active **Strategy** is Consistent Hashing
- The **Admin API** governs **Strategy** hot-swaps and surfaces per-**Backend** health and metrics
- A **Health Check** marks a **Backend** healthy or unhealthy; an unhealthy **Backend** is excluded from **Strategy** selection

## Example dialogue

> **Dev:** "Should the Strategy keep routing to a backend if its Health Check is failing?"
> **Domain expert:** "No — a Backend is only eligible for selection when its Health Check marks it healthy. The Strategy never sees unhealthy Backends."

> **Dev:** "If I Hot-swap from Least Connections to Round Robin, do the connection counts reset?"
> **Domain expert:** "No — connection counts are owned by the Backend, not the Strategy. They stay accurate across a Hot-swap."

## Flagged ambiguities

- "hot-swap via CLI flags" was used in the original brief — resolved: CLI flags set the *initial* Strategy; hot-swap refers exclusively to runtime replacement via the Admin API.
