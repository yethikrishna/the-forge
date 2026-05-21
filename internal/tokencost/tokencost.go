// Package tokencost provides prompt cost analysis and optimization.
// Analyze token usage, find redundancy, and suggest optimizations.
//
// Every token costs. Spend wisely.
package tokencost

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// Pricing holds model pricing for cost estimation.
type Pricing struct {
	InputPer1M  float64 // cost per 1M input tokens
	OutputPer1M float64 // cost per 1M output tokens
}

// Known model pricing (approximate, May 2026).
var ModelPricing = map[string]Pricing{
	"gpt-5-mini":       {InputPer1M: 0.15, OutputPer1M: 0.60},
	"gpt-5":            {InputPer1M: 2.50, OutputPer1M: 10.00},
	"claude-sonnet-4":  {InputPer1M: 3.00, OutputPer1M: 15.00},
	"claude-opus-4":    {InputPer1M: 15.00, OutputPer1M: 75.00},
	"gemini-2.5-pro":   {InputPer1M: 1.25, OutputPer1M: 10.00},
	"gemini-2.5-flash": {InputPer1M: 0.15, OutputPer1M: 0.60},
	"deepseek-r1":      {InputPer1M: 0.55, OutputPer1M: 2.19},
	"llama-4-maverick": {InputPer1M: 0.20, OutputPer1M: 0.80},
}

// Analysis is the result of analyzing a prompt.
type Analysis struct {
	CharCount       int                `json:"char_count"`
	WordCount       int                `json:"word_count"`
	EstimatedTokens int                `json:"estimated_tokens"`
	Sentences       int                `json:"sentences"`
	Redundancies    []Redundancy       `json:"redundancies,omitempty"`
	Suggestions     []Suggestion       `json:"suggestions,omitempty"`
	OptimizedTokens int                `json:"optimized_tokens"`
	SavingsPercent  float64            `json:"savings_percent"`
	CostEstimates   map[string]float64 `json:"cost_estimates,omitempty"`
}

// Redundancy identifies repeated or verbose text.
type Redundancy struct {
	Type        string  `json:"type"` // "repeat", "verbose", "whitespace"
	Description string  `json:"description"`
	Count       int     `json:"count"`
	TokensWaste int     `json:"tokens_waste"`
	Percentage  float64 `json:"percentage"`
}

