package migrate_test

import (
	"testing"

	"github.com/forge/sword/internal/migrate"
)

func TestCreate(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	mig, err := m.Create("create users table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mig.Version != 1 {
		t.Errorf("expected version 1, got %d", mig.Version)
	}
	if mig.Status != migrate.StatusPending {
		t.Errorf("expected pending, got %s", mig.Status)
	}
}

func TestCreateDuplicate(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	m.Create("create users")
	_, err := m.Create("create users")
	if err == nil {
		t.Error("expected error for duplicate name")
	}
}

func TestApply(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	m.Create("first migration")
	m.Create("second migration")

	applied, err := m.Apply(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 2 {
		t.Errorf("expected 2 applied, got %d", len(applied))
	}
}

func TestApplyTargetVersion(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	m.Create("first")
	m.Create("second")
	m.Create("third")

	applied, err := m.Apply(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 2 {
		t.Errorf("expected 2 applied, got %d", len(applied))
	}
}

func TestRollback(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	m.Create("first")
	m.Create("second")
	m.Apply(0)

	rolled, err := m.Rollback(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rolled) != 1 {
		t.Errorf("expected 1 rolled back, got %d", len(rolled))
	}
}

func TestCurrentVersion(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	if m.CurrentVersion() != 0 {
		t.Error("expected version 0")
	}

	m.Create("first")
	m.Apply(0)
	if m.CurrentVersion() != 1 {
		t.Errorf("expected version 1, got %d", m.CurrentVersion())
	}
}

func TestPending(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	m.Create("first")
	m.Create("second")
	m.Apply(1) // only apply first

	pending := m.Pending()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
}

func TestList(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	m.Create("first")
	m.Create("second")

	list := m.List()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestGet(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	mig, _ := m.Create("test migration")

	got, ok := m.Get(mig.ID)
	if !ok {
		t.Error("expected to find migration")
	}
	if got.Name != "test migration" {
		t.Errorf("expected 'test migration', got %s", got.Name)
	}
}

func TestStats(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	m.Create("first")
	m.Apply(0)

	stats := m.Stats()
	if stats["current_version"].(int) != 1 {
		t.Errorf("expected version 1, got %v", stats["current_version"])
	}
}

func TestRenderMigration(t *testing.T) {
	m := migrate.NewManager(t.TempDir())
	mig, _ := m.Create("test")
	text := migrate.RenderMigration(mig)
	if text == "" {
		t.Error("expected non-empty render")
	}
}
