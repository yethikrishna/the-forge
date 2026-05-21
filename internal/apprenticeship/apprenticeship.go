// Package apprenticeship implements an agent apprenticeship system where
// junior agents shadow senior agents, learn behavioral patterns, and earn
// certification through competency demonstrations.
package apprenticeship

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Level represents the apprentice's progression stage.
type Level string

const (
	LevelObserver   Level = "observer"
	LevelShadow     Level = "shadow"
	LevelSupervised Level = "supervised"
	LevelSolo       Level = "solo"
)

// Apprentice represents a junior agent learning from a mentor.
type Apprentice struct {
	ID               string          `json:"id"`
	MentorID         string          `json:"mentor_id"`
	Level            Level           `json:"level"`
	TasksCompleted   int             `json:"tasks_completed"`
	PatternsLearned  []string        `json:"patterns_learned"`
	Certifications   []Certification `json:"certifications"`
	ShadowSessions   []ShadowSession `json:"shadow_sessions"`
	JoinedAt         time.Time       `json:"joined_at"`
	PromotedAt       map[Level]time.Time `json:"promoted_at"`
	ProgressScore    float64         `json:"progress_score"`
}

// BehavioralPattern captures a reusable behavior extracted from mentor actions.
type BehavioralPattern struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Category     string            `json:"category"` // tool_call, decision, response, workflow
	Trigger      string            `json:"trigger"`
	Action       string            `json:"action"`
	Context      map[string]string `json:"context,omitempty"`
	Frequency    int               `json:"frequency"`
	Confidence   float64           `json:"confidence"`
	ExtractedAt  time.Time         `json:"extracted_at"`
}

// ShadowSession records a period where an apprentice observes a mentor.
type ShadowSession struct {
	ID          string           `json:"id"`
	ApprenticeID string         `json:"apprentice_id"`
	MentorID    string           `json:"mentor_id"`
	StartedAt   time.Time       `json:"started_at"`
	EndedAt     *time.Time      `json:"ended_at,omitempty"`
	Actions     []MentorAction  `json:"actions"`
	PatternsExtracted []string  `json:"patterns_extracted"`
	Status      string          `json:"status"` // active, completed, cancelled
}

// MentorAction records a single action taken by a mentor during a shadow session.
type MentorAction struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"` // tool_call, decision, response, handoff
	Tool      string                 `json:"tool,omitempty"`
	Input     string                 `json:"input,omitempty"`
	Output    string                 `json:"output,omitempty"`
	Reasoning string                 `json:"reasoning,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Certification represents a competency certification earned by an apprentice.
type Certification struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Score        float64   `json:"score"`
	PassingScore float64   `json:"passing_score"`
	Passed       bool      `json:"passed"`
	EvaluatedAt  time.Time `json:"evaluated_at"`
	ExamID       string    `json:"exam_id"`
}

// ExamScenario is a test scenario used in certification.
type ExamScenario struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Actions     []ExamAction   `json:"actions"`
	Expected    []ExamExpected `json:"expected"`
	Difficulty  float64        `json:"difficulty"` // 0-1
}

// ExamAction is a step the apprentice must handle in an exam scenario.
type ExamAction struct {
	Type     string                 `json:"type"`
	Tool     string                 `json:"tool,omitempty"`
	Input    string                 `json:"input"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ExamExpected defines the expected response for an exam action.
type ExamExpected struct {
	ActionIndex int      `json:"action_index"`
	AcceptableResponses []string `json:"acceptable_responses"`
	Weight      float64  `json:"weight"`
}

// ExamResult stores the result of a certification exam.
type ExamResult struct {
	ExamID        string              `json:"exam_id"`
	ApprenticeID  string              `json:"apprentice_id"`
	Score         float64             `json:"score"`
	PassingScore  float64             `json:"passing_score"`
	Passed        bool                `json:"passed"`
	Responses     []ExamResponse      `json:"responses"`
	CompletedAt   time.Time           `json:"completed_at"`
}

