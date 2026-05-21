// Package debate provides multi-agent debate for decision making.
// Multiple agents argue different positions, a judge evaluates,
// and the best argument wins.
//
// Truth emerges from disagreement.
package debate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Position represents a stance in a debate.
type Position string

const (
	PositionFor      Position = "for"
	PositionAgainst  Position = "against"
	PositionNeutral  Position = "neutral"
	PositionExpert   Position = "expert"
)

// Argument represents a single argument in the debate.
type Argument struct {
	ID         string   `json:"id"`
	DebaterID  string   `json:"debater_id"`
	Position   Position `json:"position"`
	Claim      string   `json:"claim"`
	Evidence   string   `json:"evidence,omitempty"`
	Reasoning  string   `json:"reasoning,omitempty"`
	RebuttalTo string   `json:"rebuttal_to,omitempty"` // ID of argument being rebutted
	Round      int      `json:"round"`
	Timestamp  time.Time `json:"timestamp"`
	Score      float64  `json:"score,omitempty"`
	Model      string   `json:"model,omitempty"`
}

// Debater represents a participant in the debate.
type Debater struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Agent    string   `json:"agent"`
	Model    string   `json:"model,omitempty"`
	Position Position `json:"position"`
	Expertise string  `json:"expertise,omitempty"`
}

// Verdict represents the judge's final decision.
type Verdict struct {
	Winner       string  `json:"winner"`        // debater ID
	Reasoning    string  `json:"reasoning"`
	Confidence   float64 `json:"confidence"`     // 0-1
	KeyPoints    []string `json:"key_points"`
	Consensus    bool    `json:"consensus"`      // did debaters agree?
	DissentingOpinion string `json:"dissenting_opinion,omitempty"`
}

// Debate represents a complete debate session.
type Debate struct {
	ID          string     `json:"id"`
	Topic       string     `json:"topic"`
	Description string     `json:"description,omitempty"`
	Debaters    []Debater  `json:"debaters"`
	Arguments   []Argument `json:"arguments"`
	Verdict     *Verdict   `json:"verdict,omitempty"`
	Rounds      int        `json:"rounds"`
	MaxRounds   int        `json:"max_rounds"`
	Status      string     `json:"status"` // open, in_progress, concluded
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Tags        []string   `json:"tags,omitempty"`
}

// Judge evaluates arguments and produces a verdict.
type Judge struct {
	ID       string `json:"id"`
	Agent    string `json:"agent"`
	Model    string `json:"model,omitempty"`
	Criteria []string `json:"criteria,omitempty"` // evaluation criteria
}

// Store manages debate records.
type Store struct {
	Dir string
}

// NewStore creates a debate store.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// Create starts a new debate.
func (s *Store) Create(topic, description string, debaters []Debater, maxRounds int) (*Debate, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create debate dir: %w", err)
	}

	if maxRounds <= 0 {
		maxRounds = 3
	}

	debate := &Debate{
		ID:          fmt.Sprintf("debate-%d", time.Now().UnixNano()),
		Topic:       topic,
		Description: description,
		Debaters:    debaters,
		MaxRounds:   maxRounds,
		Status:      "open",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.writeDebate(debate); err != nil {
		return nil, err
	}

	return debate, nil
}

// AddArgument adds an argument to the debate.
func (s *Store) AddArgument(debateID string, arg Argument) (*Debate, error) {
	debate, err := s.Get(debateID)
	if err != nil {
		return nil, err
	}

	if arg.ID == "" {
		arg.ID = fmt.Sprintf("arg-%d", time.Now().UnixNano())
	}
	if arg.Timestamp.IsZero() {
		arg.Timestamp = time.Now()
	}
	arg.Round = debate.Rounds + 1

	debate.Arguments = append(debate.Arguments, arg)
	debate.UpdatedAt = time.Now()

	if arg.Round > debate.Rounds {
		debate.Rounds = arg.Round
	}

	if debate.Rounds >= debate.MaxRounds {
		debate.Status = "concluded"
	}

	if err := s.writeDebate(debate); err != nil {
		return nil, err
	}

	return debate, nil
}

