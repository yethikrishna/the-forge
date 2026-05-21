package permission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewEnforcer(t *testing.T) {
	e := NewEnforcer("")
	if e == nil {
		t.Fatal("expected enforcer")
	}
}

func TestQuickScopeReadOnly(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	err := e.QuickScope("sess-1", ScopeReadOnly)
	if err != nil {
		t.Fatal(err)
	}

	p, ok := e.GetPolicy("sess-1")
	if !ok {
		t.Fatal("expected policy")
	}
	if p.Scope != ScopeReadOnly {
		t.Errorf("expected read-only, got %s", p.Scope)
	}
}

func TestReadOnlyAllowsRead(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeReadOnly)

	err := e.Check("sess-1", ActionRead, "/tmp/file.txt")
	if err != nil {
		t.Errorf("read should be allowed: %v", err)
	}
}

func TestReadOnlyBlocksWrite(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeReadOnly)

	err := e.Check("sess-1", ActionWrite, "/tmp/file.txt")
	if err == nil {
		t.Error("write should be blocked in read-only scope")
	}
}

func TestReadOnlyBlocksExecute(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeReadOnly)

	err := e.Check("sess-1", ActionExecute, "rm")
	if err == nil {
		t.Error("execute should be blocked in read-only scope")
	}
}

func TestSrcOnlyAllowsRead(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeSrcOnly)

	err := e.Check("sess-1", ActionRead, "src/main.go")
	if err != nil {
		t.Errorf("reading src/ should be allowed: %v", err)
	}
}

func TestSrcOnlyAllowsWrite(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeSrcOnly)

	err := e.Check("sess-1", ActionWrite, "src/main.go")
	if err != nil {
		t.Errorf("writing to src/ should be allowed: %v", err)
	}
}

func TestSrcOnlyBlocksExecute(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeSrcOnly)

	err := e.Check("sess-1", ActionExecute, "go test")
	if err == nil {
		t.Error("execute should be blocked in src-only scope")
	}
}

func TestFullAccessAllowsEverything(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeFull)

	actions := []Action{ActionRead, ActionWrite, ActionExecute, ActionDelete, ActionNetwork, ActionEnv}
	for _, a := range actions {
		err := e.Check("sess-1", a, "/tmp/test")
		if err != nil {
			t.Errorf("full scope should allow %s: %v", a, err)
		}
	}
}

func TestNoPolicyAllowsEverything(t *testing.T) {
	e := NewEnforcer(t.TempDir())

	err := e.Check("unscoped", ActionWrite, "/tmp/anything")
	if err != nil {
		t.Errorf("no policy should allow all: %v", err)
	}
}

func TestBlockedDirs(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeReadOnly)

	err := e.Check("sess-1", ActionRead, "/etc/passwd")
	if err == nil {
		t.Error("reading /etc should be blocked")
	}
}

func TestSandboxScope(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeSandbox)

	// Writing to /tmp/ should be allowed
	err := e.Check("sess-1", ActionWrite, "/tmp/test.txt")
	if err != nil {
		t.Errorf("sandbox should allow /tmp: %v", err)
	}
}

func TestViolationsRecorded(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeReadOnly)

	e.Check("sess-1", ActionWrite, "/tmp/file")

	violations := e.GetViolations("sess-1")
	if len(violations) == 0 {
		t.Error("expected violation to be recorded")
	}
	if !violations[0].Blocked {
		t.Error("violation should be blocked")
	}
}

func TestSetPolicy(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	err := e.SetPolicy(Policy{
		SessionID:      "custom",
		Scope:          ScopeReadOnly,
		AllowedActions: []Action{ActionRead},
		MaxFileSize:    1024,
	})
	if err != nil {
		t.Fatal(err)
	}

	p, ok := e.GetPolicy("custom")
	if !ok {
		t.Fatal("expected policy")
	}
	if p.MaxFileSize != 1024 {
		t.Errorf("expected max 1024, got %d", p.MaxFileSize)
	}
}

func TestSetPolicyNoSession(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	err := e.SetPolicy(Policy{})
	if err == nil {
		t.Error("expected error for missing session ID")
	}
}

func TestRemovePolicy(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeReadOnly)
	e.RemovePolicy("sess-1")

	_, ok := e.GetPolicy("sess-1")
	if ok {
		t.Error("policy should be removed")
	}
}

func TestListSessions(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("s1", ScopeReadOnly)
	e.QuickScope("s2", ScopeFull)

	sessions := e.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	e1 := NewEnforcer(dir)
	e1.QuickScope("sess-1", ScopeReadOnly)

	e2 := NewEnforcer(dir)
	p, ok := e2.GetPolicy("sess-1")
	if !ok {
		t.Fatal("expected policy to persist")
	}
	if p.Scope != ScopeReadOnly {
		t.Errorf("expected read-only, got %s", p.Scope)
	}
}

func TestFormatPolicy(t *testing.T) {
	p := &Policy{
		SessionID:      "sess-1",
		Scope:          ScopeReadOnly,
		AllowedActions: []Action{ActionRead},
		BlockedDirs:    []string{"/etc"},
		CreatedAt:      time.Now(),
	}

	s := FormatPolicy(p)
	if !strings.Contains(s, "read-only") {
		t.Error("should contain scope")
	}
	if !strings.Contains(s, "sess-1") {
		t.Error("should contain session ID")
	}
}

func TestFormatViolation(t *testing.T) {
	v := Violation{
		Action:    ActionWrite,
		Target:    "/etc/passwd",
		Reason:    "blocked directory",
		Blocked:   true,
		Timestamp: time.Now(),
	}

	s := FormatViolation(&v)
	if !strings.Contains(s, "BLOCKED") {
		t.Error("should show blocked")
	}
}

func TestViolationTrimmed(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	e.QuickScope("sess-1", ScopeReadOnly)

	for i := 0; i < 60; i++ {
		e.Check("sess-1", ActionWrite, "/tmp")
	}

	violations := e.GetViolations("sess-1")
	if len(violations) > 50 {
		t.Errorf("violations should be trimmed to 50, got %d", len(violations))
	}
}

func TestFileSizeLimit(t *testing.T) {
	e := NewEnforcer(t.TempDir())
	dir := t.TempDir()

	bigFile := filepath.Join(dir, "big.txt")
	os.WriteFile(bigFile, make([]byte, 2048), 0644)

	e.SetPolicy(Policy{
		SessionID:      "sess-1",
		Scope:          ScopeFull,
		AllowedActions: []Action{ActionRead, ActionWrite},
		MaxFileSize:    1024,
	})

	err := e.Check("sess-1", ActionWrite, bigFile)
	if err == nil {
		t.Error("should block write to oversized file")
	}
}
