// Package evidenceledger implements a cryptographically verifiable evidence
// chain for agent actions. Every claim an agent makes is backed by proof.
// Every action leaves a tamper-evident trace. Trust is earned through
// verifiable evidence, not assumed through reputation.
//
// Key invention: The Evidence Chain — a hash-chain structure where each
// agent action links to the previous one via cryptographic hash. You can
// verify the entire history of any agent, detect tampering, and audit
// any claim back to its source evidence.
package evidenceledger

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ClaimType represents the type of claim an agent makes.
type ClaimType string

const (
	ClaimCompleted   ClaimType = "completed"    // "I finished the task"
	ClaimTestPassed  ClaimType = "test_passed"  // "Tests pass"
	ClaimDeployed    ClaimType = "deployed"     // "Deployed successfully"
	ClaimResearched  ClaimType = "researched"   // "Researched 10 sources"
	ClaimReviewed    ClaimType = "reviewed"     // "Code reviewed and approved"
	ClaimFixed       ClaimType = "fixed"        // "Bug fixed"
	ClaimOptimized   ClaimType = "optimized"    // "Performance improved by X%"
	ClaimVerified    ClaimType = "verified"     // "Security audit passed"
	ClaimCost        ClaimType = "cost"         // "Spent $X on this task"
	ClaimGenerated   ClaimType = "generated"   // "Generated X lines of code"
)

// EvidenceType represents the type of evidence backing a claim.
type EvidenceType string

const (
	EvidenceOutput   EvidenceType = "output"    // Command output, logs
	EvidenceHash     EvidenceType = "hash"      // File hash, commit hash
	EvidenceURL      EvidenceType = "url"       // Link to external proof
	EvidenceMetric   EvidenceType = "metric"    // Numerical measurement
	EvidenceWitness  EvidenceType = "witness"   // Another agent's observation
	EvidenceScreenshot EvidenceType = "screenshot" // Visual proof
	EvidenceSignature EvidenceType = "signature" // Cryptographic signature
)

// EvidenceItem is a single piece of evidence backing a claim.
type EvidenceItem struct {
	Type      EvidenceType `json:"type"`
	Content   string       `json:"content"`
	Hash      string       `json:"hash,omitempty"`      // Hash of the content
	Source    string       `json:"source,omitempty"`     // Where this evidence came from
	Verified  bool         `json:"verified"`
	VerifiedBy string      `json:"verified_by,omitempty"` // Who verified it
	VerifiedAt *time.Time  `json:"verified_at,omitempty"`
}

// Claim represents an agent's claim with attached evidence.
type Claim struct {
	ID          string         `json:"id"`
	AgentID     string         `json:"agent_id"`
	TaskID      string         `json:"task_id"`
	Type        ClaimType      `json:"type"`
	Statement   string         `json:"statement"` // What the agent claims
	Evidence    []EvidenceItem `json:"evidence"`
	Verified    bool           `json:"verified"`
	TrustScore  float64        `json:"trust_score"` // 0-1, how trustworthy
	BlockHash   string         `json:"block_hash"`  // Hash of this block
	PrevHash    string         `json:"prev_hash"`   // Hash of previous block
	SequenceNum int64          `json:"sequence_num"`
	Timestamp   time.Time      `json:"timestamp"`
}

// VerificationStatus represents the outcome of verifying a claim.
type VerificationStatus string

const (
	StatusUnverified VerificationStatus = "unverified"
	StatusConfirmed  VerificationStatus = "confirmed"
	StatusRefuted    VerificationStatus = "refuted"
	StatusPartial    VerificationStatus = "partial"
	StatusExpired    VerificationStatus = "expired"
)

