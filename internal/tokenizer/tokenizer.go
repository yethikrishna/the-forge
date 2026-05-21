// Package tokenizer provides token counting for LLM cost estimation.
// Supports multiple encoding schemes with configurable models.
//
// If you can't count tokens, you can't count costs.
package tokenizer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Encoding represents a tokenization encoding scheme.
type Encoding string

const (
	EncodingCl100k Encoding = "cl100k_base" // GPT-4, GPT-3.5-turbo
	EncodingP50k   Encoding = "p50k_base"   // Code models
	EncodingR50k   Encoding = "r50k_base"   // Legacy
	EncodingO200k  Encoding = "o200k_base"  // GPT-4o, o1
	EncodingSimple Encoding = "simple"      // Approximate word-based
)

// ModelEncoding maps model names to their encodings.
var ModelEncoding = map[string]Encoding{
	"gpt-4":         EncodingCl100k,
	"gpt-4-turbo":   EncodingCl100k,
	"gpt-4o":        EncodingO200k,
	"gpt-4o-mini":   EncodingO200k,
	"gpt-3.5-turbo": EncodingCl100k,
	"o1":            EncodingO200k,
	"o1-mini":       EncodingO200k,
	"o1-pro":        EncodingO200k,
	"claude-3":      EncodingCl100k, // approximate
	"claude-3.5":    EncodingCl100k,
	"claude-4":      EncodingCl100k,
	"gemini-pro":    EncodingCl100k,
	"llama-3":       EncodingCl100k,
	"mistral":       EncodingCl100k,
}

// CharToTokenRatio maps encodings to approximate characters-per-token ratios.
var CharToTokenRatio = map[Encoding]float64{
	EncodingCl100k: 3.8, // ~3.8 chars per token for English
	EncodingP50k:   3.6,
	EncodingR50k:   3.5,
	EncodingO200k:  3.9,
	EncodingSimple: 4.0,
}

// TokenCount holds the result of token counting.
type TokenCount struct {
	Text       string   `json:"text,omitempty"`
	Tokens     int      `json:"tokens"`
	Characters int      `json:"characters"`
	Words      int      `json:"words"`
	Lines      int      `json:"lines"`
	Encoding   Encoding `json:"encoding"`
	Model      string   `json:"model,omitempty"`
	Ratio      float64  `json:"ratio"` // chars/token actually achieved
	IsEstimate bool     `json:"is_estimate"`
}

// Tokenizer counts tokens.
type Tokenizer struct {
	Encoding Encoding
	Model    string
}

// New creates a tokenizer with the specified encoding.
func New(encoding Encoding) *Tokenizer {
	return &Tokenizer{Encoding: encoding}
}

// NewForModel creates a tokenizer for a specific model.
func NewForModel(model string) *Tokenizer {
	enc, ok := ModelEncoding[model]
	if !ok {
		// Try prefix matching
		for m, e := range ModelEncoding {
			if strings.HasPrefix(model, m) {
				enc = e
				ok = true
				break
			}
		}
		if !ok {
			enc = EncodingCl100k // default
		}
	}
	return &Tokenizer{Encoding: enc, Model: model}
}

// Count counts tokens in a string.
func (t *Tokenizer) Count(text string) *TokenCount {
	if text == "" {
		return &TokenCount{
			Tokens:   0,
			Encoding: t.Encoding,
			Model:    t.Model,
		}
	}

	chars := len([]rune(text))
	words := countWords(text)
	lines := strings.Count(text, "\n") + 1

	ratio, ok := CharToTokenRatio[t.Encoding]
	if !ok {
		ratio = 3.8
	}

	// Estimate tokens based on character-to-token ratio
	// Adjust for code (more tokens per char due to symbols)
	estimated := int(float64(chars) / ratio)

	// Adjust for code content
	codeFactor := codeDensityFactor(text)
	estimated = int(float64(estimated) * codeFactor)

	// Minimum 1 token for non-empty text
	if estimated < 1 {
		estimated = 1
	}

	return &TokenCount{
		Text:       truncateText(text, 100),
		Tokens:     estimated,
		Characters: chars,
		Words:      words,
		Lines:      lines,
		Encoding:   t.Encoding,
		Model:      t.Model,
		Ratio:      float64(chars) / float64(estimated),
		IsEstimate: true,
	}
}

