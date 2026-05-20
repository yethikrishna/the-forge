package tokenizer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCountEmpty(t *testing.T) {
	tok := New(EncodingCl100k)
	result := tok.Count("")

	if result.Tokens != 0 {
		t.Errorf("expected 0 tokens for empty, got %d", result.Tokens)
	}
}

func TestCountBasic(t *testing.T) {
	tok := New(EncodingCl100k)
	result := tok.Count("Hello, world!")

	if result.Tokens <= 0 {
		t.Errorf("expected positive token count, got %d", result.Tokens)
	}
	if result.Characters != 13 {
		t.Errorf("expected 13 chars, got %d", result.Characters)
	}
	if result.Words != 2 {
		t.Errorf("expected 2 words, got %d", result.Words)
	}
}

func TestCountLongText(t *testing.T) {
	tok := New(EncodingCl100k)
	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)
	result := tok.Count(text)

	if result.Tokens <= 0 {
		t.Errorf("expected positive token count, got %d", result.Tokens)
	}
	if result.Lines != 1 {
		t.Errorf("expected 1 line, got %d", result.Lines)
	}
}

func TestCountMultiline(t *testing.T) {
	tok := New(EncodingCl100k)
	text := "Line 1\nLine 2\nLine 3"
	result := tok.Count(text)

	if result.Lines != 3 {
		t.Errorf("expected 3 lines, got %d", result.Lines)
	}
}

func TestCountCode(t *testing.T) {
	tok := New(EncodingCl100k)
	code := `func main() {
		fmt.Println("Hello, world!")
		if err != nil {
			log.Fatal(err)
		}
	}`
	result := tok.Count(code)

	if result.Tokens <= 0 {
		t.Errorf("expected positive token count for code, got %d", result.Tokens)
	}
}

func TestNewForModel(t *testing.T) {
	tests := []struct {
		model    string
		encoding Encoding
	}{
		{"gpt-4", EncodingCl100k},
		{"gpt-4o", EncodingO200k},
		{"gpt-3.5-turbo", EncodingCl100k},
		{"o1", EncodingO200k},
		{"unknown-model", EncodingCl100k}, // default
	}

	for _, tt := range tests {
		tok := NewForModel(tt.model)
		if tok.Encoding != tt.encoding {
			t.Errorf("model %s: expected %s, got %s", tt.model, tt.encoding, tok.Encoding)
		}
	}
}

func TestNewForModelPrefix(t *testing.T) {
	tok := NewForModel("gpt-4-0613")
	if tok.Encoding != EncodingCl100k {
		t.Errorf("expected cl100k for gpt-4 prefix, got %s", tok.Encoding)
	}
}

func TestCountMessages(t *testing.T) {
	tok := New(EncodingCl100k)
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello!"},
		{Role: "assistant", Content: "Hi there!"},
	}

	result := tok.CountMessages(messages)
	if result.Tokens <= 0 {
		t.Errorf("expected positive token count for messages, got %d", result.Tokens)
	}
}

func TestEstimateCost(t *testing.T) {
	ce := EstimateCost("gpt-4", 1000, 500)

	if ce.InputCost <= 0 {
		t.Errorf("expected positive input cost, got %f", ce.InputCost)
	}
	if ce.OutputCost <= 0 {
		t.Errorf("expected positive output cost, got %f", ce.OutputCost)
	}
	if ce.TotalCost != ce.InputCost+ce.OutputCost {
		t.Error("total cost should equal input + output")
	}
	if ce.Currency != "USD" {
		t.Errorf("expected USD, got %s", ce.Currency)
	}
}

func TestEstimateCostKnown(t *testing.T) {
	// gpt-4: $30/1M input, $60/1M output
	ce := EstimateCost("gpt-4", 1_000_000, 1_000_000)

	if ce.InputCost != 30.0 {
		t.Errorf("expected input cost $30, got $%.2f", ce.InputCost)
	}
	if ce.OutputCost != 60.0 {
		t.Errorf("expected output cost $60, got $%.2f", ce.OutputCost)
	}
}

func TestEstimateCostUnknown(t *testing.T) {
	ce := EstimateCost("custom-model", 1000, 500)
	if ce.TotalCost <= 0 {
		t.Error("should estimate cost for unknown model with defaults")
	}
}

func TestBatchCount(t *testing.T) {
	tok := New(EncodingCl100k)
	texts := []string{"Hello", "World", "Test"}
	results := tok.BatchCount(texts)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Tokens <= 0 {
			t.Error("expected positive token count")
		}
	}
}

func TestCodeDensityFactor(t *testing.T) {
	// Plain English
	plain := "The quick brown fox jumps over the lazy dog"
	f1 := codeDensityFactor(plain)

	// Code-heavy
	code := "func() { x := y + z; if x > 0 { return &x } }"
	f2 := codeDensityFactor(code)

	if f2 < f1 {
		t.Error("code should have higher density factor than plain text")
	}
}

func TestTokenCountIsEstimate(t *testing.T) {
	tok := New(EncodingCl100k)
	result := tok.Count("Test text")

	if !result.IsEstimate {
		t.Error("simple tokenizer should mark results as estimates")
	}
}

func TestFormatTokenCount(t *testing.T) {
	tc := &TokenCount{
		Tokens:     100,
		Characters: 380,
		Words:      60,
		Lines:      5,
		Encoding:   EncodingCl100k,
		Model:      "gpt-4",
	}

	output := FormatTokenCount(tc)
	if !strings.Contains(output, "100") {
		t.Error("expected token count in output")
	}
	if !strings.Contains(output, "gpt-4") {
		t.Error("expected model in output")
	}
}

func TestFormatCostEstimate(t *testing.T) {
	ce := &CostEstimate{
		Model:        "gpt-4o",
		InputTokens:  1000,
		OutputTokens: 500,
		InputCost:    0.0025,
		OutputCost:   0.005,
		TotalCost:    0.0075,
		Currency:     "USD",
	}

	output := FormatCostEstimate(ce)
	if !strings.Contains(output, "gpt-4o") {
		t.Error("expected model in output")
	}
	if !strings.Contains(output, "0.0075") {
		t.Error("expected total cost in output")
	}
}

func TestTokenCountSerialization(t *testing.T) {
	tc := &TokenCount{
		Tokens:     150,
		Characters: 570,
		Words:      85,
		Lines:      10,
		Encoding:   EncodingO200k,
		Model:      "gpt-4o",
		IsEstimate: true,
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var tc2 TokenCount
	if err := json.Unmarshal(data, &tc2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if tc2.Tokens != 150 {
		t.Errorf("expected 150 tokens, got %d", tc2.Tokens)
	}
	if tc2.Encoding != EncodingO200k {
		t.Errorf("expected o200k, got %s", tc2.Encoding)
	}
}

func TestModelPricingCompleteness(t *testing.T) {
	// Every model in ModelEncoding should have pricing (or at least not crash)
	for model := range ModelEncoding {
		ce := EstimateCost(model, 100, 100)
		if ce.TotalCost <= 0 {
			t.Errorf("expected positive cost for model %s", model)
		}
	}
}
