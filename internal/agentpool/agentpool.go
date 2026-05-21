// Package agentpool provides agent pool management with auto-scaling,
// health monitoring, and load balancing. It manages pools of agents
// that can scale up/down based on demand.
package agentpool

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// AgentStatus represents an agent's status in the pool.
type AgentStatus string

const (
	AgentIdle      AgentStatus = "idle"
	AgentBusy      AgentStatus = "busy"
	AgentDraining  AgentStatus = "draining"
	AgentUnhealthy AgentStatus = "unhealthy"
	AgentStopped   AgentStatus = "stopped"
)

// ScalingPolicy represents an auto-scaling policy.
type ScalingPolicy struct {
	MinAgents     int     `json:"min_agents"`
	MaxAgents     int     `json:"max_agents"`
	TargetCPU     float64 `json:"target_cpu"`      // 0-1
	TargetQueueDepth int   `json:"target_queue_depth"`
	ScaleUpCooldown  time.Duration `json:"scale_up_cooldown"`
	ScaleDownCooldown time.Duration `json:"scale_down_cooldown"`
	ScaleUpThreshold  float64 `json:"scale_up_threshold"`  // queue depth to trigger scale up
	ScaleDownThreshold float64 `json:"scale_down_threshold"` // queue depth to trigger scale down
}

// Agent represents an agent in the pool.
type Agent struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`     // "coder", "reviewer", "planner", "tester"
	Model       string            `json:"model"`
	Status      AgentStatus       `json:"status"`
	PoolID      string            `json:"pool_id"`
	JoinedAt    time.Time         `json:"joined_at"`
	LastActive  time.Time         `json:"last_active"`
	TasksDone   int               `json:"tasks_done"`
	TasksFailed int               `json:"tasks_failed"`
	Cost        float64           `json:"cost"`
	CPUUsage    float64           `json:"cpu_usage"` // 0-1
	MemoryUsage float64           `json:"memory_usage"` // 0-1
	HealthScore float64           `json:"health_score"` // 0-100
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Pool represents an agent pool.
type Pool struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	AgentType     string        `json:"agent_type"`
	DefaultModel  string        `json:"default_model"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	ScalingPolicy ScalingPolicy `json:"scaling_policy"`
	LastScaleUp   time.Time     `json:"last_scale_up,omitempty"`
	LastScaleDown time.Time     `json:"last_scale_down,omitempty"`
	Agents        []string      `json:"agents"` // agent IDs
	Tags          []string      `json:"tags"`
}

// Manager manages agent pools.
type Manager struct {
	mu     sync.RWMutex
	dir    string
	pools  map[string]*Pool
	agents map[string]*Agent
}

// NewManager creates a new pool manager.
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create pool dir: %w", err)
	}
	m := &Manager{
		dir:    dir,
		pools:  make(map[string]*Pool),
		agents: make(map[string]*Agent),
	}
	m.load()
	return m, nil
}

func (m *Manager) load() {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			continue
		}
		if e.Name()[:5] == "pool-" {
			var p Pool
			if err := json.Unmarshal(data, &p); err == nil {
				m.pools[p.ID] = &p
			}
		} else if e.Name()[:6] == "agent-" {
			var a Agent
			if err := json.Unmarshal(data, &a); err == nil {
				m.agents[a.ID] = &a
			}
		}
	}
}

func (m *Manager) savePool(p *Pool) error {
	data, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(filepath.Join(m.dir, "pool-"+p.ID+".json"), data, 0644)
}

func (m *Manager) saveAgent(a *Agent) error {
	data, _ := json.MarshalIndent(a, "", "  ")
	return os.WriteFile(filepath.Join(m.dir, "agent-"+a.ID+".json"), data, 0644)
}

// CreatePool creates a new agent pool.
func (m *Manager) CreatePool(name, agentType, model string, policy ScalingPolicy) (*Pool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := &Pool{
		ID:            fmt.Sprintf("pool-%d", time.Now().UnixNano()),
		Name:          name,
		AgentType:     agentType,
		DefaultModel:  model,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ScalingPolicy: policy,
		Agents:        []string{},
		Tags:          []string{},
	}

	if p.ScalingPolicy.MinAgents == 0 {
		p.ScalingPolicy.MinAgents = 1
	}
	if p.ScalingPolicy.MaxAgents == 0 {
		p.ScalingPolicy.MaxAgents = 10
	}
	if p.ScalingPolicy.ScaleUpCooldown == 0 {
		p.ScalingPolicy.ScaleUpCooldown = 2 * time.Minute
	}
	if p.ScalingPolicy.ScaleDownCooldown == 0 {
		p.ScalingPolicy.ScaleDownCooldown = 5 * time.Minute
	}

	m.pools[p.ID] = p
	return p, m.savePool(p)
}

