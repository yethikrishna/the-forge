package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsLikelySecret(t *testing.T) {
	tests := []struct {
		line     string
		isSecret bool
	}{
		{`api_key = "sk-abc123"`, true},
		{`api_key = os.Getenv("API_KEY")`, false},
		{`password = "secret123"`, true},
		{`password = ""`, false},
		{`private_key = nil`, false},
		{`Authorization: Bearer token123`, true},
		{`-----BEGIN RSA PRIVATE KEY-----`, true},
		{`const apiKey = process.env.API_KEY`, false},
		{`max_retries = 3`, false},
		{`fmt.Println("hello")`, false},
	}

	for _, tt := range tests {
		result := isLikelySecret(tt.line)
		if result != tt.isSecret {
			t.Errorf("isLikelySecret(%q) = %v, want %v", tt.line, result, tt.isSecret)
		}
	}
}

func TestIsDebugStatement(t *testing.T) {
	tests := []struct {
		line    string
		isDebug bool
	}{
		{`fmt.Println("debug")`, true},
		{`console.log("debug")`, true},
		{`print("debug")`, true},
		{`log.Printf("info: %s", msg)`, true},
		{`return result`, false},
		{`func main() {`, false},
		{`// fmt.Println("commented")`, false},
	}

	for _, tt := range tests {
		result := isDebugStatement(tt.line)
		if result != tt.isDebug {
			t.Errorf("isDebugStatement(%q) = %v, want %v", tt.line, result, tt.isDebug)
		}
	}
}

func TestHasBareErrorReturn(t *testing.T) {
	tests := []struct {
		line string
		bare bool
	}{
		{"return err", true},
		{"return nil, err", true},
		{"return fmt.Errorf(\"failed: %w\", err)", false},
		{"return result, nil", false},
	}

	for _, tt := range tests {
		result := hasBareErrorReturn(tt.line)
		if result != tt.bare {
			t.Errorf("hasBareErrorReturn(%q) = %v, want %v", tt.line, result, tt.bare)
		}
	}
}

func TestReviewLine(t *testing.T) {
	reviewer := NewReviewer("", DefaultConfig())

	// Long line
	longLine := strings.Repeat("a", 150)
	comments := reviewer.reviewLine("test.go", 1, longLine)
	found := false
	for _, c := range comments {
		if c.Rule == "max-line-length" {
			found = true
		}
	}
	if !found {
		t.Error("expected max-line-length comment for long line")
	}

	// Secret
	comments = reviewer.reviewLine("config.go", 1, `api_key = "sk-abc123"`)
	found = false
	for _, c := range comments {
		if c.Rule == "no-secrets" {
			found = true
		}
	}
	if !found {
		t.Error("expected no-secrets comment")
	}

	// Clean line
	comments = reviewer.reviewLine("test.go", 1, `result := calculate(input)`)
	if len(comments) != 0 {
		t.Errorf("expected no comments for clean line, got %d", len(comments))
	}
}

func TestComputeScore(t *testing.T) {
	reviewer := NewReviewer("", DefaultConfig())

	review := &Review{
		Comments: []Comment{
			{Severity: SevBlocking, Message: "blocking issue"},
			{Severity: SevWarning, Message: "warning issue"},
			{Severity: SevSuggestion, Message: "suggestion"},
			{Severity: SevNit, Message: "nit"},
		},
	}

	score := reviewer.computeScore(review)
	expected := 100 - 25 - 10 - 3 - 1 // 61
	if score != expected {
		t.Errorf("expected score %d, got %d", expected, score)
	}
}

func TestIsApproved(t *testing.T) {
	reviewer := NewReviewer("", DefaultConfig())

	// Clean review should be approved
	review := &Review{Score: 100, Comments: nil}
	if !reviewer.isApproved(review) {
		t.Error("expected clean review to be approved")
	}

	// Blocking comment should not be approved
	review = &Review{Score: 90, Comments: []Comment{{Severity: SevBlocking}}}
	if reviewer.isApproved(review) {
		t.Error("expected blocking review to not be approved")
	}

	// Low score should not be approved
	review = &Review{Score: 50, Comments: []Comment{{Severity: SevWarning}}}
	if reviewer.isApproved(review) {
		t.Error("expected low-score review to not be approved")
	}
}