// VerificationResult records the outcome of verifying a claim.
type VerificationResult struct {
	ClaimID     string            `json:"claim_id"`
	VerifierID  string            `json:"verifier_id"`
	Status      VerificationStatus `json:"status"`
	Confidence  float64           `json:"confidence"` // 0-1
	Details     string            `json:"details"`
	ChecksRun   []string          `json:"checks_run"`
	EvidenceChecked int           `json:"evidence_checked"`
	EvidenceConfirmed int         `json:"evidence_confirmed"`
	Timestamp   time.Time         `json:"timestamp"`
}

// AuditReport is a comprehensive audit of an agent's claims.
type AuditReport struct {
	AgentID        string  `json:"agent_id"`
	TotalClaims    int     `json:"total_claims"`
	VerifiedClaims int     `json:"verified_claims"`
	RefutedClaims  int     `json:"refuted_claims"`
	UnverifiedClaims int   `json:"unverified_claims"`
	TrustScore     float64 `json:"trust_score"`
	ChainIntegrity bool    `json:"chain_integrity"`
	TamperDetected bool    `json:"tamper_detected"`
	TamperedBlocks []string `json:"tampered_blocks,omitempty"`
	GeneratedAt    time.Time `json:"generated_at"`
}

// EvidenceLedger is the main ledger for the trust verification chain.
type EvidenceLedger struct {
	chain    []Claim
	byAgent  map[string][]int // agent_id → indices into chain
	byTask   map[string][]int // task_id → indices into chain
	verifications map[string][]VerificationResult // claim_id → results
	storeDir string
	headHash string
	mu       sync.Mutex
}

// NewEvidenceLedger creates a new evidence ledger.
func NewEvidenceLedger(storeDir string) *EvidenceLedger {
	os.MkdirAll(storeDir, 0755)
	el := &EvidenceLedger{
		chain:         make([]Claim, 0),
		byAgent:       make(map[string][]int),
		byTask:        make(map[string][]int),
		verifications: make(map[string][]VerificationResult),
		storeDir:      storeDir,
	}
	el.load()
	return el
}

// SubmitClaim records a new claim with evidence.
func (el *EvidenceLedger) SubmitClaim(agentID, taskID string, claimType ClaimType, statement string, evidence []EvidenceItem) *Claim {
	el.mu.Lock()
	defer el.mu.Unlock()

	// Hash evidence items
	for i := range evidence {
		if evidence[i].Hash == "" && evidence[i].Content != "" {
			hash := sha256.Sum256([]byte(evidence[i].Content))
			evidence[i].Hash = fmt.Sprintf("%x", hash)
		}
	}

	claim := Claim{
		ID:          fmt.Sprintf("cl-%d", time.Now().UnixNano()),
		AgentID:     agentID,
		TaskID:      taskID,
		Type:        claimType,
		Statement:   statement,
		Evidence:    evidence,
		SequenceNum: int64(len(el.chain)),
		PrevHash:    el.headHash,
		Timestamp:   time.Now(),
	}

	// Compute block hash (hash of entire claim)
	claimData, _ := json.Marshal(claim)
	claim.BlockHash = fmt.Sprintf("%x", sha256.Sum256(claimData))

	el.chain = append(el.chain, claim)
	el.headHash = claim.BlockHash

	idx := len(el.chain) - 1
	el.byAgent[agentID] = append(el.byAgent[agentID], idx)
	el.byTask[taskID] = append(el.byTask[taskID], idx)

	el.persist()
	return &claim
}

