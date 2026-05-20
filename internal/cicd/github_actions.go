package cicd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GitHubActionsWorkflow generates a GitHub Actions workflow YAML.
type GitHubActionsWorkflow struct {
	Name         string
	On           GitHubActionsOn
	Jobs         map[string]GitHubActionsJob
	Permissions  map[string]string
	Concurrency  *GitHubActionsConcurrency
}

// GitHubActionsOn defines trigger conditions.
type GitHubActionsOn struct {
	Push    *GitHubActionsFilter `yaml:"push,omitempty"`
	PR      *GitHubActionsFilter `yaml:"pull_request,omitempty"`
	Schedule []string             `yaml:"schedule,omitempty"`
	WorkflowDispatch bool         `yaml:"workflow_dispatch,omitempty"`
}

// GitHubActionsFilter defines branch/path filters.
type GitHubActionsFilter struct {
	Branches []string `yaml:"branches,omitempty"`
	Paths    []string `yaml:"paths,omitempty"`
}

// GitHubActionsJob defines a single job.
type GitHubActionsJob struct {
	RunsOn    string                    `yaml:"runs-on"`
	Steps     []GitHubActionsStep       `yaml:"steps"`
	Needs     []string                  `yaml:"needs,omitempty"`
	If        string                    `yaml:"if,omitempty"`
	Env       map[string]string         `yaml:"env,omitempty"`
	Timeout   int                       `yaml:"timeout-minutes,omitempty"`
	Strategy  *GitHubActionsStrategy    `yaml:"strategy,omitempty"`
	Services  map[string]GitHubActionsService `yaml:"services,omitempty"`
}

// GitHubActionsStep defines a workflow step.
type GitHubActionsStep struct {
	Name string            `yaml:"name,omitempty"`
	Uses string            `yaml:"uses,omitempty"`
	With map[string]string `yaml:"with,omitempty"`
	Run  string            `yaml:"run,omitempty"`
	Env  map[string]string `yaml:"env,omitempty"`
	If   string            `yaml:"if,omitempty"`
	ID   string            `yaml:"id,omitempty"`
}

// GitHubActionsStrategy defines matrix strategy.
type GitHubActionsStrategy struct {
	Matrix map[string][]string `yaml:"matrix,omitempty"`
	FailFast bool              `yaml:"fail-fast,omitempty"`
}

// GitHubActionsService defines a service container.
type GitHubActionsService struct {
	Image string            `yaml:"image"`
	Env   map[string]string `yaml:"env,omitempty"`
	Ports []string          `yaml:"ports,omitempty"`
}

// GitHubActionsConcurrency defines concurrency settings.
type GitHubActionsConcurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress bool   `yaml:"cancel-in-progress"`
}

// GenerateGoCI generates a standard Go CI workflow.
func GenerateGoCI(goVersion string, testFlags string) *GitHubActionsWorkflow {
	if goVersion == "" {
		goVersion = "1.23"
	}
	if testFlags == "" {
		testFlags = "-race -count=1 ./..."
	}

	return &GitHubActionsWorkflow{
		Name: "Go CI",
		On: GitHubActionsOn{
			Push: &GitHubActionsFilter{Branches: []string{"main", "master"}},
			PR:   &GitHubActionsFilter{Branches: []string{"main", "master"}},
		},
		Permissions: map[string]string{
			"contents": "read",
		},
		Jobs: map[string]GitHubActionsJob{
			"build": {
				RunsOn:  "ubuntu-latest",
				Timeout: 15,
				Steps: []GitHubActionsStep{
					{Name: "Checkout", Uses: "actions/checkout@v4"},
					{Name: "Set up Go", Uses: "actions/setup-go@v5", With: map[string]string{"go-version": goVersion}},
					{Name: "Go mod download", Run: "go mod download"},
					{Name: "Go build", Run: "go build ./..."},
					{Name: "Go vet", Run: "go vet ./..."},
					{Name: "Go test", Run: fmt.Sprintf("go test %s", testFlags)},
					{Name: "Go lint", Uses: "golangci/golangci-lint-action@v6"},
				},
			},
		},
	}
}

