// Package scanhooks provides pre/post agent run security scanning hooks.
// Integrates with forge jail to scan for secrets, vulnerabilities, and
// policy violations before and after agent execution.
//
// Scan first, ask forgiveness never.
package scanhooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// HookType is pre or post execution.
type HookType string

const (
	HookPre  HookType = "pre"
	HookPost HookType = "post"
)

// Severity is the finding severity.
type Severity string

const (
	SevInfo     Severity = "info"
	SevLow      Severity = "low"
	SevMedium   Severity = "medium"
	SevHigh     Severity = "high"
	SevCritical Severity = "critical"
)

// Finding is a security finding from a scan.
type Finding struct {
	ID          string   `json:"id"`
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Match       string   `json:"match,omitempty"`
	Fixed       bool     `json:"fixed"`
}

// ScanResult is the result of a scan hook.
type ScanResult struct {
	HookType    HookType  `json:"hook_type"`
	AgentID     string    `json:"agent_id"`
	Timestamp   time.Time `json:"timestamp"`
	Findings    []Finding `json:"findings"`
	Blocked     bool      `json:"blocked"`
	BlockReason string    `json:"block_reason,omitempty"`
	Duration    string    `json:"duration"`
}

// HookConfig configures a scan hook.
type HookConfig struct {
	Name       string     `json:"name"`
	Type       HookType   `json:"type"`
	Enabled    bool       `json:"enabled"`
	BlockOn    []Severity `json:"block_on"` // severities that block execution
	SecretScan bool       `json:"secret_scan"`
	VulnScan   bool       `json:"vuln_scan"`
	PolicyScan bool       `json:"policy_scan"`
	Paths      []string   `json:"paths"` // paths to scan
}

// Scanner runs security scanning hooks.
type Scanner struct {
	configs  map[string]*HookConfig
	history  []ScanResult
	storeDir string
	mu       sync.RWMutex
}

// NewScanner creates a security scanner.
func NewScanner(storeDir string) *Scanner {
	s := &Scanner{
		configs:  make(map[string]*HookConfig),
		storeDir: storeDir,
	}
	s.registerDefaults()
	return s
}

func (s *Scanner) registerDefaults() {
	s.configs["pre-default"] = &HookConfig{
		Name:       "Pre-execution scan",
		Type:       HookPre,
		Enabled:    true,
		BlockOn:    []Severity{SevHigh, SevCritical},
		SecretScan: true,
		PolicyScan: true,
	}
	s.configs["post-default"] = &HookConfig{
		Name:       "Post-execution scan",
		Type:       HookPost,
		Enabled:    true,
		BlockOn:    []Severity{SevCritical},
		VulnScan:   true,
		PolicyScan: true,
	}
}

// RunPreHook runs pre-execution scanning hooks.
func (s *Scanner) RunPreHook(agentID string, paths []string) (*ScanResult, error) {
	return s.runHooks(agentID, HookPre, paths)
}

// RunPostHook runs post-execution scanning hooks.
func (s *Scanner) RunPostHook(agentID string, paths []string) (*ScanResult, error) {
	return s.runHooks(agentID, HookPost, paths)
}

func (s *Scanner) runHooks(agentID string, hookType HookType, paths []string) (*ScanResult, error) {
	start := time.Now()

	result := &ScanResult{
		HookType:  hookType,
		AgentID:   agentID,
		Timestamp: start,
	}

	var allFindings []Finding

	s.mu.RLock()
	for _, cfg := range s.configs {
		if !cfg.Enabled || cfg.Type != hookType {
			continue
		}

		scanPaths := paths
		if len(cfg.Paths) > 0 {
			scanPaths = cfg.Paths
		}

		if cfg.SecretScan {
			allFindings = append(allFindings, s.scanSecrets(scanPaths)...)
		}
		if cfg.VulnScan {
			allFindings = append(allFindings, s.scanVulnerabilities(scanPaths)...)
		}
		if cfg.PolicyScan {
			allFindings = append(allFindings, s.scanPolicies(scanPaths, agentID)...)
		}
	}
	s.mu.RUnlock()

	result.Findings = allFindings
	result.Duration = time.Since(start).String()

	// Check if any finding should block
	s.mu.RLock()
	for _, cfg := range s.configs {
		if !cfg.Enabled || cfg.Type != hookType {
			continue
		}
		for _, finding := range allFindings {
			for _, blockSev := range cfg.BlockOn {
				if finding.Severity == blockSev {
					result.Blocked = true
					result.BlockReason = fmt.Sprintf("Blocked by %s: %s (%s)", cfg.Name, finding.Description, finding.Severity)
					break
				}
			}
			if result.Blocked {
				break
			}
		}
		if result.Blocked {
			break
		}
	}
	s.mu.RUnlock()

	// Store result
	s.mu.Lock()
	s.history = append(s.history, *result)
	if len(s.history) > 100 {
		s.history = s.history[len(s.history)-100:]
	}
	s.mu.Unlock()

	return result, nil
}