// CountMessages counts tokens for a chat message array.
func (t *Tokenizer) CountMessages(messages []Message) *TokenCount {
	var totalText string
	for _, msg := range messages {
		totalText += msg.Role + ": " + msg.Content + "\n"
	}

	result := t.Count(totalText)

	// Add per-message overhead (role tokens, formatting)
	overhead := len(messages) * 4 // ~4 tokens per message for formatting
	result.Tokens += overhead

	return result
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CostEstimate holds a cost estimate.
type CostEstimate struct {
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	InputCost    float64 `json:"input_cost"`
	OutputCost   float64 `json:"output_cost"`
	TotalCost    float64 `json:"total_cost"`
	Currency     string  `json:"currency"`
}

// ModelPricing maps models to their per-token prices.
var ModelPricing = map[string][2]float64{ // [input_per_1M, output_per_1M]
	"gpt-4":         {30.0, 60.0},
	"gpt-4-turbo":   {10.0, 30.0},
	"gpt-4o":        {2.5, 10.0},
	"gpt-4o-mini":   {0.15, 0.6},
	"gpt-3.5-turbo": {0.5, 1.5},
	"o1":            {15.0, 60.0},
	"o1-mini":       {3.0, 12.0},
	"claude-3.5":    {3.0, 15.0},
	"claude-4":      {3.0, 15.0},
	"gemini-pro":    {1.25, 5.0},
}

// EstimateCost estimates the cost for a model and token counts.
func EstimateCost(model string, inputTokens, outputTokens int) *CostEstimate {
	pricing, ok := ModelPricing[model]
	if !ok {
		// Try prefix match
		for m, p := range ModelPricing {
			if strings.HasPrefix(model, m) {
				pricing = p
				ok = true
				break
			}
		}
		if !ok {
			pricing = [2]float64{5.0, 15.0} // default
		}
	}

	inputCost := float64(inputTokens) / 1_000_000 * pricing[0]
	outputCost := float64(outputTokens) / 1_000_000 * pricing[1]

	return &CostEstimate{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		InputCost:    inputCost,
		OutputCost:   outputCost,
		TotalCost:    inputCost + outputCost,
		Currency:     "USD",
	}
}

// BatchCount counts tokens for multiple texts.
func (t *Tokenizer) BatchCount(texts []string) []*TokenCount {
	results := make([]*TokenCount, len(texts))
	for i, text := range texts {
		results[i] = t.Count(text)
	}
	return results
}

// codeDensityFactor adjusts token estimates for code-heavy content.
func codeDensityFactor(text string) float64 {
	symbolCount := 0
	runeCount := 0

	for _, r := range text {
		runeCount++
		if !unicode.IsLetter(r) && !unicode.IsSpace(r) && !unicode.IsDigit(r) {
			symbolCount++
		}
	}

	if runeCount == 0 {
		return 1.0
	}

	density := float64(symbolCount) / float64(runeCount)

	// Code has ~20-30% symbols; adjust factor accordingly
	if density > 0.3 {
		return 1.2 // more tokens for very symbol-heavy code
	} else if density > 0.15 {
		return 1.1
	}
	return 1.0
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// FormatTokenCount renders a token count for display.
func FormatTokenCount(tc *TokenCount) string {
	model := ""
	if tc.Model != "" {
		model = fmt.Sprintf(" (model: %s)", tc.Model)
	}
	return fmt.Sprintf("Tokens: %d | Chars: %d | Words: %d | Lines: %d | Encoding: %s%s",
		tc.Tokens, tc.Characters, tc.Words, tc.Lines, tc.Encoding, model)
}

// FormatCostEstimate renders a cost estimate for display.
func FormatCostEstimate(ce *CostEstimate) string {
	return fmt.Sprintf("Model: %s | Input: %d tokens ($%.4f) | Output: %d tokens ($%.4f) | Total: $%.4f %s",
		ce.Model, ce.InputTokens, ce.InputCost, ce.OutputTokens, ce.OutputCost, ce.TotalCost, ce.Currency)
}

// SaveTokenCount persists a token count to disk.
func SaveTokenCount(tc *TokenCount, path string) error {
	data, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, data, 0o644)
}
