package localinit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPresets(t *testing.T) {
	presets := Presets()
	if len(presets) < 5 {
		t.Fatalf("expected at least 5 presets, got %d", len(presets))
	}
}

func TestGetPreset(t *testing.T) {
	p := GetPreset("ollama-deepseek")
	if p == nil {
		t.Fatal("expected ollama-deepseek preset")
	}
	if p.Provider != "ollama" {
		t.Fatalf("expected ollama provider, got %s", p.Provider)
	}

	p = GetPreset("OLLAMA-QWEN")
	if p == nil {
		t.Fatal("case-insensitive lookup failed")
	}

	p = GetPreset("nonexistent")
	if p != nil {
		t.Fatal("expected nil for nonexistent preset")
	}
}

func TestGetPresetsByPlatform(t *testing.T) {
	presets := GetPresetsByPlatform()
	if len(presets) == 0 {
		t.Fatal("expected at least some platform-compatible presets")
	}
}

func TestLocalInit(t *testing.T) {
	dir := t.TempDir()
	li, err := NewLocalInit("ollama-deepseek", dir)
	if err != nil {
		t.Fatalf("NewLocalInit: %v", err)
	}

	if err := li.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Check forge.yaml was created
	yamlPath := filepath.Join(dir, "forge.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("forge.yaml not created: %v", err)
	}

	// Check .env was created
	envPath := filepath.Join(dir, ".env")
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf(".env not created: %v", err)
	}

	// Check .forge/preset.json
	infoPath := filepath.Join(dir, ".forge", "preset.json")
	if _, err := os.Stat(infoPath); err != nil {
		t.Fatalf(".forge/preset.json not created: %v", err)
	}

	// Check .forge/SETUP.md
	setupPath := filepath.Join(dir, ".forge", "SETUP.md")
	if _, err := os.Stat(setupPath); err != nil {
		t.Fatalf(".forge/SETUP.md not created: %v", err)
	}
}

func TestLocalInitBackup(t *testing.T) {
	dir := t.TempDir()

	// Create existing forge.yaml
	yamlPath := filepath.Join(dir, "forge.yaml")
	os.WriteFile(yamlPath, []byte("old content"), 0o644)

	li, _ := NewLocalInit("ollama-qwen", dir)
	if err := li.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Check backup was created
	backupPath := yamlPath + ".bak"
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("forge.yaml.bak not created: %v", err)
	}
}

func TestLocalInitInvalidPreset(t *testing.T) {
	_, err := NewLocalInit("nonexistent", "/tmp")
	if err == nil {
		t.Fatal("expected error for invalid preset")
	}
}

func TestFormatInstructions(t *testing.T) {
	li, _ := NewLocalInit("ollama-deepseek", "/tmp")
	instructions := li.FormatInstructions()
	if len(instructions) == 0 {
		t.Fatal("empty instructions")
	}
	// Should contain model info
	if len(li.Preset.Models) == 0 {
		t.Fatal("preset has no models")
	}
}

func TestFormatPresets(t *testing.T) {
	output := FormatPresets(Presets())
	if len(output) == 0 {
		t.Fatal("empty presets output")
	}
}

func TestAllPresetsHaveRequiredFields(t *testing.T) {
	for _, p := range Presets() {
		if p.Name == "" {
			t.Error("preset missing name")
		}
		if p.Description == "" {
			t.Errorf("preset %s missing description", p.Name)
		}
		if p.Provider == "" {
			t.Errorf("preset %s missing provider", p.Name)
		}
		if len(p.Models) == 0 {
			t.Errorf("preset %s has no models", p.Name)
		}
		if p.ForgeYAML == "" {
			t.Errorf("preset %s missing forge_yaml", p.Name)
		}
		if len(p.EnvVars) == 0 {
			t.Errorf("preset %s has no env vars", p.Name)
		}
		if p.MinRAM == "" {
			t.Errorf("preset %s missing min_ram", p.Name)
		}
	}
}

func TestModelConfigs(t *testing.T) {
	for _, p := range Presets() {
		for _, m := range p.Models {
			if m.ModelID == "" {
				t.Errorf("preset %s: model missing model_id", p.Name)
			}
			if m.Role == "" {
				t.Errorf("preset %s: model %s missing role", p.Name, m.ModelID)
			}
			if m.RAMRequired == "" {
				t.Errorf("preset %s: model %s missing ram_required", p.Name, m.ModelID)
			}
		}
	}
}
