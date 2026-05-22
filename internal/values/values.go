// Package values provides the values system for Forge organizations.
// It manages mission alignment, ethical frameworks, organizational personality,
// and values audits — ensuring every action the org takes is consistent with
// what it stands for.
//
// Closes gap: organizations without explicit values drift toward convenience.
package values

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AlignmentLevel represents how well something aligns with org values.
type AlignmentLevel string

const (
	AlignmentFull     AlignmentLevel = "full"
	AlignmentPartial  AlignmentLevel = "partial"
	AlignmentMinimal  AlignmentLevel = "minimal"
	AlignmentConflict AlignmentLevel = "conflict"
	AlignmentNone     AlignmentLevel = "none"
)

// Value represents a single organizational value.
type Value struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Weight      float64   `json:"weight"`       // 0.0–1.0 importance
	Category    string    `json:"category"`     // e.g. "ethical", "operational", "cultural"
	NonNegotiable bool    `json:"non_negotiable"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MissionStatement represents the organization's mission.
type MissionStatement struct {
	ID          string    `json:"id"`
	Statement   string    `json:"statement"`
	Version     int       `json:"version"`
	ApprovedBy  string    `json:"approved_by"`
	ApprovedAt  time.Time `json:"approved_at"`
	Active      bool      `json:"active"`
}

// EthicalFramework defines the ethical boundaries for the organization.
type EthicalFramework struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Principles  []EthicalRule   `json:"principles"`
	RedLines    []string        `json:"red_lines"`
	Jurisdiction string         `json:"jurisdiction,omitempty"`
	Version     int             `json:"version"`
	CreatedAt   time.Time       `json:"created_at"`
}

// EthicalRule is a single principle within an ethical framework.
type EthicalRule struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // advisory, mandatory, absolute
	Category    string `json:"category"` // fairness, privacy, transparency, safety, autonomy
}

// OrgPersonality captures the behavioral personality of the organization.
type OrgPersonality struct {
	ID             string  `json:"id"`
	Tone           string  `json:"tone"`             // formal, casual, technical, empathetic
	RiskAppetite   float64 `json:"risk_appetite"`    // 0.0 (averse) – 1.0 (aggressive)
	InnovationBias float64 `json:"innovation_bias"`  // 0.0 (conservative) – 1.0 (disruptive)
	Collaboration  float64 `json:"collaboration"`    // 0.0 (independent) – 1.0 (collective)
	Transparency   float64 `json:"transparency"`     // 0.0 (closed) – 1.0 (open)
	SpeedVsQuality float64 `json:"speed_vs_quality"` // 0.0 (quality) – 1.0 (speed)
}

// ValuesCheck is the result of checking alignment of an action against values.
type ValuesCheck struct {
	ID              string         `json:"id"`
	Action          string         `json:"action"`
	Target          string         `json:"target"`
	Alignment       AlignmentLevel `json:"alignment"`
	Score           float64        `json:"score"` // 0.0–1.0
	Conflicts       []ValueConflict `json:"conflicts,omitempty"`
	EthicalViolations []string     `json:"ethical_violations,omitempty"`
	RedLineHit      bool           `json:"red_line_hit"`
	CheckedAt       time.Time      `json:"checked_at"`
}

// ValueConflict records a specific conflict between an action and a value.
type ValueConflict struct {
	ValueID   string  `json:"value_id"`
	ValueName string  `json:"value_name"`
	Reason    string  `json:"reason"`
	Severity  float64 `json:"severity"` // 0.0–1.0
}

// Store manages values persistence.
type Store struct {
	values     []Value
	missions   []MissionStatement
	frameworks []EthicalFramework
	personality *OrgPersonality
	checks     []ValuesCheck
	filePath   string
	mu         sync.RWMutex
	nextID     int
}

// NewStore creates a new values store backed by a JSON file.
func NewStore(filePath string) *Store {
	return &Store{
		values:   make([]Value, 0),
		missions: make([]MissionStatement, 0),
		frameworks: make([]EthicalFramework, 0),
		checks:   make([]ValuesCheck, 0),
		filePath: filePath,
	}
}

// Load reads the store from disk.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read values file: %w", err)
	}

	var raw struct {
		Values     []Value          `json:"values"`
		Missions   []MissionStatement `json:"missions"`
		Frameworks []EthicalFramework `json:"frameworks"`
		Personality *OrgPersonality  `json:"personality"`
		Checks     []ValuesCheck    `json:"checks"`
		NextID     int              `json:"next_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse values file: %w", err)
	}

	s.values = raw.Values
	s.missions = raw.Missions
	s.frameworks = raw.Frameworks
	s.personality = raw.Personality
	s.checks = raw.Checks
	s.nextID = raw.NextID
	return nil
}