// ExamResponse captures an apprentice's response to an exam action.
type ExamResponse struct {
	ActionIndex int     `json:"action_index"`
	Response    string  `json:"response"`
	Correct     bool    `json:"correct"`
	Weight      float64 `json:"weight"`
}

// PatternStore extracts and manages behavioral patterns from mentor actions.
type PatternStore struct {
	patterns map[string]*BehavioralPattern
	mu       sync.RWMutex
}

// NewPatternStore creates a new PatternStore.
func NewPatternStore() *PatternStore {
	return &PatternStore{
		patterns: make(map[string]*BehavioralPattern),
	}
}

// ExtractPatterns analyzes mentor actions from a shadow session and extracts behavioral patterns.
func (ps *PatternStore) ExtractPatterns(session *ShadowSession) []*BehavioralPattern {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	var extracted []*BehavioralPattern
	actionCounts := make(map[string]int)
	toolSequences := make(map[string]int)

	// Count action types and tool sequences
	for i, action := range session.Actions {
		key := fmt.Sprintf("%s:%s", action.Type, action.Tool)
		actionCounts[key]++
		if i > 0 {
			prev := session.Actions[i-1]
			seq := fmt.Sprintf("%s:%s->%s:%s", prev.Type, prev.Tool, action.Type, action.Tool)
			toolSequences[seq]++
		}
	}

	// Create patterns from frequent actions
	for key, count := range actionCounts {
		if count >= 2 {
			pattern := &BehavioralPattern{
				ID:          fmt.Sprintf("pat-%d", time.Now().UnixNano()+int64(len(ps.patterns))),
				Name:        key,
				Category:    "tool_call",
				Trigger:     "task_execution",
				Action:      key,
				Frequency:   count,
				Confidence:  math.Min(float64(count)/10.0, 1.0),
				ExtractedAt: time.Now(),
			}
			ps.patterns[pattern.ID] = pattern
			extracted = append(extracted, pattern)
		}
	}

	// Create patterns from tool sequences
	for seq, count := range toolSequences {
		if count >= 2 {
			pattern := &BehavioralPattern{
				ID:          fmt.Sprintf("pat-%d", time.Now().UnixNano()+int64(len(ps.patterns))),
				Name:        seq,
				Category:    "workflow",
				Trigger:     "sequential_task",
				Action:      seq,
				Frequency:   count,
				Confidence:  math.Min(float64(count)/5.0, 1.0),
				ExtractedAt: time.Now(),
			}
			ps.patterns[pattern.ID] = pattern
			extracted = append(extracted, pattern)
		}
	}

	return extracted
}

// GetPattern retrieves a pattern by ID.
func (ps *PatternStore) GetPattern(id string) (*BehavioralPattern, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	p, ok := ps.patterns[id]
	return p, ok
}

// ListPatterns returns all patterns, optionally filtered by category.
func (ps *PatternStore) ListPatterns(category string) []*BehavioralPattern {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var result []*BehavioralPattern
	for _, p := range ps.patterns {
		if category == "" || p.Category == category {
			result = append(result, p)
		}
	}
	return result
}

// PatternCount returns the total number of stored patterns.
func (ps *PatternStore) PatternCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.patterns)
}

// ApprenticeshipSystem manages the full apprenticeship lifecycle.
type ApprenticeshipSystem struct {
	apprentices map[string]*Apprentice
	patternStore *PatternStore
	exams       map[string]*ExamScenario
	storeDir    string
	mu          sync.RWMutex
}

// NewApprenticeshipSystem creates a new apprenticeship system.
func NewApprenticeshipSystem(storeDir string) *ApprenticeshipSystem {
	os.MkdirAll(storeDir, 0o755)
	sys := &ApprenticeshipSystem{
		apprentices:  make(map[string]*Apprentice),
		patternStore: NewPatternStore(),
		exams:        make(map[string]*ExamScenario),
		storeDir:     storeDir,
	}
	sys.load()
	return sys
}

