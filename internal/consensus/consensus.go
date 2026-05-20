// Package consensus implements an agent consensus engine — run N agents
// on the same task, then aggregate results using majority voting,
// weighted scoring, or adversarial validation. Like ensemble methods
// in ML, but for agent outputs.
package consensus

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Strategy defines how agent outputs are aggregated.
type Strategy string

const (
	StrategyMajority   Strategy = "majority"    // Most common output wins
	StrategyWeighted   Strategy = "weighted"    // Weighted by agent trust scores
	StrategyUnanimous  Strategy = "unanimous"   // All must agree
	StrategyAdversarial Strategy = "adversarial" // Best of N with critic
	StrategyFirstOK    Strategy = "first-ok"    // First acceptable result
)

// RoundStatus represents the status of a consensus round.
type RoundStatus string

const (
	RoundPending   RoundStatus = "pending"
	RoundRunning   RoundStatus = "running"
	RoundComplete  RoundStatus = "complete"
	RoundFailed    RoundStatus = "failed"
	RoundTimedOut  RoundStatus = "timed_out"
)

// AgentResponse is a single agent's response in a consensus round.
type AgentResponse struct {
	AgentID   string  `json:"agent_id"`
	Model     string  `json:"model"`
	Output    string  `json:"output"`
	Score     float64 `json:"score"`      // Quality score 0-100
	Cost      float64 `json:"cost"`
	Latency   string  `json:"latency"`
	Trust     float64 `json:"trust"`      // Agent trust score 0-1
	Error     string  `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ConsensusResult is the final result of a consensus round.
type ConsensusResult struct {
	WinnerID    string          `json:"winner_id"`
	WinnerOutput string         `json:"winner_output"`
	Agreement   float64         `json:"agreement"` // 0-1, how much agents agreed
	Strategy    Strategy        `json:"strategy"`
	Responses   []AgentResponse `json:"responses"`
}

// Round represents a consensus round.
type Round struct {
	ID          string          `json:"id"`
	Task        string          `json:"task"`
	Strategy    Strategy        `json:"strategy"`
	Agents      []string        `json:"agents"`
	MinAgreement float64        `json:"min_agreement"`
	Status      RoundStatus     `json:"status"`
	Responses   []AgentResponse `json:"responses"`
	Result      *ConsensusResult `json:"result,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Duration    string          `json:"duration,omitempty"`
}

// Engine runs consensus rounds.
type Engine struct {
	rounds   map[string]*Round
	storeDir string
	mu       sync.Mutex
}

// NewEngine creates a new consensus engine.
func NewEngine(storeDir string) *Engine {
	os.MkdirAll(storeDir, 0755)
	e := &Engine{
		rounds:   make(map[string]*Round),
		storeDir: storeDir,
	}
	e.load()
	return e
}

// StartRound starts a new consensus round.
func (e *Engine) StartRound(task string, strategy Strategy, agents []string, minAgreement float64) *Round {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := generateRoundID(task)
	now := time.Now()

	round := &Round{
		ID:           id,
		Task:         task,
		Strategy:     strategy,
		Agents:       agents,
		MinAgreement: minAgreement,
		Status:       RoundRunning,
		CreatedAt:    now,
	}

	e.rounds[id] = round
	e.save()

	return round
}

// AddResponse adds an agent's response to a round.
func (e *Engine) AddResponse(roundID string, resp AgentResponse) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	round, ok := e.rounds[roundID]
	if !ok {
		return fmt.Errorf("round %s not found", roundID)
	}

	if round.Status != RoundRunning {
		return fmt.Errorf("round %s is not running", roundID)
	}

	resp.Timestamp = time.Now()
	round.Responses = append(round.Responses, resp)
	e.save()

	return nil
}

