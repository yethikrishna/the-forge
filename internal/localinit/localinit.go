// Package localinit provides zero-cloud local model presets for Forge.
// Your own forge, your own fire — no need to import fuel from distant lands.
package localinit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Preset defines a local model preset configuration.
type Preset struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Provider    string            `json:"provider"`
	Models      []ModelConfig     `json:"models"`
	Setup       []SetupStep       `json:"setup"`
	Verify      []VerifyStep      `json:"verify"`
	ForgeYAML   string            `json:"forge_yaml"`
	EnvVars     map[string]string `json:"env_vars"`
	Platform    []string          `json:"platform"`
	MinRAM      string            `json:"min_ram"`
	MinDisk     string            `json:"min_disk"`
	GPU         bool              `json:"gpu"`
}

// ModelConfig describes a model in the preset.
type ModelConfig struct {
	Name        string `json:"name"`
	ModelID     string `json:"model_id"`
	Role        string `json:"role"`
	Size        string `json:"size"`
	RAMRequired string `json:"ram_required"`
	Recommended bool   `json:"recommended"`
}

// SetupStep describes a setup step.
type SetupStep struct {
	Description string `json:"description"`
	Command     string `json:"command"`
	Optional    bool   `json:"optional"`
}

// VerifyStep describes a verification step.
type VerifyStep struct {
	Description string `json:"description"`
	Command     string `json:"command"`
	ExpectZero  bool   `json:"expect_zero"`
}

// Presets returns all available local presets.
func Presets() []Preset {
	return []Preset{
		ollamaDeepSeek(),
		ollamaQwen(),
		ollamaCommandA(),
		ollamaLlama(),
		ollamaMixtral(),
		lmStudioPreset(),
	}
}

// GetPreset returns a preset by name (case-insensitive).
func GetPreset(name string) *Preset {
	for _, p := range Presets() {
		if strings.EqualFold(p.Name, name) {
			return &p
		}
	}
	return nil
}

// GetPresetsByPlatform returns presets available for the current platform.
func GetPresetsByPlatform() []Preset {
	goos := runtime.GOOS
	var available []Preset
	for _, p := range Presets() {
		if len(p.Platform) == 0 || contains(p.Platform, goos) {
			available = append(available, p)
		}
	}
	return available
}

func ollamaDeepSeek() Preset {
	return Preset{
		Name:        "ollama-deepseek",
		Description: "Ollama + DeepSeek R1 — best open-source reasoning model, zero cloud",
		Provider:    "ollama",
		Models: []ModelConfig{
			{Name: "DeepSeek R1 8B", ModelID: "deepseek-r1:8b", Role: "primary", Size: "4.7GB", RAMRequired: "8GB", Recommended: true},
			{Name: "DeepSeek R1 14B", ModelID: "deepseek-r1:14b", Role: "primary", Size: "9GB", RAMRequired: "16GB"},
			{Name: "DeepSeek R1 32B", ModelID: "deepseek-r1:32b", Role: "primary", Size: "20GB", RAMRequired: "32GB"},
			{Name: "DeepSeek Coder V2", ModelID: "deepseek-coder-v2:16b", Role: "coder", Size: "9GB", RAMRequired: "16GB"},
		},
		Setup: []SetupStep{
			{Description: "Install Ollama", Command: "curl -fsSL https://ollama.com/install.sh | sh", Optional: false},
			{Description: "Pull DeepSeek R1 8B", Command: "ollama pull deepseek-r1:8b", Optional: false},
			{Description: "Pull DeepSeek Coder V2 (optional)", Command: "ollama pull deepseek-coder-v2:16b", Optional: true},
		},
		Verify: []VerifyStep{
			{Description: "Ollama is running", Command: "ollama list", ExpectZero: true},
			{Description: "DeepSeek R1 is available", Command: "ollama show deepseek-r1:8b", ExpectZero: true},
		},
		ForgeYAML: `# Forge local configuration — DeepSeek R1 via Ollama
# Zero cloud. All data stays on your machine.

models:
  primary: ollama/deepseek-r1:8b
  coder: ollama/deepseek-coder-v2:16b
  fallback: ollama/deepseek-r1:8b

agents:
  default:
    model: primary
    cost_cap: none  # Local models are free!
    sandbox: process

  code-review:
    model: coder
    tools: [search, build, test]

  general:
    model: primary

routing:
  code_generation: coder
  code_review: coder
  general: primary
  reasoning: primary

cost:
  budget: unlimited  # Local models have no per-token cost

data_residency:
  local_only: true
  allowed_regions: [local]

logging:
  level: info
  format: text
`,
		EnvVars: map[string]string{
			"FORGE_PROVIDER":    "ollama",
			"FORGE_OLLAMA_URL":  "http://localhost:11434",
			"FORGE_MODEL":       "deepseek-r1:8b",
			"FORGE_LOCAL_ONLY":  "true",
		},
		Platform: []string{"linux", "darwin", "windows"},
		MinRAM:   "8GB",
		MinDisk:  "10GB",
		GPU:      false,
	}
}

