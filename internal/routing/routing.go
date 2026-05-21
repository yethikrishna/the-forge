// Package routing provides multi-agent request routing strategies.
// Every sword strikes at the right target.
package routing

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Strategy defines how requests are routed to agents.
type Strategy string

const (
	// RoundRobin cycles through agents in order.
	RoundRobin Strategy = "round_robin"
	// Random selects an agent at random.
	Random Strategy = "random"
	// LeastLoaded picks the agent with fewest active requests.
	LeastLoaded Strategy = "least_loaded"
	// Weighted distributes requests by weight.
	Weighted Strategy = "weighted"
	// Fallback tries agents in order until one succeeds.
	Fallback Strategy = "fallback"
	// LatencyBased picks the agent with lowest response time.
	LatencyBased Strategy = "latency_based"
)

// Agent represents a routable agent endpoint.
type Agent struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Weight   float64           `json:"weight"`
	Active   int64             `json:"active"`
	Latency  float64           `json:"latency_ms"` // average latency in ms
	Healthy  bool              `json:"healthy"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Router routes requests to agents based on a strategy.
type Router struct {
	strategy Strategy
	agents   []*Agent
	counter  uint64
	mu       sync.RWMutex
	rng      *rand.Rand
}

// New creates a new router with the given strategy.
func New(strategy Strategy) *Router {
	return &Router{
		strategy: strategy,
		agents:   make([]*Agent, 0),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddAgent adds an agent to the routing pool.
func (r *Router) AddAgent(agent *Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if agent.Healthy {
		agent.Healthy = true
	}
	if agent.Weight == 0 {
		agent.Weight = 1.0
	}
	r.agents = append(r.agents, agent)
}

// RemoveAgent removes an agent from the routing pool.
func (r *Router) RemoveAgent(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, a := range r.agents {
		if a.ID == id {
			r.agents = append(r.agents[:i], r.agents[i+1:]...)
			return
		}
	}
}

// Route selects an agent based on the routing strategy.
func (r *Router) Route() (*Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	healthy := r.healthyAgents()
	if len(healthy) == 0 {
		return nil, fmt.Errorf("routing: no healthy agents available")
	}

	switch r.strategy {
	case RoundRobin:
		return r.routeRoundRobin(healthy), nil
	case Random:
		return r.routeRandom(healthy), nil
	case LeastLoaded:
		return r.routeLeastLoaded(healthy), nil
	case Weighted:
		return r.routeWeighted(healthy), nil
	case Fallback:
		return r.routeFallback(healthy), nil
	case LatencyBased:
		return r.routeLatencyBased(healthy), nil
	default:
		return r.routeRoundRobin(healthy), nil
	}
}

// RouteForRequest selects an agent and increments its active count.
func (r *Router) RouteForRequest() (*Agent, error) {
	agent, err := r.Route()
	if err != nil {
		return nil, err
	}
	atomic.AddInt64(&agent.Active, 1)
	return agent, nil
}

// Release decrements an agent's active count after a request completes.
func (r *Router) Release(agent *Agent, latency time.Duration) {
	atomic.AddInt64(&agent.Active, -1)
	// Update average latency with exponential moving average
	ms := float64(latency.Milliseconds())
	if agent.Latency == 0 {
		agent.Latency = ms
	} else {
		agent.Latency = 0.7*agent.Latency + 0.3*ms
	}
}

// SetHealthy marks an agent as healthy or unhealthy.
func (r *Router) SetHealthy(id string, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.agents {
		if a.ID == id {
			a.Healthy = healthy
			return
		}
	}
}

// Agents returns all agents.
func (r *Router) Agents() []*Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Agent, len(r.agents))
	copy(result, r.agents)
	return result
}

// StrategyName returns the current strategy name.
func (r *Router) StrategyName() string {
	return string(r.strategy)
}

func (r *Router) healthyAgents() []*Agent {
	var healthy []*Agent
	for _, a := range r.agents {
		if a.Healthy {
			healthy = append(healthy, a)
		}
	}
	return healthy
}

func (r *Router) routeRoundRobin(agents []*Agent) *Agent {
	idx := atomic.AddUint64(&r.counter, 1)
	return agents[idx%uint64(len(agents))]
}

func (r *Router) routeRandom(agents []*Agent) *Agent {
	return agents[r.rng.Intn(len(agents))]
}

func (r *Router) routeLeastLoaded(agents []*Agent) *Agent {
	var best *Agent
	var minActive int64 = -1
	for _, a := range agents {
		active := atomic.LoadInt64(&a.Active)
		if minActive < 0 || active < minActive {
			minActive = active
			best = a
		}
	}
	return best
}

func (r *Router) routeWeighted(agents []*Agent) *Agent {
	var totalWeight float64
	for _, a := range agents {
		totalWeight += a.Weight
	}

	point := r.rng.Float64() * totalWeight
	var cumulative float64
	for _, a := range agents {
		cumulative += a.Weight
		if point <= cumulative {
			return a
		}
	}
	return agents[len(agents)-1]
}

func (r *Router) routeFallback(agents []*Agent) *Agent {
	return agents[0]
}

func (r *Router) routeLatencyBased(agents []*Agent) *Agent {
	var best *Agent
	var minLatency float64 = -1
	for _, a := range agents {
		if a.Latency > 0 {
			if minLatency < 0 || a.Latency < minLatency {
				minLatency = a.Latency
				best = a
			}
		}
	}
	if best == nil {
		return r.routeRoundRobin(agents)
	}
	return best
}