// VerifyClaim verifies a specific claim by checking its evidence.
func (el *EvidenceLedger) VerifyClaim(claimID, verifierID string, checkFn func(Claim) (VerificationStatus, float64, string, []string)) VerificationResult {
	el.mu.Lock()
	defer el.mu.Unlock()

	var claim *Claim
	for i := range el.chain {
		if el.chain[i].ID == claimID {
			claim = &el.chain[i]
			break
		}
	}
	if claim == nil {
		return VerificationResult{
			ClaimID:    claimID,
			VerifierID: verifierID,
			Status:     StatusUnverified,
			Details:    "Claim not found",
			Timestamp:  time.Now(),
		}
	}

	status, confidence, details, checks := checkFn(*claim)

	result := VerificationResult{
		ClaimID:          claimID,
		VerifierID:       verifierID,
		Status:           status,
		Confidence:       confidence,
		Details:          details,
		ChecksRun:        checks,
		EvidenceChecked:  len(claim.Evidence),
		Timestamp:        time.Now(),
	}

	// Count confirmed evidence
	for _, e := range claim.Evidence {
		if e.Verified {
			result.EvidenceConfirmed++
		}
	}

	el.verifications[claimID] = append(el.verifications[claimID], result)

	// Update claim trust score
	if status == StatusConfirmed {
		claim.Verified = true
		claim.TrustScore = confidence
	} else if status == StatusRefuted {
		claim.TrustScore = 0
	}

	el.persist()
	return result
}

// AuditAgent produces a full audit report for an agent.
func (el *EvidenceLedger) AuditAgent(agentID string) *AuditReport {
	el.mu.Lock()
	defer el.mu.Unlock()

	report := &AuditReport{
		AgentID:     agentID,
		GeneratedAt: time.Now(),
	}

	indices, ok := el.byAgent[agentID]
	if !ok {
		return report
	}

	report.TotalClaims = len(indices)

	for _, idx := range indices {
		if idx >= len(el.chain) {
			continue
		}
		claim := el.chain[idx]

		if claim.Verified {
			report.VerifiedClaims++
		}

		// Check verification results
		if results, ok := el.verifications[claim.ID]; ok {
			for _, r := range results {
				if r.Status == StatusRefuted {
					report.RefutedClaims++
					break
				}
			}
		}
	}

	report.UnverifiedClaims = report.TotalClaims - report.VerifiedClaims - report.RefutedClaims

	// Check chain integrity
	report.ChainIntegrity, report.TamperDetected, report.TamperedBlocks = el.verifyChainIntegrity(agentID)

	// Calculate trust score
	if report.TotalClaims > 0 {
		report.TrustScore = float64(report.VerifiedClaims) / float64(report.TotalClaims) * 100
		if report.TamperDetected {
			report.TrustScore = 0
		}
	}

	return report
}

// AuditTask produces claims for a specific task.
func (el *EvidenceLedger) AuditTask(taskID string) []*Claim {
	el.mu.Lock()
	defer el.mu.Unlock()

	indices, ok := el.byTask[taskID]
	if !ok {
		return nil
	}

	var claims []*Claim
	for _, idx := range indices {
		if idx < len(el.chain) {
			claims = append(claims, &el.chain[idx])
		}
	}
	return claims
}

// VerifyChainIntegrity checks the entire chain for tampering.
func (el *EvidenceLedger) VerifyChainIntegrity() (bool, []string) {
	el.mu.Lock()
	defer el.mu.Unlock()

	integrity, _, tampered := el.verifyChainIntegrity("")
	return integrity, tampered
}

func (el *EvidenceLedger) verifyChainIntegrity(agentID string) (bool, bool, []string) {
	var chain []Claim
	if agentID != "" {
		indices, ok := el.byAgent[agentID]
		if !ok {
			return true, false, nil
		}
		for _, idx := range indices {
			if idx < len(el.chain) {
				chain = append(chain, el.chain[idx])
			}
		}
	} else {
		chain = el.chain
	}

	// Sort by sequence number
	sorted := make([]Claim, len(chain))
	copy(sorted, chain)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].SequenceNum > sorted[j].SequenceNum {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var tampered []string
	prevHash := ""

	for _, claim := range sorted {
		// Verify block hash
		claimCopy := claim
		claimCopy.BlockHash = ""
		data, _ := json.Marshal(claimCopy)
		expected := fmt.Sprintf("%x", sha256.Sum256(data))

		if claim.BlockHash != expected {
			tampered = append(tampered, claim.ID)
		}

		// Verify chain linkage
		if prevHash != "" && claim.PrevHash != prevHash {
			tampered = append(tampered, claim.ID)
		}

		prevHash = claim.BlockHash
	}

	return len(tampered) == 0, len(tampered) > 0, tampered
}

