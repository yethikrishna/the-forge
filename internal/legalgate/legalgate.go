// Package legalgate implements compliance gates for regulated actions.
// Regulated actions require legal division sign-off before execution.
// The system enforces policy, not just suggests it.
//
// Key invention: The Policy Engine — a rule-based system where every action
// is classified by risk level, checked against policies, and either approved,
// blocked, or escalated. The legal division has veto power over any action
// that creates liability. This is NOT a suggestion system — it's enforcement.
package legalgate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RiskLevel classifies the risk of an action.
type RiskLevel string

const (
	RiskNone      RiskLevel = "none"       // No risk, auto-approve
	RiskLow       RiskLevel = "low"        // Low risk, log and approve
	RiskMedium    RiskLevel = "medium"     // Medium risk, notify legal
	RiskHigh      RiskLevel = "high"       // High risk, requires legal approval
	RiskCritical  RiskLevel = "critical"   // Critical risk, requires legal + human approval
)

// ActionDomain classifies what domain an action belongs to.
type ActionDomain string

const (
	DomainDataHandling    ActionDomain = "data_handling"    // Collecting, storing, sharing data
	DomainCommunication   ActionDomain = "communication"    // Sending emails, posting, publishing
	DomainFinancial       ActionDomain = "financial"        // Payments, pricing, contracts
	DomainCodeDeployment  ActionDomain = "code_deployment"  // Deploying code to production
	DomainThirdParty      ActionDomain = "third_party"      // Integrating with external services
	DomainIP              ActionDomain = "ip"               // IP, copyright, patents
	DomainCustomerData    ActionDomain = "customer_data"    // Accessing customer data
	DomainPublicFacing    ActionDomain = "public_facing"    // Anything visible to public
	DomainEmployment      ActionDomain = "employment"       // Hiring, firing, HR
	DomainRegulatory      ActionDomain = "regulatory"       // Regulatory filings, compliance
)

// GateDecision represents the gate's decision on an action.
type GateDecision string

const (
	DecisionApproved   GateDecision = "approved"    // Action allowed
	DecisionBlocked    GateDecision = "blocked"     // Action denied
	DecisionEscalated  GateDecision = "escalated"   // Requires human/legal review
	DecisionDeferred   GateDecision = "deferred"    // Needs more information
	DecisionExempted   GateDecision = "exempted"    // Exempted from review (emergency)
)

// PolicyRule defines a compliance rule.
type PolicyRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Domain      ActionDomain `json:"domain"`
	RiskLevel   RiskLevel    `json:"risk_level"`
	Condition   string       `json:"condition"`   // Natural language condition
	Keywords    []string     `json:"keywords"`    // Keywords that trigger this rule
	Blocked     bool         `json:"blocked"`      // Hard block vs soft block
	RequiresApproval string  `json:"requires_approval"` // Who must approve: "legal", "human", "division_head"
	Exemptions  []string     `json:"exemptions"`  // Conditions under which rule doesn't apply
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ComplianceAction represents an action being evaluated for compliance.
type ComplianceAction struct {
	ID          string            `json:"id"`
	AgentID     string            `json:"agent_id"`
	Division    string            `json:"division"`
	Domain      ActionDomain      `json:"domain"`
	Description string            `json:"description"`
	Target      string            `json:"target"`       // What's being affected
	DataInvolved []string         `json:"data_involved"` // Types of data involved
	IsPublic    bool              `json:"is_public"`     // Public-facing?
	IsCustomer  bool              `json:"is_customer"`   // Involves customers?
	IsFinancial bool              `json:"is_financial"`  // Involves money?
	Tags        []string          `json:"tags"`
	Context     map[string]string `json:"context"`
}