// RegisterApprentice creates a new apprentice under a mentor.
func (as *ApprenticeshipSystem) RegisterApprentice(id, mentorID string) (*Apprentice, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	if _, exists := as.apprentices[id]; exists {
		return nil, fmt.Errorf("apprentice %s already exists", id)
	}

	apprentice := &Apprentice{
		ID:              id,
		MentorID:        mentorID,
		Level:           LevelObserver,
		TasksCompleted:  0,
		PatternsLearned: []string{},
		Certifications:  []Certification{},
		ShadowSessions:  []ShadowSession{},
		JoinedAt:        time.Now(),
		PromotedAt:      map[Level]time.Time{LevelObserver: time.Now()},
		ProgressScore:   0.0,
	}

	as.apprentices[id] = apprentice
	as.persist()
	return apprentice, nil
}

// GetApprentice retrieves an apprentice by ID.
func (as *ApprenticeshipSystem) GetApprentice(id string) (*Apprentice, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()
	a, ok := as.apprentices[id]
	if !ok {
		return nil, fmt.Errorf("apprentice %s not found", id)
	}
	return a, nil
}

// StartShadowSession begins a shadow session between an apprentice and mentor.
func (as *ApprenticeshipSystem) StartShadowSession(apprenticeID, mentorID string) (*ShadowSession, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	apprentice, ok := as.apprentices[apprenticeID]
	if !ok {
		return nil, fmt.Errorf("apprentice %s not found", apprenticeID)
	}

	if apprentice.Level == LevelSolo {
		return nil, fmt.Errorf("solo apprentice does not need shadow sessions")
	}

	session := &ShadowSession{
		ID:            fmt.Sprintf("ss-%d", time.Now().UnixNano()),
		ApprenticeID:  apprenticeID,
		MentorID:      mentorID,
		StartedAt:     time.Now(),
		Actions:       []MentorAction{},
		PatternsExtracted: []string{},
		Status:        "active",
	}

	apprentice.ShadowSessions = append(apprentice.ShadowSessions, *session)
	as.persist()
	return session, nil
}

// RecordMentorAction adds a mentor action to the active shadow session.
func (as *ApprenticeshipSystem) RecordMentorAction(apprenticeID string, action MentorAction) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	apprentice, ok := as.apprentices[apprenticeID]
	if !ok {
		return fmt.Errorf("apprentice %s not found", apprenticeID)
	}

	for i := len(apprentice.ShadowSessions) - 1; i >= 0; i-- {
		sess := &apprentice.ShadowSessions[i]
		if sess.Status == "active" {
			sess.Actions = append(sess.Actions, action)
			as.persist()
			return nil
		}
	}

	return fmt.Errorf("no active shadow session for apprentice %s", apprenticeID)
}

// EndShadowSession completes the active shadow session and extracts patterns.
func (as *ApprenticeshipSystem) EndShadowSession(apprenticeID string) (*ShadowSession, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	apprentice, ok := as.apprentices[apprenticeID]
	if !ok {
		return nil, fmt.Errorf("apprentice %s not found", apprenticeID)
	}

	for i := len(apprentice.ShadowSessions) - 1; i >= 0; i-- {
		sess := &apprentice.ShadowSessions[i]
		if sess.Status == "active" {
			now := time.Now()
			sess.EndedAt = &now
			sess.Status = "completed"

			patterns := as.patternStore.ExtractPatterns(sess)
			for _, p := range patterns {
				sess.PatternsExtracted = append(sess.PatternsExtracted, p.ID)
				apprentice.PatternsLearned = append(apprentice.PatternsLearned, p.ID)
			}

			as.updateProgress(apprentice)
			as.persist()
			return sess, nil
		}
	}

	return nil, fmt.Errorf("no active shadow session for apprentice %s", apprenticeID)
}

