package scanhooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewScanner(t *testing.T) {
	s := NewScanner("")
	if s == nil {
		t.Fatal("expected scanner")
	}
}

func TestPreHookNoFindings(t *testing.T) {
	s := NewScanner("")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "safe.txt"), []byte("hello world"), 0644)

	result, err := s.RunPreHook("agent-1", []string{filepath.Join(dir, "safe.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Blocked {
		t.Error("should not block safe file")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings, got %d", len(result.Findings))
	}
}

func TestPreHookSecretDetection(t *testing.T) {
	s := NewScanner("")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"), []byte("api_key=AKIAIOSFODNN7EXAMPLE\n"), 0644)

	result, err := s.RunPreHook("agent-1", []string{filepath.Join(dir, "config.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) == 0 {
		t.Error("expected secret findings")
	}
	if !result.Blocked {
		t.Error("critical finding should block")
	}
}

func TestPostHookNoFindings(t *testing.T) {
	s := NewScanner("")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "output.go"), []byte("package main\nfunc main() {}\n"), 0644)

	result, err := s.RunPostHook("agent-1", []string{filepath.Join(dir, "output.go")})
	if err != nil {
		t.Fatal(err)
	}
	if result.HookType != HookPost {
		t.Errorf("expected post hook, got %s", result.HookType)
	}
}

func TestProtectedPathPolicy(t *testing.T) {
	s := NewScanner("")

	result, err := s.RunPreHook("agent-1", []string{"/etc/passwd"})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range result.Findings {
		if f.Rule == "Protected Path Access" {
			found = true
		}
	}
	if !found {
		t.Error("expected protected path finding")
	}
}

func TestGitHubTokenDetection(t *testing.T) {
	s := NewScanner("")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.txt"), []byte("token=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n"), 0644)

	result, _ := s.RunPreHook("agent-1", []string{filepath.Join(dir, "config.txt")})
	found := false
	for _, f := range result.Findings {
		if f.Rule == "GitHub Token" {
			found = true
		}
	}
	if !found {
		t.Error("expected GitHub token finding")
	}
}

func TestPrivateKeyDetection(t *testing.T) {
	s := NewScanner("")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "key.pem"), []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIE..."), 0644)

	result, _ := s.RunPreHook("agent-1", []string{filepath.Join(dir, "key.pem")})
	found := false
	for _, f := range result.Findings {
		if f.Rule == "Private Key" {
			found = true
		}
	}
	if !found {
		t.Error("expected private key finding")
	}
}

func TestSQLInjectionDetection(t *testing.T) {
	s := NewScanner("")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "query.go"), []byte(`query := "SELECT * FROM users WHERE id=" + req.Param("id")`), 0644)

	result, _ := s.RunPostHook("agent-1", []string{filepath.Join(dir, "query.go")})
	found := false
	for _, f := range result.Findings {
		if f.Rule == "SQL Injection" {
			found = true
		}
	}
	if !found {
		t.Error("expected SQL injection finding")
	}
}

func TestAddConfig(t *testing.T) {
	s := NewScanner("")
	s.AddConfig(HookConfig{
		Name:    "custom",
		Type:    HookPre,
		Enabled: true,
		BlockOn: []Severity{SevCritical},
	})
}

func TestHistory(t *testing.T) {
	s := NewScanner("")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "safe.txt"), []byte("ok"), 0644)

	s.RunPreHook("a1", []string{filepath.Join(dir, "safe.txt")})
	s.RunPostHook("a1", []string{filepath.Join(dir, "safe.txt")})

	history := s.History(0)
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}
}

func TestHistoryLimit(t *testing.T) {
	s := NewScanner("")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "safe.txt"), []byte("ok"), 0644)

	for i := 0; i < 10; i++ {
		s.RunPreHook("a1", []string{filepath.Join(dir, "safe.txt")})
	}

	history := s.History(3)
	if len(history) != 3 {
		t.Errorf("expected 3, got %d", len(history))
	}
}

func TestFormatResult(t *testing.T) {
	r := &ScanResult{
		HookType:  HookPre,
		AgentID:   "agent-1",
		Timestamp: parseTime(),
		Duration:  "10ms",
		Findings: []Finding{
			{Rule: "Test", Severity: SevMedium, Description: "Test finding"},
		},
		Blocked:    true,
		BlockReason: "Test block",
	}

	s := FormatResult(r)
	if !strings.Contains(s, "BLOCKED") {
		t.Error("should show blocked")
	}
	if !strings.Contains(s, "Test finding") {
		t.Error("should show finding")
	}
}

func TestFormatResultPassed(t *testing.T) {
	r := &ScanResult{
		HookType:  HookPre,
		AgentID:   "agent-1",
		Timestamp: parseTime(),
		Duration:  "5ms",
	}

	s := FormatResult(r)
	if !strings.Contains(s, "PASSED") {
		t.Error("should show passed")
	}
}

func TestNonexistentFile(t *testing.T) {
	s := NewScanner("")
	result, err := s.RunPreHook("agent-1", []string{"/nonexistent/file.txt"})
	if err != nil {
		t.Fatal(err)
	}
	// Should not error, just skip the file
	if result == nil {
		t.Error("should return result even with bad files")
	}
}

func parseTime() time.Time {
	t, _ := time.Parse(time.RFC3339, "2025-01-01T00:00:00Z")
	return t
}