// AddAgent adds an agent to a pool.
func (m *Manager) AddAgent(poolID, name, model string) (*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("pool %s not found", poolID)
	}

	a := &Agent{
		ID:          fmt.Sprintf("agent-%d", time.Now().UnixNano()),
		Name:        name,
		Type:        pool.AgentType,
		Model:       model,
		Status:      AgentIdle,
		PoolID:      poolID,
		JoinedAt:    time.Now(),
		LastActive:  time.Now(),
		HealthScore: 100,
		Metadata:    make(map[string]string),
	}

	pool.Agents = append(pool.Agents, a.ID)
	pool.UpdatedAt = time.Now()
	m.agents[a.ID] = a

	m.savePool(pool)
	m.saveAgent(a)
	return a, nil
}

// RemoveAgent removes an agent from its pool.
func (m *Manager) RemoveAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	pool, ok := m.pools[agent.PoolID]
	if !ok {
		return fmt.Errorf("pool %s not found", agent.PoolID)
	}

	// Remove from pool
	filtered := make([]string, 0, len(pool.Agents))
	for _, id := range pool.Agents {
		if id != agentID {
			filtered = append(filtered, id)
		}
	}
	pool.Agents = filtered
	pool.UpdatedAt = time.Now()

	delete(m.agents, agentID)
	os.Remove(filepath.Join(m.dir, "agent-"+agentID+".json"))
	m.savePool(pool)
	return nil
}

// GetAgent retrieves an agent.
func (m *Manager) GetAgent(id string) (*Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.agents[id]
	return a, ok
}

// GetPool retrieves a pool.
func (m *Manager) GetPool(id string) (*Pool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.pools[id]
	return p, ok
}

// ListPools lists all pools.
func (m *Manager) ListPools() []Pool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Pool
	for _, p := range m.pools {
		result = append(result, *p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// PoolAgents returns all agents in a pool.
func (m *Manager) PoolAgents(poolID string) []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.poolAgentsUnlocked(poolID)
}

func (m *Manager) poolAgentsUnlocked(poolID string) []*Agent {
	pool, ok := m.pools[poolID]
	if !ok {
		return nil
	}

	var agents []*Agent
	for _, id := range pool.Agents {
		if a, ok := m.agents[id]; ok {
			agents = append(agents, a)
		}
	}
	return agents
}

// AssignTask assigns a task to the least-busy agent in a pool.
func (m *Manager) AssignTask(poolID string) (*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	agents := m.poolAgentsUnlocked(poolID)
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agents in pool %s", poolID)
	}

	// Find idle agent first
	for _, a := range agents {
		if a.Status == AgentIdle && a.HealthScore > 50 {
			a.Status = AgentBusy
			a.LastActive = time.Now()
			m.saveAgent(a)
			return a, nil
		}
	}

	// Find least busy healthy agent
	var best *Agent
	bestHealth := 0.0
	for _, a := range agents {
		if a.Status != AgentUnhealthy && a.Status != AgentDraining && a.HealthScore > bestHealth {
			best = a
			bestHealth = a.HealthScore
		}
	}

	if best != nil {
		best.LastActive = time.Now()
		m.saveAgent(best)
		return best, nil
	}

	return nil, fmt.Errorf("no available agents in pool %s", poolID)
}

// ReleaseAgent marks an agent as idle.
func (m *Manager) ReleaseAgent(agentID string, success bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	a.Status = AgentIdle
	a.LastActive = time.Now()
	if success {
		a.TasksDone++
	} else {
		a.TasksFailed++
	}
	return m.saveAgent(a)
}

// UpdateHealth updates an agent's health metrics.
func (m *Manager) UpdateHealth(agentID string, cpuUsage, memoryUsage, healthScore float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	a.CPUUsage = cpuUsage
	a.MemoryUsage = memoryUsage
	a.HealthScore = healthScore

	if healthScore < 25 {
		a.Status = AgentUnhealthy
	}

	return m.saveAgent(a)
}

// DrainAgent marks an agent for draining (won't receive new tasks).
func (m *Manager) DrainAgent(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	a.Status = AgentDraining
	return m.saveAgent(a)
}

// PoolStats represents pool statistics.
type PoolStats struct {
	PoolID      string  `json:"pool_id"`
	TotalAgents int     `json:"total_agents"`
	Idle        int     `json:"idle"`
	Busy        int     `json:"busy"`
	Draining    int     `json:"draining"`
	Unhealthy   int     `json:"unhealthy"`
	AvgHealth   float64 `json:"avg_health"`
	AvgCPU      float64 `json:"avg_cpu"`
	TotalTasks  int     `json:"total_tasks"`
	TotalCost   float64 `json:"total_cost"`
}