// scanSecrets scans for leaked secrets.
func (s *Scanner) scanSecrets(paths []string) []Finding {
	var findings []Finding

	patterns := []struct {
		name  string
		regex string
		sev   Severity
	}{
		{"AWS Access Key", `AKIA[0-9A-Z]{16}`, SevCritical},
		{"AWS Secret Key", `(?i)aws_secret_access_key\s*[=:]\s*\S{20,}`, SevCritical},
		{"GitHub Token", `ghp_[0-9a-zA-Z]{36}`, SevCritical},
		{"Generic API Key", `(?i)(api[_-]?key|apikey)\s*[=:]\s*['"]?[0-9a-zA-Z]{20,}`, SevHigh},
		{"Private Key", `-----BEGIN (RSA |EC )?PRIVATE KEY-----`, SevCritical},
		{"JWT Secret", `(?i)(jwt[_-]?secret|jwt[_-]?key)\s*[=:]\s*\S{10,}`, SevHigh},
		{"Database URL", `(?i)(postgres|mysql|mongodb)://\S+:\S+@`, SevHigh},
		{"Slack Token", `xox[baprs]-[0-9a-zA-Z-]+`, SevHigh},
		{"Generic Password", `(?i)password\s*[=:]\s*['"]?[^\s'"]{8,}`, SevMedium},
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		lines := strings.Split(content, "\n")

		for _, p := range patterns {
			re, err := regexp.Compile(p.regex)
			if err != nil {
				continue
			}
			for i, line := range lines {
				if re.MatchString(line) {
					findings = append(findings, Finding{
						ID:          fmt.Sprintf("secret-%d", len(findings)+1),
						Rule:        p.name,
						Severity:    p.sev,
						Description: fmt.Sprintf("Potential %s detected", p.name),
						File:        path,
						Line:        i + 1,
						Match:       truncate(line, 80),
					})
				}
			}
		}
	}

	return findings
}

// scanVulnerabilities scans for known vulnerability patterns.
func (s *Scanner) scanVulnerabilities(paths []string) []Finding {
	var findings []Finding

	patterns := []struct {
		name  string
		regex string
		sev   Severity
	}{
		{"SQL Injection", `(?i)(SELECT|INSERT|UPDATE|DELETE).*\+\s*(req|params|input)`, SevHigh},
		{"Command Injection", `(?i)(exec|system|popen|shell)\s*\(\s*.*\+`, SevHigh},
		{"Path Traversal", `\.\./\.\./`, SevMedium},
		{"Hardcoded IP", `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`, SevLow},
		{"TODO Security", `(?i)(TODO|FIXME|HACK|XXX).*(security|auth|encrypt|password)`, SevInfo},
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")

		for _, p := range patterns {
			re, _ := regexp.Compile(p.regex)
			if re == nil {
				continue
			}
			for i, line := range lines {
				if re.MatchString(line) {
					findings = append(findings, Finding{
						ID:          fmt.Sprintf("vuln-%d", len(findings)+1),
						Rule:        p.name,
						Severity:    p.sev,
						Description: fmt.Sprintf("Potential %s pattern", p.name),
						File:        path,
						Line:        i + 1,
						Match:       truncate(line, 80),
					})
				}
			}
		}
	}

	return findings
}

// scanPolicies scans for policy violations.
func (s *Scanner) scanPolicies(paths []string, agentID string) []Finding {
	var findings []Finding

	// Policy: no direct file writes to /etc, /var, /sys
	for _, path := range paths {
		if strings.HasPrefix(path, "/etc/") || strings.HasPrefix(path, "/var/") || strings.HasPrefix(path, "/sys/") {
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("policy-%d", len(findings)+1),
				Rule:        "Protected Path Access",
				Severity:    SevHigh,
				Description: fmt.Sprintf("Agent %s accessing protected path: %s", agentID, path),
				File:        path,
			})
		}
	}

	return findings
}

// History returns scan history.
func (s *Scanner) History(limit int) []ScanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	result := make([]ScanResult, limit)
	copy(result, s.history[len(s.history)-limit:])
	return result
}

// AddConfig adds a hook configuration.
func (s *Scanner) AddConfig(cfg HookConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[cfg.Name] = &cfg
}

// FormatResult formats a scan result for display.
func FormatResult(r *ScanResult) string {
	status := "PASSED"
	if r.Blocked {
		status = "BLOCKED"
	}

	s := fmt.Sprintf("Scan [%s] %s — %s\n", r.HookType, r.AgentID, status)
	s += fmt.Sprintf("Time: %s (%s)\n", r.Timestamp.Format(time.RFC3339), r.Duration)

	if len(r.Findings) > 0 {
		s += fmt.Sprintf("Findings (%d):\n", len(r.Findings))
		for _, f := range r.Findings {
			s += fmt.Sprintf("  [%s] %s: %s\n", f.Severity, f.Rule, f.Description)
			if f.File != "" {
				s += fmt.Sprintf("    %s:%d\n", f.File, f.Line)
			}
		}
	}

	if r.Blocked {
		s += fmt.Sprintf("\n⚠ BLOCKED: %s\n", r.BlockReason)
	}

	return s
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// MarshalJSON custom marshals to include only non-sensitive data.
func (r *ScanResult) MarshalJSON() ([]byte, error) {
	type Alias ScanResult
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

// Save persists scan history.
func (s *Scanner) Save() error {
	if s.storeDir == "" {
		return nil
	}
	os.MkdirAll(s.storeDir, 0755)
	data, _ := json.MarshalIndent(s.history, "", "  ")
	return os.WriteFile(filepath.Join(s.storeDir, "scan_history.json"), data, 0644)
}
