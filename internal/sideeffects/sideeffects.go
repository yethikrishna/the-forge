// Package sideeffects analyzes second-order effects before optimization.
// Optimizing one metric often degrades another — cutting costs may reduce
// quality, speeding delivery may burn out teams. This package traces effect
// chains and assesses risk before actions are committed, closing the gap of
// unintended consequences from narrow optimization.
package sideeffects

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ProposedAction is an action being considered for optimization.
type ProposedAction struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	TargetMetric string           `json:"target_metric"`
	ExpectedDelta float64         `json:"expected_delta"` // positive = improvement
	Category    string            `json:"category"` // "cost", "speed", "quality", "scale"
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// SideEffect represents a potential secondary consequence.
type SideEffect struct {
	ID           string    `json:"id"`
	ActionID     string    `json:"action_id"`
	AffectedArea string    `json:"affected_area"`
	Description  string    `json:"description"`
	Direction    string    `json:"direction"` // "positive", "negative", "neutral"
	Magnitude    float64   `json:"magnitude"` // 0.0–1.0
	Probability  float64   `json:"probability"` // 0.0–1.0
	Order        int       `json:"order"` // 1=primary side effect, 2=second order, etc.
	IdentifiedAt time.Time `json:"identified_at"`
}

// EffectChain traces a cascade of effects from one action.
type EffectChain struct {
	ID          string       `json:"id"`
	ActionID    string       `json:"action_id"`
	Effects     []SideEffect `json:"effects"`
	TotalRisk   float64      `json:"total_risk"`
	NetImpact   float64      `json:"net_impact"`
	BuiltAt     time.Time    `json:"built_at"`
}

// RiskAssessment summarizes the risk profile of an action.
type RiskAssessment struct {
	ID               string    `json:"id"`
	ActionID         string    `json:"action_id"`
	RiskLevel        string    `json:"risk_level"` // "low", "medium", "high", "critical"
	NegativeEffects  int       `json:"negative_effects"`
	PositiveEffects  int       `json:"positive_effects"`
	MaxCascadeDepth  int       `json:"max_cascade_depth"`
	WorstCaseImpact  float64   `json:"worst_case_impact"`
	Recommendation   string    `json:"recommendation"`
	AssessedAt       time.Time `json:"assessed_at"`
}

// Store provides thread-safe JSON file persistence.
type Store struct {
	mu       sync.Mutex
	filePath string
	data     storeData
}

type storeData struct {
	Actions  map[string]ProposedAction  `json:"actions"`
	Effects  map[string]SideEffect      `json:"effects"`
	Chains   map[string]EffectChain     `json:"chains"`
	Risks    map[string]RiskAssessment  `json:"risks"`
}

// NewStore creates a Store backed by filePath.
func NewStore(filePath string) *Store {
	return &Store{
		filePath: filePath,
		data: storeData{
			Actions: make(map[string]ProposedAction),
			Effects: make(map[string]SideEffect),
			Chains:  make(map[string]EffectChain),
			Risks:   make(map[string]RiskAssessment),
		},
	}
}

// Load reads persisted data from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(raw, &s.data)
}

// Save writes current data to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, raw, 0644)
}