// Resolve determines the consensus result for a round.
func (e *Engine) Resolve(roundID string) (*ConsensusResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	round, ok := e.rounds[roundID]
	if !ok {
		return nil, fmt.Errorf("round %s not found", roundID)
	}

	if len(round.Responses) == 0 {
		return nil, fmt.Errorf("no responses in round %s", roundID)
	}

	var result *ConsensusResult

	switch round.Strategy {
	case StrategyMajority:
		result = resolveMajority(round.Responses)
	case StrategyWeighted:
		result = resolveWeighted(round.Responses)
	case StrategyUnanimous:
		result = resolveUnanimous(round.Responses)
	case StrategyAdversarial:
		result = resolveAdversarial(round.Responses)
	case StrategyFirstOK:
		result = resolveFirstOK(round.Responses)
	default:
		result = resolveMajority(round.Responses)
	}

	round.Result = result
	round.Status = RoundComplete
	now := time.Now()
	round.CompletedAt = &now
	round.Duration = now.Sub(round.CreatedAt).Round(time.Millisecond).String()
	e.save()

	return result, nil
}

// ResolveWithSimulatedResponses resolves a round with simulated agent responses.
// Used for testing and demos when real agents aren't available.
func (e *Engine) ResolveWithSimulatedResponses(roundID string) (*ConsensusResult, error) {
	e.mu.Lock()
	round, ok := e.rounds[roundID]
	if !ok {
		e.mu.Unlock()
		return nil, fmt.Errorf("round %s not found", roundID)
	}
	e.mu.Unlock()

	// Add simulated responses for each agent
	for i, agentID := range round.Agents {
		resp := AgentResponse{
			AgentID: agentID,
			Model:   []string{"gpt-4", "claude-sonnet-4", "deepseek-v3"}[i%3],
			Output:  fmt.Sprintf("Response from %s for: %s", agentID, truncate(round.Task, 50)),
			Score:   85.0 + float64(i)*3,
			Cost:    0.01 + float64(i)*0.005,
			Latency: fmt.Sprintf("%.1fs", 1.0+float64(i)*0.3),
			Trust:   0.9 - float64(i)*0.05,
		}
		e.AddResponse(roundID, resp)
	}

	return e.Resolve(roundID)
}

// GetRound retrieves a round by ID.
func (e *Engine) GetRound(id string) (*Round, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.rounds[id]
	return r, ok
}

// ListRounds lists all rounds.
func (e *Engine) ListRounds() []*Round {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]*Round, 0, len(e.rounds))
	for _, r := range e.rounds {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// DeleteRound removes a round.
func (e *Engine) DeleteRound(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rounds[id]; !ok {
		return fmt.Errorf("round %s not found", id)
	}
	delete(e.rounds, id)
	e.save()
	return nil
}

// RoundReport generates a human-readable round report.
func RoundReport(r *Round) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Consensus Round: %s\n", r.ID))
	b.WriteString(fmt.Sprintf("Task: %s\n", truncate(r.Task, 80)))
	b.WriteString(fmt.Sprintf("Strategy: %s | Agents: %d | Status: %s\n", r.Strategy, len(r.Agents), r.Status))

	if r.Duration != "" {
		b.WriteString(fmt.Sprintf("Duration: %s\n", r.Duration))
	}

	b.WriteString("\nResponses:\n")
	for _, resp := range r.Responses {
		winner := ""
		if r.Result != nil && resp.AgentID == r.Result.WinnerID {
			winner = " 🏆"
		}
		b.WriteString(fmt.Sprintf("  %-15s [%s] Score: %.1f Cost: $%.4f Latency: %s%s\n",
			resp.AgentID, resp.Model, resp.Score, resp.Cost, resp.Latency, winner))
	}

	if r.Result != nil {
		b.WriteString(fmt.Sprintf("\nResult:\n"))
		b.WriteString(fmt.Sprintf("  Winner: %s (agreement: %.0f%%)\n", r.Result.WinnerID, r.Result.Agreement*100))
		b.WriteString(fmt.Sprintf("  Output: %s\n", truncate(r.Result.WinnerOutput, 100)))
	}

	return b.String()
}

// Stats returns engine statistics.
func (e *Engine) Stats() map[string]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()

	total := len(e.rounds)
	byStatus := make(map[RoundStatus]int)
	byStrategy := make(map[Strategy]int)

	for _, r := range e.rounds {
		byStatus[r.Status]++
		byStrategy[r.Strategy]++
	}

	return map[string]interface{}{
		"total_rounds": total,
		"by_status":    byStatus,
		"by_strategy":  byStrategy,
	}
}

// Resolution strategies