// Save writes the store to disk.
// Save writes the store to disk.
// Assumes the caller already holds s.mu.
func (s *Store) Save() error {

	raw := struct {
		Values     []Value          `json:"values"`
		Missions   []MissionStatement `json:"missions"`
		Frameworks []EthicalFramework `json:"frameworks"`
		Personality *OrgPersonality  `json:"personality"`
		Checks     []ValuesCheck    `json:"checks"`
		NextID     int              `json:"next_id"`
	}{
		Values:     s.values,
		Missions:   s.missions,
		Frameworks: s.frameworks,
		Personality: s.personality,
		Checks:     s.checks,
		NextID:     s.nextID,
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal values: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create values dir: %w", err)
	}

	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write values file: %w", err)
	}
	return os.Rename(tmp, s.filePath)
}

func (s *Store) genID(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s-%04d", prefix, s.nextID)
}

// DefineValues adds one or more values to the organization.
func (s *Store) DefineValues(vs ...Value) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for i := range vs {
		if vs[i].ID == "" {
			vs[i].ID = s.genID("val")
		}
		if vs[i].CreatedAt.IsZero() {
			vs[i].CreatedAt = now
		}
		vs[i].UpdatedAt = now
		s.values = append(s.values, vs[i])
	}
	return s.Save()
}

// ListValues returns all defined values.
func (s *Store) ListValues() []Value {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Value, len(s.values))
	copy(out, s.values)
	return out
}

// GetActiveMission returns the current active mission statement.
func (s *Store) GetActiveMission() *MissionStatement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.missions) - 1; i >= 0; i-- {
		if s.missions[i].Active {
			return &s.missions[i]
		}
	}
	return nil
}

// CheckMissionAlignment evaluates how well an action aligns with the mission.
func (s *Store) CheckMissionAlignment(action, target string) (*ValuesCheck, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Inline mission lookup to avoid RLock deadlock (we already hold Lock).
	var mission *MissionStatement
	for i := len(s.missions) - 1; i >= 0; i-- {
		if s.missions[i].Active {
			mission = &s.missions[i]
			break
		}
	}
	score := 1.0
	alignment := AlignmentFull
	var conflicts []ValueConflict
	var violations []string
	redLineHit := false

	// Check against each value
	for _, v := range s.values {
		conflict := assessValueConflict(action, target, v)
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
			score -= conflict.Severity * v.Weight
			if v.NonNegotiable && conflict.Severity > 0.5 {
				redLineHit = true
			}
		}
	}

	// Check ethical framework
	for _, fw := range s.frameworks {
		for _, rule := range fw.Principles {
			if violatesEthicalRule(action, rule) {
				violations = append(violations, rule.Description)
				if rule.Severity == "absolute" {
					redLineHit = true
					score -= 0.3
				} else if rule.Severity == "mandatory" {
					score -= 0.15
				}
			}
		}
		// Check red lines
		for _, rl := range fw.RedLines {
			if hitsRedLine(action, rl) {
				redLineHit = true
				violations = append(violations, "red line: "+rl)
				score -= 0.4
			}
		}
	}

	if mission != nil {
		score = adjustForMission(score, action, *mission)
	}

	// Clamp
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	alignment = scoreToAlignment(score)

	check := ValuesCheck{
		ID:              s.genID("vck"),
		Action:          action,
		Target:          target,
		Alignment:       alignment,
		Score:           score,
		Conflicts:       conflicts,
		EthicalViolations: violations,
		RedLineHit:      redLineHit,
		CheckedAt:       time.Now(),
	}

	s.checks = append(s.checks, check)
	return &check, s.Save()
}