// Conclude ends a debate with a verdict.
func (s *Store) Conclude(debateID string, verdict Verdict) (*Debate, error) {
	debate, err := s.Get(debateID)
	if err != nil {
		return nil, err
	}

	debate.Verdict = &verdict
	debate.Status = "concluded"
	debate.UpdatedAt = time.Now()

	if err := s.writeDebate(debate); err != nil {
		return nil, err
	}

	return debate, nil
}

// Get retrieves a debate by ID or topic.
func (s *Store) Get(idOrTopic string) (*Debate, error) {
	debates, err := s.List()
	if err != nil {
		return nil, err
	}

	for _, d := range debates {
		if d.ID == idOrTopic || strings.EqualFold(d.Topic, idOrTopic) {
			return d, nil
		}
	}
	return nil, fmt.Errorf("debate %q not found", idOrTopic)
}

// List returns all debates sorted by creation time (newest first).
func (s *Store) List() ([]*Debate, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var debates []*Debate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var d Debate
		if err := json.Unmarshal(data, &d); err != nil {
			continue
		}
		debates = append(debates, &d)
	}

	sort.Slice(debates, func(i, k int) bool {
		return debates[i].CreatedAt.After(debates[k].CreatedAt)
	})

	return debates, nil
}

// Delete removes a debate.
func (s *Store) Delete(idOrTopic string) error {
	debate, err := s.Get(idOrTopic)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.Dir, debate.ID+".json"))
}

// EvaluateArguments scores all arguments in a debate.
// Simple heuristic scoring based on evidence, reasoning, and rebuttals.
func EvaluateArguments(debate *Debate) {
	for i := range debate.Arguments {
		arg := &debate.Arguments[i]
		score := 50.0 // base score

		// Bonus for evidence
		if arg.Evidence != "" {
			score += 15
		}

		// Bonus for reasoning
		if arg.Reasoning != "" {
			score += 10
		}

		// Bonus for rebuttal (shows engagement)
		if arg.RebuttalTo != "" {
			score += 10
		}

		// Later rounds get a small bonus (more context)
		score += float64(arg.Round) * 2

		// Cap at 100
		if score > 100 {
			score = 100
		}

		arg.Score = score
	}
}

// FormatDebate renders a debate as a readable string.
func FormatDebate(debate *Debate) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Debate: %s\n", debate.Topic))
	if debate.Description != "" {
		sb.WriteString(fmt.Sprintf("  %s\n", debate.Description))
	}
	sb.WriteString(fmt.Sprintf("  Status: %s | Rounds: %d/%d | Debaters: %d\n\n",
		debate.Status, debate.Rounds, debate.MaxRounds, len(debate.Debaters)))

	// Group arguments by round
	byRound := make(map[int][]Argument)
	for _, arg := range debate.Arguments {
		byRound[arg.Round] = append(byRound[arg.Round], arg)
	}

	for round := 1; round <= debate.Rounds; round++ {
		args := byRound[round]
		if len(args) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("Round %d:\n", round))
		for _, arg := range args {
			positionIcon := "⚖"
			switch arg.Position {
			case PositionFor:
				positionIcon = "👍"
			case PositionAgainst:
				positionIcon = "👎"
			case PositionExpert:
				positionIcon = "🎓"
			}
			sb.WriteString(fmt.Sprintf("  %s [%s] %s\n", positionIcon, arg.DebaterID, arg.Claim))
			if arg.Evidence != "" {
				sb.WriteString(fmt.Sprintf("    Evidence: %s\n", arg.Evidence))
			}
			if arg.RebuttalTo != "" {
				sb.WriteString(fmt.Sprintf("    Rebuttal to: %s\n", arg.RebuttalTo))
			}
		}
		sb.WriteString("\n")
	}

	if debate.Verdict != nil {
		sb.WriteString(fmt.Sprintf("Verdict: %s (confidence: %.0f%%)\n",
			debate.Verdict.Winner, debate.Verdict.Confidence*100))
		sb.WriteString(fmt.Sprintf("  %s\n", debate.Verdict.Reasoning))
		if debate.Verdict.Consensus {
			sb.WriteString("  ✅ Consensus reached\n")
		} else {
			sb.WriteString("  ⚖️ No consensus — dissenting opinion exists\n")
		}
	}

	return sb.String()
}

func (s *Store) writeDebate(debate *Debate) error {
	data, err := json.MarshalIndent(debate, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, debate.ID+".json"), data, 0o644)
}