// Suggestion is an optimization recommendation.
type Suggestion struct {
	Type        string `json:"type"` // "compress", "remove", "restructure"
	Description string `json:"description"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
	TokensSaved int    `json:"tokens_saved"`
}

// Analyze performs a full analysis of a prompt.
func Analyze(prompt string) *Analysis {
	a := &Analysis{
		CharCount: len(prompt),
	}

	// Basic counts
	a.WordCount = countWords(prompt)
	a.EstimatedTokens = EstimateTokens(prompt)
	a.Sentences = countSentences(prompt)

	// Find redundancies
	a.Redundancies = findRedundancies(prompt)

	// Generate suggestions
	a.Suggestions = generateSuggestions(prompt, a.Redundancies)

	// Calculate optimized token count
	waste := 0
	for _, r := range a.Redundancies {
		waste += r.TokensWaste
	}
	a.OptimizedTokens = a.EstimatedTokens - waste
	if a.OptimizedTokens < 0 {
		a.OptimizedTokens = 0
	}
	if a.EstimatedTokens > 0 {
		a.SavingsPercent = math.Round(float64(waste)/float64(a.EstimatedTokens)*10000) / 100
	}

	// Cost estimates
	a.CostEstimates = make(map[string]float64)
	for model := range ModelPricing {
		a.CostEstimates[model] = EstimateCost(model, a.EstimatedTokens, 0)
	}

	return a
}

// EstimateTokens gives a rough token count using the ~4 chars per token heuristic.
// More accurate than nothing, less accurate than tiktoken.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}

	// Heuristic: ~4 chars per token for English, ~2 for CJK
	cjk := 0
	ascii := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			cjk++
		} else if r < 128 {
			ascii++
		} else {
			cjk++ // treat other Unicode as CJK-density
		}
	}

	tokens := ascii/4 + cjk/2
	if tokens == 0 {
		tokens = 1
	}

	// Whitespace tokens
	spaceTokens := strings.Count(text, " ") / 4
	tokens += spaceTokens

	// Punctuation tokens
	punctTokens := 0
	for _, r := range text {
		if unicode.IsPunct(r) && r != ' ' {
			punctTokens++
		}
	}
	tokens += punctTokens / 3

	if tokens < 1 {
		tokens = 1
	}

	return tokens
}

// EstimateCost calculates the cost for a given model and token counts.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := ModelPricing[model]
	if !ok {
		// Use average pricing
		pricing = Pricing{InputPer1M: 1.0, OutputPer1M: 5.0}
	}
	inputCost := float64(inputTokens) / 1_000_000 * pricing.InputPer1M
	outputCost := float64(outputTokens) / 1_000_000 * pricing.OutputPer1M
	return math.Round((inputCost+outputCost)*1_000_000) / 1_000_000
}

// FormatCost formats a cost as a human-readable string.
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	if cost < 1.0 {
		return fmt.Sprintf("$%.3f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

// countWords counts words in text.
func countWords(text string) int {
	return len(strings.Fields(text))
}

// countSentences counts approximate sentences.
func countSentences(text string) int {
	count := 0
	for _, r := range text {
		if r == '.' || r == '!' || r == '?' {
			count++
		}
	}
	if count == 0 && len(strings.TrimSpace(text)) > 0 {
		count = 1
	}
	return count
}

// findRedundancies detects repeated phrases, excessive whitespace, verbose patterns.
func findRedundancies(text string) []Redundancy {
	var reds []Redundancy

	// 1. Excessive whitespace
	spaceCount := countExcessWhitespace(text)
	if spaceCount > 0 {
		wasteTokens := EstimateTokens(strings.Repeat(" ", spaceCount))
		reds = append(reds, Redundancy{
			Type:        "whitespace",
			Description: fmt.Sprintf("%d excess whitespace characters", spaceCount),
			Count:       spaceCount,
			TokensWaste: wasteTokens,
		})
	}

	// 2. Repeated phrases (3+ word sequences appearing 2+ times)
	repeats := findRepeatedPhrases(text, 3)
	for phrase, count := range repeats {
		phraseTokens := EstimateTokens(phrase)
		reds = append(reds, Redundancy{
			Type:        "repeat",
			Description: fmt.Sprintf("%q appears %d times", truncate(phrase, 30), count),
			Count:       count,
			TokensWaste: phraseTokens * (count - 1), // first occurrence is free
		})
	}

	// 3. Verbose patterns
	verbose := findVerbosePatterns(text)
	for _, v := range verbose {
		reds = append(reds, v)
	}

	return reds
}

// countExcessWhitespace counts runs of 2+ spaces or 2+ newlines.
func countExcessWhitespace(text string) int {
	count := 0
	// Multiple spaces
	re := regexp.MustCompile(` {2,}`)
	matches := re.FindAllString(text, -1)
	for _, m := range matches {
		count += len(m) - 1 // excess beyond single space
	}

	// Triple+ newlines
	re2 := regexp.MustCompile(`\n{3,}`)
	matches2 := re2.FindAllString(text, -1)
	for _, m := range matches2 {
		count += len(m) - 2 // excess beyond double newline
	}

	return count
}

// findRepeatedPhrases finds n-word sequences appearing 2+ times.
func findRepeatedPhrases(text string, minWords int) map[string]int {
	words := strings.Fields(text)
	if len(words) < minWords {
		return nil
	}

	phraseCount := make(map[string]int)
	for i := 0; i <= len(words)-minWords; i++ {
		for length := minWords; length <= min(len(words)-i, 8); length++ {
			phrase := strings.Join(words[i:i+length], " ")
			phraseCount[phrase]++
		}
	}

	// Filter to phrases appearing 2+ times
	result := make(map[string]int)
	for phrase, count := range phraseCount {
		if count >= 2 && len(strings.Fields(phrase)) >= minWords {
			// Only keep longest version of overlapping phrases
			isSubstring := false
			for other, otherCount := range phraseCount {
				if other != phrase && otherCount >= 2 && strings.Contains(other, phrase) && len(other) > len(phrase) {
					isSubstring = true
					break
				}
			}
			if !isSubstring {
				result[phrase] = count
			}
		}
	}

	return result
}

// verbosePatterns maps verbose phrases to concise alternatives.
var verbosePatterns = []struct {
	verbose string
	concise string
}{
	{"in order to", "to"},
	{"due to the fact that", "because"},
	{"for the purpose of", "to"},
	{"at this point in time", "now"},
	{"in the event that", "if"},
	{"it is important to note that", "note:"},
	{"it is worth noting that", "note:"},
	{"please note that", "note:"},
	{"as a matter of fact", "actually"},
	{"in spite of the fact that", "although"},
	{"on account of", "because"},
	{"with regard to", "regarding"},
	{"in the near future", "soon"},
	{"a large number of", "many"},
	{"a small number of", "few"},
	{"the vast majority of", "most"},
	{"each and every", "every"},
	{"first and foremost", "first"},
}

func findVerbosePatterns(text string) []Redundancy {
	var reds []Redundancy
	lower := strings.ToLower(text)

	for _, vp := range verbosePatterns {
		count := strings.Count(lower, vp.verbose)
		if count > 0 {
			verboseTokens := EstimateTokens(vp.verbose)
			conciseTokens := EstimateTokens(vp.concise)
			savings := (verboseTokens - conciseTokens) * count
			if savings < 0 {
				savings = 0
			}
			reds = append(reds, Redundancy{
				Type:        "verbose",
				Description: fmt.Sprintf("%q → %q (×%d)", vp.verbose, vp.concise, count),
				Count:       count,
				TokensWaste: savings,
			})
		}
	}

	return reds
}

// generateSuggestions creates actionable optimization suggestions.
func generateSuggestions(text string, reds []Redundancy) []Suggestion {
	var suggestions []Suggestion

	for _, r := range reds {
		switch r.Type {
		case "whitespace":
			suggestions = append(suggestions, Suggestion{
				Type:        "remove",
				Description: "Remove excess whitespace",
				TokensSaved: r.TokensWaste,
			})
		case "repeat":
			suggestions = append(suggestions, Suggestion{
				Type:        "restructure",
				Description: fmt.Sprintf("Consolidate repeated phrase: %s", r.Description),
				TokensSaved: r.TokensWaste,
			})
		case "verbose":
			suggestions = append(suggestions, Suggestion{
				Type:        "compress",
				Description: r.Description,
				TokensSaved: r.TokensWaste,
			})
		}
	}

	// General suggestions based on length
	tokens := EstimateTokens(text)
	if tokens > 4000 {
		suggestions = append(suggestions, Suggestion{
			Type:        "compress",
			Description: fmt.Sprintf("Prompt is %d tokens — consider splitting into system + user messages", tokens),
			TokensSaved: tokens / 4,
		})
	}

	if tokens > 1000 && countSentences(text) < 3 {
		suggestions = append(suggestions, Suggestion{
			Type:        "restructure",
			Description: "Long prompt with few sentences — consider using bullet points or numbered lists",
		})
	}

	return suggestions
}

// CompareModels returns cost estimates for all models.
func CompareModels(inputTokens, outputTokens int) map[string]float64 {
	results := make(map[string]float64)
	for model := range ModelPricing {
		results[model] = EstimateCost(model, inputTokens, outputTokens)
	}
	return results
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
