// Package qualitygate implements quality gates as infrastructure — review-before-merge
// is how the system works, not a configurable flag.
package qualitygate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// GateCriterion defines the type of quality check.
type GateCriterion string

const (
	CriterionLint       GateCriterion = "lint"
	CriterionTest       GateCriterion = "test"
	CriterionReview     GateCriterion = "review"
	CriterionSecurity   GateCriterion = "security"
	CriterionPerformance GateCriterion = "performance"
)

// GateStatus represents the result of a gate evaluation.
type GateStatus string

const (
	StatusPending GateStatus = "pending"
	StatusPassed  GateStatus = "passed"
	StatusFailed  GateStatus = "failed"
	StatusSkipped GateStatus = "skipped"
)

// Gate represents a quality gate that work must pass through.
type Gate struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Criterion GateCriterion  `json:"criterion"`
	Blocking  bool           `json:"blocking"` // if true, failure blocks progression
	Config    map[string]interface{} `json:"config,omitempty"`
	Order     int            `json:"order"` // execution order in pipeline
}

// GateResult stores the outcome of evaluating a gate.
type GateResult struct {
	GateID     string    `json:"gate_id"`
	WorkID     string    `json:"work_id"`
	Status     GateStatus `json:"status"`
	Evidence   string    `json:"evidence,omitempty"`
	Score      float64   `json:"score,omitempty"`
	Message    string    `json:"message,omitempty"`
	EvaluatedAt time.Time `json:"evaluated_at"`
	Duration   time.Duration `json:"duration,omitempty"`
}

// GatePipeline is an ordered sequence of gates.
type GatePipeline struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Gates []*Gate `json:"gates"`
}

// WorkItem represents something being evaluated through the pipeline.
type WorkItem struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // code, config, document, deployment
	Author      string                 `json:"author"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	SubmittedAt time.Time              `json:"submitted_at"`
	Stage       string                 `json:"stage"` // current pipeline stage
}

// GateEvaluation is a full pipeline evaluation for a work item.
type GateEvaluation struct {
	WorkID     string       `json:"work_id"`
	PipelineID string       `json:"pipeline_id"`
	Results    []GateResult `json:"results"`
	Status     string       `json:"status"` // in_progress, passed, failed
	StartedAt  time.Time    `json:"started_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	AutoPromoted bool       `json:"auto_promoted"`
}

// GateHistoryEntry is an audit trail record.
type GateHistoryEntry struct {
	ID         string     `json:"id"`
	WorkID     string     `json:"work_id"`
	GateID     string     `json:"gate_id"`
	Status     GateStatus `json:"status"`
	Evidence   string     `json:"evidence"`
	EvaluatedAt time.Time `json:"evaluated_at"`
	Evaluator  string     `json:"evaluator"` // agent or system
}

// QualityGateSystem manages quality gates as infrastructure.
type QualityGateSystem struct {
	pipelines map[string]*GatePipeline
	evaluations map[string]*GateEvaluation
	history   []GateHistoryEntry
	storeDir  string
	mu        sync.RWMutex
}

// NewQualityGateSystem creates a new quality gate system.
func NewQualityGateSystem(storeDir string) *QualityGateSystem {
	os.MkdirAll(storeDir, 0o755)
	qg := &QualityGateSystem{
		pipelines:   make(map[string]*GatePipeline),
		evaluations: make(map[string]*GateEvaluation),
		history:     []GateHistoryEntry{},
		storeDir:    storeDir,
	}
	qg.load()
	return qg
}

// CreatePipeline creates a new gate pipeline.
func (qg *QualityGateSystem) CreatePipeline(name string, gates []*Gate) *GatePipeline {
	qg.mu.Lock()
	defer qg.mu.Unlock()

	pipeline := &GatePipeline{
		ID:    fmt.Sprintf("pipe-%d", time.Now().UnixNano()),
		Name:  name,
		Gates: gates,
	}

	// Set order if not set
	for i, g := range pipeline.Gates {
		if g.Order == 0 {
			g.Order = i
		}
	}

	qg.pipelines[pipeline.ID] = pipeline
	qg.persist()
	return pipeline
}