// PromoteApprentice advances an apprentice to the next level if criteria are met.
func (as *ApprenticeshipSystem) PromoteApprentice(apprenticeID string) (Level, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	apprentice, ok := as.apprentices[apprenticeID]
	if !ok {
		return "", fmt.Errorf("apprentice %s not found", apprenticeID)
	}

	var next Level
	switch apprentice.Level {
	case LevelObserver:
		next = LevelShadow
	if len(apprentice.PatternsLearned) < 1 {
			return "", fmt.Errorf("need at least 1 pattern learned, have %d", len(apprentice.PatternsLearned))
		}
	case LevelShadow:
		next = LevelSupervised
		if apprentice.TasksCompleted < 5 {
			return "", fmt.Errorf("need at least 5 tasks completed, have %d", apprentice.TasksCompleted)
		}
	case LevelSupervised:
		next = LevelSolo
		hasCert := false
		for _, c := range apprentice.Certifications {
			if c.Passed {
				hasCert = true
				break
			}
		}
		if !hasCert {
			return "", fmt.Errorf("need at least one passing certification")
		}
	case LevelSolo:
		return LevelSolo, fmt.Errorf("already at highest level")
	}

	apprentice.Level = next
	apprentice.PromotedAt[next] = time.Now()
	as.persist()
	return next, nil
}

// CompleteTask records a completed task for an apprentice.
func (as *ApprenticeshipSystem) CompleteTask(apprenticeID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	apprentice, ok := as.apprentices[apprenticeID]
	if !ok {
		return fmt.Errorf("apprentice %s not found", apprenticeID)
	}

	apprentice.TasksCompleted++
	as.updateProgress(apprentice)
	as.persist()
	return nil
}

// RegisterExam adds a certification exam scenario.
func (as *ApprenticeshipSystem) RegisterExam(exam *ExamScenario) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.exams[exam.ID] = exam
	as.persist()
}

// EvaluateExam grades an apprentice's exam responses.
func (as *ApprenticeshipSystem) EvaluateExam(apprenticeID string, examID string, responses []ExamResponse) (*ExamResult, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	apprentice, ok := as.apprentices[apprenticeID]
	if !ok {
		return nil, fmt.Errorf("apprentice %s not found", apprenticeID)
	}

	exam, ok := as.exams[examID]
	if !ok {
		return nil, fmt.Errorf("exam %s not found", examID)
	}

	var totalWeight, earnedWeight float64
	for _, resp := range responses {
		expected := exam.Expected[resp.ActionIndex]
		resp.Weight = expected.Weight
		totalWeight += expected.Weight

		for _, acceptable := range expected.AcceptableResponses {
			if resp.Response == acceptable {
				resp.Correct = true
				earnedWeight += expected.Weight
				break
			}
		}
	}

	score := 0.0
	if totalWeight > 0 {
		score = earnedWeight / totalWeight * 100
	}

	passed := score >= exam.Expected[0].Weight*100 // simplified: use first weight as threshold proxy
	if len(exam.Expected) > 0 {
		passed = score >= 70.0 // standard passing score
	}

	result := &ExamResult{
		ExamID:       examID,
		ApprenticeID: apprenticeID,
		Score:        score,
		PassingScore: 70.0,
		Passed:       passed,
		Responses:    responses,
		CompletedAt:  time.Now(),
	}

	cert := Certification{
		ID:           fmt.Sprintf("cert-%d", time.Now().UnixNano()),
		Name:         exam.Name,
		Score:        score,
		PassingScore: 70.0,
		Passed:       passed,
		EvaluatedAt:  time.Now(),
		ExamID:       examID,
	}

	apprentice.Certifications = append(apprentice.Certifications, cert)
	as.updateProgress(apprentice)
	as.persist()
	return result, nil
}

// ProgressReport returns a detailed progress report for an apprentice.
type ProgressReport struct {
	ApprenticeID     string  `json:"apprentice_id"`
	Level            Level   `json:"level"`
	TasksCompleted   int     `json:"tasks_completed"`
	PatternsLearned  int     `json:"patterns_learned"`
	PatternsMastered int     `json:"patterns_mastered"`
	CertificationsPassed int  `json:"certifications_passed"`
	ProgressScore    float64 `json:"progress_score"`
	TotalPatterns    int     `json:"total_patterns"`
	MasteryPercent   float64 `json:"mastery_percent"`
}

