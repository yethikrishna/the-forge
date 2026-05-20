package tokencost

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		min   int
		max   int
	}{
		{"Hello world", 2, 8},
		{"", 0, 0},
		{"a", 1, 3},
		{"The quick brown fox jumps over the lazy dog", 8, 20},
		{strings.Repeat("word ", 100), 80, 150},
	}

	for _, tt := range tests {
		got := EstimateTokens(tt.input)
		if tt.input == "" {
			if got != 0 {
				t.Errorf("empty string should be 0 tokens, got %d", got)
			}
			continue
		}
		if got < tt.min || got > tt.max {
			t.Errorf("EstimateTokens(%q) = %d, want between %d and %d", truncate(tt.input, 30), got, tt.min, tt.max)
		}
	}
}

func TestEstimateTokensCJK(t *testing.T) {
	got := EstimateTokens("你好世界")
	if got < 2 {
		t.Errorf("CJK text should have reasonable token count, got %d", got)
	}
}

func TestEstimateCost(t *testing.T) {
	cost := EstimateCost("gpt-5-mini", 1000, 0)
	if cost <= 0 {
		t.Error("cost should be positive")
	}
	if cost > 1.0 {
		t.Errorf("1000 tokens on gpt-5-mini should be < $1, got %f", cost)
	}
}

func TestEstimateCostUnknownModel(t *testing.T) {
	cost := EstimateCost("unknown-model", 1000, 0)
	if cost <= 0 {
		t.Error("should use default pricing for unknown models")
	}
}

func TestEstimateCostWithOutput(t *testing.T) {
	inputCost := EstimateCost("gpt-5-mini", 1000, 0)
	totalCost := EstimateCost("gpt-5-mini", 1000, 1000)
	if totalCost <= inputCost {
		t.Error("total cost should be higher than input-only cost")
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0.001, "$0.0010"},
		{0.05, "$0.050"},
		{1.50, "$1.50"},
		{100.0, "$100.00"},
	}
	for _, tt := range tests {
		got := FormatCost(tt.cost)
		if got != tt.want {
			t.Errorf("FormatCost(%f) = %q, want %q", tt.cost, got, tt.want)
		}
	}
}

func TestAnalyzeBasic(t *testing.T) {
	a := Analyze("Hello world, this is a test prompt.")
	if a.CharCount == 0 {
		t.Error("expected non-zero char count")
	}
	if a.WordCount == 0 {
		t.Error("expected non-zero word count")
	}
	if a.EstimatedTokens == 0 {
		t.Error("expected non-zero token estimate")
	}
	if len(a.CostEstimates) == 0 {
		t.Error("expected cost estimates")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	a := Analyze("")
	if a.CharCount != 0 {
		t.Error("empty should have 0 chars")
	}
}

func TestAnalyzeRepeatedPhrases(t *testing.T) {
	text := "Please review the code. Please review the code carefully. Then please review the code again."
	a := Analyze(text)

	found := false
	for _, r := range a.Redundancies {
		if r.Type == "repeat" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find repeated phrases")
	}
}

func TestAnalyzeVerbose(t *testing.T) {
	text := "In order to fix this bug, please note that in order to test it, you need in order to run the tests."
	a := Analyze(text)

	found := false
	for _, r := range a.Redundancies {
		if r.Type == "verbose" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find verbose patterns")
	}
}

func TestAnalyzeExcessWhitespace(t *testing.T) {
	text := "Hello    world    with    extra    spaces"
	a := Analyze(text)

	found := false
	for _, r := range a.Redundancies {
		if r.Type == "whitespace" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find excess whitespace")
	}
}

func TestAnalyzeSuggestions(t *testing.T) {
	text := "In order to do this, in order to do that, in order to finish"
	a := Analyze(text)

	if len(a.Suggestions) == 0 {
		t.Error("expected optimization suggestions")
	}
}

func TestAnalyzeSavingsPercent(t *testing.T) {
	text := "Hello    world    test    " // lots of whitespace
	a := Analyze(text)
	// Savings should be >= 0
	if a.SavingsPercent < 0 {
		t.Error("savings percent should not be negative")
	}
}

func TestAnalyzeOptimizedTokens(t *testing.T) {
	a := Analyze("simple test")
	if a.OptimizedTokens > a.EstimatedTokens {
		t.Error("optimized tokens should not exceed estimated tokens")
	}
}

func TestCountWords(t *testing.T) {
	if countWords("hello world") != 2 {
		t.Error("expected 2 words")
	}
	if countWords("") != 0 {
		t.Error("expected 0 words for empty string")
	}
	if countWords("   ") != 0 {
		t.Error("expected 0 words for whitespace")
	}
}

func TestCountSentences(t *testing.T) {
	if countSentences("Hello. World!") != 2 {
		t.Error("expected 2 sentences")
	}
	if countSentences("Hello") != 1 {
		t.Error("expected 1 sentence for non-empty text without punctuation")
	}
	if countSentences("") != 0 {
		t.Error("expected 0 sentences for empty string")
	}
}

func TestCompareModels(t *testing.T) {
	results := CompareModels(1000, 500)
	if len(results) == 0 {
		t.Error("expected model cost comparisons")
	}
	for model, cost := range results {
		if cost <= 0 {
			t.Errorf("cost for %s should be positive", model)
		}
	}
}

func TestModelPricingExists(t *testing.T) {
	if len(ModelPricing) == 0 {
		t.Error("ModelPricing should have entries")
	}
}

func TestVerbosePatterns(t *testing.T) {
	if len(verbosePatterns) == 0 {
		t.Error("should have verbose patterns defined")
	}
}

func TestTruncateHelper(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	result := truncate("hello world this is long", 10)
	if len(result) > 10 {
		t.Errorf("truncated should be <= 10, got %d: %q", len(result), result)
	}
}

func TestMinInt(t *testing.T) {
	if minInt(3, 5) != 3 {
		t.Error("minInt(3,5) should be 3")
	}
	if minInt(5, 3) != 3 {
		t.Error("minInt(5,3) should be 3")
	}
}

func TestFindRepeatedPhrasesShort(t *testing.T) {
	result := findRepeatedPhrases("hi", 3)
	if len(result) > 0 {
		t.Error("short text should have no repeated phrases")
	}
}

func TestAnalyzeLongPrompt(t *testing.T) {
	text := strings.Repeat("This is a sentence about testing. ", 200)
	a := Analyze(text)
	if a.EstimatedTokens < 100 {
		t.Errorf("long prompt should have many tokens, got %d", a.EstimatedTokens)
	}

	// Should suggest splitting
	foundSplit := false
	for _, s := range a.Suggestions {
		if strings.Contains(s.Description, "splitting") {
			foundSplit = true
		}
	}
	if !foundSplit {
		t.Error("long prompt should suggest splitting")
	}
}