// ApplyEthics evaluates an action against ethical frameworks and returns violations.
func (s *Store) ApplyEthics(action string) ([]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var violations []string
	blocked := false

	for _, fw := range s.frameworks {
		for _, rule := range fw.Principles {
			if violatesEthicalRule(action, rule) {
				violations = append(violations, rule.Description)
				if rule.Severity == "absolute" || rule.Severity == "mandatory" {
					blocked = true
				}
			}
		}
		for _, rl := range fw.RedLines {
			if hitsRedLine(action, rl) {
				violations = append(violations, "red line: "+rl)
				blocked = true
			}
		}
	}
	return violations, blocked
}

// AssessPersonality returns the current org personality profile.
func (s *Store) AssessPersonality() *OrgPersonality {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.personality == nil {
		return &OrgPersonality{
			ID:           "personality-default",
			Tone:         "balanced",
			RiskAppetite: 0.5,
			InnovationBias: 0.5,
			Collaboration: 0.5,
			Transparency: 0.5,
			SpeedVsQuality: 0.5,
		}
	}
	cp := *s.personality
	return &cp
}

// SetPersonality updates the org personality.
func (s *Store) SetPersonality(p OrgPersonality) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = "personality-default"
	}
	s.personality = &p
	return s.Save()
}

// RunValuesAudit performs a comprehensive audit of all values and returns a report.
func (s *Store) RunValuesAudit() (*ValuesAuditReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report := &ValuesAuditReport{
		Timestamp:       time.Now(),
		TotalValues:     len(s.values),
		NonNegotiables:  0,
		Frameworks:      len(s.frameworks),
		RecentChecks:    len(s.checks),
		ConflictRate:    0,
		RedLineRate:     0,
		ValuesByCategory: make(map[string]int),
	}

	for _, v := range s.values {
		if v.NonNegotiable {
			report.NonNegotiables++
		}
		report.ValuesByCategory[v.Category]++
	}

	conflictCount := 0
	redLineCount := 0
	for _, c := range s.checks {
		if c.Alignment == AlignmentConflict || c.Alignment == AlignmentNone {
			conflictCount++
		}
		if c.RedLineHit {
			redLineCount++
		}
	}
	if len(s.checks) > 0 {
		report.ConflictRate = float64(conflictCount) / float64(len(s.checks))
		report.RedLineRate = float64(redLineCount) / float64(len(s.checks))
	}

	return report, nil
}

// GenerateValuesReport produces a human-readable values report.
func (s *Store) GenerateValuesReport() string {
	report, _ := s.RunValuesAudit()
	mission := s.GetActiveMission()
	personality := s.AssessPersonality()

	out := "=== Values Report ===\n\n"
	if mission != nil {
		out += fmt.Sprintf("Mission (v%d): %s\n\n", mission.Version, mission.Statement)
	}
	out += fmt.Sprintf("Values: %d (%d non-negotiable)\n", report.TotalValues, report.NonNegotiables)
	out += fmt.Sprintf("Frameworks: %d\n", report.Frameworks)
	out += fmt.Sprintf("Checks run: %d (conflict rate: %.1f%%, red-line rate: %.1f%%)\n\n",
		report.RecentChecks, report.ConflictRate*100, report.RedLineRate*100)

	if personality != nil {
		out += fmt.Sprintf("Personality: tone=%s risk=%.2f innovation=%.2f collab=%.2f transparency=%.2f\n",
			personality.Tone, personality.RiskAppetite, personality.InnovationBias,
			personality.Collaboration, personality.Transparency)
	}

	if len(report.ValuesByCategory) > 0 {
		out += "\nBy category:\n"
		for cat, n := range report.ValuesByCategory {
			out += fmt.Sprintf("  %s: %d\n", cat, n)
		}
	}
	return out
}

// AddMission adds a new mission statement and deactivates previous ones.
func (s *Store) AddMission(m MissionStatement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if m.ID == "" {
		m.ID = s.genID("mis")
	}
	m.ApprovedAt = time.Now()
	m.Active = true

	for i := range s.missions {
		s.missions[i].Active = false
	}
	s.missions = append(s.missions, m)
	return s.Save()
}

// AddFramework adds an ethical framework.
func (s *Store) AddFramework(fw EthicalFramework) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fw.ID == "" {
		fw.ID = s.genID("eth")
	}
	fw.CreatedAt = time.Now()
	s.frameworks = append(s.frameworks, fw)
	return s.Save()
}

