// Package succession implements the Knowledge Transfer Protocol — a structured,
// multi-phase system where departing agents produce transferable knowledge
// capsules that incoming agents consume through guided onboarding.
//
// This is NOT dumping memory into a file. It's distillation of expertise,
// pattern recognition, contextual awareness, and failure-mode knowledge into
// a versioned, signed, verifiable capsule that preserves institutional
// knowledge across agent generations.
package succession

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Phase represents a phase in the succession process.
type Phase string

const (
	PhaseExtract      Phase = "extract"      // Departing agent's knowledge being distilled
	PhaseCapsuleBuild Phase = "capsule_build" // Capsule being assembled
	PhaseObserve      Phase = "observe"       // Successor observes capsule knowledge
	PhaseShadow       Phase = "shadow"        // Successor shadows predecessor (if still active)
	PhaseSolo         Phase = "solo"          // Successor works solo with capsule support
	PhaseVerified     Phase = "verified"      // Transfer verified, predecessor decommissioned
)

// ExpertiseCategory classifies a type of expertise.
type ExpertiseCategory string

const (
	CatDecisionMaking ExpertiseCategory = "decision_making"
	CatToolUsage      ExpertiseCategory = "tool_usage"
	CatDomainKnowledge ExpertiseCategory = "domain_knowledge"
	CatFailureModes   ExpertiseCategory = "failure_modes"
	CatHeuristics     ExpertiseCategory = "heuristics"
	CatRelationships  ExpertiseCategory = "relationships" // who to talk to, escalation paths
	CatContext        ExpertiseCategory = "context"        // project-specific context
	CatPreferences    ExpertiseCategory = "preferences"    // user/team preferences
)

// DistilledKnowledge is a single teachable unit extracted from experience.
type DistilledKnowledge struct {
	ID          string             `json:"id"`
	Category    ExpertiseCategory  `json:"category"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Trigger     string             `json:"trigger"`      // When this knowledge applies
	Action      string             `json:"action"`       // What to do
	Rationale   string             `json:"rationale"`    // Why this approach
	Failures    []FailureRecord    `json:"failures,omitempty"` // Times this went wrong
	Successes   int                `json:"successes"`    // Times this worked
	Confidence  float64            `json:"confidence"`   // 0-1, how reliable this is
	SourceTasks []string           `json:"source_tasks"` // Task IDs this was learned from
	Tags        []string           `json:"tags,omitempty"`
}

// FailureRecord captures a specific failure and its lesson.
type FailureRecord struct {
	TaskID      string `json:"task_id"`
	WhatWentWrong string `json:"what"`
	RootCause   string `json:"root_cause"`
	Lesson      string `json:"lesson"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// KnowledgeCapsule is a versioned, signed package of distilled expertise.
type KnowledgeCapsule struct {
	ID             string              `json:"id"`
	Version        string              `json:"version"`
	AgentID        string              `json:"agent_id"`         // Departing agent
	Division       string              `json:"division"`
	Role           string              `json:"role"`
	Tenure         time.Duration       `json:"tenure"`
	KnowledgeUnits []DistilledKnowledge `json:"knowledge_units"`
	DecisionLog    []DecisionEntry     `json:"decision_log,omitempty"`
	Relationships  []RelationshipMap   `json:"relationships,omitempty"`
	ActiveContext  []ContextEntry      `json:"active_context,omitempty"`
	Preferences    []PreferenceEntry   `json:"preferences,omitempty"`
	Statistics     CapsuleStats        `json:"statistics"`
	Hash           string              `json:"hash"`           // Cryptographic integrity
	CreatedAt      time.Time           `json:"created_at"`
}

// DecisionEntry captures a significant decision for context transfer.
type DecisionEntry struct {
	TaskID    string `json:"task_id"`
	Decision  string `json:"decision"`
	Reasoning string `json:"reasoning"`
	Outcome   string `json:"outcome"` // "success", "partial", "failure"
	Timestamp time.Time `json:"timestamp"`
}

// RelationshipMap documents working relationships.
type RelationshipMap struct {
	TargetAgentID string `json:"target_agent_id"`
	Relationship  string `json:"relationship"` // "escalate_to", "collaborate_with", "report_to"
	Context       string `json:"context"`      // When/how to interact
}

// ContextEntry captures active project context.
type ContextEntry struct {
	Project   string `json:"project"`
	Status    string `json:"status"`
	NextSteps string `json:"next_steps"`
	Blockers  string `json:"blockers"`
	Notes     string `json:"notes"`
}

// PreferenceEntry captures user or team preferences.
type PreferenceEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Scope string `json:"scope"` // "user", "team", "division"
}

