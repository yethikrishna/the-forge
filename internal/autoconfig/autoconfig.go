// Package autoconfig provides zero-config auto-detection for forge.
// Detects API keys from environment, project type from files,
// and git remote → workspace mapping automatically.
//
// Convention over configuration.
package autoconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DetectedConfig holds all auto-detected configuration.
type DetectedConfig struct {
	ProjectType   ProjectType `json:"project_type"`
	Language      string      `json:"language"`
	PackageMgr    string      `json:"package_manager"`
	TestFramework string      `json:"test_framework"`
	BuildTool     string      `json:"build_tool"`
	GitRemote     string      `json:"git_remote"`
	GitBranch     string      `json:"git_branch"`
	APIKeys       []string    `json:"detected_api_keys"` // names only, never values
	Editor        string      `json:"editor"`
	HasDocker     bool        `json:"has_docker"`
	HasCI         bool        `json:"has_ci"`
	HasTests      bool        `json:"has_tests"`
	HasLinter     bool        `json:"has_linter"`
	HasFormatter  bool        `json:"has_formatter"`
	Confidence    float64     `json:"confidence"` // 0-1
}

// ProjectType is the type of detected project.
type ProjectType string

const (
	ProjectGo      ProjectType = "go"
	ProjectPython  ProjectType = "python"
	ProjectNode    ProjectType = "node"
	ProjectRust    ProjectType = "rust"
	ProjectJava    ProjectType = "java"
	ProjectRuby    ProjectType = "ruby"
	ProjectUnknown ProjectType = "unknown"
	ProjectMulti   ProjectType = "multi"
)

// Detector auto-detects project configuration.
type Detector struct {
	rootDir string
}

// NewDetector creates an auto-detector for a directory.
func NewDetector(rootDir string) *Detector {
	return &Detector{rootDir: rootDir}
}

// Detect runs all detection heuristics.
func (d *Detector) Detect() *DetectedConfig {
	cfg := &DetectedConfig{}

	d.detectProjectType(cfg)
	d.detectGit(cfg)
	d.detectAPIKeys(cfg)
	d.detectTools(cfg)
	d.detectEditor(cfg)
	d.calculateConfidence(cfg)

	return cfg
}

