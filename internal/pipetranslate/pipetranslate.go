// Pipeline translation converts between natural language descriptions
// and Forgefile YAML pipeline definitions. It enables users to describe
// workflows in plain English and get a valid forge.yaml, or explain
// an existing forge.yaml in natural language.
package pipetranslate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Pipeline represents a forge.yaml pipeline definition.
type Pipeline struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Steps       []Step `yaml:"steps" json:"steps"`
	CostCap     string `yaml:"cost_cap,omitempty" json:"cost_cap,omitempty"`
	Timeout     string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Model       string `yaml:"model,omitempty" json:"model,omitempty"`
}

// Step represents a single step in a pipeline.
type Step struct {
	Name        string            `yaml:"name" json:"name"`
	Agent       string            `yaml:"agent,omitempty" json:"agent,omitempty"`
	Model       string            `yaml:"model,omitempty" json:"model,omitempty"`
	Prompt      string            `yaml:"prompt" json:"prompt"`
	Input       string            `yaml:"input,omitempty" json:"input,omitempty"`
	Output      string            `yaml:"output,omitempty" json:"output,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Condition   string            `yaml:"condition,omitempty" json:"condition,omitempty"`
	Approval    bool              `yaml:"approval,omitempty" json:"approval,omitempty"`
	Timeout     string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	CostCap     string            `yaml:"cost_cap,omitempty" json:"cost_cap,omitempty"`
	Retry       int               `yaml:"retry,omitempty" json:"retry,omitempty"`
	Tools       []string          `yaml:"tools,omitempty" json:"tools,omitempty"`
	Environment map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// TranslationResult holds the output of a translation.
type TranslationResult struct {
	Pipeline    *Pipeline `json:"pipeline"`
	YAML        string    `json:"yaml"`
	Explanation string    `json:"explanation,omitempty"`
	Confidence  float64   `json:"confidence"`
	Suggestions []string  `json:"suggestions,omitempty"`
}

// Translator converts between natural language and pipeline definitions.
type Translator struct {
	templates map[string]*Pipeline
}

// NewTranslator creates a new pipeline translator.
func NewTranslator() *Translator {
	return &Translator{
		templates: builtinTemplates(),
	}
}

// FromNaturalLanguage translates a natural language description into a pipeline.
func (t *Translator) FromNaturalLanguage(desc string) (*TranslationResult, error) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return nil, fmt.Errorf("empty description")
	}

	// Try matching against known patterns
	pipeline := t.matchPattern(desc)
	if pipeline == nil {
		pipeline = t.generateFromDescription(desc)
	}

	yaml := pipelineToYAML(pipeline)
	confidence := t.estimateConfidence(desc, pipeline)
	suggestions := t.generateSuggestions(pipeline)

	return &TranslationResult{
		Pipeline:    pipeline,
		YAML:        yaml,
		Confidence:  confidence,
		Suggestions: suggestions,
	}, nil
}

// ToNaturalLanguage explains a pipeline in plain English.
func (t *Translator) ToNaturalLanguage(pipeline *Pipeline) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("The pipeline %q", pipeline.Name))
	if pipeline.Description != "" {
		b.WriteString(fmt.Sprintf(" (%s)", pipeline.Description))
	}
	b.WriteString(" performs the following steps:\n\n")

	for i, step := range pipeline.Steps {
		b.WriteString(fmt.Sprintf("%d. **%s**: ", i+1, step.Name))
		b.WriteString(step.Prompt)

		if step.Agent != "" {
			b.WriteString(fmt.Sprintf(" (using the %s agent)", step.Agent))
		}
		if step.Model != "" {
			b.WriteString(fmt.Sprintf(" on %s", step.Model))
		}
		if len(step.DependsOn) > 0 {
			b.WriteString(fmt.Sprintf(" — depends on %s", strings.Join(step.DependsOn, ", ")))
		}
		if step.Approval {
			b.WriteString(" [requires approval]")
		}
		b.WriteString("\n")
	}

	if pipeline.CostCap != "" {
		b.WriteString(fmt.Sprintf("\nTotal budget: %s\n", pipeline.CostCap))
	}
	if pipeline.Timeout != "" {
		b.WriteString(fmt.Sprintf("Total timeout: %s\n", pipeline.Timeout))
	}

	return b.String()
}

// matchPattern tries to match the description against known workflow patterns.
// It requires ALL keywords from a template name to appear in the description.
// If multiple templates match, the one with the most specific match (most keywords) wins.
func (t *Translator) matchPattern(desc string) *Pipeline {
	lower := strings.ToLower(desc)

	var bestMatch *Pipeline
	bestScore := 0

	for pattern, template := range t.templates {
		keywords := extractKeywords(pattern)
		if len(keywords) == 0 {
			continue
		}

		// All keywords must be present for a match
		allMatch := true
		for _, keyword := range keywords {
			if !strings.Contains(lower, keyword) {
				allMatch = false
				break
			}
		}

		if allMatch && len(keywords) > bestScore {
			bestScore = len(keywords)
			p := clonePipeline(template)
			p.Description = desc
			bestMatch = p
		}
	}

	// Only use template match if the description seems focused on that pattern
	// If the description mentions deploy/implementation/etc, don't use a partial template
	if bestMatch != nil {
		templateHasDeploy := false
		for _, s := range bestMatch.Steps {
			if strings.Contains(strings.ToLower(s.Name), "deploy") {
				templateHasDeploy = true
			}
		}
		descWantsDeploy := containsAny(lower, []string{"deploy", "ship", "release"})
		if descWantsDeploy && !templateHasDeploy {
			// Description wants deployment but template doesn't have it — don't use template
			return nil
		}
	}

	return bestMatch
}

// generateFromDescription creates a pipeline from a free-form description.
func (t *Translator) generateFromDescription(desc string) *Pipeline {
	lower := strings.ToLower(desc)

	// Detect common patterns
	steps := []Step{}

	// Check for code review pattern
	if containsAny(lower, []string{"review", "code review", "pr review"}) {
		steps = append(steps, Step{
			Name:   "review",
			Agent:  "reviewer",
			Prompt: "Review the code changes for bugs, style issues, and security concerns",
			Tools:  []string{"git", "read"},
		})
	}

	// Check for testing pattern
	if containsAny(lower, []string{"test", "testing", "write tests", "generate tests"}) {
		steps = append(steps, Step{
			Name:      "test",
			Agent:     "tester",
			Prompt:    "Write comprehensive tests for the code",
			DependsOn: dependsOn(steps),
			Tools:     []string{"git", "read", "write", "exec"},
		})
	}

	// Check for implementation pattern
	if containsAny(lower, []string{"implement", "build", "create", "write", "develop", "code"}) {
		implStep := Step{
			Name:   "implement",
			Agent:  "coder",
			Prompt: desc,
			Tools:  []string{"git", "read", "write", "exec"},
		}
		// Insert implementation before review/test
		steps = append([]Step{implStep}, steps...)
		if len(steps) > 1 {
			for i := 1; i < len(steps); i++ {
				if len(steps[i].DependsOn) == 0 {
					steps[i].DependsOn = []string{steps[0].Name}
				}
			}
		}
	}

	// Check for deployment pattern
	if containsAny(lower, []string{"deploy", "ship", "release", "publish"}) {
		deployDeps := []string{}
		for _, s := range steps {
			deployDeps = append(deployDeps, s.Name)
		}
		steps = append(steps, Step{
			Name:      "deploy",
			Agent:     "deployer",
			Prompt:    "Deploy the changes to the target environment",
			DependsOn: deployDeps,
			Approval:  true,
			Tools:     []string{"exec"},
		})
	}

	// If no patterns matched, create a single-step pipeline
	if len(steps) == 0 {
		steps = append(steps, Step{
			Name:   "execute",
			Agent:  "default",
			Prompt: desc,
			Tools:  []string{"git", "read", "write", "exec"},
		})
	}

	// Derive pipeline name from description
	name := deriveName(desc)

	return &Pipeline{
		Name:        name,
		Description: desc,
		Steps:       steps,
		CostCap:     "$1.00",
		Timeout:     "10m",
	}
}

// estimateConfidence returns how confident we are in the translation.
func (t *Translator) estimateConfidence(desc string, pipeline *Pipeline) float64 {
	confidence := 0.5

	// More specific descriptions → higher confidence
	words := strings.Fields(desc)
	if len(words) > 10 {
		confidence += 0.1
	}
	if len(words) > 20 {
		confidence += 0.1
	}

	// Known patterns → higher confidence
	lower := strings.ToLower(desc)
	if containsAny(lower, []string{"then", "after", "before", "and then", "followed by"}) {
		confidence += 0.1
	}

	// Multiple steps → higher confidence (structured intent)
	if len(pipeline.Steps) > 1 {
		confidence += 0.1
	}

	// Approval gates → higher confidence (safety-aware)
	for _, step := range pipeline.Steps {
		if step.Approval {
			confidence += 0.05
		}
	}

	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// generateSuggestions returns improvement suggestions for the pipeline.
func (t *Translator) generateSuggestions(pipeline *Pipeline) []string {
	var suggestions []string

	// Suggest adding cost cap
	if pipeline.CostCap == "" {
		suggestions = append(suggestions, "Add a cost_cap to prevent runaway spending")
	}

	// Suggest adding timeout
	if pipeline.Timeout == "" {
		suggestions = append(suggestions, "Add a timeout to prevent stuck pipelines")
	}

	// Suggest approval for destructive steps
	for _, step := range pipeline.Steps {
		if isDestructive(step) && !step.Approval {
			suggestions = append(suggestions, fmt.Sprintf("Add approval gate to step %q (it modifies production state)", step.Name))
		}
	}

	// Suggest model selection
	hasModel := pipeline.Model != ""
	for _, step := range pipeline.Steps {
		if step.Model != "" {
			hasModel = true
		}
	}
	if !hasModel {
		suggestions = append(suggestions, "Specify models per step for cost optimization (e.g., use gpt-4.1-mini for simple tasks)")
	}

	// Suggest retries
	for _, step := range pipeline.Steps {
		if step.Retry == 0 {
			suggestions = append(suggestions, fmt.Sprintf("Add retry count to step %q for resilience", step.Name))
			break // Only suggest once
		}
	}

	return suggestions
}

func isDestructive(step Step) bool {
	lower := strings.ToLower(step.Name + " " + step.Prompt)
	return containsAny(lower, []string{"deploy", "delete", "remove", "drop", "destroy", "publish", "release"})
}

func builtinTemplates() map[string]*Pipeline {
	return map[string]*Pipeline{
		"code-review": {
			Name:        "code-review",
			Description: "Automated code review pipeline",
			Steps: []Step{
				{Name: "review", Agent: "reviewer", Prompt: "Review the code changes for bugs, security issues, and style", Tools: []string{"git", "read"}},
				{Name: "summarize", Agent: "summarizer", Prompt: "Create a concise summary of the review findings", DependsOn: []string{"review"}, Tools: []string{"read"}},
			},
			CostCap: "$0.50",
			Timeout: "5m",
		},
		"full-cicd": {
			Name:        "full-cicd",
			Description: "Complete CI/CD pipeline with code review, testing, and deployment",
			Steps: []Step{
				{Name: "review", Agent: "reviewer", Prompt: "Review all code changes", Tools: []string{"git", "read"}},
				{Name: "test", Agent: "tester", Prompt: "Write and run tests for the changes", DependsOn: []string{"review"}, Tools: []string{"git", "read", "write", "exec"}},
				{Name: "deploy", Agent: "deployer", Prompt: "Deploy to staging environment", DependsOn: []string{"test"}, Approval: true, Tools: []string{"exec"}},
			},
			CostCap: "$2.00",
			Timeout: "15m",
		},
		"security-scan": {
			Name:        "security-scan",
			Description: "Security scanning pipeline",
			Steps: []Step{
				{Name: "scan", Agent: "security-scanner", Prompt: "Scan codebase for vulnerabilities, secrets, and misconfigurations", Tools: []string{"git", "read", "exec"}},
				{Name: "report", Agent: "reporter", Prompt: "Generate a security report with severity ratings and fix suggestions", DependsOn: []string{"scan"}, Tools: []string{"read", "write"}},
			},
			CostCap: "$1.00",
			Timeout: "10m",
		},
		"refactor": {
			Name:        "refactor",
			Description: "Automated refactoring pipeline",
			Steps: []Step{
				{Name: "analyze", Agent: "analyzer", Prompt: "Analyze the codebase for refactoring opportunities", Tools: []string{"git", "read"}},
				{Name: "refactor", Agent: "coder", Prompt: "Apply refactoring changes based on the analysis", DependsOn: []string{"analyze"}, Tools: []string{"git", "read", "write"}},
				{Name: "test", Agent: "tester", Prompt: "Verify refactored code passes all tests", DependsOn: []string{"refactor"}, Tools: []string{"exec"}},
				{Name: "review", Agent: "reviewer", Prompt: "Review the refactoring for correctness and style", DependsOn: []string{"test"}, Tools: []string{"git", "read"}},
			},
			CostCap: "$3.00",
			Timeout: "20m",
		},
		"documentation": {
			Name:        "documentation",
			Description: "Documentation generation pipeline",
			Steps: []Step{
				{Name: "read-code", Agent: "reader", Prompt: "Read and understand the codebase structure", Tools: []string{"git", "read"}},
				{Name: "generate-docs", Agent: "writer", Prompt: "Generate comprehensive documentation from the code", DependsOn: []string{"read-code"}, Tools: []string{"read", "write"}},
				{Name: "review-docs", Agent: "reviewer", Prompt: "Review documentation for accuracy and completeness", DependsOn: []string{"generate-docs"}, Tools: []string{"read"}},
			},
			CostCap: "$1.50",
			Timeout: "10m",
		},
	}
}

// pipelineToYAML converts a pipeline to YAML string.
func pipelineToYAML(p *Pipeline) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("name: %q\n", p.Name))
	if p.Description != "" {
		b.WriteString(fmt.Sprintf("description: %q\n", p.Description))
	}
	if p.CostCap != "" {
		b.WriteString(fmt.Sprintf("cost_cap: %q\n", p.CostCap))
	}
	if p.Timeout != "" {
		b.WriteString(fmt.Sprintf("timeout: %q\n", p.Timeout))
	}
	if p.Model != "" {
		b.WriteString(fmt.Sprintf("model: %q\n", p.Model))
	}

	b.WriteString("steps:\n")
	for _, step := range p.Steps {
		b.WriteString(fmt.Sprintf("  - name: %q\n", step.Name))
		if step.Agent != "" {
			b.WriteString(fmt.Sprintf("    agent: %q\n", step.Agent))
		}
		if step.Model != "" {
			b.WriteString(fmt.Sprintf("    model: %q\n", step.Model))
		}
		b.WriteString(fmt.Sprintf("    prompt: %q\n", step.Prompt))
		if step.Input != "" {
			b.WriteString(fmt.Sprintf("    input: %q\n", step.Input))
		}
		if step.Output != "" {
			b.WriteString(fmt.Sprintf("    output: %q\n", step.Output))
		}
		if len(step.DependsOn) > 0 {
			b.WriteString("    depends_on:\n")
			for _, dep := range step.DependsOn {
				b.WriteString(fmt.Sprintf("      - %q\n", dep))
			}
		}
		if step.Approval {
			b.WriteString("    approval: true\n")
		}
		if step.Timeout != "" {
			b.WriteString(fmt.Sprintf("    timeout: %q\n", step.Timeout))
		}
		if step.CostCap != "" {
			b.WriteString(fmt.Sprintf("    cost_cap: %q\n", step.CostCap))
		}
		if step.Retry > 0 {
			b.WriteString(fmt.Sprintf("    retry: %d\n", step.Retry))
		}
		if len(step.Tools) > 0 {
			b.WriteString("    tools:\n")
			for _, tool := range step.Tools {
				b.WriteString(fmt.Sprintf("      - %q\n", tool))
			}
		}
	}

	return b.String()
}

func extractKeywords(pattern string) []string {
	// Split on spaces and hyphens
	normalized := strings.ReplaceAll(strings.ToLower(pattern), "-", " ")
	words := strings.Fields(normalized)
	var keywords []string
	for _, w := range words {
		if len(w) > 3 {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

func clonePipeline(p *Pipeline) *Pipeline {
	clone := *p
	clone.Steps = make([]Step, len(p.Steps))
	copy(clone.Steps, p.Steps)
	return &clone
}

func containsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func dependsOn(steps []Step) []string {
	if len(steps) == 0 {
		return nil
	}
	deps := make([]string, len(steps))
	for i, s := range steps {
		deps[i] = s.Name
	}
	return deps
}

func deriveName(desc string) string {
	// Take first few words, clean up
	words := strings.Fields(desc)
	if len(words) > 4 {
		words = words[:4]
	}
	name := strings.Join(words, "-")
	name = strings.ToLower(name)
	name = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(name, "-")
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}

// ListTemplates returns the names of all built-in pipeline templates.
func (t *Translator) ListTemplates() []string {
	var names []string
	for name := range t.templates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetTemplate returns a specific built-in template.
func (t *Translator) GetTemplate(name string) (*Pipeline, error) {
	p, ok := t.templates[name]
	if !ok {
		return nil, fmt.Errorf("template %q not found", name)
	}
	return clonePipeline(p), nil
}

// PipelineToYAML exports the pipelineToYAML function for external use.
func PipelineToYAML(p *Pipeline) string {
	return pipelineToYAML(p)
}