func TestGenerateSummary(t *testing.T) {
	reviewer := NewReviewer("", DefaultConfig())

	review := &Review{
		FilesReviewed: 3,
		Comments: []Comment{
			{Severity: SevBlocking},
			{Severity: SevWarning},
			{Severity: SevSuggestion},
			{Severity: SevNit},
		},
	}

	summary := reviewer.generateSummary(review)
	if !strings.Contains(summary, "blocking") {
		t.Error("summary should mention blocking issues")
	}
	if !strings.Contains(summary, "1 warning") {
		t.Error("summary should mention 1 warning")
	}

	// Clean review
	review2 := &Review{FilesReviewed: 1}
	summary2 := reviewer.generateSummary(review2)
	if !strings.Contains(summary2, "Clean") {
		t.Errorf("expected clean summary, got %s", summary2)
	}
}

func TestShouldExclude(t *testing.T) {
	reviewer := NewReviewer("", DefaultConfig())

	if !reviewer.shouldExclude("vendor/pkg/module.go") {
		t.Error("expected vendor/ to be excluded")
	}
	if !reviewer.shouldExclude("node_modules/react/index.js") {
		t.Error("expected node_modules/ to be excluded")
	}
	if reviewer.shouldExclude("main.go") {
		t.Error("main.go should not be excluded")
	}
}

func TestParseDiff(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
@@ -1,3 +1,4 @@
 package main
 
+import "fmt"
 func main() {
-    fmt.Println("hello")
+    fmt.Println("world")
 }`

	sections := parseDiff(diff)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].File != "main.go" {
		t.Errorf("expected file main.go, got %s", sections[0].File)
	}

	// Check lines
	added := 0
	removed := 0
	for _, l := range sections[0].Lines {
		if l.Type == "+" {
			added++
		}
		if l.Type == "-" {
			removed++
		}
	}
	if added != 2 {
		t.Errorf("expected 2 added lines, got %d", added)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed line, got %d", removed)
	}
}

func TestParseDiffMultipleFiles(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
@@ -1 +1 @@
-old
+new
diff --git a/b.go b/b.go
@@ -1 +1 @@
-old2
+new2`

	sections := parseDiff(diff)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
}

func TestFormatReview(t *testing.T) {
	review := &Review{
		Score:    85,
		Approved: true,
		Summary:  "Found 1 suggestion(s) across 1 file(s)",
		Comments: []Comment{
			{File: "main.go", Line: 5, Severity: SevSuggestion, Message: "Consider using fmt.Errorf"},
		},
	}

	output := FormatReview(review)
	if !strings.Contains(output, "APPROVED") {
		t.Error("expected APPROVED in output")
	}
	if !strings.Contains(output, "85/100") {
		t.Error("expected score in output")
	}
	if !strings.Contains(output, "main.go") {
		t.Error("expected file name in output")
	}
}

func TestReviewSerialization(t *testing.T) {
	review := &Review{
		ID:            "review-123",
		Target:        "main",
		Reviewer:      "forge-reviewer",
		FilesReviewed: 2,
		LinesAdded:    10,
		LinesRemoved:  5,
		Comments: []Comment{
			{File: "a.go", Line: 10, Severity: SevWarning, Message: "Debug statement", Rule: "no-debug"},
		},
		Score:    90,
		Approved: true,
		Summary:  "1 warning",
	}

	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var review2 Review
	if err := json.Unmarshal(data, &review2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if review2.ID != "review-123" {
		t.Errorf("expected ID review-123, got %s", review2.ID)
	}
	if len(review2.Comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(review2.Comments))
	}
}

func TestSaveReview(t *testing.T) {
	tmpDir := t.TempDir()
	review := &Review{
		ID:       "review-test",
		Score:    100,
		Approved: true,
		Summary:  "Clean review",
	}

	err := SaveReview(review, tmpDir)
	if err != nil {
		t.Fatalf("SaveReview failed: %v", err)
	}

	// Check file exists
	path := filepath.Join(tmpDir, "review-test.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("review file should exist")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxLineLength != 120 {
		t.Errorf("expected max line length 120, got %d", cfg.MaxLineLength)
	}
	if !cfg.RequireTests {
		t.Error("expected RequireTests to be true")
	}
	if !cfg.BlockSecrets {
		t.Error("expected BlockSecrets to be true")
	}
}

func TestScoreMinimum(t *testing.T) {
	reviewer := NewReviewer("", DefaultConfig())
	review := &Review{
		Comments: []Comment{
			{Severity: SevBlocking},
			{Severity: SevBlocking},
			{Severity: SevBlocking},
			{Severity: SevBlocking},
			{Severity: SevBlocking},
		},
	}
	score := reviewer.computeScore(review)
	if score < 0 {
		t.Errorf("score should not be negative, got %d", score)
	}
	if score != 0 {
		t.Errorf("expected score 0, got %d", score)
	}
}
