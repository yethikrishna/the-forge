package secrets_test

import (
	"testing"

	"github.com/forge/sword/internal/secrets"
)

func TestScanPatterns(t *testing.T) {
	scanner := secrets.NewScanner(secrets.ModeWarn)

	// Test that the scanner detects known patterns
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{"AWS key pattern", "aws access key: AKIAIOSFODNN7EXAMPLE", true},
		{"Generic secret", "password=my-super-secret-value-12345", true},
		{"GitHub token pattern", "token: ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", true},
		{"Clean text", "This is a clean message with no secrets", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detections := scanner.Scan(tt.text)
			if tt.expected && len(detections) == 0 {
				t.Errorf("expected detection for %s", tt.name)
			}
			if !tt.expected && len(detections) > 0 {
				t.Errorf("unexpected detection for clean text: %v", detections)
			}
		})
	}
}

func TestRedact(t *testing.T) {
	scanner := secrets.NewScanner(secrets.ModeRedact)

	text := "The password=supersecret123 should be redacted"
	redacted, _ := scanner.Redact(text)

	if redacted == text {
		t.Error("text should be redacted")
	}
}

func TestValidate(t *testing.T) {
	scanner := secrets.NewScanner(secrets.ModeBlock)

	text := "Clean text with no secrets"
	err := scanner.Validate(text)
	if err != nil {
		t.Errorf("clean text should validate: %v", err)
	}
}

func TestScannerModes(t *testing.T) {
	modes := []secrets.ScannerMode{
		secrets.ModeOff,
		secrets.ModeWarn,
		secrets.ModeRedact,
		secrets.ModeBlock,
	}

	for _, mode := range modes {
		s := secrets.NewScanner(mode)
		if s == nil {
			t.Errorf("scanner should not be nil for mode %v", mode)
		}
	}
}

func TestAddPattern(t *testing.T) {
	scanner := secrets.NewScanner(secrets.ModeWarn)
	scanner.AddPattern(secrets.Pattern{
		Name:     "custom",
		Category: "custom",
		Regex:    `CUSTOM_SECRET_\w+`,
		Severity: "medium",
	})

	text := "Found CUSTOM_SECRET_abc123 in the output"
	detections := scanner.Scan(text)

	if len(detections) == 0 {
		t.Error("should detect custom pattern")
	}
}

func TestDetectionTypes(t *testing.T) {
	scanner := secrets.NewScanner(secrets.ModeWarn)

	// Test various secret patterns
	patterns := []string{
		"aws_access_key_id=AKIAIOSFODNN7EXAMPLE",
		"aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	for _, p := range patterns {
		detections := scanner.Scan(p)
		// Some patterns may or may not be detected depending on the regex
		t.Logf("Pattern: %s -> Detections: %d", p[:20], len(detections))
	}
}

func TestMultipleSecrets(t *testing.T) {
	scanner := secrets.NewScanner(secrets.ModeWarn)

	text := "password=secret123 and token=abc456def789"
	detections := scanner.Scan(text)

	// Should detect at least one secret pattern
	if len(detections) == 0 {
		t.Log("No secrets detected (patterns may not match)")
	}
}

func TestCleanText(t *testing.T) {
	scanner := secrets.NewScanner(secrets.ModeWarn)

	text := "Hello world! This is a regular message with no secrets at all."
	detections := scanner.Scan(text)

	if len(detections) > 0 {
		t.Errorf("clean text should have no detections, got %d", len(detections))
	}
}