// detectProjectType detects the project type from files.
func (d *Detector) detectProjectType(cfg *DetectedConfig) {
	detected := map[ProjectType]int{}

	// Go
	if d.fileExists("go.mod") {
		detected[ProjectGo] += 3
		cfg.PackageMgr = "go modules"
		cfg.Language = "Go"
	}
	if d.fileExists("go.sum") {
		detected[ProjectGo] += 1
	}
	if d.hasFiles("*.go") {
		detected[ProjectGo] += 2
		cfg.BuildTool = "go build"
		cfg.TestFramework = "go test"
	}

	// Python
	if d.fileExists("requirements.txt") || d.fileExists("Pipfile") || d.fileExists("pyproject.toml") {
		detected[ProjectPython] += 3
		cfg.Language = "Python"
		if d.fileExists("Pipfile") {
			cfg.PackageMgr = "pipenv"
		} else if d.fileExists("pyproject.toml") {
			cfg.PackageMgr = "poetry"
		} else {
			cfg.PackageMgr = "pip"
		}
	}
	if d.fileExists("setup.py") || d.fileExists("setup.cfg") {
		detected[ProjectPython] += 2
	}
	if d.hasFiles("*.py") {
		detected[ProjectPython] += 1
		cfg.TestFramework = "pytest"
	}

	// Node
	if d.fileExists("package.json") {
		detected[ProjectNode] += 3
		cfg.Language = "JavaScript/TypeScript"
		cfg.PackageMgr = "npm"
		cfg.BuildTool = "npm run build"
		cfg.TestFramework = "jest"
	}
	if d.fileExists("yarn.lock") {
		detected[ProjectNode] += 1
		cfg.PackageMgr = "yarn"
	}
	if d.fileExists("pnpm-lock.yaml") {
		detected[ProjectNode] += 1
		cfg.PackageMgr = "pnpm"
	}
	if d.fileExists("tsconfig.json") {
		detected[ProjectNode] += 1
		cfg.Language = "TypeScript"
	}

	// Rust
	if d.fileExists("Cargo.toml") {
		detected[ProjectRust] += 3
		cfg.Language = "Rust"
		cfg.PackageMgr = "cargo"
		cfg.BuildTool = "cargo build"
		cfg.TestFramework = "cargo test"
	}

	// Java
	if d.fileExists("pom.xml") {
		detected[ProjectJava] += 3
		cfg.Language = "Java"
		cfg.PackageMgr = "maven"
		cfg.BuildTool = "mvn"
	}
	if d.fileExists("build.gradle") || d.fileExists("build.gradle.kts") {
		detected[ProjectJava] += 3
		cfg.Language = "Java/Kotlin"
		cfg.PackageMgr = "gradle"
		cfg.BuildTool = "gradle"
	}

	// Ruby
	if d.fileExists("Gemfile") {
		detected[ProjectRuby] += 3
		cfg.Language = "Ruby"
		cfg.PackageMgr = "bundler"
	}

	// Determine project type
	count := 0
	for pt, score := range detected {
		if score > 0 {
			count++
		}
		if score > 2 && (cfg.ProjectType == "" || score > detected[cfg.ProjectType]) {
			cfg.ProjectType = pt
		}
	}

	if count > 1 {
		cfg.ProjectType = ProjectMulti
	} else if count == 0 {
		cfg.ProjectType = ProjectUnknown
	}
}

// detectGit detects git configuration.
func (d *Detector) detectGit(cfg *DetectedConfig) {
	gitDir := filepath.Join(d.rootDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return
	}

	// Read HEAD for branch
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err == nil {
		line := strings.TrimSpace(string(head))
		if strings.HasPrefix(line, "ref: refs/heads/") {
			cfg.GitBranch = strings.TrimPrefix(line, "ref: refs/heads/")
		}
	}

	// Read config for remote
	config, err := os.ReadFile(filepath.Join(gitDir, "config"))
	if err == nil {
		for _, line := range strings.Split(string(config), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "url = ") {
				cfg.GitRemote = strings.TrimPrefix(line, "url = ")
				break
			}
		}
	}
}

// detectAPIKeys detects available API keys from environment.
func (d *Detector) detectAPIKeys(cfg *DetectedConfig) {
	keyEnvs := []string{
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"GEMINI_API_KEY",
		"AZURE_OPENAI_API_KEY",
		"AWS_ACCESS_KEY_ID",
		"GITHUB_TOKEN",
		"GITLAB_TOKEN",
		"SLACK_TOKEN",
	}

	for _, key := range keyEnvs {
		if os.Getenv(key) != "" {
			cfg.APIKeys = append(cfg.APIKeys, key)
		}
	}
}

// detectTools detects available development tools.
func (d *Detector) detectTools(cfg *DetectedConfig) {
	// Docker
	if _, err := os.Stat(filepath.Join(d.rootDir, "Dockerfile")); err == nil {
		cfg.HasDocker = true
	}
	if _, err := os.Stat(filepath.Join(d.rootDir, "docker-compose.yml")); err == nil {
		cfg.HasDocker = true
	}

	// CI
	ciFiles := []string{
		".github/workflows", ".gitlab-ci.yml", ".circleci/config.yml",
		"Jenkinsfile", ".travis.yml",
	}
	for _, f := range ciFiles {
		if d.fileExists(f) {
			cfg.HasCI = true
			break
		}
	}

	// Tests
	testDirs := []string{"test", "tests", "__tests__", "spec", "testdata"}
	for _, dir := range testDirs {
		path := filepath.Join(d.rootDir, dir)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			cfg.HasTests = true
			break
		}
	}
	if d.hasFiles("*_test.go") || d.hasFiles("test_*.py") {
		cfg.HasTests = true
	}

	// Linter
	linterFiles := []string{".golangci.yml", ".eslintrc", ".eslintrc.js", ".pylintrc", ".rubocop.yml", "clippy.toml"}
	for _, f := range linterFiles {
		if d.fileExists(f) {
			cfg.HasLinter = true
			break
		}
	}

	// Formatter
	formatterFiles := []string{".prettierrc", ".prettierrc.js", ".editorconfig"}
	for _, f := range formatterFiles {
		if d.fileExists(f) {
			cfg.HasFormatter = true
			break
		}
	}
}