// Stats returns statistics for a pool.
func (m *Manager) Stats(poolID string) (*PoolStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("pool %s not found", poolID)
	}

	stats := &PoolStats{PoolID: poolID}
	agents := m.poolAgentsUnlocked(poolID)
	stats.TotalAgents = len(agents)

	var totalHealth, totalCPU, totalCost float64
	for _, a := range agents {
		totalHealth += a.HealthScore
		totalCPU += a.CPUUsage
		totalCost += a.Cost
		stats.TotalTasks += a.TasksDone + a.TasksFailed

		switch a.Status {
		case AgentIdle:
			stats.Idle++
		case AgentBusy:
			stats.Busy++
		case AgentDraining:
			stats.Draining++
		case AgentUnhealthy:
			stats.Unhealthy++
		}
	}

	if stats.TotalAgents > 0 {
		stats.AvgHealth = totalHealth / float64(stats.TotalAgents)
		stats.AvgCPU = totalCPU / float64(stats.TotalAgents)
	}
	stats.TotalCost = totalCost

	return stats, nil
}

// ScaleUp adds agents to a pool.
func (m *Manager) ScaleUp(poolID, model string, count int) ([]*Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("pool %s not found", poolID)
	}

	if len(pool.Agents)+count > pool.ScalingPolicy.MaxAgents {
		count = pool.ScalingPolicy.MaxAgents - len(pool.Agents)
		if count <= 0 {
			return nil, fmt.Errorf("pool already at max capacity (%d)", pool.ScalingPolicy.MaxAgents)
		}
	}

	var agents []*Agent
	for i := 0; i < count; i++ {
		a := &Agent{
			ID:          fmt.Sprintf("agent-%d-%d", time.Now().UnixNano(), i),
			Name:        fmt.Sprintf("%s-%d", pool.AgentType, len(pool.Agents)+i+1),
			Type:        pool.AgentType,
			Model:       model,
			Status:      AgentIdle,
			PoolID:      poolID,
			JoinedAt:    time.Now(),
			LastActive:  time.Now(),
			HealthScore: 100,
			Metadata:    make(map[string]string),
		}
		pool.Agents = append(pool.Agents, a.ID)
		m.agents[a.ID] = a
		m.saveAgent(a)
		agents = append(agents, a)
	}

	pool.LastScaleUp = time.Now()
	pool.UpdatedAt = time.Now()
	m.savePool(pool)

	return agents, nil
}

// ScaleDown removes idle agents from a pool.
func (m *Manager) ScaleDown(poolID string, count int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return 0, fmt.Errorf("pool %s not found", poolID)
	}

	// Only remove idle agents
	var removed int
	for _, id := range pool.Agents {
		if removed >= count {
			break
		}
		if len(pool.Agents)-removed <= pool.ScalingPolicy.MinAgents {
			break
		}
		a, ok := m.agents[id]
		if ok && a.Status == AgentIdle {
			a.Status = AgentStopped
			m.saveAgent(a)
			removed++
		}
	}

	if removed > 0 {
		// Clean up stopped agents from pool
		filtered := make([]string, 0)
		for _, id := range pool.Agents {
			if a, ok := m.agents[id]; ok && a.Status != AgentStopped {
				filtered = append(filtered, id)
			} else {
				delete(m.agents, id)
				os.Remove(filepath.Join(m.dir, "agent-"+id+".json"))
			}
		}
		pool.Agents = filtered
		pool.LastScaleDown = time.Now()
		pool.UpdatedAt = time.Now()
		m.savePool(pool)
	}

	return removed, nil
}

// CheckScaling evaluates whether a pool needs to scale.
func (m *Manager) CheckScaling(poolID string, queueDepth int) (action string, count int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return "none", 0
	}

	policy := pool.ScalingPolicy

	// Scale up?
	if queueDepth > int(policy.ScaleUpThreshold) && len(pool.Agents) < policy.MaxAgents {
		if time.Since(pool.LastScaleUp) > policy.ScaleUpCooldown {
			needed := int(math.Ceil(float64(queueDepth) / policy.ScaleUpThreshold))
			if needed > policy.MaxAgents-len(pool.Agents) {
				needed = policy.MaxAgents - len(pool.Agents)
			}
			return "scale-up", needed
		}
	}

	// Scale down?
	if queueDepth < int(policy.ScaleDownThreshold) && len(pool.Agents) > policy.MinAgents {
		if time.Since(pool.LastScaleDown) > policy.ScaleDownCooldown {
			excess := len(pool.Agents) - policy.MinAgents
			return "scale-down", excess
		}
	}

	return "none", 0
}

var _ = math.Ceil