// GenerateReleaseWorkflow generates a release workflow with goreleaser.
func GenerateReleaseWorkflow(goVersion string) *GitHubActionsWorkflow {
	if goVersion == "" {
		goVersion = "1.23"
	}
	return &GitHubActionsWorkflow{
		Name: "Release",
		On: GitHubActionsOn{
			Push: &GitHubActionsFilter{Branches: []string{"main"}, Paths: []string{"go.mod"}},
			WorkflowDispatch: true,
		},
		Permissions: map[string]string{
			"contents": "write",
		},
		Jobs: map[string]GitHubActionsJob{
			"release": {
				RunsOn:  "ubuntu-latest",
				Timeout: 30,
				Steps: []GitHubActionsStep{
					{Name: "Checkout", Uses: "actions/checkout@v4", With: map[string]string{"fetch-depth": "0"}},
					{Name: "Set up Go", Uses: "actions/setup-go@v5", With: map[string]string{"go-version": goVersion}},
					{Name: "Run GoReleaser", Uses: "goreleaser/goreleaser-action@v6", With: map[string]string{"args": "release --clean"}},
				},
			},
		},
	}
}

// GenerateDockerWorkflow generates a Docker build and push workflow.
func GenerateDockerWorkflow(imageName string) *GitHubActionsWorkflow {
	return &GitHubActionsWorkflow{
		Name: "Docker",
		On: GitHubActionsOn{
			Push: &GitHubActionsFilter{Branches: []string{"main"}},
		},
		Permissions: map[string]string{
			"contents": "read",
			"packages": "write",
		},
		Jobs: map[string]GitHubActionsJob{
			"build": {
				RunsOn:  "ubuntu-latest",
				Timeout: 20,
				Steps: []GitHubActionsStep{
					{Name: "Checkout", Uses: "actions/checkout@v4"},
					{Name: "Set up Docker Buildx", Uses: "docker/setup-buildx-action@v3"},
					{Name: "Login to GHCR", Uses: "docker/login-action@v3", With: map[string]string{"registry": "ghcr.io", "username": "${{ github.actor }}", "password": "${{ secrets.GITHUB_TOKEN }}"}},
					{Name: "Build and push", Uses: "docker/build-push-action@v6", With: map[string]string{"context": ".", "push": "true", "tags": fmt.Sprintf("ghcr.io/${{ github.repository }}/%s:latest,ghcr.io/${{ github.repository }}/%s:${{ github.sha }}", imageName, imageName)}},
				},
			},
		},
	}
}

// GenerateForgeWorkflow generates a Forge-specific CI workflow.
func GenerateForgeWorkflow() *GitHubActionsWorkflow {
	return &GitHubActionsWorkflow{
		Name: "Forge CI",
		On: GitHubActionsOn{
			Push: &GitHubActionsFilter{Branches: []string{"main"}},
			PR:   &GitHubActionsFilter{Branches: []string{"main"}},
		},
		Permissions: map[string]string{
			"contents": "read",
		},
		Concurrency: &GitHubActionsConcurrency{
			Group:            "ci-${{ github.ref }}",
			CancelInProgress: true,
		},
		Jobs: map[string]GitHubActionsJob{
			"build": {
				RunsOn:  "ubuntu-latest",
				Timeout: 10,
				Steps: []GitHubActionsStep{
					{Name: "Checkout", Uses: "actions/checkout@v4"},
					{Name: "Set up Go", Uses: "actions/setup-go@v5", With: map[string]string{"go-version": "1.23"}},
					{Name: "Build", Run: "go build ./..."},
					{Name: "Vet", Run: "go vet ./..."},
				},
			},
			"test": {
				RunsOn:  "ubuntu-latest",
				Needs:   []string{"build"},
				Timeout: 15,
				Steps: []GitHubActionsStep{
					{Name: "Checkout", Uses: "actions/checkout@v4"},
					{Name: "Set up Go", Uses: "actions/setup-go@v5", With: map[string]string{"go-version": "1.23"}},
					{Name: "Test", Run: "go test -race -count=1 -coverprofile=coverage.out ./..."},
					{Name: "Coverage", Run: "go tool cover -func=coverage.out"},
				},
			},
			"lint": {
				RunsOn:  "ubuntu-latest",
				Needs:   []string{"build"},
				Timeout: 10,
				Steps: []GitHubActionsStep{
					{Name: "Checkout", Uses: "actions/checkout@v4"},
					{Name: "Lint", Uses: "golangci/golangci-lint-action@v6"},
				},
			},
		},
	}
}

// WriteWorkflow writes a workflow YAML file.
func WriteWorkflow(wf *GitHubActionsWorkflow, dir string) error {
	os.MkdirAll(dir, 0o755)
	filename := strings.ToLower(strings.ReplaceAll(wf.Name, " ", "-")) + ".yml"
	path := filepath.Join(dir, filename)

	yaml := workflowToYAML(wf)
	return os.WriteFile(path, []byte(yaml), 0o644)
}