// CapsuleStats holds aggregate statistics about the capsule.
type CapsuleStats struct {
	TotalTasksCompleted  int     `json:"total_tasks_completed"`
	TotalDecisions       int     `json:"total_decisions"`
	SuccessRate          float64 `json:"success_rate"`
	AverageTaskTime      float64 `json:"average_task_time_seconds"`
	KnowledgeUnitsCount  int     `json:"knowledge_units_count"`
	FailureLessonsCount  int     `json:"failure_lessons_count"`
	DistillationTime     float64 `json:"distillation_time_seconds"`
}

// SuccessionSession tracks a single knowledge transfer process.
type SuccessionSession struct {
	ID              string    `json:"id"`
	DepartingAgentID string  `json:"departing_agent_id"`
	SuccessorAgentID string  `json:"successor_agent_id"`
	CapsuleID       string    `json:"capsule_id"`
	Phase           Phase     `json:"phase"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	ContinuityScore float64   `json:"continuity_score"` // 0-1, how much transferred
	VerificationResults []VerificationResult `json:"verification_results,omitempty"`
	Blockers        []string  `json:"blockers,omitempty"`
}

// VerificationResult tests whether a successor can handle a known task.
type VerificationResult struct {
	TaskID       string  `json:"task_id"`
	TaskType     string  `json:"task_type"`
	Difficulty   float64 `json:"difficulty"`
	Passed       bool    `json:"passed"`
	Score        float64 `json:"score"` // 0-100
	CapsuleUnits []string `json:"capsule_units_used"` // Which knowledge units helped
	Gaps         []string `json:"gaps,omitempty"`     // Knowledge gaps found
}

// DistillationEngine extracts teachable patterns from raw task history.
type DistillationEngine struct {
	knowledge map[string]*DistilledKnowledge
	decisionPatterns map[string]int
	mu sync.Mutex
}

// NewDistillationEngine creates a new engine.
func NewDistillationEngine() *DistillationEngine {
	return &DistillationEngine{
		knowledge:        make(map[string]*DistilledKnowledge),
		decisionPatterns: make(map[string]int),
	}
}

// TaskRecord represents a completed task for distillation.
type TaskRecord struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Outcome     string    `json:"outcome"` // "success", "partial", "failure"
	ToolCalls   []ToolCall `json:"tool_calls,omitempty"`
	Decisions   []string  `json:"decisions,omitempty"`
	Duration    float64   `json:"duration_seconds"`
	Error       string    `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// ToolCall records a tool/API call made during a task.
type ToolCall struct {
	Tool    string `json:"tool"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

// Distill extracts knowledge from task records.
func (de *DistillationEngine) Distill(records []TaskRecord) []*DistilledKnowledge {
	de.mu.Lock()
	defer de.mu.Unlock()

	var results []*DistilledKnowledge

	// Group by type for pattern extraction
	byType := make(map[string][]TaskRecord)
	for _, r := range records {
		byType[r.Type] = append(byType[r.Type], r)
	}

	for taskType, tasks := range byType {
		// Extract success patterns
		knowledge := de.extractPatterns(taskType, tasks)
		results = append(results, knowledge...)
	}

	// Extract failure lessons
	failureLessons := de.extractFailureLessons(records)
	results = append(results, failureLessons...)

	// Extract tool usage heuristics
	toolKnowledge := de.extractToolHeuristics(records)
	results = append(results, toolKnowledge...)

	// Store all
	for _, k := range results {
		de.knowledge[k.ID] = k
	}

	return results
}

// extractPatterns finds successful patterns in a task type.
func (de *DistillationEngine) extractPatterns(taskType string, tasks []TaskRecord) []*DistilledKnowledge {
	var results []*DistilledKnowledge

	// Find common tool sequences in successful tasks
	toolSeqs := make(map[string]int)
	successCount := 0
	for _, t := range tasks {
		if t.Outcome == "success" {
			successCount++
			seq := toolCallSequence(t.ToolCalls)
			if seq != "" {
				toolSeqs[seq]++
			}
		}
	}

	for seq, count := range toolSeqs {
		if count >= 2 {
			confidence := math.Min(float64(count)/float64(successCount), 1.0)
			id := fmt.Sprintf("dk-%x", sha256.Sum256([]byte("pattern:"+taskType+seq)))
			k := &DistilledKnowledge{
				ID:          id[:20],
				Category:    CatDecisionMaking,
				Title:       fmt.Sprintf("Successful %s workflow", taskType),
				Description: fmt.Sprintf("For %s tasks, this tool sequence succeeded %d times", taskType, count),
				Trigger:     fmt.Sprintf("task_type=%s", taskType),
				Action:      seq,
				Rationale:   fmt.Sprintf("Observed success rate: %.0f%% in %d tasks", confidence*100, count),
				Successes:   count,
				Confidence:  confidence,
				Tags:        []string{taskType, "workflow", "extracted"},
			}
			results = append(results, k)
		}
	}

	return results
}

// extractFailureLessons extracts knowledge from failed tasks.
func (de *DistillationEngine) extractFailureLessons(records []TaskRecord) []*DistilledKnowledge {
	var results []*DistilledKnowledge
	failureByTool := make(map[string][]TaskRecord)

	for _, r := range records {
		if r.Outcome == "failure" && r.Error != "" {
			for _, tc := range r.ToolCalls {
				if !tc.Success {
					failureByTool[tc.Tool] = append(failureByTool[tc.Tool], r)
				}
			}
		}
	}

	for tool, failures := range failureByTool {
		if len(failures) >= 1 {
			id := fmt.Sprintf("dk-%x", sha256.Sum256([]byte("failure:"+tool)))
			lessons := make([]FailureRecord, 0, len(failures))
			for _, f := range failures {
				lessons = append(lessons, FailureRecord{
					TaskID:        f.ID,
					WhatWentWrong: f.Error,
					RootCause:     fmt.Sprintf("Tool %s call failed", tool),
					Lesson:        fmt.Sprintf("When using %s, watch for: %s", tool, f.Error),
					OccurredAt:    f.Timestamp,
				})
			}

			k := &DistilledKnowledge{
				ID:          id[:20],
				Category:    CatFailureModes,
				Title:       fmt.Sprintf("Failure patterns with %s", tool),
				Description: fmt.Sprintf("Documented failures using %s across %d tasks", tool, len(failures)),
				Trigger:     fmt.Sprintf("tool=%s", tool),
				Action:      fmt.Sprintf("Consider alternatives to %s or add error handling", tool),
				Rationale:   "Learned from production failures",
				Failures:    lessons,
				Confidence:  math.Min(float64(len(failures))/5.0, 1.0),
				Tags:        []string{"failure", tool, "lesson"},
			}
			results = append(results, k)
		}
	}

	return results
}

// extractToolHeuristics extracts tool selection knowledge.
func (de *DistillationEngine) extractToolHeuristics(records []TaskRecord) []*DistilledKnowledge {
	var results []*DistilledKnowledge
	toolByContext := make(map[string]map[string]int) // context → tool → count

	for _, r := range records {
		if r.Outcome == "success" {
			context := r.Type
			if toolByContext[context] == nil {
				toolByContext[context] = make(map[string]int)
			}
			for _, tc := range r.ToolCalls {
				toolByContext[context][tc.Tool]++
			}
		}
	}

	for context, tools := range toolByContext {
		type toolCount struct {
			tool  string
			count int
		}
		var sorted []toolCount
		for t, c := range tools {
			sorted = append(sorted, toolCount{t, c})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

		if len(sorted) > 0 {
			id := fmt.Sprintf("dk-%x", sha256.Sum256([]byte("tool:"+context)))
			preferred := sorted[0].tool
			total := 0
			for _, s := range sorted {
				total += s.count
			}

			k := &DistilledKnowledge{
				ID:          id[:20],
				Category:    CatToolUsage,
				Title:       fmt.Sprintf("Preferred tools for %s", context),
				Description: fmt.Sprintf("For %s tasks, %s is most effective (%d/%d uses)", context, preferred, sorted[0].count, total),
				Trigger:     fmt.Sprintf("task_type=%s", context),
				Action:      fmt.Sprintf("Use %s as primary tool", preferred),
				Rationale:   fmt.Sprintf("Historical success rate: %.0f%%", float64(sorted[0].count)/float64(total)*100),
				Successes:   sorted[0].count,
				Confidence:  math.Min(float64(sorted[0].count)/10.0, 1.0),
				Tags:        []string{"tool_selection", context},
			}
			results = append(results, k)
		}
	}

	return results
}

func toolCallSequence(calls []ToolCall) string {
	var parts []string
	for _, c := range calls {
		parts = append(parts, c.Tool)
	}
	return strings.Join(parts, "→")
}

// BuildCapsule assembles a KnowledgeCapsule from distilled knowledge.
func BuildCapsule(agentID, division, role string, tenure time.Duration, knowledge []*DistilledKnowledge, records []TaskRecord) *KnowledgeCapsule {
	capsule := &KnowledgeCapsule{
		ID:             fmt.Sprintf("cap-%x", sha256.Sum256([]byte(agentID+time.Now().String()))),
		Version:        "1.0",
		AgentID:        agentID,
		Division:       division,
		Role:           role,
		Tenure:         tenure,
		KnowledgeUnits: func() []DistilledKnowledge {
			units := make([]DistilledKnowledge, len(knowledge))
			for i, k := range knowledge {
				units[i] = *k
			}
			return units
		}(),
		CreatedAt:      time.Now(),
	}

	// Build decision log from records
	successCount := 0
	totalTasks := len(records)
	totalTime := 0.0
	failureLessons := 0

	for _, r := range records {
		totalTime += r.Duration
		if r.Outcome == "success" {
			successCount++
		}
		for _, k := range knowledge {
			failureLessons += len(k.Failures)
		}
		if len(r.Decisions) > 0 {
			for _, d := range r.Decisions {
				capsule.DecisionLog = append(capsule.DecisionLog, DecisionEntry{
					TaskID:    r.ID,
					Decision:  d,
					Reasoning: fmt.Sprintf("Made during %s task", r.Type),
					Outcome:   r.Outcome,
					Timestamp: r.Timestamp,
				})
			}
		}
	}

	var successRate float64
	if totalTasks > 0 {
		successRate = float64(successCount) / float64(totalTasks) * 100
	}
	var avgTime float64
	if totalTasks > 0 {
		avgTime = totalTime / float64(totalTasks)
	}

	capsule.Statistics = CapsuleStats{
		TotalTasksCompleted: totalTasks,
		TotalDecisions:      len(capsule.DecisionLog),
		SuccessRate:         successRate,
		AverageTaskTime:     avgTime,
		KnowledgeUnitsCount: len(knowledge),
		FailureLessonsCount: failureLessons,
	}

	// Compute hash for integrity
	data, _ := json.Marshal(capsule)
	capsule.Hash = fmt.Sprintf("%x", sha256.Sum256(data))

	return capsule
}

// ContinuityVerifier tests a successor's ability to handle known tasks.
type ContinuityVerifier struct {
	testTasks []TaskRecord
	results   []VerificationResult
	mu        sync.Mutex
}

// NewContinuityVerifier creates a verifier with test tasks from the departing agent.
func NewContinuityVerifier(records []TaskRecord) *ContinuityVerifier {
	// Select representative tasks: mix of easy, medium, hard
	sort.Slice(records, func(i, j int) bool {
		return records[i].Duration < records[j].Duration
	})

	// Take samples from each difficulty tier
	var selected []TaskRecord
	n := len(records)
	if n > 10 {
		// 3 easy, 4 medium, 3 hard
		selected = append(selected, records[:3]...)
		selected = append(selected, records[n/3:n/3+4]...)
		selected = append(selected, records[n-3:]...)
	} else {
		selected = records
	}

	return &ContinuityVerifier{testTasks: selected}
}

// TestTask evaluates a successor's performance on a known task.
func (cv *ContinuityVerifier) TestTask(taskType string, handler func(TaskRecord) (bool, float64, []string)) VerificationResult {
	cv.mu.Lock()
	defer cv.mu.Unlock()

	for _, task := range cv.testTasks {
		if task.Type == taskType || taskType == "" {
			passed, score, gaps := handler(task)
			result := VerificationResult{
				TaskID:     task.ID,
				TaskType:   task.Type,
				Difficulty: task.Duration / 60.0, // rough difficulty by time
				Passed:     passed,
				Score:      score,
				Gaps:       gaps,
			}
			cv.results = append(cv.results, result)
			return result
		}
	}
	return VerificationResult{TaskType: taskType, Score: 0}
}

// ContinuityScore computes the overall transfer success rate.
func (cv *ContinuityVerifier) ContinuityScore() float64 {
	cv.mu.Lock()
	defer cv.mu.Unlock()

	if len(cv.results) == 0 {
		return 0
	}

	var totalScore float64
	for _, r := range cv.results {
		totalScore += r.Score
	}
	return totalScore / float64(len(cv.results)) / 100.0
}

// Gaps returns all identified knowledge gaps.
func (cv *ContinuityVerifier) Gaps() []string {
	cv.mu.Lock()
	defer cv.mu.Unlock()

	gapSet := make(map[string]bool)
	for _, r := range cv.results {
		for _, g := range r.Gaps {
			gapSet[g] = true
		}
	}
	var gaps []string
	for g := range gapSet {
		gaps = append(gaps, g)
	}
	return gaps
}

// InstitutionalBank preserves collective knowledge across agent generations.
type InstitutionalBank struct {
	capsules map[string]*KnowledgeCapsule
	storeDir string
	mu       sync.RWMutex
}

// NewInstitutionalBank creates a new institutional knowledge bank.
func NewInstitutionalBank(storeDir string) *InstitutionalBank {
	os.MkdirAll(storeDir, 0755)
	bank := &InstitutionalBank{
		capsules: make(map[string]*KnowledgeCapsule),
		storeDir: storeDir,
	}
	bank.load()
	return bank
}

// Deposit stores a capsule in the institutional bank.
func (ib *InstitutionalBank) Deposit(capsule *KnowledgeCapsule) error {
	ib.mu.Lock()
	defer ib.mu.Unlock()

	// Verify integrity
	checkCopy := *capsule
	checkCopy.Hash = ""
	data, _ := json.Marshal(checkCopy)
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if hash != capsule.Hash {
		return fmt.Errorf("capsule integrity check failed: hash mismatch")
	}

	ib.capsules[capsule.ID] = capsule
	ib.persist()
	return nil
}

// Withdraw retrieves a capsule by ID.
func (ib *InstitutionalBank) Withdraw(id string) (*KnowledgeCapsule, bool) {
	ib.mu.RLock()
	defer ib.mu.RUnlock()
	c, ok := ib.capsules[id]
	return c, ok
}

// WithdrawForRole retrieves the most relevant capsule for a given role.
func (ib *InstitutionalBank) WithdrawForRole(division, role string) (*KnowledgeCapsule, bool) {
	ib.mu.RLock()
	defer ib.mu.RUnlock()

	var best *KnowledgeCapsule
	bestScore := 0.0

	for _, c := range ib.capsules {
		score := 0.0
		if c.Division == division {
			score += 0.5
		}
		if c.Role == role {
			score += 0.3
		}
		score += c.Statistics.SuccessRate / 1000.0
		score += float64(c.Statistics.KnowledgeUnitsCount) / 100.0

		if score > bestScore {
			bestScore = score
			best = c
		}
	}

	return best, best != nil
}

// SearchKnowledge searches across all capsules for relevant knowledge.
func (ib *InstitutionalBank) SearchKnowledge(query string, limit int) []*DistilledKnowledge {
	ib.mu.RLock()
	defer ib.mu.RUnlock()

	query = strings.ToLower(query)
	type scored struct {
		k     *DistilledKnowledge
		score float64
	}
	var results []scored

	for _, c := range ib.capsules {
		for _, k := range c.KnowledgeUnits {
			s := 0.0
			if strings.Contains(strings.ToLower(k.Title), query) {
				s += 2.0
			}
			if strings.Contains(strings.ToLower(k.Description), query) {
				s += 1.5
			}
			if strings.Contains(strings.ToLower(k.Trigger), query) {
				s += 1.0
			}
			if strings.Contains(strings.ToLower(k.Action), query) {
				s += 1.0
			}
			s += k.Confidence // boost by confidence
			if s > 0 {
				kCopy := k
			results = append(results, scored{&kCopy, s})
			}
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	var out []*DistilledKnowledge
	for _, r := range results {
		out = append(out, r.k)
	}
	return out
}

// GenerationalStats returns statistics about knowledge across generations.
func (ib *InstitutionalBank) GenerationalStats() map[string]int {
	ib.mu.RLock()
	defer ib.mu.RUnlock()

	stats := map[string]int{
		"total_capsules":       len(ib.capsules),
		"total_knowledge_units": 0,
		"total_decisions":       0,
		"total_failure_lessons": 0,
	}

	for _, c := range ib.capsules {
		stats["total_knowledge_units"] += c.Statistics.KnowledgeUnitsCount
		stats["total_decisions"] += c.Statistics.TotalDecisions
		stats["total_failure_lessons"] += c.Statistics.FailureLessonsCount
	}

	return stats
}

func (ib *InstitutionalBank) persist() {
	data, _ := json.MarshalIndent(ib.capsules, "", "  ")
	os.WriteFile(filepath.Join(ib.storeDir, "institutional_bank.json"), data, 0644)
}

func (ib *InstitutionalBank) load() {
	data, err := os.ReadFile(filepath.Join(ib.storeDir, "institutional_bank.json"))
	if err == nil {
		json.Unmarshal(data, &ib.capsules)
	}
}

// SuccessionManager orchestrates the full knowledge transfer lifecycle.
type SuccessionManager struct {
	sessions       map[string]*SuccessionSession
	distiller      *DistillationEngine
	bank           *InstitutionalBank
	storeDir       string
	mu             sync.Mutex
}

// NewSuccessionManager creates a new succession manager.
func NewSuccessionManager(storeDir string) *SuccessionManager {
	os.MkdirAll(storeDir, 0755)
	sm := &SuccessionManager{
		sessions:  make(map[string]*SuccessionSession),
		distiller: NewDistillationEngine(),
		bank:      NewInstitutionalBank(filepath.Join(storeDir, "bank")),
		storeDir:  storeDir,
	}
	sm.load()
	return sm
}

// InitiateSuccession begins a knowledge transfer from departing to successor.
func (sm *SuccessionManager) InitiateSuccession(departingID, successorID string) (*SuccessionSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &SuccessionSession{
		ID:               fmt.Sprintf("succ-%d", time.Now().UnixNano()),
		DepartingAgentID: departingID,
		SuccessorAgentID: successorID,
		Phase:            PhaseExtract,
		StartedAt:        time.Now(),
	}

	sm.sessions[session.ID] = session
	sm.persist()
	return session, nil
}

// ExtractKnowledge distills knowledge from the departing agent's task history.
func (sm *SuccessionManager) ExtractKnowledge(sessionID string, records []TaskRecord) (*KnowledgeCapsule, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Distill knowledge
	knowledge := sm.distiller.Distill(records)

	// Compute tenure
	var earliest, latest time.Time
	for _, r := range records {
		if earliest.IsZero() || r.Timestamp.Before(earliest) {
			earliest = r.Timestamp
		}
		if latest.IsZero() || r.Timestamp.After(latest) {
			latest = r.Timestamp
		}
	}
	tenure := latest.Sub(earliest)

	// Build capsule
	capsule := BuildCapsule(session.DepartingAgentID, "", "", tenure, knowledge, records)
	session.CapsuleID = capsule.ID
	session.Phase = PhaseCapsuleBuild

	// Deposit in bank
	sm.bank.Deposit(capsule)

	session.Phase = PhaseObserve
	sm.persist()
	return capsule, nil
}

// VerifyContinuity tests the successor against known tasks.
func (sm *SuccessionManager) VerifyContinuity(sessionID string, records []TaskRecord, handler func(TaskRecord) (bool, float64, []string)) (float64, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return 0, fmt.Errorf("session %s not found", sessionID)
	}

	verifier := NewContinuityVerifier(records)
	for _, task := range verifier.testTasks {
		verifier.TestTask(task.Type, handler)
	}

	session.ContinuityScore = verifier.ContinuityScore()
	session.VerificationResults = verifier.results
	session.Blockers = verifier.Gaps()

	if session.ContinuityScore >= 0.7 {
		session.Phase = PhaseVerified
		now := time.Now()
		session.CompletedAt = &now
	} else {
		session.Phase = PhaseShadow // Need more shadowing
	}

	sm.persist()
	return session.ContinuityScore, nil
}

// GetSession retrieves a succession session.
func (sm *SuccessionManager) GetSession(id string) (*SuccessionSession, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	s, ok := sm.sessions[id]
	return s, ok
}

func (sm *SuccessionManager) persist() {
	data, _ := json.MarshalIndent(sm.sessions, "", "  ")
	os.WriteFile(filepath.Join(sm.storeDir, "succession_sessions.json"), data, 0644)
}

func (sm *SuccessionManager) load() {
	data, err := os.ReadFile(filepath.Join(sm.storeDir, "succession_sessions.json"))
	if err == nil {
		json.Unmarshal(data, &sm.sessions)
	}
}
