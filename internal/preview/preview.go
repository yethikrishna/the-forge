// Package preview generates action previews for destructive operations.
// Shows the plan, lets the user approve/modify/reject before execution.
//
// Look before you leap.
package preview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ActionType is the type of destructive action.
type ActionType string

const (
	TypeFileDelete  ActionType = "file_delete"
	TypeFileWrite   ActionType = "file_write"
	TypeFileMove    ActionType = "file_move"
	TypeCommandExec ActionType = "command_exec"
	TypeBulkChange  ActionType = "bulk_change"
)

// Impact represents the estimated impact of an action.
type Impact string

const (
	ImpactLow         Impact = "low"
	ImpactMedium      Impact = "medium"
	ImpactHigh        Impact = "high"
	ImpactDestructive Impact = "destructive"
)

// Change represents a single file/line change in the preview.
type Change struct {
	File    string `json:"file"`
	Old     string `json:"old,omitempty"`
	New     string `json:"new,omitempty"`
	Line    int    `json:"line,omitempty"`
	Added   bool   `json:"added"`
	Removed bool   `json:"removed"`
}

// Plan is a preview of a proposed action.
type Plan struct {
	ID           string     `json:"id"`
	AgentID      string     `json:"agent_id"`
	Type         ActionType `json:"type"`
	Target       string     `json:"target"`
	Description  string     `json:"description"`
	Impact       Impact     `json:"impact"`
	Changes      []Change   `json:"changes"`
	Reversible   bool       `json:"reversible"`
	BackupPath   string     `json:"backup_path,omitempty"`
	Risks        []string   `json:"risks"`
	Alternatives []string   `json:"alternatives,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	Status       string     `json:"status"` // pending, approved, rejected, modified, expired
	ApprovedBy   string     `json:"approved_by,omitempty"`
	Reason       string     `json:"reason,omitempty"`
}

// Previewer generates action previews.
type Previewer struct {
	plans    map[string]*Plan
	backups  map[string]string // plan ID -> backup path
	storeDir string
	nextID   int
	mu       sync.RWMutex
}

// NewPreviewer creates an action previewer.
func NewPreviewer(storeDir string) *Previewer {
	p := &Previewer{
		plans:    make(map[string]*Plan),
		backups:  make(map[string]string),
		storeDir: storeDir,
	}
	if storeDir != "" {
		os.MkdirAll(storeDir, 0755)
		p.load()
	}
	return p
}

// Create creates a preview plan for a proposed action.
func (p *Previewer) Create(agentID string, actionType ActionType, target, description string) (*Plan, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.nextID++
	plan := &Plan{
		ID:           fmt.Sprintf("plan-%d", p.nextID),
		AgentID:      agentID,
		Type:         actionType,
		Target:       target,
		Description:  description,
		Impact:       assessImpact(actionType, target),
		Reversible:   isReversible(actionType),
		Risks:        assessRisks(actionType, target),
		Alternatives: suggestAlternatives(actionType, target),
		CreatedAt:    time.Now(),
		Status:       "pending",
	}

	// For file operations, create a backup
	if actionType == TypeFileWrite || actionType == TypeFileDelete {
		if data, err := os.ReadFile(target); err == nil {
			backupDir := filepath.Join(p.storeDir, "backups")
			os.MkdirAll(backupDir, 0755)
			backupPath := filepath.Join(backupDir, fmt.Sprintf("%s.%d.bak", filepath.Base(target), time.Now().Unix()))
			os.WriteFile(backupPath, data, 0644)
			plan.BackupPath = backupPath
			p.backups[plan.ID] = backupPath
		}
	}

	p.plans[plan.ID] = plan
	p.save()
	return plan, nil
}

// Approve approves a plan.
func (p *Previewer) Approve(planID, approver, reason string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	plan, ok := p.plans[planID]
	if !ok {
		return fmt.Errorf("plan %q not found", planID)
	}
	if plan.Status != "pending" {
		return fmt.Errorf("plan is %s, not pending", plan.Status)
	}

	plan.Status = "approved"
	plan.ApprovedBy = approver
	plan.Reason = reason
	p.save()
	return nil
}

// Reject rejects a plan.
func (p *Previewer) Reject(planID, reason string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	plan, ok := p.plans[planID]
	if !ok {
		return fmt.Errorf("plan %q not found", planID)
	}

	plan.Status = "rejected"
	plan.Reason = reason
	// Clean up backup
	if backupPath, ok := p.backups[planID]; ok && backupPath != "" {
		os.Remove(backupPath)
		delete(p.backups, planID)
	}
	p.save()
	return nil
}

// Modify updates a plan with modifications.
func (p *Previewer) Modify(planID string, changes []Change) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	plan, ok := p.plans[planID]
	if !ok {
		return fmt.Errorf("plan %q not found", planID)
	}

	plan.Changes = changes
	plan.Status = "modified"
	p.save()
	return nil
}

// Get returns a plan by ID.
func (p *Previewer) Get(planID string) (*Plan, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	plan, ok := p.plans[planID]
	if !ok {
		return nil, false
	}
	copy := *plan
	return &copy, true
}

// ListPending returns all pending plans.
func (p *Previewer) ListPending() []Plan {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []Plan
	for _, plan := range p.plans {
		if plan.Status == "pending" {
			result = append(result, *plan)
		}
	}
	return result
}

// RestoreBackup restores a file from a plan's backup.
func (p *Previewer) RestoreBackup(planID string) error {
	p.mu.RLock()
	backupPath, ok := p.backups[planID]
	p.mu.RUnlock()

	if !ok || backupPath == "" {
		return fmt.Errorf("no backup for plan %s", planID)
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	plan, ok := p.plans[planID]
	if !ok {
		return fmt.Errorf("plan not found")
	}

	return os.WriteFile(plan.Target, data, 0644)
}

func assessImpact(actionType ActionType, target string) Impact {
	switch actionType {
	case TypeFileDelete:
		if strings.HasPrefix(target, "/etc") || strings.HasPrefix(target, "/var") {
			return ImpactDestructive
		}
		return ImpactHigh
	case TypeCommandExec:
		if strings.Contains(target, "rm") || strings.Contains(target, "format") {
			return ImpactDestructive
		}
		return ImpactMedium
	case TypeFileWrite:
		return ImpactMedium
	case TypeBulkChange:
		return ImpactHigh
	default:
		return ImpactLow
	}
}

func isReversible(actionType ActionType) bool {
	switch actionType {
	case TypeFileDelete:
		return false
	case TypeCommandExec:
		return false
	default:
		return true
	}
}

func assessRisks(actionType ActionType, target string) []string {
	var risks []string

	switch actionType {
	case TypeFileDelete:
		risks = append(risks, "Data loss — deleted files cannot be recovered without backup")
		if strings.HasSuffix(target, ".go") {
			risks = append(risks, "May break compilation if other files depend on this")
		}
	case TypeFileWrite:
		risks = append(risks, "Overwrites existing content")
	case TypeCommandExec:
		risks = append(risks, "Command execution has side effects")
		if strings.Contains(target, "sudo") {
			risks = append(risks, "Elevated privileges required")
		}
	case TypeBulkChange:
		risks = append(risks, "Multiple files affected — review all changes")
	}

	return risks
}

func suggestAlternatives(actionType ActionType, target string) []string {
	switch actionType {
	case TypeFileDelete:
		return []string{
			"Move to trash instead of permanent delete",
			"Archive the file with timestamp suffix",
		}
	case TypeFileWrite:
		return []string{
			"Create a new file instead of overwriting",
			"Write to a temporary file first, then verify",
		}
	case TypeCommandExec:
		return []string{
			"Run with --dry-run flag first",
			"Test in a sandboxed environment",
		}
	default:
		return nil
	}
}

func (p *Previewer) save() {
	if p.storeDir == "" {
		return
	}
	data, _ := json.MarshalIndent(p.plans, "", "  ")
	os.WriteFile(filepath.Join(p.storeDir, "plans.json"), data, 0644)
}

func (p *Previewer) load() {
	data, err := os.ReadFile(filepath.Join(p.storeDir, "plans.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &p.plans)
}

// FormatPlan formats a plan for display.
func FormatPlan(plan *Plan) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Plan:     %s\n", plan.ID))
	b.WriteString(fmt.Sprintf("Agent:    %s\n", plan.AgentID))
	b.WriteString(fmt.Sprintf("Action:   %s\n", plan.Type))
	b.WriteString(fmt.Sprintf("Target:   %s\n", plan.Target))
	b.WriteString(fmt.Sprintf("Impact:   %s\n", plan.Impact))
	b.WriteString(fmt.Sprintf("Desc:     %s\n", plan.Description))
	b.WriteString(fmt.Sprintf("Reversible: %v\n", plan.Reversible))
	b.WriteString(fmt.Sprintf("Status:   %s\n", plan.Status))

	if plan.BackupPath != "" {
		b.WriteString(fmt.Sprintf("Backup:   %s\n", plan.BackupPath))
	}

	if len(plan.Risks) > 0 {
		b.WriteString("Risks:\n")
		for _, r := range plan.Risks {
			b.WriteString(fmt.Sprintf("  ⚠ %s\n", r))
		}
	}

	if len(plan.Alternatives) > 0 {
		b.WriteString("Alternatives:\n")
		for _, a := range plan.Alternatives {
			b.WriteString(fmt.Sprintf("  → %s\n", a))
		}
	}

	if len(plan.Changes) > 0 {
		b.WriteString("Changes:\n")
		for _, c := range plan.Changes {
			prefix := "  +"
			if c.Removed {
				prefix = "  -"
			}
			b.WriteString(fmt.Sprintf("%s %s:%d %s\n", prefix, c.File, c.Line, c.Old+c.New))
		}
	}

	return b.String()
}
