package pipetranslate_test

import (
	"strings"
	"testing"

	"github.com/forge/sword/internal/pipetranslate"
)

func TestFromNaturalLanguage_Review(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	result, err := tr.FromNaturalLanguage("review the code changes and find bugs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pipeline == nil {
		t.Fatal("expected pipeline")
	}
	if result.Confidence <= 0 {
		t.Errorf("expected positive confidence, got %.2f", result.Confidence)
	}
	if result.YAML == "" {
		t.Error("expected YAML output")
	}
	// Should have a review step
	found := false
	for _, step := range result.Pipeline.Steps {
		if step.Name == "review" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'review' step in pipeline")
	}
}

func TestFromNaturalLanguage_FullCI(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	result, err := tr.FromNaturalLanguage("implement the feature, write tests, review the code, and deploy to production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Pipeline.Steps) < 3 {
		t.Errorf("expected at least 3 steps, got %d", len(result.Pipeline.Steps))
	}
	// Should have deployment with approval
	for _, step := range result.Pipeline.Steps {
		if strings.Contains(step.Name, "deploy") && !step.Approval {
			t.Error("deploy step should require approval")
		}
	}
}

func TestFromNaturalLanguage_Implement(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	result, err := tr.FromNaturalLanguage("build a REST API for user management with authentication")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pipeline == nil {
		t.Fatal("expected pipeline")
	}
	if len(result.Pipeline.Steps) == 0 {
		t.Error("expected at least one step")
	}
}

func TestFromNaturalLanguage_Empty(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	_, err := tr.FromNaturalLanguage("")
	if err == nil {
		t.Error("expected error for empty description")
	}
}

func TestToNaturalLanguage(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	pipeline := &pipetranslate.Pipeline{
		Name:        "test-pipeline",
		Description: "A test pipeline",
		Steps: []pipetranslate.Step{
			{Name: "review", Agent: "reviewer", Prompt: "Review the code"},
			{Name: "test", Agent: "tester", Prompt: "Run tests", DependsOn: []string{"review"}},
			{Name: "deploy", Agent: "deployer", Prompt: "Deploy changes", DependsOn: []string{"test"}, Approval: true},
		},
		CostCap: "$1.00",
	}

	explanation := tr.ToNaturalLanguage(pipeline)
	if explanation == "" {
		t.Error("expected non-empty explanation")
	}
	if !strings.Contains(explanation, "test-pipeline") {
		t.Error("expected pipeline name in explanation")
	}
	if !strings.Contains(explanation, "review") {
		t.Error("expected review step in explanation")
	}
	if !strings.Contains(explanation, "approval") {
		t.Error("expected approval mention in explanation")
	}
}

func TestListTemplates(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	templates := tr.ListTemplates()
	if len(templates) < 3 {
		t.Errorf("expected at least 3 templates, got %d", len(templates))
	}
}

func TestGetTemplate(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	pipeline, err := tr.GetTemplate("code-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pipeline.Name != "code-review" {
		t.Errorf("expected code-review, got %s", pipeline.Name)
	}
}

func TestGetTemplateNotFound(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	_, err := tr.GetTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestSuggestions(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	result, err := tr.FromNaturalLanguage("write a hello world program")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}
}

func TestSecurityScan(t *testing.T) {
	tr := pipetranslate.NewTranslator()
	result, err := tr.FromNaturalLanguage("scan the codebase for security vulnerabilities")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should match security-scan template
	found := false
	for _, step := range result.Pipeline.Steps {
		if strings.Contains(step.Name, "scan") || strings.Contains(step.Name, "security") {
			found = true
		}
	}
	if !found {
		t.Error("expected security scan step")
	}
}

func TestConfidence(t *testing.T) {
	tr := pipetranslate.NewTranslator()

	// Short description → lower confidence
	short, _ := tr.FromNaturalLanguage("review code")
	
	// Detailed description → higher confidence
	detailed, _ := tr.FromNaturalLanguage("review the code changes in the pull request, check for security issues, suggest improvements, and then write unit tests for the new functionality")

	if detailed.Confidence <= short.Confidence {
		t.Errorf("detailed description should have higher confidence: %.2f vs %.2f", detailed.Confidence, short.Confidence)
	}
}

func TestPipelineToYAML(t *testing.T) {
	pipeline := &pipetranslate.Pipeline{
		Name:    "test",
		Steps: []pipetranslate.Step{
			{Name: "step1", Prompt: "Do something", Tools: []string{"git", "read"}},
		},
		CostCap: "$0.50",
		Timeout: "5m",
	}

	yaml := pipetranslate.PipelineToYAML(pipeline)
	if yaml == "" {
		t.Error("expected non-empty YAML")
	}
	if !strings.Contains(yaml, "name:") {
		t.Error("expected name field in YAML")
	}
	if !strings.Contains(yaml, "steps:") {
		t.Error("expected steps field in YAML")
	}
}
