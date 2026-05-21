package desktop

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDesktopCreate(t *testing.T) {
	dir := t.TempDir()
	dm := NewDesktopManager(dir)

	d, err := dm.Create(DesktopConfig{Name: "test-desktop"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.State != DesktopCreated {
		t.Errorf("expected created, got %s", d.State)
	}
	if d.Config.Width != 1280 || d.Config.Height != 720 {
		t.Errorf("expected 1280x720, got %dx%d", d.Config.Width, d.Config.Height)
	}
	if d.Config.Depth != 24 {
		t.Errorf("expected depth 24, got %d", d.Config.Depth)
	}
	if d.Config.Display != ":99" {
		t.Errorf("expected :99, got %s", d.Config.Display)
	}
}

func TestDesktopCreateCustom(t *testing.T) {
	dir := t.TempDir()
	dm := NewDesktopManager(dir)

	d, err := dm.Create(DesktopConfig{
		Name:    "custom",
		Width:   1920,
		Height:  1080,
		Display: ":5",
		Shell:   "/bin/zsh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Config.Width != 1920 {
		t.Errorf("expected 1920, got %d", d.Config.Width)
	}
	if d.Config.Display != ":5" {
		t.Errorf("expected :5, got %s", d.Config.Display)
	}
	if d.Config.Shell != "/bin/zsh" {
		t.Errorf("expected /bin/zsh, got %s", d.Config.Shell)
	}
}

func TestDesktopGet(t *testing.T) {
	dir := t.TempDir()
	dm := NewDesktopManager(dir)

	d, _ := dm.Create(DesktopConfig{Name: "find-me"})
	found, err := dm.Get(d.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found.Name != "find-me" {
		t.Errorf("expected find-me, got %s", found.Name)
	}
}

func TestDesktopGetNotFound(t *testing.T) {
	dir := t.TempDir()
	dm := NewDesktopManager(dir)

	_, err := dm.Get("nonexistent")
	if err == nil {
		t.Error("expected not found")
	}
}

func TestDesktopList(t *testing.T) {
	dir := t.TempDir()
	dm := NewDesktopManager(dir)

	dm.Create(DesktopConfig{Name: "a"})
	dm.Create(DesktopConfig{Name: "b"})

	desks := dm.List()
	if len(desks) != 2 {
		t.Errorf("expected 2, got %d", len(desks))
	}
}

func TestDesktopStop(t *testing.T) {
	dir := t.TempDir()
	dm := NewDesktopManager(dir)

	d, _ := dm.Create(DesktopConfig{Name: "stop-test"})
	// Start sets state to running (without Xvfb in test)
	d.State = DesktopRunning
	dm.persist(d)

	if err := dm.Stop(context.Background(), d.ID); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stopped, _ := dm.Get(d.ID)
	if stopped.State != DesktopStopped {
		t.Errorf("expected stopped, got %s", stopped.State)
	}
}

func TestDesktopDestroy(t *testing.T) {
	dir := t.TempDir()
	dm := NewDesktopManager(dir)

	d, _ := dm.Create(DesktopConfig{Name: "destroy-me"})
	dm.Destroy(context.Background(), d.ID)

	if len(dm.List()) != 0 {
		t.Error("expected 0 after destroy")
	}
}

func TestDesktopPersistence(t *testing.T) {
	dir := t.TempDir()
	dm := NewDesktopManager(dir)

	d, _ := dm.Create(DesktopConfig{Name: "persist-test", Width: 1920})

	dm2 := NewDesktopManager(dir)
	loaded, err := dm2.Get(d.ID)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Name != "persist-test" {
		t.Errorf("expected persist-test, got %s", loaded.Name)
	}
	if loaded.Config.Width != 1920 {
		t.Errorf("expected 1920, got %d", loaded.Config.Width)
	}
}

func TestDesktopSerialization(t *testing.T) {
	d := &Desktop{
		ID:   "desk-1",
		Name: "test",
		Config: DesktopConfig{
			Width:  1920,
			Height: 1080,
			Display: ":0",
		},
		State: DesktopRunning,
		URL:   "http://localhost:6080",
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}

	var d2 Desktop
	json.Unmarshal(data, &d2)
	if d2.Config.Width != 1920 {
		t.Errorf("expected 1920, got %d", d2.Config.Width)
	}
	if d2.URL != "http://localhost:6080" {
		t.Errorf("unexpected URL: %s", d2.URL)
	}
}