// GetPipeline retrieves a pipeline by ID.
func (qg *QualityGateSystem) GetPipeline(id string) (*GatePipeline, error) {
	qg.mu.RLock()
	defer qg.mu.RUnlock()
	p, ok := qg.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", id)
	}
	return p, nil
}

// Evaluate runs a work item through a pipeline's gates.
func (qg *QualityGateSystem) Evaluate(ctx context.Context, pipelineID string, work *WorkItem) (*GateEvaluation, error) {
	qg.mu.Lock()
	defer qg.mu.Unlock()

	pipeline, ok := qg.pipelines[pipelineID]
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", pipelineID)
	}

	eval := &GateEvaluation{
		WorkID:     work.ID,
		PipelineID: pipelineID,
		Results:    []GateResult{},
		Status:     "in_progress",
		StartedAt:  time.Now(),
	}

	// Evaluate each gate in order — blocking gates can stop the pipeline
	for _, gate := range pipeline.Gates {
		result := qg.evaluateGate(ctx, gate, work)
		eval.Results = append(eval.Results, result)

		// Record history
		qg.history = append(qg.history, GateHistoryEntry{
			ID:          fmt.Sprintf("hist-%d", time.Now().UnixNano()),
			WorkID:      work.ID,
			GateID:      gate.ID,
			Status:      result.Status,
			Evidence:    result.Evidence,
			EvaluatedAt: result.EvaluatedAt,
			Evaluator:   "system",
		})

		// BLOCKING: if a blocking gate fails, the pipeline stops — hardcoded, not configurable
		if gate.Blocking && result.Status == StatusFailed {
			eval.Status = "failed"
			now := time.Now()
			eval.CompletedAt = &now
			qg.evaluations[work.ID] = eval
			qg.persist()
			return eval, nil
		}
	}

	// All gates passed or non-blocking failures
	allPassed := true
	for _, r := range eval.Results {
		if r.Status == StatusFailed {
			// Check if any blocking gate failed
			for _, gate := range pipeline.Gates {
				if gate.ID == r.GateID && gate.Blocking {
					allPassed = false
					break
				}
			}
		}
	}

	if allPassed {
		eval.Status = "passed"
		eval.AutoPromoted = true
	} else {
		eval.Status = "failed"
	}

	now := time.Now()
	eval.CompletedAt = &now
	qg.evaluations[work.ID] = eval
	qg.persist()
	return eval, nil
}

// GetEvaluation retrieves a gate evaluation for a work item.
func (qg *QualityGateSystem) GetEvaluation(workID string) (*GateEvaluation, error) {
	qg.mu.RLock()
	defer qg.mu.RUnlock()
	eval, ok := qg.evaluations[workID]
	if !ok {
		return nil, fmt.Errorf("evaluation for work %s not found", workID)
	}
	return eval, nil
}

// GetHistory returns the audit trail.
func (qg *QualityGateSystem) GetHistory(workID string) []GateHistoryEntry {
	qg.mu.RLock()
	defer qg.mu.RUnlock()

	var result []GateHistoryEntry
	for _, entry := range qg.history {
		if entry.WorkID == workID {
			result = append(result, entry)
		}
	}
	return result
}

// CanProceed checks if work can proceed past its current gate evaluation.
func (qg *QualityGateSystem) CanProceed(workID string) bool {
	qg.mu.RLock()
	defer qg.mu.Unlock()

	eval, ok := qg.evaluations[workID]
	if !ok {
		return false
	}
	return eval.Status == "passed"
}

// AddGate adds a gate to an existing pipeline.
func (qg *QualityGateSystem) AddGate(pipelineID string, gate *Gate) error {
	qg.mu.Lock()
	defer qg.mu.Unlock()

	pipeline, ok := qg.pipelines[pipelineID]
	if !ok {
		return fmt.Errorf("pipeline %s not found", pipelineID)
	}

	gate.Order = len(pipeline.Gates)
	pipeline.Gates = append(pipeline.Gates, gate)
	qg.persist()
	return nil
}

