// Package secrets provides secret scanning and redaction for AI agent I/O.
// The forge keeps its secrets hidden from prying eyes.
package secrets

import (
	"fmt"
	"regexp"
	"strings"
)

// Pattern represents a detectable secret pattern.
type Pattern struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Regex       string `json:"regex"`
	Severity    string `json:"severity"` // critical, high, medium, low
	Category    string `json:"category"` // api_key, password, token, certificate, pii
}

// Detection is a found secret in text.
type Detection struct {
	Pattern  Pattern `json:"pattern"`
	Match    string  `json:"match"`
	Start    int     `json:"start"`
	End      int     `json:"end"`
	Redacted string  `json:"redacted"`
}

// Scanner scans text for secrets and PII.
type Scanner struct {
	patterns []compiledPattern
	mode     Mode
}

// Mode controls how detected secrets are handled.
type Mode string

const (
	ModeOff     Mode = "off"     // No scanning
	ModeWarn    Mode = "warn"    // Log warnings, don't modify text
	ModeRedact  Mode = "redact"  // Replace secrets with [REDACTED]
	ModeBlock   Mode = "block"   // Return error if secrets found
	ModeReplace Mode = "replace" // Replace with type-specific placeholders
)

// ScannerMode is an alias for Mode for backward compatibility.
type ScannerMode = Mode

type compiledPattern struct {
	info Pattern
	re   *regexp.Regexp
}

// DefaultPatterns returns the built-in secret detection patterns.
func DefaultPatterns() []Pattern {
	return []Pattern{
		// API Keys
		{Name: "openai_api_key", Description: "OpenAI API key", Regex: `sk-[a-zA-Z0-9_-]{20,}T3BlbkFJ[a-zA-Z0-9_-]{20,}`, Severity: "critical", Category: "api_key"},
		{Name: "anthropic_api_key", Description: "Anthropic API key", Regex: `sk-ant-api03-[a-zA-Z0-9\-]{20,}`, Severity: "critical", Category: "api_key"},
		{Name: "generic_api_key", Description: "Generic API key", Regex: `(?:api[_-]?key|apikey)\s*[:=]\s*['"]?[a-zA-Z0-9]{20,}['"]?`, Severity: "high", Category: "api_key"},
		{Name: "bearer_token", Description: "Bearer token", Regex: `Bearer\s+[a-zA-Z0-9\-._~+/]+=*`, Severity: "critical", Category: "token"},
		{Name: "github_token", Description: "GitHub personal access token", Regex: `ghp_[a-zA-Z0-9]{36}`, Severity: "critical", Category: "api_key"},
		{Name: "github_oauth", Description: "GitHub OAuth token", Regex: `gho_[a-zA-Z0-9]{36}`, Severity: "critical", Category: "token"},
		{Name: "aws_access_key", Description: "AWS access key ID", Regex: `AKIA[0-9A-Z]{16}`, Severity: "critical", Category: "api_key"},
		{Name: "aws_secret_key", Description: "AWS secret access key", Regex: `(?:aws[_-]?secret[_-]?key)\s*[:=]\s*['"]?[A-Za-z0-9/+=]{40}['"]?`, Severity: "critical", Category: "api_key"},

		// Passwords
		{Name: "password_assignment", Description: "Password in assignment", Regex: `(?:password|passwd|pwd)\s*[:=]\s*['"]?[^\s'"]{8,}['"]?`, Severity: "high", Category: "password"},
		{Name: "password_in_url", Description: "Password in URL", Regex: `://[^:]+:[^@]+@`, Severity: "high", Category: "password"},

		// Tokens
		{Name: "jwt", Description: "JSON Web Token", Regex: `eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`, Severity: "high", Category: "token"},
		{Name: "slack_token", Description: "Slack token", Regex: `xox[baprs]-[a-zA-Z0-9\-]{10,}`, Severity: "critical", Category: "token"},
		{Name: "stripe_key", Description: "Stripe API key", Regex: `(?:sk|pk)_(?:test|live)_[a-zA-Z0-9]{24,}`, Severity: "critical", Category: "api_key"},

		// PII
		{Name: "ssn", Description: "US Social Security Number", Regex: `\b\d{3}-\d{2}-\d{4}\b`, Severity: "critical", Category: "pii"},
		{Name: "credit_card", Description: "Credit card number", Regex: `\b(?:\d{4}[-\s]?){3}\d{4}\b`, Severity: "critical", Category: "pii"},
		{Name: "email", Description: "Email address", Regex: `\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`, Severity: "medium", Category: "pii"},
		{Name: "phone", Description: "US phone number", Regex: `\b(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`, Severity: "medium", Category: "pii"},
		{Name: "ip_address", Description: "IP address", Regex: `\b(?:\d{1,3}\.){3}\d{1,3}\b`, Severity: "low", Category: "pii"},

		// Private keys
		{Name: "private_key", Description: "Private key", Regex: `-----BEGIN (?:RSA |EC |DSA )?PRIVATE KEY-----`, Severity: "critical", Category: "certificate"},
	}
}