func ollamaQwen() Preset {
	return Preset{
		Name:        "ollama-qwen",
		Description: "Ollama + Qwen 3 — Alibaba's latest open model, excellent multilingual support",
		Provider:    "ollama",
		Models: []ModelConfig{
			{Name: "Qwen3 8B", ModelID: "qwen3:8b", Role: "primary", Size: "5GB", RAMRequired: "8GB", Recommended: true},
			{Name: "Qwen3 14B", ModelID: "qwen3:14b", Role: "primary", Size: "9GB", RAMRequired: "16GB"},
			{Name: "Qwen3 Coder 8B", ModelID: "qwen3-coder:8b", Role: "coder", Size: "5GB", RAMRequired: "8GB"},
			{Name: "Qwen2.5 Coder 32B", ModelID: "qwen2.5-coder:32b", Role: "coder", Size: "20GB", RAMRequired: "32GB"},
		},
		Setup: []SetupStep{
			{Description: "Install Ollama", Command: "curl -fsSL https://ollama.com/install.sh | sh", Optional: false},
			{Description: "Pull Qwen3 8B", Command: "ollama pull qwen3:8b", Optional: false},
			{Description: "Pull Qwen3 Coder (optional)", Command: "ollama pull qwen3-coder:8b", Optional: true},
		},
		Verify: []VerifyStep{
			{Description: "Ollama is running", Command: "ollama list", ExpectZero: true},
			{Description: "Qwen3 is available", Command: "ollama show qwen3:8b", ExpectZero: true},
		},
		ForgeYAML: `models:
  primary: ollama/qwen3:8b
  coder: ollama/qwen3-coder:8b
  fallback: ollama/qwen3:8b

agents:
  default:
    model: primary
    cost_cap: none
    sandbox: process

routing:
  code_generation: coder
  code_review: coder
  general: primary
  multilingual: primary

cost:
  budget: unlimited

data_residency:
  local_only: true
  allowed_regions: [local]
`,
		EnvVars: map[string]string{
			"FORGE_PROVIDER":   "ollama",
			"FORGE_OLLAMA_URL": "http://localhost:11434",
			"FORGE_MODEL":      "qwen3:8b",
			"FORGE_LOCAL_ONLY": "true",
		},
		Platform: []string{"linux", "darwin", "windows"},
		MinRAM:   "8GB",
		MinDisk:  "10GB",
		GPU:      false,
	}
}

