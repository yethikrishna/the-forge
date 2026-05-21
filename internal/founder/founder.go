// Package founder provides founder-level intelligence: prioritization intuition,
// customer validation loops, and a pushback/argument engine that challenges
// assumptions instead of agreeing with everything.
//
// Speed without direction is chaos faster.
package founder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// --- Prioritization ---

// PriorityScorer scores items across multiple dimensions.
type PriorityScorer struct {
	Weights ScoreWeights `json:"weights"`
}

// ScoreWeights controls how dimensions are weighted.
type ScoreWeights struct {
	Impact     float64 `json:"impact"`     // how much value this creates
	Urgency    float64 `json:"urgency"`    // time sensitivity
	Effort     float64 `json:"effort"`     // inverse — lower effort = higher score
	Risk       float64 `json:"risk"`       // risk of NOT doing it
	Alignment  float64 `json:"alignment"`  // alignment with current goals
	Evidence   float64 `json:"evidence"`   // how much data supports this
}

// DefaultWeights returns balanced default weights.
func DefaultWeights() ScoreWeights {
	return ScoreWeights{
		Impact:    0.25,
		Urgency:   0.20,
		Effort:    0.15,
		Risk:      0.15,
		Alignment: 0.15,
		Evidence:  0.10,
	}
}

// Item represents something to be prioritized.
type Item struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Category    string  `json:"category,omitempty"` // feature, bug, tech-debt, research
	Impact      float64 `json:"impact"`    // 0-10
	Urgency     float64 `json:"urgency"`   // 0-10
	Effort      float64 `json:"effort"`    // 0-10 (higher = more effort)
	Risk        float64 `json:"risk"`      // 0-10 risk of NOT doing
	Alignment   float64 `json:"alignment"` // 0-10 alignment with goals
	Evidence    float64 `json:"evidence"`  // 0-10 data support
	Score       float64 `json:"score"`
	Rank        int     `json:"rank"`
	Rationale   string  `json:"rationale,omitempty"`
}

// ScoreItem calculates the weighted priority score for an item.
func (ps *PriorityScorer) ScoreItem(item *Item) *Item {
	w := ps.Weights
	item.Score = (item.Impact*w.Impact +
		item.Urgency*w.Urgency +
		(10-item.Effort)*w.Effort + // inverse
		item.Risk*w.Risk +
		item.Alignment*w.Alignment +
		item.Evidence*w.Evidence)
	return item
}

