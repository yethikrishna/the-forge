// Package security provides blast radius analysis, supply chain vetting,
// data loss prevention, and adversarial red-team testing. It closes the gap
// where organizations ship fast but don't understand what a compromise would
// touch, whether their dependencies are safe, whether data is leaking, or
// whether an attacker could break in. Every component has a known blast radius;
// every dependency is vetted; every exfiltration path is scanned; every system
// is red-teamed.
package security

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

// Severity levels for findings.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// BlastRadius represents the impact scope of a compromised component.
type BlastRadius struct {
	ID              string   `json:"id"`
	ComponentID     string   `json:"component_id"`
	ComponentType   string   `json:"component_type"` // "service", "database", "agent", "api"
	DirectImpact    []string `json:"direct_impact"`   // directly affected systems
	IndirectImpact  []string `json:"indirect_impact"` // downstream dependencies
	DataExposed     []string `json:"data_exposed"`    // data types at risk
	UsersAffected   int64    `json:"users_affected"`
	EstimatedCost   float64  `json:"estimated_cost"`  // USD
	RiskScore       float64  `json:"risk_score"`      // 0-1
	Mitigations     []string `json:"mitigations,omitempty"`
	AnalyzedAt      time.Time `json:"analyzed_at"`
}

// SupplyChainVuln represents a vulnerability in a dependency.
type SupplyChainVuln struct {
	ID           string   `json:"id"`
	Package      string   `json:"package"`
	Version      string   `json:"version"`
	Vulnerability string  `json:"vulnerability"` // CVE or description
	Severity     Severity `json:"severity"`
	FixedIn      string   `json:"fixed_in,omitempty"`
	Score        float64  `json:"score"` // CVSS-like 0-10
	Transitive   bool     `json:"transitive"` // is it a transitive dep?
	DetectedAt   time.Time `json:"detected_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// DLPPolicy is a data loss prevention rule.
type DLPPolicy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Pattern     string   `json:"pattern"`     // regex or keyword pattern
	Category    string   `json:"category"`    // "pii", "financial", "credentials", "ip"
	Action      string   `json:"action"`      // "block", "warn", "log"
	Enabled     bool     `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// DLPScanResult is the result of scanning for exfiltration.
type DLPScanResult struct {
	ID         string    `json:"id"`
	PolicyID   string    `json:"policy_id"`
	Source     string    `json:"source"` // what was scanned
	MatchType  string    `json:"match_type"`
	MatchValue string    `json:"match_value"` // the detected content (masked)
	Category   string    `json:"category"`
	Action     string    `json:"action"`
	ScannedAt  time.Time `json:"scanned_at"`
}

// RedTeamResult captures an adversarial test result.
type RedTeamResult struct {
	ID            string   `json:"id"`
	Target        string   `json:"target"` // what was tested
	TestType      string   `json:"test_type"` // "prompt_injection", "data_exfil", "privilege_escalation", "auth_bypass"
	Success       bool     `json:"success"` // did the attack succeed?
	Severity      Severity `json:"severity"`
	Description   string   `json:"description"`
	Reproduction  string   `json:"reproduction,omitempty"`
	Mitigation    string   `json:"mitigation,omitempty"`
	TestedAt      time.Time `json:"tested_at"`
}

// SecurityPosture is an overall security assessment.
type SecurityPosture struct {
	ID               string  `json:"id"`
	Score            float64 `json:"score"` // 0-100
	OpenVulns        int     `json:"open_vulns"`
	CriticalVulns    int     `json:"critical_vulns"`
	UnvettedDeps     int     `json:"unvetted_deps"`
	RedTeamFailRate  float64 `json:"red_team_fail_rate"` // 0-1
	DLPViolations    int     `json:"dlp_violations"`
	AssessedAt       time.Time `json:"assessed_at"`
}

// SecurityHub manages security analysis and testing.
type SecurityHub struct {
	mu          sync.RWMutex
	blastRadii  map[string]*BlastRadius
	vulns       map[string]*SupplyChainVuln
	dlpPolicies map[string]*DLPPolicy
	dlpResults  map[string]*DLPScanResult
	redTeam     map[string]*RedTeamResult
	postures    map[string]*SecurityPosture
	path        string
}

// NewSecurityHub creates a new SecurityHub store.
func NewSecurityHub(persistPath string) *SecurityHub {
	sh := &SecurityHub{
		blastRadii:  make(map[string]*BlastRadius),
		vulns:       make(map[string]*SupplyChainVuln),
		dlpPolicies: make(map[string]*DLPPolicy),
		dlpResults:  make(map[string]*DLPScanResult),
		redTeam:     make(map[string]*RedTeamResult),
		postures:    make(map[string]*SecurityPosture),
		path:        persistPath,
	}
	sh.load()
	return sh
}

// --- Blast Radius ---

// AnalyzeBlastRadius computes the blast radius of a compromised component.
func (sh *SecurityHub) AnalyzeBlastRadius(componentID, componentType string, directImpact, indirectImpact, dataExposed []string, usersAffected int64, estimatedCost float64) (*BlastRadius, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	riskScore := 0.0
	if len(directImpact) > 0 {
		riskScore += 0.3
	}
	if len(indirectImpact) > 3 {
		riskScore += 0.3
	} else if len(indirectImpact) > 0 {
		riskScore += 0.15
	}
	if len(dataExposed) > 2 {
		riskScore += 0.25
	} else if len(dataExposed) > 0 {
		riskScore += 0.1
	}
	if usersAffected > 10000 {
		riskScore += 0.15
	} else if usersAffected > 1000 {
		riskScore += 0.05
	}
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	br := &BlastRadius{
		ID:             genID("br"),
		ComponentID:    componentID,
		ComponentType:  componentType,
		DirectImpact:   directImpact,
		IndirectImpact: indirectImpact,
		DataExposed:    dataExposed,
		UsersAffected:  usersAffected,
		EstimatedCost:  estimatedCost,
		RiskScore:      riskScore,
		AnalyzedAt:     time.Now().UTC(),
	}
	sh.blastRadii[br.ID] = br
	sh.persist()
	return br, nil
}

// GetBlastRadius returns the blast radius for a component.
func (sh *SecurityHub) GetBlastRadius(componentID string) (*BlastRadius, error) {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	for _, br := range sh.blastRadii {
		if br.ComponentID == componentID {
			return br, nil
		}
	}
	return nil, fmt.Errorf("no blast radius analysis for %s", componentID)
}

// --- Supply Chain ---

// VetDependency records a dependency vulnerability.
func (sh *SecurityHub) VetDependency(pkg, version, vulnerability string, severity Severity, score float64, fixedIn string, transitive bool) (*SupplyChainVuln, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	v := &SupplyChainVuln{
		ID:            genID("vuln"),
		Package:       pkg,
		Version:       version,
		Vulnerability: vulnerability,
		Severity:      severity,
		Score:         score,
		FixedIn:       fixedIn,
		Transitive:    transitive,
		DetectedAt:    time.Now().UTC(),
	}
	sh.vulns[v.ID] = v
	sh.persist()
	return v, nil
}

// ResolveVuln marks a vulnerability as resolved.
func (sh *SecurityHub) ResolveVuln(vulnID string) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	v, ok := sh.vulns[vulnID]
	if !ok {
		return fmt.Errorf("vuln %s not found", vulnID)
	}
	now := time.Now().UTC()
	v.ResolvedAt = &now
	sh.persist()
	return nil
}