func resolveMajority(responses []AgentResponse) *ConsensusResult {
	// Group by similar output
	groups := make(map[string][]AgentResponse)
	for _, r := range responses {
		key := normalizeOutput(r.Output)
		groups[key] = append(groups[key], r)
	}

	// Find largest group
	var largest string
	largestSize := 0
	for key, group := range groups {
		if len(group) > largestSize {
			largestSize = len(group)
			largest = key
		}
	}

	// Pick highest-scoring from largest group
	winner := groups[largest][0]
	for _, r := range groups[largest] {
		if r.Score > winner.Score {
			winner = r
		}
	}

	agreement := float64(largestSize) / float64(len(responses))

	return &ConsensusResult{
		WinnerID:     winner.AgentID,
		WinnerOutput: winner.Output,
		Agreement:    agreement,
		Strategy:     StrategyMajority,
		Responses:    responses,
	}
}

func resolveWeighted(responses []AgentResponse) *ConsensusResult {
	// Weight = trust * score
	bestScore := 0.0
	var winner AgentResponse

	for _, r := range responses {
		weighted := r.Trust * r.Score
		if weighted > bestScore {
			bestScore = weighted
			winner = r
		}
	}

	agreement := 0.0
	if len(responses) > 0 {
		// Agreement = how close others are to winner
		totalDiff := 0.0
		for _, r := range responses {
			totalDiff += abs(r.Score - winner.Score)
		}
		avgDiff := totalDiff / float64(len(responses))
		agreement = 1.0 - (avgDiff / 100.0)
		if agreement < 0 {
			agreement = 0
		}
	}

	return &ConsensusResult{
		WinnerID:     winner.AgentID,
		WinnerOutput: winner.Output,
		Agreement:    agreement,
		Strategy:     StrategyWeighted,
		Responses:    responses,
	}
}

func resolveUnanimous(responses []AgentResponse) *ConsensusResult {
	// All must agree (within tolerance)
	agreement := computeAgreement(responses)

	winner := responses[0]
	for _, r := range responses {
		if r.Score > winner.Score {
			winner = r
		}
	}

	return &ConsensusResult{
		WinnerID:     winner.AgentID,
		WinnerOutput: winner.Output,
		Agreement:    agreement,
		Strategy:     StrategyUnanimous,
		Responses:    responses,
	}
}

func resolveAdversarial(responses []AgentResponse) *ConsensusResult {
	// Pick the response with the highest score (adversarial — best wins)
	winner := responses[0]
	for _, r := range responses {
		if r.Score > winner.Score {
			winner = r
		}
	}

	agreement := computeAgreement(responses)

	return &ConsensusResult{
		WinnerID:     winner.AgentID,
		WinnerOutput: winner.Output,
		Agreement:    agreement,
		Strategy:     StrategyAdversarial,
		Responses:    responses,
	}
}

func resolveFirstOK(responses []AgentResponse) *ConsensusResult {
	// First response with score >= 70
	for _, r := range responses {
		if r.Score >= 70 {
			return &ConsensusResult{
				WinnerID:     r.AgentID,
				WinnerOutput: r.Output,
				Agreement:    computeAgreement(responses),
				Strategy:     StrategyFirstOK,
				Responses:    responses,
			}
		}
	}

	// Fallback to highest score
	return resolveAdversarial(responses)
}

func computeAgreement(responses []AgentResponse) float64 {
	if len(responses) <= 1 {
		return 1.0
	}

	totalDiff := 0.0
	count := 0
	for i := 0; i < len(responses); i++ {
		for j := i + 1; j < len(responses); j++ {
			totalDiff += abs(responses[i].Score - responses[j].Score)
			count++
		}
	}

	avgDiff := totalDiff / float64(count)
	agreement := 1.0 - (avgDiff / 100.0)
	if agreement < 0 {
		return 0
	}
	return agreement
}

func normalizeOutput(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func generateRoundID(task string) string {
	h := sha256.Sum256([]byte(task + time.Now().String()))
	return fmt.Sprintf("round-%x", h[:8])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func (e *Engine) save() {
	data, _ := json.MarshalIndent(e.rounds, "", "  ")
	os.WriteFile(filepath.Join(e.storeDir, "rounds.json"), data, 0644)
}

func (e *Engine) load() {
	data, err := os.ReadFile(filepath.Join(e.storeDir, "rounds.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &e.rounds)
}