// AnalyzeAction registers an action and identifies primary side effects.
func (s *Store) AnalyzeAction(action ProposedAction) ([]SideEffect, ProposedAction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if action.ID == "" {
		action.ID = fmt.Sprintf("action-%d", time.Now().UTC().UnixNano())
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	s.data.Actions[action.ID] = action

	// Heuristic side effect identification based on category
	effects := identifyPrimaryEffects(action)
	for i := range effects {
		effects[i].ID = fmt.Sprintf("fx-%d-%d", time.Now().UTC().UnixNano(), i)
		effects[i].ActionID = action.ID
		effects[i].IdentifiedAt = time.Now().UTC()
		s.data.Effects[effects[i].ID] = effects[i]
	}

	return effects, action
}

// PredictSecondOrder generates second-order effects from primary ones.
func (s *Store) PredictSecondOrder(actionID string) []SideEffect {
	s.mu.Lock()
	defer s.mu.Unlock()

	var primaries []SideEffect
	for _, fx := range s.data.Effects {
		if fx.ActionID == actionID && fx.Order == 1 {
			primaries = append(primaries, fx)
		}
	}

	var secondOrder []SideEffect
	for _, primary := range primaries {
		derived := deriveSecondOrder(primary)
		for i := range derived {
			derived[i].ID = fmt.Sprintf("fx2-%d-%d", time.Now().UTC().UnixNano(), i)
			derived[i].ActionID = actionID
			derived[i].Order = 2
			derived[i].IdentifiedAt = time.Now().UTC()
			s.data.Effects[derived[i].ID] = derived[i]
			secondOrder = append(secondOrder, derived[i])
		}
	}

	return secondOrder
}

// BuildEffectChain constructs a full effect chain for an action.
func (s *Store) BuildEffectChain(actionID string) EffectChain {
	s.mu.Lock()
	defer s.mu.Unlock()

	var effects []SideEffect
	for _, fx := range s.data.Effects {
		if fx.ActionID == actionID {
			effects = append(effects, fx)
		}
	}

	totalRisk := 0.0
	netImpact := 0.0
	for _, fx := range effects {
		if fx.Direction == "negative" {
			totalRisk += fx.Magnitude * fx.Probability
			netImpact -= fx.Magnitude * fx.Probability
		} else if fx.Direction == "positive" {
			netImpact += fx.Magnitude * fx.Probability
		}
	}

	chain := EffectChain{
		ID:        fmt.Sprintf("chain-%d", time.Now().UTC().UnixNano()),
		ActionID:  actionID,
		Effects:   effects,
		TotalRisk: totalRisk,
		NetImpact: netImpact,
		BuiltAt:   time.Now().UTC(),
	}
	s.data.Chains[chain.ID] = chain
	return chain
}

// AssessRisk produces a risk assessment for an action based on its effect chain.
func (s *Store) AssessRisk(actionID string) RiskAssessment {
	s.mu.Lock()
	defer s.mu.Unlock()

	negCount := 0
	posCount := 0
	worstCase := 0.0
	maxDepth := 0

	for _, fx := range s.data.Effects {
		if fx.ActionID == actionID {
			if fx.Direction == "negative" {
				negCount++
				impact := fx.Magnitude * fx.Probability
				if impact > worstCase {
					worstCase = impact
				}
			} else if fx.Direction == "positive" {
				posCount++
			}
			if fx.Order > maxDepth {
				maxDepth = fx.Order
			}
		}
	}

	riskLevel := "low"
	switch {
	case worstCase > 0.7:
		riskLevel = "critical"
	case worstCase > 0.4:
		riskLevel = "high"
	case worstCase > 0.2:
		riskLevel = "medium"
	}

	recommendation := "proceed"
	if riskLevel == "critical" {
		recommendation = "do not proceed without mitigation"
	} else if riskLevel == "high" {
		recommendation = "proceed with caution and monitoring"
	} else if riskLevel == "medium" {
		recommendation = "proceed with awareness"
	}

	ra := RiskAssessment{
		ID:              fmt.Sprintf("risk-%d", time.Now().UTC().UnixNano()),
		ActionID:        actionID,
		RiskLevel:       riskLevel,
		NegativeEffects: negCount,
		PositiveEffects: posCount,
		MaxCascadeDepth: maxDepth,
		WorstCaseImpact: worstCase,
		Recommendation:  recommendation,
		AssessedAt:      time.Now().UTC(),
	}
	s.data.Risks[ra.ID] = ra
	return ra
}

// GenerateSideEffectsReport produces a summary of all tracked side effects.
func (s *Store) GenerateSideEffectsReport() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	neg := 0
	pos := 0
	for _, fx := range s.data.Effects {
		if fx.Direction == "negative" {
			neg++
		} else if fx.Direction == "positive" {
			pos++
		}
	}

	riskBreakdown := map[string]int{}
	for _, ra := range s.data.Risks {
		riskBreakdown[ra.RiskLevel]++
	}

	return map[string]interface{}{
		"action_count":     len(s.data.Actions),
		"total_effects":    len(s.data.Effects),
		"negative_effects": neg,
		"positive_effects": pos,
		"chain_count":      len(s.data.Chains),
		"risk_breakdown":   riskBreakdown,
	}
}