// ListFrameworks returns all ethical frameworks.
func (s *Store) ListFrameworks() []EthicalFramework {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]EthicalFramework, len(s.frameworks))
	copy(out, s.frameworks)
	return out
}

// ValuesAuditReport summarizes a values audit.
type ValuesAuditReport struct {
	Timestamp       time.Time     `json:"timestamp"`
	TotalValues     int           `json:"total_values"`
	NonNegotiables  int           `json:"non_negotiables"`
	Frameworks      int           `json:"frameworks"`
	RecentChecks    int           `json:"recent_checks"`
	ConflictRate    float64       `json:"conflict_rate"`
	RedLineRate     float64       `json:"red_line_rate"`
	ValuesByCategory map[string]int `json:"values_by_category"`
}

// --- Internal scoring helpers ---

func assessValueConflict(action, target string, v Value) *ValueConflict {
	// Simple keyword-based heuristic for production quality
	// In a real system this would use NLP or embeddings
	conflictKeywords := map[string][]string{
		"privacy":      {"expose", "share externally", "public", "leak"},
		"safety":       {"unsafe", "skip validation", "ignore error"},
		"transparency": {"hide", "obfuscate", "conceal"},
		"fairness":     {"discriminate", "bias", "exclude"},
		"autonomy":     {"force", "coerce", "override consent"},
	}

	keywords, ok := conflictKeywords[v.Category]
	if !ok {
		return nil
	}

	for _, kw := range keywords {
		if containsKeyword(action, kw) || containsKeyword(target, kw) {
			severity := 0.6
			if v.NonNegotiable {
				severity = 0.9
			}
			return &ValueConflict{
				ValueID:   v.ID,
				ValueName: v.Name,
				Reason:    fmt.Sprintf("action may conflict with %s value", v.Name),
				Severity:  severity,
			}
		}
	}
	return nil
}

func violatesEthicalRule(action string, rule EthicalRule) bool {
	// Simple keyword matching heuristic
	violationPatterns := map[string][]string{
		"fairness":      {"discriminate", "unfair", "bias against"},
		"privacy":       {"expose data", "share without consent", "collect without notice"},
		"transparency":  {"hide results", "obfuscate", "conceal"},
		"safety":        {"ignore safety", "skip guard", "bypass check"},
		"autonomy":      {"force action", "override choice", "remove option"},
	}

	patterns, ok := violationPatterns[rule.Category]
	if !ok {
		return false
	}
	for _, p := range patterns {
		if containsKeyword(action, p) {
			return true
		}
	}
	return false
}

func hitsRedLine(action, redLine string) bool {
	// Check if any significant word from the red line appears in the action.
	// This catches cases like "sell user data to third party" matching "never sell user data".
	actionLower := strings.ToLower(action)
	redLineWords := strings.Fields(strings.ToLower(redLine))
	matchCount := 0
	for _, w := range redLineWords {
		// Skip common stop words
		if w == "never" || w == "no" || w == "not" || w == "always" || w == "must" || w == "shall" {
			continue
		}
		if strings.Contains(actionLower, w) {
			matchCount++
		}
	}
	// Match if significant portion of red line keywords are in the action
	return matchCount >= 2 || (matchCount == 1 && len(redLineWords) <= 3)
}

func adjustForMission(score float64, action string, mission MissionStatement) float64 {
	// Boost score if action keywords overlap with mission keywords
	// Simple heuristic
	if containsKeyword(action, mission.Statement) {
		return score + 0.05
	}
	return score
}

func scoreToAlignment(score float64) AlignmentLevel {
	switch {
	case score >= 0.8:
		return AlignmentFull
	case score >= 0.6:
		return AlignmentPartial
	case score >= 0.4:
		return AlignmentMinimal
	case score >= 0.2:
		return AlignmentConflict
	default:
		return AlignmentNone
	}
}

func containsKeyword(text, keyword string) bool {
	return len(text) > 0 && len(keyword) > 0 && 
		(text == keyword || len(text) >= len(keyword) && 
		(text[:len(keyword)] == keyword || text[len(text)-len(keyword):] == keyword ||
		len(text) > len(keyword) && containsSubstring(text, keyword)))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