// ListOpenVulns returns unresolved vulnerabilities.
func (sh *SecurityHub) ListOpenVulns() []*SupplyChainVuln {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	var result []*SupplyChainVuln
	for _, v := range sh.vulns {
		if v.ResolvedAt == nil {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	return result
}

// --- DLP ---

// AddDLPPolicy adds a data loss prevention policy.
func (sh *SecurityHub) AddDLPPolicy(name, pattern, category, action string) (*DLPPolicy, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	p := &DLPPolicy{
		ID:        genID("dlp"),
		Name:      name,
		Pattern:   pattern,
		Category:  category,
		Action:    action,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
	}
	sh.dlpPolicies[p.ID] = p
	sh.persist()
	return p, nil
}

// ScanForExfiltration scans content against DLP policies.
func (sh *SecurityHub) ScanForExfiltration(source, content string) ([]*DLPScanResult, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	var results []*DLPScanResult
	for _, p := range sh.dlpPolicies {
		if !p.Enabled {
			continue
		}
		if strings.Contains(strings.ToLower(content), strings.ToLower(p.Pattern)) {
			masked := maskContent(content, p.Pattern)
			r := &DLPScanResult{
				ID:         genID("scan"),
				PolicyID:   p.ID,
				Source:     source,
				MatchType:  p.Pattern,
				MatchValue: masked,
				Category:   p.Category,
				Action:     p.Action,
				ScannedAt:  time.Now().UTC(),
			}
			sh.dlpResults[r.ID] = r
			results = append(results, r)
		}
	}
	sh.persist()
	return results, nil
}

// --- Red Team ---

// RunRedTeamTest records an adversarial test result.
func (sh *SecurityHub) RunRedTeamTest(target, testType string, success bool, severity Severity, description, reproduction, mitigation string) (*RedTeamResult, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	r := &RedTeamResult{
		ID:           genID("red"),
		Target:       target,
		TestType:     testType,
		Success:      success,
		Severity:     severity,
		Description:  description,
		Reproduction: reproduction,
		Mitigation:   mitigation,
		TestedAt:     time.Now().UTC(),
	}
	sh.redTeam[r.ID] = r
	sh.persist()
	return r, nil
}

// ListRedTeamResults returns red team results, optionally filtered by target.
func (sh *SecurityHub) ListRedTeamResults(target string) []*RedTeamResult {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	var result []*RedTeamResult
	for _, r := range sh.redTeam {
		if target == "" || r.Target == target {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].TestedAt.After(result[j].TestedAt) })
	return result
}

// --- Reports ---

// GenerateSecurityReport produces a comprehensive security posture report.
func (sh *SecurityHub) GenerateSecurityReport() (*SecurityPosture, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	openVulns, criticalVulns := 0, 0
	unvetted := 0
	for _, v := range sh.vulns {
		if v.ResolvedAt == nil {
			openVulns++
			if v.Severity == SeverityCritical || v.Score >= 9.0 {
				criticalVulns++
			}
		} else {
			// resolved but still count for unvetted if transitive
			if v.Transitive {
				unvetted++
			}
		}
	}

	dlpViolations := len(sh.dlpResults)

	totalRed, failedRed := 0, 0
	for _, r := range sh.redTeam {
		totalRed++
		if r.Success { // attack succeeded
			failedRed++
		}
	}

	redTeamFailRate := 0.0
	if totalRed > 0 {
		redTeamFailRate = float64(failedRed) / float64(totalRed)
	}

	score := 100.0
	score -= float64(criticalVulns) * 15
	score -= float64(openVulns-criticalVulns) * 5
	score -= float64(dlpViolations) * 3
	score -= redTeamFailRate * 30
	score -= float64(unvetted) * 2
	if score < 0 {
		score = 0
	}

	posture := &SecurityPosture{
		ID:              genID("posture"),
		Score:           score,
		OpenVulns:       openVulns,
		CriticalVulns:   criticalVulns,
		UnvettedDeps:    unvetted,
		RedTeamFailRate: redTeamFailRate,
		DLPViolations:   dlpViolations,
		AssessedAt:      time.Now().UTC(),
	}
	sh.postures[posture.ID] = posture
	sh.persist()
	return posture, nil
}

// --- Helpers ---

func maskContent(content, pattern string) string {
	idx := strings.Index(strings.ToLower(content), strings.ToLower(pattern))
	if idx < 0 {
		return "***"
	}
	// Show first and last char, mask middle
	p := content[idx : idx+len(pattern)]
	if len(p) <= 2 {
		return "***"
	}
	return string(p[0]) + strings.Repeat("*", len(p)-2) + string(p[len(p)-1])
}

func (sh *SecurityHub) persist() {
	if sh.path == "" {
		return
	}
	data := struct {
		BlastRadii  map[string]*BlastRadius    `json:"blast_radii"`
		Vulns       map[string]*SupplyChainVuln `json:"vulns"`
		DLPPolicies map[string]*DLPPolicy       `json:"dlp_policies"`
		DLPResults  map[string]*DLPScanResult   `json:"dlp_results"`
		RedTeam     map[string]*RedTeamResult   `json:"red_team"`
		Postures    map[string]*SecurityPosture `json:"postures"`
	}{sh.blastRadii, sh.vulns, sh.dlpPolicies, sh.dlpResults, sh.redTeam, sh.postures}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.MkdirAll(filepath.Dir(sh.path), 0755)
	os.WriteFile(sh.path, raw, 0644)
}

func (sh *SecurityHub) load() {
	if sh.path == "" {
		return
	}
	raw, err := os.ReadFile(sh.path)
	if err != nil {
		return
	}
	var data struct {
		BlastRadii  map[string]*BlastRadius    `json:"blast_radii"`
		Vulns       map[string]*SupplyChainVuln `json:"vulns"`
		DLPPolicies map[string]*DLPPolicy       `json:"dlp_policies"`
		DLPResults  map[string]*DLPScanResult   `json:"dlp_results"`
		RedTeam     map[string]*RedTeamResult   `json:"red_team"`
		Postures    map[string]*SecurityPosture `json:"postures"`
	}
	if json.Unmarshal(raw, &data) == nil {
		if data.BlastRadii != nil {
			sh.blastRadii = data.BlastRadii
		}
		if data.Vulns != nil {
			sh.vulns = data.Vulns
		}
		if data.DLPPolicies != nil {
			sh.dlpPolicies = data.DLPPolicies
		}
		if data.DLPResults != nil {
			sh.dlpResults = data.DLPResults
		}
		if data.RedTeam != nil {
			sh.redTeam = data.RedTeam
		}
		if data.Postures != nil {
			sh.postures = data.Postures
		}
	}
}

func genID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