// NewScanner creates a new secret scanner.
func NewScanner(mode Mode, customPatterns ...Pattern) *Scanner {
	s := &Scanner{
		mode: mode,
	}

	if mode == ModeOff {
		return s
	}

	allPatterns := append(DefaultPatterns(), customPatterns...)
	for _, p := range allPatterns {
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			continue // skip invalid patterns
		}
		s.patterns = append(s.patterns, compiledPattern{info: p, re: re})
	}

	return s
}

// AddPattern adds a custom detection pattern at runtime.
func (s *Scanner) AddPattern(p Pattern) {
	re, err := regexp.Compile(p.Regex)
	if err != nil {
		return
	}
	s.patterns = append(s.patterns, compiledPattern{info: p, re: re})
}

// Validate checks text for secrets and returns an error if found.
// Alias for Check() for backward compatibility.
func (s *Scanner) Validate(text string) error {
	return s.Check(text)
}

// Scan scans text for secrets and returns detections.
func (s *Scanner) Scan(text string) []Detection {
	var detections []Detection

	for _, cp := range s.patterns {
		matches := cp.re.FindAllStringIndex(text, -1)
		for _, match := range matches {
			detections = append(detections, Detection{
				Pattern:  cp.info,
				Match:    text[match[0]:match[1]],
				Start:    match[0],
				End:      match[1],
				Redacted: redactForType(cp.info.Category),
			})
		}
	}

	return detections
}

// Redact scans text and replaces secrets with redacted placeholders.
func (s *Scanner) Redact(text string) (string, []Detection) {
	detections := s.Scan(text)
	if len(detections) == 0 {
		return text, nil
	}

	// Sort by start position descending to replace from end
	sorted := make([]Detection, len(detections))
	copy(sorted, detections)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Start < sorted[j].Start {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	result := text
	for _, d := range sorted {
		result = result[:d.Start] + d.Redacted + result[d.End:]
	}

	return result, detections
}

// Check scans text and returns an error if secrets are found (for block mode).
func (s *Scanner) Check(text string) error {
	detections := s.Scan(text)
	if len(detections) == 0 {
		return nil
	}

	var msgs []string
	for _, d := range detections {
		msgs = append(msgs, fmt.Sprintf("%s: %s detected", d.Pattern.Name, d.Pattern.Category))
	}
	return fmt.Errorf("secrets detected: %s", strings.Join(msgs, ", "))
}

// Process scans text and handles it according to the scanner mode.
func (s *Scanner) Process(text string) (string, []Detection, error) {
	detections := s.Scan(text)
	if len(detections) == 0 {
		return text, nil, nil
	}

	switch s.mode {
	case ModeRedact:
		redacted, _ := s.Redact(text)
		return redacted, detections, nil
	case ModeBlock:
		return text, detections, s.Check(text)
	case ModeReplace:
		replaced, _ := s.Redact(text)
		return replaced, detections, nil
	case ModeWarn:
		return text, detections, nil
	default:
		return text, detections, nil
	}
}

// HasCritical returns whether any detection is critical severity.
func HasCritical(detections []Detection) bool {
	for _, d := range detections {
		if d.Pattern.Severity == "critical" {
			return true
		}
	}
	return false
}

// HasPII returns whether any detection is PII.
func HasPII(detections []Detection) bool {
	for _, d := range detections {
		if d.Pattern.Category == "pii" {
			return true
		}
	}
	return false
}

// Summary returns a human-readable summary of detections.
func Summary(detections []Detection) string {
	if len(detections) == 0 {
		return "No secrets detected"
	}

	counts := map[string]int{}
	for _, d := range detections {
		counts[d.Pattern.Category]++
	}

	var parts []string
	for cat, count := range counts {
		parts = append(parts, fmt.Sprintf("%d %s(s)", count, cat))
	}

	severity := "low"
	if HasCritical(detections) {
		severity = "CRITICAL"
	} else if HasPII(detections) {
		severity = "medium"
	}

	return fmt.Sprintf("[%s] Detected: %s", severity, strings.Join(parts, ", "))
}

func redactForType(category string) string {
	switch category {
	case "api_key":
		return "[REDACTED_API_KEY]"
	case "password":
		return "[REDACTED_PASSWORD]"
	case "token":
		return "[REDACTED_TOKEN]"
	case "certificate":
		return "[REDACTED_PRIVATE_KEY]"
	case "pii":
		return "[REDACTED_PII]"
	default:
		return "[REDACTED]"
	}
}

// Need fmt import
var _ = fmt.Sprintf