// detectEditor detects the configured editor.
func (d *Detector) detectEditor(cfg *DetectedConfig) {
	if editor := os.Getenv("EDITOR"); editor != "" {
		cfg.Editor = editor
	} else if visual := os.Getenv("VISUAL"); visual != "" {
		cfg.Editor = visual
	}
}

// calculateConfidence calculates detection confidence.
func (d *Detector) calculateConfidence(cfg *DetectedConfig) {
	score := 0.0
	total := 8.0

	if cfg.ProjectType != ProjectUnknown {
		score += 2
	}
	if cfg.Language != "" {
		score += 1
	}
	if cfg.PackageMgr != "" {
		score += 1
	}
	if cfg.GitRemote != "" {
		score += 1
	}
	if cfg.GitBranch != "" {
		score += 0.5
	}
	if len(cfg.APIKeys) > 0 {
		score += 1
	}
	if cfg.HasTests {
		score += 0.5
	}
	if cfg.HasCI {
		score += 0.5
	}
	if cfg.HasDocker {
		score += 0.5
	}

	cfg.Confidence = score / total
	if cfg.Confidence > 1.0 {
		cfg.Confidence = 1.0
	}
}

func (d *Detector) fileExists(name string) bool {
	_, err := os.Stat(filepath.Join(d.rootDir, name))
	return err == nil
}

func (d *Detector) hasFiles(pattern string) bool {
	matches, _ := filepath.Glob(filepath.Join(d.rootDir, pattern))
	return len(matches) > 0
}

// FormatConfig formats detected config for display.
func FormatConfig(c *DetectedConfig) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Project:     %s\n", c.ProjectType))
	if c.Language != "" {
		b.WriteString(fmt.Sprintf("Language:    %s\n", c.Language))
	}
	if c.PackageMgr != "" {
		b.WriteString(fmt.Sprintf("Package Mgr: %s\n", c.PackageMgr))
	}
	if c.BuildTool != "" {
		b.WriteString(fmt.Sprintf("Build Tool:  %s\n", c.BuildTool))
	}
	if c.TestFramework != "" {
		b.WriteString(fmt.Sprintf("Tests:       %s\n", c.TestFramework))
	}
	if c.GitRemote != "" {
		b.WriteString(fmt.Sprintf("Git Remote:  %s\n", c.GitRemote))
	}
	if c.GitBranch != "" {
		b.WriteString(fmt.Sprintf("Git Branch:  %s\n", c.GitBranch))
	}
	if len(c.APIKeys) > 0 {
		b.WriteString(fmt.Sprintf("API Keys:    %s\n", strings.Join(c.APIKeys, ", ")))
	}
	if c.Editor != "" {
		b.WriteString(fmt.Sprintf("Editor:      %s\n", c.Editor))
	}

	b.WriteString(fmt.Sprintf("Docker: %v | CI: %v | Tests: %v | Linter: %v | Formatter: %v\n",
		c.HasDocker, c.HasCI, c.HasTests, c.HasLinter, c.HasFormatter))
	b.WriteString(fmt.Sprintf("Confidence:  %.0f%%\n", c.Confidence*100))

	return b.String()
}
