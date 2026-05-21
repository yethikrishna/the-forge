package persona

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreatePersona(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	p := &Persona{
		Name:        "test-coder",
		Description: "A test coding persona",
		Style:       Style{Tone: "technical", Verbosity: "concise"},
	}

	if err := store.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if p.ID == "" {
		t.Error("Expected ID to be set")
	}
	if p.TrustLevel != TrustStandard {
		t.Errorf("Expected standard trust, got %s", p.TrustLevel)
	}

	// Verify persisted
	if _, err := os.Stat(filepath.Join(dir, p.ID+".json")); err != nil {
		t.Fatalf("File not persisted: %v", err)
	}
}

func TestGetPersona(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	p := &Persona{Name: "coder", Description: "test"}
	store.Create(p)

	retrieved, ok := store.Get(p.ID)
	if !ok {
		t.Fatal("Expected to find persona")
	}
	if retrieved.Name != "coder" {
		t.Errorf("Expected 'coder', got %q", retrieved.Name)
	}
}

func TestGetByName(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.Create(&Persona{Name: "reviewer", Description: "test"})

	retrieved, ok := store.GetByName("reviewer")
	if !ok {
		t.Fatal("Expected to find persona by name")
	}
	if retrieved.Name != "reviewer" {
		t.Errorf("Expected 'reviewer', got %q", retrieved.Name)
	}
}

func TestListPersonas(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.Create(&Persona{Name: "alpha", Description: "test"})
	store.Create(&Persona{Name: "beta", Description: "test"})

	list := store.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 personas, got %d", len(list))
	}
}

func TestUpdatePersona(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	p := &Persona{Name: "test", Description: "original"}
	store.Create(p)

	p.Description = "updated"
	if err := store.Update(p); err != nil {
		t.Fatalf("Update: %v", err)
	}

	retrieved, _ := store.Get(p.ID)
	if retrieved.Description != "updated" {
		t.Errorf("Expected 'updated', got %q", retrieved.Description)
	}
}

func TestDeletePersona(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	p := &Persona{Name: "test", Description: "test"}
	store.Create(p)

	if err := store.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, ok := store.Get(p.ID)
	if ok {
		t.Error("Expected persona to be deleted")
	}
}

func TestRecordUse(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	p := &Persona{Name: "test", Description: "test"}
	store.Create(p)

	store.RecordUse(p.ID)
	store.RecordUse(p.ID)

	retrieved, _ := store.Get(p.ID)
	if retrieved.UseCount != 2 {
		t.Errorf("Expected 2 uses, got %d", retrieved.UseCount)
	}
}

func TestUpdateTrust(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	p := &Persona{Name: "test", Description: "test", TrustScore: 50}
	store.Create(p)

	store.UpdateTrust(p.ID, 30)

	retrieved, _ := store.Get(p.ID)
	if retrieved.TrustScore != 80 {
		t.Errorf("Expected trust score 80, got %.0f", retrieved.TrustScore)
	}
	if retrieved.TrustLevel != TrustTrusted {
		t.Errorf("Expected trusted level, got %s", retrieved.TrustLevel)
	}

	// Test negative delta
	store.UpdateTrust(p.ID, -100)
	retrieved, _ = store.Get(p.ID)
	if retrieved.TrustScore != 0 {
		t.Errorf("Expected trust score 0 (clamped), got %.0f", retrieved.TrustScore)
	}
	if retrieved.TrustLevel != TrustUntrusted {
		t.Errorf("Expected untrusted level, got %s", retrieved.TrustLevel)
	}
}

func TestPreferences(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	p := &Persona{Name: "test", Description: "test"}
	store.Create(p)

	store.SetPreference(p.ID, "language", "go", 5)
	store.SetPreference(p.ID, "editor", "vim", 3)

	val, ok := store.GetPreference(p.ID, "language")
	if !ok || val != "go" {
		t.Errorf("Expected 'go', got %q", val)
	}

	// Update existing preference
	store.SetPreference(p.ID, "language", "python", 5)
	val, _ = store.GetPreference(p.ID, "language")
	if val != "python" {
		t.Errorf("Expected 'python' after update, got %q", val)
	}
}

func TestDefaultPersonas(t *testing.T) {
	defaults := DefaultPersonas()
	if len(defaults) < 3 {
		t.Errorf("Expected at least 3 default personas, got %d", len(defaults))
	}

	names := make(map[string]bool)
	for _, p := range defaults {
		if p.Name == "" {
			t.Error("Expected non-empty name")
		}
		if names[p.Name] {
			t.Errorf("Duplicate persona name: %s", p.Name)
		}
		names[p.Name] = true
	}
}

func TestFormatSystemPrompt(t *testing.T) {
	p := &Persona{
		Name:        "coder",
		Description: "Technical coding assistant",
		Style: Style{
			Tone:        "technical",
			Verbosity:   "concise",
			Proactivity: 0.8,
			CodeBlocks:  true,
		},
		Preferences: []Preference{
			{Key: "language", Value: "go", Priority: 5},
			{Key: "testing", Value: "always write tests", Priority: 4},
		},
		MaxCost: 5.00,
		Scope:   "full",
	}

	prompt := FormatSystemPrompt(p)
	if prompt == "" {
		t.Error("Expected non-empty system prompt")
	}
	if len(prompt) < 100 {
		t.Error("Expected detailed system prompt")
	}
}

func TestLoadExistingPersonas(t *testing.T) {
	dir := t.TempDir()

	// Create a persona file manually
	p := Persona{
		ID:          "test-persona",
		Name:        "Pre-existing",
		Description: "Loaded from disk",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		TrustLevel:  TrustStandard,
		TrustScore:  50,
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "test-persona.json"), data, 0644)

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	retrieved, ok := store.Get("test-persona")
	if !ok {
		t.Fatal("Expected to load pre-existing persona")
	}
	if retrieved.Name != "Pre-existing" {
		t.Errorf("Expected 'Pre-existing', got %q", retrieved.Name)
	}
}

func jsonMarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}