// GetAction retrieves a proposed action by ID.
func (s *Store) GetAction(id string) (ProposedAction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.data.Actions[id]
	return a, ok
}

func identifyPrimaryEffects(action ProposedAction) []SideEffect {
	var effects []SideEffect

	switch action.Category {
	case "cost":
		effects = append(effects,
			SideEffect{AffectedArea: "quality", Description: "cost cutting may reduce quality", Direction: "negative", Magnitude: 0.6, Probability: 0.7, Order: 1},
			SideEffect{AffectedArea: "morale", Description: "budget pressure affects team morale", Direction: "negative", Magnitude: 0.4, Probability: 0.5, Order: 1},
			SideEffect{AffectedArea: "efficiency", Description: "leaner operations may improve focus", Direction: "positive", Magnitude: 0.3, Probability: 0.4, Order: 1},
		)
	case "speed":
		effects = append(effects,
			SideEffect{AffectedArea: "quality", Description: "rushing may introduce defects", Direction: "negative", Magnitude: 0.5, Probability: 0.6, Order: 1},
			SideEffect{AffectedArea: "burnout", Description: "faster pace increases burnout risk", Direction: "negative", Magnitude: 0.7, Probability: 0.5, Order: 1},
			SideEffect{AffectedArea: "responsiveness", Description: "faster delivery improves customer satisfaction", Direction: "positive", Magnitude: 0.5, Probability: 0.7, Order: 1},
		)
	case "quality":
		effects = append(effects,
			SideEffect{AffectedArea: "cost", Description: "higher quality increases cost", Direction: "negative", Magnitude: 0.5, Probability: 0.6, Order: 1},
			SideEffect{AffectedArea: "speed", Description: "quality focus slows delivery", Direction: "negative", Magnitude: 0.4, Probability: 0.5, Order: 1},
			SideEffect{AffectedArea: "loyalty", Description: "quality builds customer loyalty", Direction: "positive", Magnitude: 0.7, Probability: 0.8, Order: 1},
		)
	case "scale":
		effects = append(effects,
			SideEffect{AffectedArea: "complexity", Description: "scaling adds system complexity", Direction: "negative", Magnitude: 0.6, Probability: 0.8, Order: 1},
			SideEffect{AffectedArea: "cost", Description: "scaling increases operational cost", Direction: "negative", Magnitude: 0.5, Probability: 0.7, Order: 1},
			SideEffect{AffectedArea: "reach", Description: "scaling expands market reach", Direction: "positive", Magnitude: 0.7, Probability: 0.6, Order: 1},
		)
	default:
		effects = append(effects,
			SideEffect{AffectedArea: "unknown", Description: "unpredictable side effect", Direction: "negative", Magnitude: 0.3, Probability: 0.3, Order: 1},
		)
	}
	return effects
}

func deriveSecondOrder(primary SideEffect) []SideEffect {
	var derived []SideEffect
	if primary.Direction == "negative" {
		derived = append(derived, SideEffect{
			AffectedArea: "trust",
			Description:  fmt.Sprintf("%s in %s erodes stakeholder trust", primary.Direction, primary.AffectedArea),
			Direction:    "negative",
			Magnitude:    primary.Magnitude * 0.5,
			Probability:  primary.Probability * 0.6,
		})
	}
	if primary.Direction == "positive" {
		derived = append(derived, SideEffect{
			AffectedArea: "momentum",
			Description:  fmt.Sprintf("positive %s creates momentum for further gains", primary.AffectedArea),
			Direction:    "positive",
			Magnitude:    primary.Magnitude * 0.4,
			Probability:  primary.Probability * 0.5,
		})
	}
	return derived
}