// GetProgressReport generates a progress report for an apprentice.
func (as *ApprenticeshipSystem) GetProgressReport(apprenticeID string) (*ProgressReport, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	apprentice, ok := as.apprentices[apprenticeID]
	if !ok {
		return nil, fmt.Errorf("apprentice %s not found", apprenticeID)
	}

	totalPatterns := as.patternStore.PatternCount()
	mastered := 0
	for _, pid := range apprentice.PatternsLearned {
		if p, ok := as.patternStore.GetPattern(pid); ok && p.Confidence >= 0.8 {
			mastered++
		}
	}

	var masteryPct float64
	if totalPatterns > 0 {
		masteryPct = float64(mastered) / float64(totalPatterns) * 100
	}

	certsPassed := 0
	for _, c := range apprentice.Certifications {
		if c.Passed {
			certsPassed++
		}
	}

	return &ProgressReport{
		ApprenticeID:     apprentice.ID,
		Level:            apprentice.Level,
		TasksCompleted:   apprentice.TasksCompleted,
		PatternsLearned:  len(apprentice.PatternsLearned),
		PatternsMastered: mastered,
		CertificationsPassed: certsPassed,
		ProgressScore:    apprentice.ProgressScore,
		TotalPatterns:    totalPatterns,
		MasteryPercent:   masteryPct,
	}, nil
}

// updateProgress recalculates the apprentice's progress score.
func (as *ApprenticeshipSystem) updateProgress(a *Apprentice) {
	totalPatterns := as.patternStore.PatternCount()
	if totalPatterns == 0 {
		a.ProgressScore = 0
		return
	}

	patternScore := float64(len(a.PatternsLearned)) / float64(totalPatterns)
	taskScore := math.Min(float64(a.TasksCompleted)/20.0, 1.0)

	levelMultiplier := 0.25
	switch a.Level {
	case LevelShadow:
		levelMultiplier = 0.5
	case LevelSupervised:
		levelMultiplier = 0.75
	case LevelSolo:
		levelMultiplier = 1.0
	}

	a.ProgressScore = (patternScore*0.4 + taskScore*0.3 + levelMultiplier*0.3) * 100
}

// persist saves apprenticeship data to disk.
func (as *ApprenticeshipSystem) persist() {
	data, err := json.MarshalIndent(as.apprentices, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(as.storeDir, "apprentices.json"), data, 0o644)

	examData, _ := json.MarshalIndent(as.exams, "", "  ")
	os.WriteFile(filepath.Join(as.storeDir, "exams.json"), examData, 0o644)
}

// load restores apprenticeship data from disk.
func (as *ApprenticeshipSystem) load() {
	data, err := os.ReadFile(filepath.Join(as.storeDir, "apprentices.json"))
	if err == nil {
		json.Unmarshal(data, &as.apprentices)
	}
	examData, err := os.ReadFile(filepath.Join(as.storeDir, "exams.json"))
	if err == nil {
		json.Unmarshal(examData, &as.exams)
	}
}

// RunExamSimulator simulates an exam for testing purposes.
func RunExamSimulator(_ context.Context, exam *ExamScenario, handler func(ExamAction) string) *ExamResult {
	var responses []ExamResponse
	for i, action := range exam.Actions {
		response := handler(action)
		responses = append(responses, ExamResponse{
			ActionIndex: i,
			Response:    response,
		})
	}

	score := 0.0
	totalWeight := 0.0
	for _, resp := range responses {
		if resp.ActionIndex < len(exam.Expected) {
			expected := exam.Expected[resp.ActionIndex]
			totalWeight += expected.Weight
			for _, acceptable := range expected.AcceptableResponses {
				if resp.Response == acceptable {
					resp.Correct = true
					score += expected.Weight
					break
				}
			}
		}
	}

	if totalWeight > 0 {
		score = score / totalWeight * 100
	}

	return &ExamResult{
		ExamID:       exam.ID,
		Score:        score,
		PassingScore: 70.0,
		Passed:       score >= 70.0,
		Responses:    responses,
		CompletedAt:  time.Now(),
	}
}