func ollamaCommandA() Preset {
	return Preset{
		Name:        "ollama-command-a",
		Description: "Ollama + Cohere Command A+ — Apache 2.0 enterprise model, excellent for agentic tasks",
		Provider:    "ollama",
		Models: []ModelConfig{
			{Name: "Command R7B", ModelID: "command-r7b:latest", Role: "primary", Size: "4.5GB", RAMRequired: "8GB", Recommended: true},
			{Name: "Command R", ModelID: "command-r:latest", Role: "primary", Size: "20GB", RAMRequired: "32GB"},
		},
		Setup: []SetupStep{
			{Description: "Install Ollama", Command: "curl -fsSL https://ollama.com/install.sh | sh", Optional: false},
			{Description: "Pull Command R7B", Command: "ollama pull command-r7b:latest", Optional: false},
		},
		Verify: []VerifyStep{
			{Description: "Ollama is running", Command: "ollama list", ExpectZero: true},
			{Description: "Command R7B is available", Command: "ollama show command-r7b:latest", ExpectZero: true},
		},
		ForgeYAML: `models:
  primary: ollama/command-r7b:latest
  fallback: ollama/command-r7b:latest

agents:
  default:
    model: primary
    cost_cap: none
    sandbox: process

routing:
  general: primary
  agentic: primary
  enterprise: primary

cost:
  budget: unlimited

data_residency:
  local_only: true
  allowed_regions: [local]
`,
		EnvVars: map[string]string{
			"FORGE_PROVIDER":   "ollama",
			"FORGE_OLLAMA_URL": "http://localhost:11434",
			"FORGE_MODEL":      "command-r7b:latest",
			"FORGE_LOCAL_ONLY": "true",
		},
		Platform: []string{"linux", "darwin", "windows"},
		MinRAM:   "8GB",
		MinDisk:  "8GB",
		GPU:      false,
	}
}

func ollamaLlama() Preset {
	return Preset{
		Name:        "ollama-llama",
		Description: "Ollama + Llama 4 — Meta's flagship open model, best for general-purpose tasks",
		Provider:    "ollama",
		Models: []ModelConfig{
			{Name: "Llama 3.3 8B", ModelID: "llama3.3:8b", Role: "primary", Size: "5GB", RAMRequired: "8GB", Recommended: true},
			{Name: "Llama 3.3 70B", ModelID: "llama3.3:70b", Role: "primary", Size: "42GB", RAMRequired: "64GB"},
			{Name: "Llama 3.2 Vision 11B", ModelID: "llama3.2-vision:11b", Role: "vision", Size: "7GB", RAMRequired: "16GB"},
		},
		Setup: []SetupStep{
			{Description: "Install Ollama", Command: "curl -fsSL https://ollama.com/install.sh | sh", Optional: false},
			{Description: "Pull Llama 3.3 8B", Command: "ollama pull llama3.3:8b", Optional: false},
		},
		Verify: []VerifyStep{
			{Description: "Ollama is running", Command: "ollama list", ExpectZero: true},
		},
		ForgeYAML: `models:
  primary: ollama/llama3.3:8b
  vision: ollama/llama3.2-vision:11b
  fallback: ollama/llama3.3:8b

agents:
  default:
    model: primary
    cost_cap: none
    sandbox: process

routing:
  general: primary
  vision: vision
  code_generation: primary

cost:
  budget: unlimited

data_residency:
  local_only: true
  allowed_regions: [local]
`,
		EnvVars: map[string]string{
			"FORGE_PROVIDER":   "ollama",
			"FORGE_OLLAMA_URL": "http://localhost:11434",
			"FORGE_MODEL":      "llama3.3:8b",
			"FORGE_LOCAL_ONLY": "true",
		},
		Platform: []string{"linux", "darwin", "windows"},
		MinRAM:   "8GB",
		MinDisk:  "10GB",
		GPU:      false,
	}
}

func ollamaMixtral() Preset {
	return Preset{
		Name:        "ollama-mixtral",
		Description: "Ollama + Mistral/Mixtral — European-made, strong on code and reasoning",
		Provider:    "ollama",
		Models: []ModelConfig{
			{Name: "Mistral 7B", ModelID: "mistral:7b", Role: "primary", Size: "4.1GB", RAMRequired: "8GB", Recommended: true},
			{Name: "Mixtral 8x7B", ModelID: "mixtral:8x7b", Role: "primary", Size: "26GB", RAMRequired: "32GB"},
			{Name: "Codestral 22B", ModelID: "codestral:22b", Role: "coder", Size: "13GB", RAMRequired: "16GB"},
		},
		Setup: []SetupStep{
			{Description: "Install Ollama", Command: "curl -fsSL https://ollama.com/install.sh | sh", Optional: false},
			{Description: "Pull Mistral 7B", Command: "ollama pull mistral:7b", Optional: false},
			{Description: "Pull Codestral (optional)", Command: "ollama pull codestral:22b", Optional: true},
		},
		Verify: []VerifyStep{
			{Description: "Ollama is running", Command: "ollama list", ExpectZero: true},
		},
		ForgeYAML: `models:
  primary: ollama/mistral:7b
  coder: ollama/codestral:22b
  fallback: ollama/mistral:7b

agents:
  default:
    model: primary
    cost_cap: none
    sandbox: process

routing:
  general: primary
  code_generation: coder
  code_review: coder

cost:
  budget: unlimited

data_residency:
  local_only: true
  allowed_regions: [local]
`,
		EnvVars: map[string]string{
			"FORGE_PROVIDER":   "ollama",
			"FORGE_OLLAMA_URL": "http://localhost:11434",
			"FORGE_MODEL":      "mistral:7b",
			"FORGE_LOCAL_ONLY": "true",
		},
		Platform: []string{"linux", "darwin", "windows"},
		MinRAM:   "8GB",
		MinDisk:  "8GB",
		GPU:      false,
	}
}