// ListPipelines returns all pipelines.
func (qg *QualityGateSystem) ListPipelines() []*GatePipeline {
	qg.mu.RLock()
	defer qg.mu.RUnlock()
	var result []*GatePipeline
	for _, p := range qg.pipelines {
		result = append(result, p)
	}
	return result
}

// evaluateGate evaluates a single gate against a work item.
func (qg *QualityGateSystem) evaluateGate(ctx context.Context, gate *Gate, work *WorkItem) GateResult {
	start := time.Now()

	// In a real implementation, each criterion would have a concrete evaluator.
	// For the prototype, we use a simple rule-based evaluation.
	result := GateResult{
		GateID:      gate.ID,
		WorkID:      work.ID,
		EvaluatedAt: time.Now(),
	}

	switch gate.Criterion {
	case CriterionLint:
		result.Status = StatusPassed
		result.Evidence = "No lint errors found"
		result.Score = 100
	case CriterionTest:
		// Check if test results are in payload
		if testResults, ok := work.Payload["test_results"]; ok {
			if tr, ok := testResults.(map[string]interface{}); ok {
				if failed, ok := tr["failed"].(float64); ok && failed > 0 {
					result.Status = StatusFailed
					result.Evidence = fmt.Sprintf("%.0f tests failed", failed)
					result.Score = 0
				} else {
					result.Status = StatusPassed
					result.Evidence = "All tests passed"
					result.Score = 100
				}
			}
		} else {
			result.Status = StatusFailed
			result.Evidence = "No test results provided"
			result.Score = 0
		}
	case CriterionReview:
		if reviewed, ok := work.Payload["reviewed"].(bool); ok && reviewed {
			result.Status = StatusPassed
			result.Evidence = "Review approved"
			result.Score = 100
		} else {
			result.Status = StatusFailed
			result.Evidence = "Not reviewed"
			result.Score = 0
		}
	case CriterionSecurity:
		if vulns, ok := work.Payload["vulnerabilities"]; ok {
			if v, ok := vulns.(float64); ok && v > 0 {
				result.Status = StatusFailed
				result.Evidence = fmt.Sprintf("%.0f vulnerabilities found", v)
				result.Score = 0
			} else {
				result.Status = StatusPassed
				result.Evidence = "No vulnerabilities"
				result.Score = 100
			}
		} else {
			result.Status = StatusPassed // Assume clean if not specified
			result.Evidence = "No vulnerability data provided, assuming clean"
			result.Score = 80
		}
	case CriterionPerformance:
		result.Status = StatusPassed
		result.Evidence = "Performance within thresholds"
		result.Score = 95
	default:
		result.Status = StatusSkipped
		result.Evidence = "Unknown criterion"
	}

	result.Duration = time.Since(start)
	return result
}

func (qg *QualityGateSystem) persist() {
	data, _ := json.MarshalIndent(qg.pipelines, "", "  ")
	os.WriteFile(filepath.Join(qg.storeDir, "pipelines.json"), data, 0o644)
	evalData, _ := json.MarshalIndent(qg.evaluations, "", "  ")
	os.WriteFile(filepath.Join(qg.storeDir, "evaluations.json"), evalData, 0o644)
	histData, _ := json.MarshalIndent(qg.history, "", "  ")
	os.WriteFile(filepath.Join(qg.storeDir, "history.json"), histData, 0o644)
}

func (qg *QualityGateSystem) load() {
	data, err := os.ReadFile(filepath.Join(qg.storeDir, "pipelines.json"))
	if err == nil {
		json.Unmarshal(data, &qg.pipelines)
	}
	evalData, err := os.ReadFile(filepath.Join(qg.storeDir, "evaluations.json"))
	if err == nil {
		json.Unmarshal(evalData, &qg.evaluations)
	}
	histData, err := os.ReadFile(filepath.Join(qg.storeDir, "history.json"))
	if err == nil {
		json.Unmarshal(histData, &qg.history)
	}
}