// GateResult records the outcome of a compliance gate check.
type GateResult struct {
	ActionID    string        `json:"action_id"`
	Decision    GateDecision  `json:"decision"`
	RiskLevel   RiskLevel     `json:"risk_level"`
	MatchedRules []string     `json:"matched_rules"` // PolicyRule IDs that matched
	Reasons     []string      `json:"reasons"`
	RequiredApprovals []string `json:"required_approvals"`
	ApprovedBy  []Approval    `json:"approved_by"`
	ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

// Approval records who approved an action.
type Approval struct {
	ApproverID string    `json:"approver_id"`
	Role       string    `json:"role"` // "legal", "human", "division_head"
	Decision   GateDecision `json:"decision"`
	Notes      string    `json:"notes"`
	Timestamp  time.Time `json:"timestamp"`
}

// AuditEntry records a compliance audit event.
type AuditEntry struct {
	ID        string    `json:"id"`
	ActionID  string    `json:"action_id"`
	AgentID   string    `json:"agent_id"`
	EventType string    `json:"event_type"` // "gate_check", "approval", "block", "escalation"
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

// LegalGate is the main compliance gate system.
type LegalGate struct {
	rules      map[string]*PolicyRule
	pending    map[string]*GateResult // action_id → pending results
	audit      []AuditEntry
	approvals  map[string][]Approval // action_id → approvals
	storeDir   string
	mu         sync.Mutex
}

// NewLegalGate creates a new legal gate system with default policies.
func NewLegalGate(storeDir string) *LegalGate {
	os.MkdirAll(storeDir, 0755)
	lg := &LegalGate{
		rules:     make(map[string]*PolicyRule),
		pending:   make(map[string]*GateResult),
		audit:     make([]AuditEntry, 0),
		approvals: make(map[string][]Approval),
		storeDir:  storeDir,
	}
	lg.loadDefaults()
	lg.load()
	return lg
}

// loadDefaults loads sensible default policies.
func (lg *LegalGate) loadDefaults() {
	defaults := []PolicyRule{
		{
			ID: "POL-001", Name: "GDPR Data Collection", Description: "Collecting personal data requires consent documentation",
			Domain: DomainCustomerData, RiskLevel: RiskHigh, Condition: "Any collection of personally identifiable data",
			Keywords: []string{"email", "name", "address", "phone", "personal", "PII", "GDPR"},
			Blocked: false, RequiresApproval: "legal",
		},
		{
			ID: "POL-002", Name: "Public Communication Review", Description: "Public-facing communications must be reviewed before publication",
			Domain: DomainPublicFacing, RiskLevel: RiskHigh, Condition: "Any content visible to the public",
			Keywords: []string{"publish", "post", "tweet", "blog", "press", "public", "website"},
			Blocked: false, RequiresApproval: "legal",
		},
		{
			ID: "POL-003", Name: "Financial Commitment Gate", Description: "Any financial commitment over $100 requires approval",
			Domain: DomainFinancial, RiskLevel: RiskCritical, Condition: "Payments or financial commitments exceeding threshold",
			Keywords: []string{"pay", "purchase", "subscribe", "contract", "commit", "$", "price", "cost"},
			Blocked: true, RequiresApproval: "human",
		},
		{
			ID: "POL-004", Name: "Third-Party Integration Review", Description: "Integrating with external services requires security review",
			Domain: DomainThirdParty, RiskLevel: RiskMedium, Condition: "Connecting to any external API or service",
			Keywords: []string{"integrate", "API", "connect", "webhook", "OAuth", "external", "third-party"},
			Blocked: false, RequiresApproval: "division_head",
		},
		{
			ID: "POL-005", Name: "IP Contamination Prevention", Description: "Code generation must not reproduce copyrighted material",
			Domain: DomainIP, RiskLevel: RiskHigh, Condition: "Any code that might reproduce copyrighted or licensed material",
			Keywords: []string{"copy", "reproduce", "license", "copyright", "GPL", "MIT", "patent", "trademark"},
			Blocked: true, RequiresApproval: "legal",
		},
		{
			ID: "POL-006", Name: "Data Exfiltration Prevention", Description: "Bulk data transfers to external parties are blocked",
			Domain: DomainDataHandling, RiskLevel: RiskCritical, Condition: "Transferring large amounts of data externally",
			Keywords: []string{"export", "download", "transfer", "send", "bulk", "dump", "backup external"},
			Blocked: true, RequiresApproval: "human",
		},
		{
			ID: "POL-007", Name: "Production Deployment Review", Description: "Production deployments require code review and test verification",
			Domain: DomainCodeDeployment, RiskLevel: RiskMedium, Condition: "Deploying code to production environment",
			Keywords: []string{"deploy", "release", "production", "prod", "ship", "launch"},
			Blocked: false, RequiresApproval: "division_head",
		},
		{
			ID: "POL-008", Name: "Customer Data Access Log", Description: "Accessing customer data must be logged and justified",
			Domain: DomainCustomerData, RiskLevel: RiskMedium, Condition: "Any access to customer data",
			Keywords: []string{"customer", "user data", "database", "query", "lookup", "access"},
			Blocked: false, RequiresApproval: "division_head",
		},
		{
			ID: "POL-009", Name: "Regulatory Filing Gate", Description: "Regulatory filings require legal sign-off",
			Domain: DomainRegulatory, RiskLevel: RiskCritical, Condition: "Any regulatory filing or compliance report",
			Keywords: []string{"file", "report", "SEC", "FDA", "GDPR", "compliance", "regulatory", "audit"},
			Blocked: true, RequiresApproval: "legal",
		},
		{
			ID: "POL-010", Name: "Contract Execution Gate", Description: "Contracts and legal agreements require legal review",
			Domain: DomainFinancial, RiskLevel: RiskCritical, Condition: "Entering into any legal agreement or contract",
			Keywords: []string{"contract", "agreement", "sign", "terms", "SLA", "NDA", "commitment"},
			Blocked: true, RequiresApproval: "legal",
		},
	}

	now := time.Now()
	for i := range defaults {
		defaults[i].CreatedAt = now
		defaults[i].UpdatedAt = now
		lg.rules[defaults[i].ID] = &defaults[i]
	}
}

// Check evaluates an action against all policies.
func (lg *LegalGate) Check(action ComplianceAction) *GateResult {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	result := &GateResult{
		ActionID:  action.ID,
		RiskLevel: RiskNone,
		CreatedAt: time.Now(),
	}

	var matchedRules []*PolicyRule
	description := strings.ToLower(action.Description)
	target := strings.ToLower(action.Target)

	// Check each rule
	for _, rule := range lg.rules {
		matched := false

		// Domain match
		if rule.Domain == action.Domain {
			matched = true
		}

		// Keyword match
		for _, kw := range rule.Keywords {
			if strings.Contains(description, strings.ToLower(kw)) || strings.Contains(target, strings.ToLower(kw)) {
				matched = true
				break
			}
		}

		// Public-facing check
		if action.IsPublic && rule.Domain == DomainPublicFacing {
			matched = true
		}

		// Customer data check
		if action.IsCustomer && rule.Domain == DomainCustomerData {
			matched = true
		}

		// Financial check
		if action.IsFinancial && rule.Domain == DomainFinancial {
			matched = true
		}

		if matched {
			matchedRules = append(matchedRules, rule)
			result.MatchedRules = append(result.MatchedRules, rule.ID)
		}
	}

	// Determine overall risk level (highest wins)
	for _, rule := range matchedRules {
		if compareRisk(rule.RiskLevel, result.RiskLevel) > 0 {
			result.RiskLevel = rule.RiskLevel
		}
	}

	// Determine decision
	if len(matchedRules) == 0 {
		result.Decision = DecisionApproved
		result.Reasons = append(result.Reasons, "No policy rules matched — auto-approved")
	} else {
		// Check for hard blocks
		blocked := false
		for _, rule := range matchedRules {
			if rule.Blocked {
				blocked = true
				result.RequiredApprovals = append(result.RequiredApprovals, rule.RequiresApproval)
				result.Reasons = append(result.Reasons, fmt.Sprintf("Blocked by %s: %s", rule.ID, rule.Name))
			}
		}

		if blocked {
			result.Decision = DecisionBlocked
		} else {
			// Determine if escalation is needed
			switch result.RiskLevel {
			case RiskLow:
				result.Decision = DecisionApproved
				result.Reasons = append(result.Reasons, "Low risk — approved with logging")
			case RiskMedium:
				result.Decision = DecisionEscalated
				for _, rule := range matchedRules {
					if rule.RequiresApproval != "" {
						result.RequiredApprovals = append(result.RequiredApprovals, rule.RequiresApproval)
					}
				}
				result.Reasons = append(result.Reasons, "Medium risk — escalation required")
			case RiskHigh, RiskCritical:
				result.Decision = DecisionBlocked
				for _, rule := range matchedRules {
					result.RequiredApprovals = append(result.RequiredApprovals, rule.RequiresApproval)
				}
				result.Reasons = append(result.Reasons, "High/critical risk — requires approval before execution")
			}
		}
	}

	// Log audit event
	lg.audit = append(lg.audit, AuditEntry{
		ID:        fmt.Sprintf("aud-%d", time.Now().UnixNano()),
		ActionID:  action.ID,
		AgentID:   action.AgentID,
		EventType: "gate_check",
		Details:   fmt.Sprintf("Decision: %s, Risk: %s, Rules: %v", result.Decision, result.RiskLevel, result.MatchedRules),
		Timestamp: time.Now(),
	})

	// Store pending if not approved
	if result.Decision != DecisionApproved {
		lg.pending[action.ID] = result
	}

	lg.persist()
	return result
}

// Approve records an approval for a pending action.
func (lg *LegalGate) Approve(actionID, approverID, role, notes string) (*GateResult, error) {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	result, ok := lg.pending[actionID]
	if !ok {
		return nil, fmt.Errorf("no pending action %s", actionID)
	}

	approval := Approval{
		ApproverID: approverID,
		Role:       role,
		Decision:   DecisionApproved,
		Notes:      notes,
		Timestamp:  time.Now(),
	}

	lg.approvals[actionID] = append(lg.approvals[actionID], approval)
	result.ApprovedBy = append(result.ApprovedBy, approval)

	// Check if all required approvals are satisfied
	allApproved := true
	for _, req := range result.RequiredApprovals {
		found := false
		for _, a := range result.ApprovedBy {
			if a.Role == req {
				found = true
				break
			}
		}
		if !found {
			allApproved = false
		}
	}

	if allApproved {
		now := time.Now()
		result.Decision = DecisionApproved
		result.ResolvedAt = &now
		delete(lg.pending, actionID)
	}

	lg.audit = append(lg.audit, AuditEntry{
		ID:        fmt.Sprintf("aud-%d", time.Now().UnixNano()),
		ActionID:  actionID,
		EventType: "approval",
		Details:   fmt.Sprintf("Approved by %s (%s): %s", approverID, role, notes),
		Timestamp: time.Now(),
	})

	lg.persist()
	return result, nil
}

// Reject rejects a pending action.
func (lg *LegalGate) Reject(actionID, rejecterID, role, reason string) (*GateResult, error) {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	result, ok := lg.pending[actionID]
	if !ok {
		return nil, fmt.Errorf("no pending action %s", actionID)
	}

	now := time.Now()
	result.Decision = DecisionBlocked
	result.ResolvedAt = &now

	lg.approvals[actionID] = append(lg.approvals[actionID], Approval{
		ApproverID: rejecterID,
		Role:       role,
		Decision:   DecisionBlocked,
		Notes:      reason,
		Timestamp:  now,
	})

	delete(lg.pending, actionID)

	lg.audit = append(lg.audit, AuditEntry{
		ID:        fmt.Sprintf("aud-%d", time.Now().UnixNano()),
		ActionID:  actionID,
		EventType: "block",
		Details:   fmt.Sprintf("Rejected by %s (%s): %s", rejecterID, role, reason),
		Timestamp: now,
	})

	lg.persist()
	return result, nil
}

// AddPolicy adds a new compliance policy.
func (lg *LegalGate) AddPolicy(rule PolicyRule) {
	lg.mu.Lock()
	defer lg.mu.Unlock()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	lg.rules[rule.ID] = &rule
	lg.persist()
}

// RemovePolicy removes a compliance policy.
func (lg *LegalGate) RemovePolicy(ruleID string) {
	lg.mu.Lock()
	defer lg.mu.Unlock()
	delete(lg.rules, ruleID)
	lg.persist()
}

// PendingActions returns all actions awaiting approval.
func (lg *LegalGate) PendingActions() []*GateResult {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	var results []*GateResult
	for _, r := range lg.pending {
		results = append(results, r)
	}
	return results
}

// AuditLog returns the compliance audit log.
func (lg *LegalGate) AuditLog(limit int) []AuditEntry {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	if limit > 0 && len(lg.audit) > limit {
		return lg.audit[len(lg.audit)-limit:]
	}
	return lg.audit
}

// PolicyList returns all active policies.
func (lg *LegalGate) PolicyList() []*PolicyRule {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	var rules []*PolicyRule
	for _, r := range lg.rules {
		rules = append(rules, r)
	}
	return rules
}

// ExemptAction grants an emergency exemption for an action.
func (lg *LegalGate) ExemptAction(actionID, granterID, reason string) (*GateResult, error) {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	result, ok := lg.pending[actionID]
	if !ok {
		return nil, fmt.Errorf("no pending action %s", actionID)
	}

	now := time.Now()
	result.Decision = DecisionExempted
	result.ResolvedAt = &now
	delete(lg.pending, actionID)

	lg.audit = append(lg.audit, AuditEntry{
		ID:        fmt.Sprintf("aud-%d", time.Now().UnixNano()),
		ActionID:  actionID,
		EventType: "exemption",
		Details:   fmt.Sprintf("Emergency exemption granted by %s: %s", granterID, reason),
		Timestamp: now,
	})

	lg.persist()
	return result, nil
}

// compareRisk returns >0 if a is higher risk than b.
func compareRisk(a, b RiskLevel) int {
	riskOrder := map[RiskLevel]int{
		RiskNone:     0,
		RiskLow:      1,
		RiskMedium:   2,
		RiskHigh:     3,
		RiskCritical: 4,
	}
	return riskOrder[a] - riskOrder[b]
}

func (lg *LegalGate) persist() {
	type data struct {
		Rules     map[string]*PolicyRule `json:"rules"`
		Pending   map[string]*GateResult `json:"pending"`
		Audit     []AuditEntry           `json:"audit"`
		Approvals map[string][]Approval  `json:"approvals"`
	}
	d := data{Rules: lg.rules, Pending: lg.pending, Audit: lg.audit, Approvals: lg.approvals}
	bytes, _ := json.MarshalIndent(d, "", "  ")
	os.WriteFile(filepath.Join(lg.storeDir, "legal_gate.json"), bytes, 0644)
}

func (lg *LegalGate) load() {
	data, err := os.ReadFile(filepath.Join(lg.storeDir, "legal_gate.json"))
	if err != nil {
		return
	}
	var d struct {
		Rules     map[string]*PolicyRule `json:"rules"`
		Pending   map[string]*GateResult `json:"pending"`
		Audit     []AuditEntry           `json:"audit"`
		Approvals map[string][]Approval  `json:"approvals"`
	}
	if json.Unmarshal(data, &d) == nil {
		if len(d.Rules) > 0 {
			lg.rules = d.Rules
		}
		lg.pending = d.Pending
		if d.Audit != nil {
			lg.audit = d.Audit
		}
		lg.approvals = d.Approvals
	}
}