func lmStudioPreset() Preset {
	return Preset{
		Name:        "lmstudio",
		Description: "LM Studio — GUI-based local model runner with OpenAI-compatible API",
		Provider:    "lmstudio",
		Models: []ModelConfig{
			{Name: "Any GGUF model", ModelID: "user-selected", Role: "primary", Size: "varies", RAMRequired: "8GB", Recommended: true},
		},
		Setup: []SetupStep{
			{Description: "Download LM Studio from https://lmstudio.ai", Command: "", Optional: false},
			{Description: "Start LM Studio and download a model", Command: "", Optional: false},
			{Description: "Enable the local API server in LM Studio", Command: "", Optional: false},
		},
		Verify: []VerifyStep{
			{Description: "LM Studio API is running", Command: "curl -s http://localhost:1234/v1/models", ExpectZero: false},
		},
		ForgeYAML: `models:
  primary: lmstudio/default

agents:
  default:
    model: primary
    cost_cap: none
    sandbox: process

cost:
  budget: unlimited

data_residency:
  local_only: true
`,
		EnvVars: map[string]string{
			"FORGE_PROVIDER":   "lmstudio",
			"FORGE_LMSTUDIO_URL": "http://localhost:1234/v1",
			"FORGE_LOCAL_ONLY": "true",
		},
		Platform: []string{"linux", "darwin", "windows"},
		MinRAM:   "8GB",
		MinDisk:  "10GB",
		GPU:      true,
	}
}

// LocalInit initializes a project with a local preset.
type LocalInit struct {
	Preset  *Preset
	Dir     string
	Verbose bool
}

// NewLocalInit creates a local initializer.
func NewLocalInit(presetName string, dir string) (*LocalInit, error) {
	preset := GetPreset(presetName)
	if preset == nil {
		available := make([]string, len(Presets()))
		for i, p := range Presets() {
			available[i] = p.Name
		}
		return nil, fmt.Errorf("preset %q not found. Available: %s", presetName, strings.Join(available, ", "))
	}

	return &LocalInit{Preset: preset, Dir: dir}, nil
}