// SearchClaims finds claims matching a query.
func (el *EvidenceLedger) SearchClaims(query string, limit int) []*Claim {
	el.mu.Lock()
	defer el.mu.Unlock()

	query = strings.ToLower(query)
	var results []*Claim

	for i := range el.chain {
		c := &el.chain[i]
		if strings.Contains(strings.ToLower(c.Statement), query) ||
			strings.Contains(strings.ToLower(c.AgentID), query) ||
			strings.Contains(strings.ToLower(c.TaskID), query) ||
			string(c.Type) == query {
			results = append(results, c)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results
}

// AgentTrustScore computes a trust score for an agent based on evidence.
func (el *EvidenceLedger) AgentTrustScore(agentID string) float64 {
	el.mu.Lock()
	defer el.mu.Unlock()

	indices, ok := el.byAgent[agentID]
	if !ok || len(indices) == 0 {
		return 50.0 // Neutral starting score
	}

	var totalScore float64
	claimCount := 0

	for _, idx := range indices {
		if idx >= len(el.chain) {
			continue
		}
		claim := el.chain[idx]
		claimCount++

		// Base score from verification
		if claim.Verified {
			totalScore += claim.TrustScore * 100
		} else {
			// Unverified claims are neutral
			totalScore += 50.0
		}

		// Check if refuted
		if results, ok := el.verifications[claim.ID]; ok {
			for _, r := range results {
				if r.Status == StatusRefuted {
					totalScore -= 30.0 // Penalty for refuted claims
					break
				}
			}
		}

		// Evidence bonus: more evidence = more trustworthy
		totalScore += float64(len(claim.Evidence)) * 2.0
	}

	if claimCount == 0 {
		return 50.0
	}

	score := totalScore / float64(claimCount)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// ChainLength returns the total number of claims in the ledger.
func (el *EvidenceLedger) ChainLength() int {
	el.mu.Lock()
	defer el.mu.Unlock()
	return len(el.chain)
}

// GetClaim retrieves a claim by ID.
func (el *EvidenceLedger) GetClaim(id string) (*Claim, bool) {
	el.mu.Lock()
	defer el.mu.Unlock()

	for i := range el.chain {
		if el.chain[i].ID == id {
			return &el.chain[i], true
		}
	}
	return nil, false
}

func (el *EvidenceLedger) persist() {
	type ledgerData struct {
		Chain         []Claim                        `json:"chain"`
		Verifications map[string][]VerificationResult `json:"verifications"`
		HeadHash      string                         `json:"head_hash"`
	}
	data, _ := json.MarshalIndent(ledgerData{
		Chain: el.chain, Verifications: el.verifications, HeadHash: el.headHash,
	}, "", "  ")
	os.WriteFile(filepath.Join(el.storeDir, "evidence_ledger.json"), data, 0644)
}

func (el *EvidenceLedger) load() {
	data, err := os.ReadFile(filepath.Join(el.storeDir, "evidence_ledger.json"))
	if err != nil {
		return
	}
	var ld struct {
		Chain         []Claim                        `json:"chain"`
		Verifications map[string][]VerificationResult `json:"verifications"`
		HeadHash      string                         `json:"head_hash"`
	}
	if json.Unmarshal(data, &ld) == nil {
		el.chain = ld.Chain
		el.verifications = ld.Verifications
		el.headHash = ld.HeadHash

		// Rebuild indices
		for i, c := range el.chain {
			el.byAgent[c.AgentID] = append(el.byAgent[c.AgentID], i)
			el.byTask[c.TaskID] = append(el.byTask[c.TaskID], i)
		}
	}
}