// RankItems scores and ranks a list of items.
func (ps *PriorityScorer) RankItems(items []*Item) []*Item {
	for _, item := range items {
		ps.ScoreItem(item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	for i, item := range items {
		item.Rank = i + 1
	}
	return items
}

// --- Customer Validation ---

// ValidationStatus tracks a hypothesis through validation.
type ValidationStatus string

const (
	ValidationDraft      ValidationStatus = "draft"
	ValidationTesting    ValidationStatus = "testing"
	ValidationValidated  ValidationStatus = "validated"
	ValidationInvalidated ValidationStatus = "invalidated"
	ValidationInconclusive ValidationStatus = "inconclusive"
)

// Hypothesis is a testable customer assumption.
type Hypothesis struct {
	ID           string            `json:"id"`
	Statement    string            `json:"statement"`    // "Customers will pay $X for Y"
	Type         string            `json:"type"`         // value, pain, solution, channel, price
	Status       ValidationStatus  `json:"status"`
	Evidence     []ValidationEvidence `json:"evidence,omitempty"`
	Confidence   float64           `json:"confidence"` // 0-100
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	MinConfidence float64          `json:"min_confidence"` // threshold to consider validated
}

// ValidationEvidence is a data point for/against a hypothesis.
type ValidationEvidence struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"` // interview, analytics, survey, experiment, observation
	Supports  bool      `json:"supports"`
	Details   string    `json:"details"`
	Weight    float64   `json:"weight"` // 0-1, how strong is this evidence
	CreatedAt time.Time `json:"created_at"`
}

// ValidationResult is the outcome of a validation cycle.
type ValidationResult struct {
	ID             string  `json:"id"`
	HypothesisID   string  `json:"hypothesis_id"`
	Status         ValidationStatus `json:"status"`
	Confidence     float64 `json:"confidence"`
	EvidenceCount  int     `json:"evidence_count"`
	Supporting     int     `json:"supporting"`
	Contradicting  int     `json:"contradicting"`
	Recommendation string  `json:"recommendation"`
}

// --- Pushback / Argument Engine ---

// ArgumentSide represents which side of an argument.
type ArgumentSide string

const (
	SidePro    ArgumentSide = "pro"
	SideCon    ArgumentSide = "con"
	SideDevil  ArgumentSide = "devil_advocate"
)

// Argument is a structured argument for/against something.
type Argument struct {
	ID        string       `json:"id"`
	Topic     string       `json:"topic"`
	Side      ArgumentSide `json:"side"`
	Point     string       `json:"point"`
	Evidence  string       `json:"evidence,omitempty"`
	Strength  float64      `json:"strength"` // 0-10
	CounterTo string      `json:"counter_to,omitempty"` // ID of argument this counters
	CreatedAt time.Time   `json:"created_at"`
}

// DebateResult is the outcome of a structured debate.
type DebateResult struct {
	Topic       string  `json:"topic"`
	ProScore    float64 `json:"pro_score"`
	ConScore    float64 `json:"con_score"`
	Winner      string  `json:"winner"`
	Conclusion  string  `json:"conclusion"`
	BlindSpots  []string `json:"blind_spots,omitempty"`
	Confidence  float64 `json:"confidence"`
}

// --- Engine ---

// Engine is the founder intelligence engine.
type Engine struct {
	mu          sync.RWMutex
	scorer      *PriorityScorer
	hypotheses  map[string]*Hypothesis
	arguments   map[string]*Argument
	backlog     []*Item
	path        string
}

// NewEngine creates a new founder engine.
func NewEngine(weights ScoreWeights, persistPath string) *Engine {
	e := &Engine{
		scorer:     &PriorityScorer{Weights: weights},
		hypotheses: make(map[string]*Hypothesis),
		arguments:  make(map[string]*Argument),
		backlog:    make([]*Item, 0),
		path:       persistPath,
	}
	e.load()
	return e
}

// --- Prioritization ---

// AddItem adds an item to the prioritization backlog.
func (e *Engine) AddItem(title, description, category string, impact, urgency, effort, risk, alignment, evidence float64) *Item {
	e.mu.Lock()
	defer e.mu.Unlock()

	item := &Item{
		ID:          genID("item"),
		Title:       title,
		Description: description,
		Category:    category,
		Impact:      impact,
		Urgency:     urgency,
		Effort:      effort,
		Risk:        risk,
		Alignment:   alignment,
		Evidence:    evidence,
	}
	e.scorer.ScoreItem(item)
	e.backlog = append(e.backlog, item)
	e.persist()
	return item
}

// Prioritize re-ranks the entire backlog.
func (e *Engine) Prioritize() []*Item {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.scorer.RankItems(e.backlog)
	e.persist()
	return e.backlog
}

// GetTop returns the top N prioritized items.
func (e *Engine) GetTop(n int) []*Item {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ranked := make([]*Item, len(e.backlog))
	copy(ranked, e.backlog)
	e.scorer.RankItems(ranked)

	if n > len(ranked) {
		n = len(ranked)
	}
	return ranked[:n]
}

// --- Customer Validation ---

// CreateHypothesis creates a testable hypothesis.
func (e *Engine) CreateHypothesis(statement, hType string, minConfidence float64) (*Hypothesis, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().UTC()
	h := &Hypothesis{
		ID:            genID("hyp"),
		Statement:     statement,
		Type:          hType,
		Status:        ValidationDraft,
		Confidence:    50, // start neutral
		MinConfidence: minConfidence,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	e.hypotheses[h.ID] = h
	e.persist()
	return h, nil
}

// AddEvidence adds evidence for/against a hypothesis.
func (e *Engine) AddEvidence(hypothesisID, source, details string, supports bool, weight float64) (*ValidationEvidence, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	h, ok := e.hypotheses[hypothesisID]
	if !ok {
		return nil, fmt.Errorf("hypothesis %s not found", hypothesisID)
	}

	ev := &ValidationEvidence{
		ID:        genID("ev"),
		Source:    source,
		Supports:  supports,
		Details:   details,
		Weight:    weight,
		CreatedAt: time.Now().UTC(),
	}

	h.Evidence = append(h.Evidence, *ev)
	h.UpdatedAt = time.Now().UTC()

	// Recalculate confidence
	e.recalcConfidence(h)
	e.persist()
	return ev, nil
}

// ValidateHypothesis assesses the current state of a hypothesis.
func (e *Engine) ValidateHypothesis(hypothesisID string) (*ValidationResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	h, ok := e.hypotheses[hypothesisID]
	if !ok {
		return nil, fmt.Errorf("hypothesis %s not found", hypothesisID)
	}

	supporting := 0
	contradicting := 0
	for _, ev := range h.Evidence {
		if ev.Supports {
			supporting++
		} else {
			contradicting++
		}
	}

	result := &ValidationResult{
		ID:            genID("val"),
		HypothesisID:  hypothesisID,
		Confidence:    h.Confidence,
		EvidenceCount: len(h.Evidence),
		Supporting:    supporting,
		Contradicting: contradicting,
	}

	if h.Confidence >= h.MinConfidence {
		h.Status = ValidationValidated
		result.Status = ValidationValidated
		result.Recommendation = "Hypothesis validated. Proceed with this assumption."
	} else if h.Confidence <= (100-h.MinConfidence) {
		h.Status = ValidationInvalidated
		result.Status = ValidationInvalidated
		result.Recommendation = "Hypothesis invalidated. Pivot or abandon this assumption."
	} else {
		h.Status = ValidationTesting
		result.Status = ValidationTesting
		result.Recommendation = "Insufficient evidence. Continue testing."
	}

	h.UpdatedAt = time.Now().UTC()
	e.persist()
	return result, nil
}

func (e *Engine) recalcConfidence(h *Hypothesis) {
	if len(h.Evidence) == 0 {
		h.Confidence = 50
		return
	}

	var totalWeight float64
	var supportWeight float64
	for _, ev := range h.Evidence {
		totalWeight += ev.Weight
		if ev.Supports {
			supportWeight += ev.Weight
		}
	}
	if totalWeight == 0 {
		return
	}
	h.Confidence = (supportWeight / totalWeight) * 100
}

// --- Pushback Engine ---

// Argue generates devil's advocate arguments against a proposal.
func (e *Engine) Argue(topic, proposal string) []Argument {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Generate structured counterarguments
	counterPoints := []struct {
		point    string
		evidence string
		strength float64
	}{
		{
			point:    fmt.Sprintf("'%s' assumes demand exists without validation", proposal),
			evidence: "No customer interviews or market data cited",
			strength: 7,
		},
		{
			point:    fmt.Sprintf("'%s' may not be the highest priority right now", proposal),
			evidence: "Check if this aligns with current top-ranked goals",
			strength: 6,
		},
		{
			point:    "Opportunity cost: time spent here is time NOT spent elsewhere",
			evidence: "Consider what gets delayed or dropped",
			strength: 8,
		},
		{
			point:    "Sunk cost risk: starting this creates commitment bias",
			evidence: "Set explicit kill criteria before starting",
			strength: 5,
		},
		{
			point:    "Second-order effects may be negative",
			evidence: "What breaks or gets worse if we do this?",
			strength: 6,
		},
	}

	now := time.Now().UTC()
	var args []Argument
	for _, cp := range counterPoints {
		arg := Argument{
			ID:        genID("arg"),
			Topic:     topic,
			Side:      SideDevil,
			Point:     cp.point,
			Evidence:  cp.evidence,
			Strength:  cp.strength,
			CreatedAt: now,
		}
		e.arguments[arg.ID] = &arg
		args = append(args, arg)
	}
	e.persist()
	return args
}

// Debate creates a structured pro/con debate on a topic.
func (e *Engine) Debate(topic string, pros, cons []struct {
	Point, Evidence string
	Strength        float64
}) *DebateResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	var proScore, conScore float64
	now := time.Now().UTC()

	for _, p := range pros {
		arg := Argument{
			ID:        genID("arg"),
			Topic:     topic,
			Side:      SidePro,
			Point:     p.Point,
			Evidence:  p.Evidence,
			Strength:  p.Strength,
			CreatedAt: now,
		}
		e.arguments[arg.ID] = &arg
		proScore += p.Strength
	}
	for _, c := range cons {
		arg := Argument{
			ID:        genID("arg"),
			Topic:     topic,
			Side:      SideCon,
			Point:     c.Point,
			Evidence:  c.Evidence,
			Strength:  c.Strength,
			CreatedAt: now,
		}
		e.arguments[arg.ID] = &arg
		conScore += c.Strength
	}

	result := &DebateResult{
		Topic:    topic,
		ProScore: proScore,
		ConScore: conScore,
	}

	if proScore > conScore {
		result.Winner = "pro"
		result.Confidence = proScore / (proScore + conScore) * 100
	} else if conScore > proScore {
		result.Winner = "con"
		result.Confidence = conScore / (proScore + conScore) * 100
	} else {
		result.Winner = "tie"
		result.Confidence = 50
	}

	// Identify blind spots
	if len(pros) == 0 {
		result.BlindSpots = append(result.BlindSpots, "No pro arguments — is this worth doing at all?")
	}
	if len(cons) == 0 {
		result.BlindSpots = append(result.BlindSpots, "No con arguments — confirmation bias? Someone should argue against this.")
	}

	total := proScore + conScore
	if total == 0 {
		result.Conclusion = "No arguments on either side. Not enough thought given."
	} else {
		result.Conclusion = fmt.Sprintf("Pro: %.1f vs Con: %.1f. %s wins with %.0f%% confidence.",
			proScore, conScore, strings.Title(result.Winner), result.Confidence)
	}

	e.persist()
	return result
}

// GetArguments returns arguments for a topic.
func (e *Engine) GetArguments(topic string) []Argument {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []Argument
	for _, a := range e.arguments {
		if a.Topic == topic {
			result = append(result, *a)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Strength > result[j].Strength
	})
	return result
}

// --- Persistence ---

type founderData struct {
	Items       []*Item              `json:"items"`
	Hypotheses  map[string]*Hypothesis `json:"hypotheses"`
	Arguments   map[string]*Argument `json:"arguments"`
	Weights     ScoreWeights         `json:"weights"`
}

func (e *Engine) persist() {
	if e.path == "" {
		return
	}
	data := founderData{
		Items:      e.backlog,
		Hypotheses: e.hypotheses,
		Arguments:  e.arguments,
		Weights:    e.scorer.Weights,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(e.path), 0755)
	os.WriteFile(e.path, raw, 0644)
}

func (e *Engine) load() {
	if e.path == "" {
		return
	}
	raw, err := os.ReadFile(e.path)
	if err != nil {
		return
	}
	var data founderData
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	if data.Items != nil {
		e.backlog = data.Items
	}
	if data.Hypotheses != nil {
		e.hypotheses = data.Hypotheses
	}
	if data.Arguments != nil {
		e.arguments = data.Arguments
	}
	if data.Weights != (ScoreWeights{}) {
		e.scorer.Weights = data.Weights
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
