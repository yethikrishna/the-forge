package preview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewPreviewer(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	if p == nil {
		t.Fatal("expected previewer")
	}
}

func TestCreatePlan(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	plan, err := p.Create("agent-1", TypeFileWrite, "/tmp/test.txt", "Overwrite test file")
	if err != nil {
		t.Fatal(err)
	}
	if plan.ID == "" {
		t.Error("expected plan ID")
	}
	if plan.Status != "pending" {
		t.Errorf("expected pending, got %s", plan.Status)
	}
	if plan.AgentID != "agent-1" {
		t.Error("agent ID mismatch")
	}
}

func TestApprovePlan(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	plan, _ := p.Create("agent-1", TypeFileWrite, "/tmp/test.txt", "Write file")

	err := p.Approve(plan.ID, "user", "looks good")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := p.Get(plan.ID)
	if got.Status != "approved" {
		t.Errorf("expected approved, got %s", got.Status)
	}
	if got.ApprovedBy != "user" {
		t.Error("approver mismatch")
	}
}

func TestRejectPlan(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	plan, _ := p.Create("agent-1", TypeFileDelete, "/tmp/old.txt", "Delete old file")

	err := p.Reject(plan.ID, "too risky")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := p.Get(plan.ID)
	if got.Status != "rejected" {
		t.Errorf("expected rejected, got %s", got.Status)
	}
}

func TestModifyPlan(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	plan, _ := p.Create("agent-1", TypeFileWrite, "/tmp/test.txt", "Write")

	changes := []Change{
		{File: "test.txt", Old: "old line", New: "new line", Line: 5, Removed: true},
		{File: "test.txt", New: "added line", Line: 6, Added: true},
	}
	p.Modify(plan.ID, changes)

	got, _ := p.Get(plan.ID)
	if got.Status != "modified" {
		t.Errorf("expected modified, got %s", got.Status)
	}
	if len(got.Changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(got.Changes))
	}
}

func TestApproveNotFound(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	err := p.Approve("nonexistent", "user", "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestApproveAlreadyResolved(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	plan, _ := p.Create("agent-1", TypeFileWrite, "/tmp/x", "Write")
	p.Approve(plan.ID, "user", "ok")

	err := p.Approve(plan.ID, "user", "again")
	if err == nil {
		t.Error("already approved plan should error")
	}
}

func TestGetNotFound(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	_, ok := p.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent plan")
	}
}

func TestListPending(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	p.Create("a1", TypeFileWrite, "/tmp/1", "Write 1")
	p.Create("a2", TypeFileDelete, "/tmp/2", "Delete 2")

	pending := p.ListPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestListPendingExcludesResolved(t *testing.T) {
	p := NewPreviewer(t.TempDir())
	plan, _ := p.Create("a1", TypeFileWrite, "/tmp/1", "Write")
	p.Create("a2", TypeFileDelete, "/tmp/2", "Delete")
	p.Approve(plan.ID, "user", "ok")

	pending := p.ListPending()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending after approval, got %d", len(pending))
	}
}

func TestBackupCreated(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	os.WriteFile(target, []byte("original content"), 0644)

	p := NewPreviewer(filepath.Join(dir, "store"))
	plan, _ := p.Create("agent-1", TypeFileWrite, target, "Overwrite")

	if plan.BackupPath == "" {
		t.Error("expected backup path for file write")
	}

	data, err := os.ReadFile(plan.BackupPath)
	if err != nil {
		t.Fatalf("backup should exist: %v", err)
	}
	if string(data) != "original content" {
		t.Error("backup should contain original content")
	}
}

func TestRestoreBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	os.WriteFile(target, []byte("original"), 0644)

	p := NewPreviewer(filepath.Join(dir, "store"))
	plan, _ := p.Create("agent-1", TypeFileWrite, target, "Overwrite")

	// Overwrite the file
	os.WriteFile(target, []byte("modified"), 0644)

	// Restore
	err := p.RestoreBackup(plan.ID)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(target)
	if string(data) != "original" {
		t.Errorf("expected original content, got %s", string(data))
	}
}

func TestAssessImpact(t *testing.T) {
	tests := []struct {
		action  ActionType
		target  string
		impact  Impact
	}{
		{TypeFileDelete, "/tmp/test.txt", ImpactHigh},
		{TypeFileDelete, "/etc/passwd", ImpactDestructive},
		{TypeFileWrite, "/tmp/test.txt", ImpactMedium},
		{TypeCommandExec, "ls", ImpactMedium},
		{TypeCommandExec, "rm -rf /", ImpactDestructive},
		{TypeBulkChange, ".", ImpactHigh},
	}
	for _, tt := range tests {
		got := assessImpact(tt.action, tt.target)
		if got != tt.impact {
			t.Errorf("assessImpact(%s, %s) = %s, want %s", tt.action, tt.target, got, tt.impact)
		}
	}
}

func TestIsReversible(t *testing.T) {
	if isReversible(TypeFileDelete) {
		t.Error("file delete should not be reversible")
	}
	if isReversible(TypeCommandExec) {
		t.Error("command exec should not be reversible")
	}
	if !isReversible(TypeFileWrite) {
		t.Error("file write should be reversible")
	}
}

func TestAssessRisks(t *testing.T) {
	risks := assessRisks(TypeFileDelete, "main.go")
	if len(risks) == 0 {
		t.Error("file delete should have risks")
	}
	found := false
	for _, r := range risks {
		if strings.Contains(r, "compilation") {
			found = true
		}
	}
	if !found {
		t.Error("should warn about compilation for .go files")
	}
}

func TestSuggestAlternatives(t *testing.T) {
	alts := suggestAlternatives(TypeFileDelete, "test.txt")
	if len(alts) == 0 {
		t.Error("should suggest alternatives for delete")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	p1 := NewPreviewer(dir)
	plan, _ := p1.Create("agent-1", TypeFileWrite, "/tmp/x", "Write")
	p1.Approve(plan.ID, "user", "ok")

	p2 := NewPreviewer(dir)
	got, ok := p2.Get(plan.ID)
	if !ok {
		t.Fatal("plan should persist")
	}
	if got.Status != "approved" {
		t.Errorf("expected approved, got %s", got.Status)
	}
}

func TestFormatPlan(t *testing.T) {
	plan := &Plan{
		ID:          "plan-1",
		AgentID:     "agent-1",
		Type:        TypeFileDelete,
		Target:      "/tmp/old.txt",
		Description: "Delete old temp file",
		Impact:      ImpactHigh,
		Reversible:  false,
		Risks:       []string{"Data loss"},
		Alternatives: []string{"Archive instead"},
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	s := FormatPlan(plan)
	if !strings.Contains(s, "plan-1") {
		t.Error("should contain plan ID")
	}
	if !strings.Contains(s, "Data loss") {
		t.Error("should show risks")
	}
	if !strings.Contains(s, "Archive") {
		t.Error("should show alternatives")
	}
}

func TestRejectCleansBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	os.WriteFile(target, []byte("data"), 0644)

	p := NewPreviewer(filepath.Join(dir, "store"))
	plan, _ := p.Create("agent-1", TypeFileWrite, target, "Write")
	backupPath := plan.BackupPath

	p.Reject(plan.ID, "nope")

	if _, err := os.Stat(backupPath); err == nil {
		t.Error("backup should be cleaned on reject")
	}
}
