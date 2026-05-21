package featuregate

import (
	"testing"
)

func TestCreateGate(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{
		Name:        "new-ui",
		Description: "New dashboard UI",
		Owner:       "team-a",
	}
	if err := store.Create(g); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if g.ID == "" {
		t.Error("Expected ID to be set")
	}
	if g.Status != StatusDisabled {
		t.Errorf("Expected disabled status, got %s", g.Status)
	}
}

func TestEnableDisable(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{Name: "test", Description: "test"}
	store.Create(g)

	if err := store.Enable(g.ID); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	retrieved, _ := store.Get(g.ID)
	if retrieved.Status != StatusActive {
		t.Errorf("Expected active, got %s", retrieved.Status)
	}
	if retrieved.RolloutPct != 100 {
		t.Errorf("Expected 100%% rollout, got %.0f%%", retrieved.RolloutPct)
	}

	if err := store.Disable(g.ID); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	retrieved, _ = store.Get(g.ID)
	if retrieved.Status != StatusDisabled {
		t.Errorf("Expected disabled, got %s", retrieved.Status)
	}
}

func TestKillSwitch(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{Name: "test", Description: "test"}
	store.Create(g)
	store.Enable(g.ID)

	if err := store.Kill(g.ID); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	retrieved, _ := store.Get(g.ID)
	if !retrieved.KillSwitch {
		t.Error("Expected kill switch to be active")
	}
	if retrieved.Status != StatusKilled {
		t.Errorf("Expected killed status, got %s", retrieved.Status)
	}

	// Should not be able to enable a killed gate
	if err := store.Enable(g.ID); err == nil {
		t.Error("Expected error enabling killed gate")
	}

	// Unkill should work
	if err := store.Unkill(g.ID); err != nil {
		t.Fatalf("Unkill: %v", err)
	}

	retrieved, _ = store.Get(g.ID)
	if retrieved.KillSwitch {
		t.Error("Expected kill switch to be inactive")
	}
}

func TestRollout(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{Name: "test", Description: "test"}
	store.Create(g)

	if err := store.Rollout(g.ID, 25); err != nil {
		t.Fatalf("Rollout: %v", err)
	}

	retrieved, _ := store.Get(g.ID)
	if retrieved.RolloutPct != 25 {
		t.Errorf("Expected 25%% rollout, got %.0f%%", retrieved.RolloutPct)
	}
	if retrieved.Status != StatusGradual {
		t.Errorf("Expected gradual status, got %s", retrieved.Status)
	}

	// Complete rollout
	store.Rollout(g.ID, 100)
	retrieved, _ = store.Get(g.ID)
	if retrieved.Status != StatusCompleted {
		t.Errorf("Expected completed status, got %s", retrieved.Status)
	}
}

func TestCheckEnabled(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{Name: "test", Description: "test"}
	store.Create(g)
	store.Enable(g.ID)

	result := store.Check(g.ID, EvaluationContext{UserID: "user-1"})
	if !result.Allowed {
		t.Error("Expected enabled gate to allow")
	}
}

func TestCheckDisabled(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{Name: "test", Description: "test"}
	store.Create(g)

	result := store.Check(g.ID, EvaluationContext{UserID: "user-1"})
	if result.Allowed {
		t.Error("Expected disabled gate to block")
	}
}

func TestCheckKilled(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{Name: "test", Description: "test"}
	store.Create(g)
	store.Enable(g.ID)
	store.Kill(g.ID)

	result := store.Check(g.ID, EvaluationContext{UserID: "user-1"})
	if result.Allowed {
		t.Error("Expected killed gate to block")
	}
}

func TestCheckTargeting(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{
		Name:        "beta",
		Description: "beta feature",
		Target: TargetRule{
			UserIDs: []string{"user-1", "user-2"},
		},
	}
	store.Create(g)
	store.Enable(g.ID)

	result := store.Check(g.ID, EvaluationContext{UserID: "user-1"})
	if !result.Allowed {
		t.Error("Expected targeted user to be allowed")
	}

	result = store.Check(g.ID, EvaluationContext{UserID: "user-999"})
	if result.Allowed {
		t.Error("Expected non-targeted user to be blocked")
	}
}

func TestCheckTagTargeting(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{
		Name:        "internal",
		Description: "internal feature",
		Target: TargetRule{
			Tags: []string{"beta-tester", "internal"},
		},
	}
	store.Create(g)
	store.Enable(g.ID)

	result := store.Check(g.ID, EvaluationContext{UserID: "user-1", Tags: []string{"beta-tester"}})
	if !result.Allowed {
		t.Error("Expected tagged user to be allowed")
	}

	result = store.Check(g.ID, EvaluationContext{UserID: "user-2", Tags: []string{"external"}})
	if result.Allowed {
		t.Error("Expected non-tagged user to be blocked")
	}
}

func TestCheckDependencies(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Create parent gate
	parent := &Gate{Name: "parent", Description: "parent feature"}
	store.Create(parent)

	// Create child gate that depends on parent
	child := &Gate{Name: "child", Description: "child feature", DependsOn: []string{parent.ID}}
	store.Create(child)

	// Child should be blocked when parent is disabled
	result := store.Check(child.ID, EvaluationContext{UserID: "user-1"})
	if result.Allowed {
		t.Error("Expected child to be blocked when parent disabled")
	}

	// Enable parent
	store.Enable(parent.ID)
	store.Enable(child.ID)

	result = store.Check(child.ID, EvaluationContext{UserID: "user-1"})
	if !result.Allowed {
		t.Error("Expected child to be allowed when parent enabled")
	}
}

func TestCheckNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	result := store.Check("nonexistent", EvaluationContext{})
	if result.Allowed {
		t.Error("Expected not found gate to block")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g := &Gate{Name: "test", Description: "test"}
	store.Create(g)
	store.Delete(g.ID)

	_, ok := store.Get(g.ID)
	if ok {
		t.Error("Expected gate to be deleted")
	}
}

func TestStats(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.Create(&Gate{Name: "a", Description: "test"})
	store.Create(&Gate{Name: "b", Description: "test"})

	stats := store.Stats()
	total, ok := stats["total"]
	if !ok || total.(int) != 2 {
		t.Errorf("Expected total 2, got %v", stats["total"])
	}
}

func TestListFiltered(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	g1 := &Gate{Name: "active", Description: "test"}
	store.Create(g1)
	store.Enable(g1.ID)

	g2 := &Gate{Name: "disabled", Description: "test"}
	store.Create(g2)

	active := store.List(StatusActive)
	if len(active) != 1 {
		t.Errorf("Expected 1 active gate, got %d", len(active))
	}

	disabled := store.List(StatusDisabled)
	if len(disabled) != 1 {
		t.Errorf("Expected 1 disabled gate, got %d", len(disabled))
	}
}