// Run executes the local initialization.
func (li *LocalInit) Run() error {
	fmt.Printf("🏔️ Initializing Forge with %s preset...\n", li.Preset.Name)
	fmt.Printf("   %s\n\n", li.Preset.Description)

	// Create directory
	if li.Dir == "" {
		li.Dir = "."
	}
	if err := os.MkdirAll(li.Dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// Write forge.yaml
	yamlPath := filepath.Join(li.Dir, "forge.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		fmt.Printf("   forge.yaml already exists, backing up...\n")
		os.Rename(yamlPath, yamlPath+".bak")
	}
	if err := os.WriteFile(yamlPath, []byte(li.Preset.ForgeYAML), 0o644); err != nil {
		return fmt.Errorf("write forge.yaml: %w", err)
	}
	fmt.Printf("   ✓ Created forge.yaml\n")

	// Write .env file
	envPath := filepath.Join(li.Dir, ".env")
	var envLines []string
	envLines = append(envLines, "# Forge local environment — "+li.Preset.Name)
	envLines = append(envLines, "# Generated by forge init --local "+li.Preset.Name)
	envLines = append(envLines, "")
	for k, v := range li.Preset.EnvVars {
		envLines = append(envLines, fmt.Sprintf("%s=%s", k, v))
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(envLines, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}
	fmt.Printf("   ✓ Created .env\n")

	// Create .forge directory
	forgeDir := filepath.Join(li.Dir, ".forge")
	os.MkdirAll(forgeDir, 0o755)

	// Write preset info
	infoPath := filepath.Join(forgeDir, "preset.json")
	infoData, _ := json.MarshalIndent(li.Preset, "", "  ")
	os.WriteFile(infoPath, infoData, 0o644)

	// Write setup instructions
	instructions := li.FormatInstructions()
	instrPath := filepath.Join(forgeDir, "SETUP.md")
	os.WriteFile(instrPath, []byte(instructions), 0o644)
	fmt.Printf("   ✓ Created .forge/SETUP.md\n")

	fmt.Printf("\n   🎉 Local preset initialized!\n")
	fmt.Printf("   Next steps:\n")
	fmt.Printf("     1. Follow .forge/SETUP.md to install dependencies\n")
	fmt.Printf("     2. Run: forge doctor --local\n")
	fmt.Printf("     3. Start: forge chat\n")
	fmt.Printf("\n   Your data never leaves this machine. 🏔️\n")

	return nil
}

// FormatInstructions returns setup instructions as markdown.
func (li *LocalInit) FormatInstructions() string {
	var sb strings.Builder
	sb.WriteString("# Forge Local Setup — ")
	sb.WriteString(li.Preset.Name)
	sb.WriteString("\n\n")
	sb.WriteString(li.Preset.Description)
	sb.WriteString("\n\n")

	sb.WriteString("## Requirements\n")
	sb.WriteString(fmt.Sprintf("- **RAM:** %s\n", li.Preset.MinRAM))
	sb.WriteString(fmt.Sprintf("- **Disk:** %s\n", li.Preset.MinDisk))
	if li.Preset.GPU {
		sb.WriteString("- **GPU:** Recommended (not required)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Setup Steps\n")
	for i, step := range li.Preset.Setup {
		optional := ""
		if step.Optional {
			optional = " (optional)"
		}
		sb.WriteString(fmt.Sprintf("%d. %s%s\n", i+1, step.Description, optional))
		if step.Command != "" {
			sb.WriteString(fmt.Sprintf("   ```\n   %s\n   ```\n", step.Command))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Available Models\n")
	sb.WriteString("| Model | ID | Role | Size | RAM | Recommended |\n")
	sb.WriteString("|-------|-----|------|------|-----|-------------|\n")
	for _, m := range li.Preset.Models {
		rec := ""
		if m.Recommended {
			rec = "✅"
		}
		sb.WriteString(fmt.Sprintf("| %s | `%s` | %s | %s | %s | %s |\n", m.Name, m.ModelID, m.Role, m.Size, m.RAMRequired, rec))
	}
	sb.WriteString("\n")

	sb.WriteString("## Verification\n")
	for _, v := range li.Preset.Verify {
		sb.WriteString(fmt.Sprintf("- %s: `%s`\n", v.Description, v.Command))
	}
	sb.WriteString("\n")

	sb.WriteString("## Environment Variables\n")
	for k, v := range li.Preset.EnvVars {
		sb.WriteString(fmt.Sprintf("- `%s=%s`\n", k, v))
	}

	return sb.String()
}

// FormatPresets renders all available presets for display.
func FormatPresets(presets []Preset) string {
	var sb strings.Builder
	sb.WriteString("Available Local Presets:\n\n")
	for _, p := range presets {
		gpu := ""
		if p.GPU {
			gpu = " [GPU recommended]"
		}
		sb.WriteString(fmt.Sprintf("  %-20s %s (RAM: %s, Disk: %s%s)\n", p.Name, p.Description, p.MinRAM, p.MinDisk, gpu))
		for _, m := range p.Models {
			rec := ""
			if m.Recommended {
				rec = " ⭐"
			}
			sb.WriteString(fmt.Sprintf("    %-30s %s  %s  %s%s\n", m.ModelID, m.Role, m.Size, m.RAMRequired, rec))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