// workflowToYAML converts a workflow to YAML (simple hand-rolled for zero deps).
func workflowToYAML(wf *GitHubActionsWorkflow) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("name: %s\n\n", wf.Name))

	// On
	sb.WriteString("on:\n")
	if wf.On.Push != nil {
		sb.WriteString("  push:\n")
		if len(wf.On.Push.Branches) > 0 {
			sb.WriteString(fmt.Sprintf("    branches: %s\n", yamlList(wf.On.Push.Branches)))
		}
		if len(wf.On.Push.Paths) > 0 {
			sb.WriteString(fmt.Sprintf("    paths: %s\n", yamlList(wf.On.Push.Paths)))
		}
	}
	if wf.On.PR != nil {
		sb.WriteString("  pull_request:\n")
		if len(wf.On.PR.Branches) > 0 {
			sb.WriteString(fmt.Sprintf("    branches: %s\n", yamlList(wf.On.PR.Branches)))
		}
	}
	if len(wf.On.Schedule) > 0 {
		sb.WriteString("  schedule:\n")
		for _, s := range wf.On.Schedule {
			sb.WriteString(fmt.Sprintf("    - cron: '%s'\n", s))
		}
	}
	if wf.On.WorkflowDispatch {
		sb.WriteString("  workflow_dispatch:\n")
	}

	// Permissions
	if len(wf.Permissions) > 0 {
		sb.WriteString("\npermissions:\n")
		for k, v := range wf.Permissions {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	// Concurrency
	if wf.Concurrency != nil {
		sb.WriteString(fmt.Sprintf("\nconcurrency:\n  group: %s\n  cancel-in-progress: %v\n", wf.Concurrency.Group, wf.Concurrency.CancelInProgress))
	}

	// Jobs
	sb.WriteString("\njobs:\n")
	for name, job := range wf.Jobs {
		sb.WriteString(fmt.Sprintf("  %s:\n", name))
		sb.WriteString(fmt.Sprintf("    runs-on: %s\n", job.RunsOn))
		if job.Timeout > 0 {
			sb.WriteString(fmt.Sprintf("    timeout-minutes: %d\n", job.Timeout))
		}
		if len(job.Needs) > 0 {
			sb.WriteString(fmt.Sprintf("    needs: %s\n", yamlList(job.Needs)))
		}
		if job.If != "" {
			sb.WriteString(fmt.Sprintf("    if: %s\n", job.If))
		}
		if len(job.Env) > 0 {
			sb.WriteString("    env:\n")
			for k, v := range job.Env {
				sb.WriteString(fmt.Sprintf("      %s: %s\n", k, v))
			}
		}
		if job.Strategy != nil && len(job.Strategy.Matrix) > 0 {
			sb.WriteString("    strategy:\n")
			if job.Strategy.FailFast {
				sb.WriteString("      fail-fast: true\n")
			}
			sb.WriteString("      matrix:\n")
			for k, v := range job.Strategy.Matrix {
				sb.WriteString(fmt.Sprintf("        %s: %s\n", k, yamlList(v)))
			}
		}
		sb.WriteString("    steps:\n")
		for _, step := range job.Steps {
			sb.WriteString(stepToYAML(step))
		}
	}

	return sb.String()
}

func stepToYAML(step GitHubActionsStep) string {
	var sb strings.Builder
	sb.WriteString("      -")
	if step.Name != "" {
		sb.WriteString(fmt.Sprintf(" name: %s\n", step.Name))
	}
	if step.Uses != "" {
		sb.WriteString(fmt.Sprintf("        uses: %s\n", step.Uses))
	}
	if step.Run != "" {
		// Multi-line run
		if strings.Contains(step.Run, "\n") {
			sb.WriteString("        run: |\n")
			for _, line := range strings.Split(step.Run, "\n") {
				sb.WriteString(fmt.Sprintf("          %s\n", line))
			}
		} else {
			sb.WriteString(fmt.Sprintf("        run: %s\n", step.Run))
		}
	}
	if len(step.With) > 0 {
		sb.WriteString("        with:\n")
		for k, v := range step.With {
			sb.WriteString(fmt.Sprintf("          %s: %s\n", k, v))
		}
	}
	if len(step.Env) > 0 {
		sb.WriteString("        env:\n")
		for k, v := range step.Env {
			sb.WriteString(fmt.Sprintf("          %s: %s\n", k, v))
		}
	}
	if step.If != "" {
		sb.WriteString(fmt.Sprintf("        if: %s\n", step.If))
	}
	return sb.String()
}

func yamlList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	if len(items) == 1 {
		return fmt.Sprintf("[%s]", items[0])
	}
	return fmt.Sprintf("[%s]", strings.Join(items, ", "))
}
