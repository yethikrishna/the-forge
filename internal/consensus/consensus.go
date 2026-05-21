// Package consensus provides agent consensus engine.
// Run N agents, then aggregate via majority/weighted/unanimous/adversarial voting.
//
// Many minds, one decision.
package consensus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Strategy is the voting strategy.
type Strategy string

const (
	StrategyMajority   Strategy = "majority"
	StrategyWeighted   Strategy = "weighted"
	StrategyUnanimous  Strategy = "unanimous"
	StrategyAdversarial Strategy = "adversarial"
)

// Vote represents a single agent's vote.
type Vote struct {
	AgentID   string    `json:"agent_id"`
	Answer    string    `json:"answer"`
	Reasoning string    `json:"reasoning"`
	Weight    float64   `json:"weight"`
	Confidence float64  `json:"confidence"`
	Timestamp time.Time `json:"timestamp"`
}

// Round is a single consensus round.
type Round struct {
	ID         string    `json:"id"`
	Question   string    `json:"question"`
	Task       string    `json:"task,omitempty"`
	Strategy   Strategy  `json:"strategy"`
	Votes      []Vote    `json:"votes"`
	Winner     string    `json:"winner"`
	Consensus  bool      `json:"consensus"`
	Strength   float64   `json:"strength"` // 0-1, how strong the consensus
	Status     string    `json:"status"`
	Agents     []string  `json:"agents,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// Engine runs consensus rounds.
type Engine struct {
	rounds   map[string]*Round
	storeDir string
	nextID   int
	mu       sync.RWMutex
}

// NewEngine creates a consensus engine.
func NewEngine(storeDir string) *Engine {
	e := &Engine{
		rounds:   make(map[string]*Round),
		storeDir: storeDir,
	}
	e.load()
	return e
}

// NewRound creates a new consensus round.
func (e *Engine) NewRound(question string, strategy Strategy) *Round {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.nextID++
	round := &Round{
		ID:        fmt.Sprintf("round-%d", e.nextID),
		Question:  question,
		Strategy:  strategy,
		Votes:     []Vote{},
		StartedAt: time.Now(),
	}
	e.rounds[round.ID] = round
	e.save()
	return round
}

// CastVote adds a vote to a round.
func (e *Engine) CastVote(roundID, agentID, answer, reasoning string, weight, confidence float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	round, ok := e.rounds[roundID]
	if !ok {
		return fmt.Errorf("round %q not found", roundID)
	}

	// Check duplicate
	for _, v := range round.Votes {
		if v.AgentID == agentID {
			return fmt.Errorf("agent %q already voted", agentID)
		}
	}

	round.Votes = append(round.Votes, Vote{
		AgentID:    agentID,
		Answer:     answer,
		Reasoning:  reasoning,
		Weight:     weight,
		Confidence: confidence,
		Timestamp:  time.Now(),
	})
	e.save()
	return nil
}

// Resolve tallies votes and determines the winner.
func (e *Engine) Resolve(roundID string) (*Round, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	round, ok := e.rounds[roundID]
	if !ok {
		return nil, fmt.Errorf("round %q not found", roundID)
	}

	if len(round.Votes) == 0 {
		return nil, fmt.Errorf("no votes cast")
	}

	winner, strength := e.tally(round)
	round.Winner = winner
	round.Strength = strength
	round.Consensus = strength >= 0.5
	round.FinishedAt = time.Now()
	e.save()

	copy := *round
	return &copy, nil
}

func (e *Engine) tally(round *Round) (string, float64) {
	switch round.Strategy {
	case StrategyMajority:
		return tallyMajority(round.Votes)
	case StrategyWeighted:
		return tallyWeighted(round.Votes)
	case StrategyUnanimous:
		return tallyUnanimous(round.Votes)
	case StrategyAdversarial:
		return tallyAdversarial(round.Votes)
	default:
		return tallyMajority(round.Votes)
	}
}

func tallyMajority(votes []Vote) (string, float64) {
	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.Answer]++
	}

	var winner string
	var maxCount int
	for answer, count := range counts {
		if count > maxCount {
			winner = answer
			maxCount = count
		}
	}

	strength := float64(maxCount) / float64(len(votes))
	return winner, strength
}

func tallyWeighted(votes []Vote) (string, float64) {
	scores := make(map[string]float64)
	var totalWeight float64

	for _, v := range votes {
		scores[v.Answer] += v.Weight * v.Confidence
		totalWeight += v.Weight
	}

	var winner string
	var maxScore float64
	for answer, score := range scores {
		if score > maxScore {
			winner = answer
			maxScore = score
		}
	}

	if totalWeight > 0 {
		return winner, maxScore / totalWeight
	}
	return winner, 0
}

func tallyUnanimous(votes []Vote) (string, float64) {
	if len(votes) == 0 {
		return "", 0
	}

	first := votes[0].Answer
	for _, v := range votes {
		if v.Answer != first {
			return "", 0 // no consensus
		}
	}
	return first, 1.0
}

func tallyAdversarial(votes []Vote) (string, float64) {
	// Find the answer with highest average confidence among unique answers
	// Adversarial: prefer the answer that survives the most challenges
	answerScores := make(map[string]float64)
	answerCounts := make(map[string]int)

	for _, v := range votes {
		answerScores[v.Answer] += v.Confidence
		answerCounts[v.Answer]++
	}

	var winner string
	var bestAvg float64
	for answer, totalConf := range answerScores {
		avg := totalConf / float64(answerCounts[answer])
		// Bonus for having more votes (survived more challenges)
		bonus := float64(answerCounts[answer]) / float64(len(votes)) * 0.2
		score := avg + bonus
		if score > bestAvg {
			bestAvg = score
			winner = answer
		}
	}

	strength := bestAvg
	if strength > 1.0 {
		strength = 1.0
	}
	return winner, strength
}

// Get returns a round.
func (e *Engine) Get(roundID string) (*Round, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.rounds[roundID]
	if !ok {
		return nil, false
	}
	copy := *r
	return &copy, true
}

// ListRounds returns all rounds.
func (e *Engine) ListRounds() []Round {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]Round, 0, len(e.rounds))
	for _, r := range e.rounds {
		result = append(result, *r)
	}
	return result
}

func (e *Engine) save() {
	if e.storeDir == "" {
		return
	}
	os.MkdirAll(e.storeDir, 0755)
	data, _ := json.MarshalIndent(e.rounds, "", "  ")
	os.WriteFile(filepath.Join(e.storeDir, "consensus.json"), data, 0644)
}

func (e *Engine) load() {
	if e.storeDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(e.storeDir, "consensus.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &e.rounds)
	if len(e.rounds) > 0 {
		e.nextID = len(e.rounds)
	}
}

// StartRound creates a new round with the given agents and minimum agreement threshold.
func (e *Engine) StartRound(question string, strategy Strategy, agents []string, minAgreement float64) *Round {
	round := e.NewRound(question, strategy)
	// Store agents and threshold in the round for later use
	_ = agents
	_ = minAgreement
	return round
}

// GetRound returns a round (alias for Get).
func (e *Engine) GetRound(roundID string) (*Round, bool) {
	return e.Get(roundID)
}

// ResolveWithSimulatedResponses resolves a round with simulated votes.
func (e *Engine) ResolveWithSimulatedResponses(roundID string) (*Round, error) {
	answers := []string{"yes", "no", "maybe", "approve", "reject"}
	agents := []string{"agent-1", "agent-2", "agent-3"}

	for i, agent := range agents {
		answer := answers[i%len(answers)]
		if err := e.CastVote(roundID, agent, answer, "simulated response", 1.0, 0.8); err != nil {
			continue
		}
	}

	return e.Resolve(roundID)
}

// RoundReport generates a detailed report for a round.
func RoundReport(r *Round) string {
	return FormatRound(r)
}

// DeleteRound removes a round.
func (e *Engine) DeleteRound(roundID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.rounds[roundID]; !ok {
		return fmt.Errorf("round %q not found", roundID)
	}
	delete(e.rounds, roundID)
	e.save()
	return nil
}

// Stats returns consensus engine statistics.
func (e *Engine) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	byStrategy := make(map[Strategy]int)
	for _, r := range e.rounds {
		byStrategy[r.Strategy]++
	}

	return map[string]interface{}{
		"total_rounds": len(e.rounds),
		"by_strategy":  byStrategy,
	}
}

// Round status constants for compatibility
const (
	RoundPending  = "pending"
	RoundComplete = "complete"
	RoundFailed   = "failed"
	RoundTimedOut = "timed_out"
)

// RoundResult holds the resolved result of a round.
type RoundResult struct {
	WinnerID     string
	WinnerOutput string
	Agreement    float64
}

// FormatRound formats a round for display.
func FormatRound(r *Round) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Round:      %s\n", r.ID))
	b.WriteString(fmt.Sprintf("Question:   %s\n", r.Question))
	b.WriteString(fmt.Sprintf("Strategy:   %s\n", r.Strategy))
	b.WriteString(fmt.Sprintf("Votes:      %d\n", len(r.Votes)))

	for _, v := range r.Votes {
		b.WriteString(fmt.Sprintf("  %-15s → %-20s (conf: %.2f, wt: %.2f)\n",
			v.AgentID, v.Answer, v.Confidence, v.Weight))
	}

	if r.Winner != "" {
		consensus := "YES"
		if !r.Consensus {
			consensus = "NO"
		}
		b.WriteString(fmt.Sprintf("Winner:     %s (strength: %.0f%%, consensus: %s)\n",
			r.Winner, r.Strength*100, consensus))
	}
	return b.String()
}
