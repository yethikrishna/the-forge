// Package cost provides LLM pricing data and comparison utilities.
// Know the price of every spell before you cast it.
package cost

import (
	"fmt"
	"sort"
	"strings"
)

// Pricing represents per-token pricing for a model.
type Pricing struct {
	InputPer1M       float64 `json:"input_per_1m"`       // USD per 1M input tokens
	OutputPer1M      float64 `json:"output_per_1m"`      // USD per 1M output tokens
	CacheReadPer1M   float64 `json:"cache_read_per_1m"`  // USD per 1M cached input tokens
	CacheWritePer1M  float64 `json:"cache_write_per_1m"` // USD per 1M cache write tokens
}

// ModelPricing bundles a model with its pricing.
type ModelPricing struct {
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Pricing  Pricing `json:"pricing"`
}

// EstimateResult holds a cost estimate.
type EstimateResult struct {
	Model       string  `json:"model"`
	InputTokens int64   `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	InputCost   float64 `json:"input_cost"`
	OutputCost  float64 `json:"output_cost"`
	TotalCost   float64 `json:"total_cost"`
}

// Catalog returns all known model pricing.
func Catalog() []ModelPricing {
	return []ModelPricing{
		// Anthropic
		{Provider: "anthropic", Model: "claude-sonnet-4-20250514", Pricing: Pricing{InputPer1M: 3, OutputPer1M: 15, CacheReadPer1M: 0.3, CacheWritePer1M: 3.75}},
		{Provider: "anthropic", Model: "claude-opus-4-20250514", Pricing: Pricing{InputPer1M: 15, OutputPer1M: 75, CacheReadPer1M: 1.5, CacheWritePer1M: 18.75}},
		{Provider: "anthropic", Model: "claude-haiku-3.5", Pricing: Pricing{InputPer1M: 0.8, OutputPer1M: 4, CacheReadPer1M: 0.08, CacheWritePer1M: 1}},
		// OpenAI
		{Provider: "openai", Model: "gpt-4o", Pricing: Pricing{InputPer1M: 2.5, OutputPer1M: 10, CacheReadPer1M: 1.25}},
		{Provider: "openai", Model: "gpt-4o-mini", Pricing: Pricing{InputPer1M: 0.15, OutputPer1M: 0.6, CacheReadPer1M: 0.075}},
		{Provider: "openai", Model: "gpt-4.1", Pricing: Pricing{InputPer1M: 2, OutputPer1M: 8, CacheReadPer1M: 0.5, CacheWritePer1M: 2}},
		{Provider: "openai", Model: "gpt-4.1-mini", Pricing: Pricing{InputPer1M: 0.4, OutputPer1M: 1.6, CacheReadPer1M: 0.1, CacheWritePer1M: 0.4}},
		{Provider: "openai", Model: "gpt-4.1-nano", Pricing: Pricing{InputPer1M: 0.1, OutputPer1M: 0.4, CacheReadPer1M: 0.025, CacheWritePer1M: 0.1}},
		{Provider: "openai", Model: "o3", Pricing: Pricing{InputPer1M: 2, OutputPer1M: 8, CacheReadPer1M: 0.5}},
		{Provider: "openai", Model: "o4-mini", Pricing: Pricing{InputPer1M: 1.1, OutputPer1M: 4.4, CacheReadPer1M: 0.275}},
		// Google
		{Provider: "google", Model: "gemini-2.5-pro", Pricing: Pricing{InputPer1M: 1.25, OutputPer1M: 10, CacheReadPer1M: 0.31}},
		{Provider: "google", Model: "gemini-2.5-flash", Pricing: Pricing{InputPer1M: 0.15, OutputPer1M: 0.6, CacheReadPer1M: 0.0375}},
		{Provider: "google", Model: "gemini-2.0-flash", Pricing: Pricing{InputPer1M: 0.1, OutputPer1M: 0.4, CacheReadPer1M: 0.025}},
		// DeepSeek
		{Provider: "deepseek", Model: "deepseek-r1", Pricing: Pricing{InputPer1M: 0.55, OutputPer1M: 2.19, CacheReadPer1M: 0.14, CacheWritePer1M: 0.55}},
		{Provider: "deepseek", Model: "deepseek-v3", Pricing: Pricing{InputPer1M: 0.27, OutputPer1M: 1.1, CacheReadPer1M: 0.07, CacheWritePer1M: 0.27}},
		// Meta
		{Provider: "meta", Model: "llama-4-maverick", Pricing: Pricing{InputPer1M: 0.2, OutputPer1M: 0.8}},
		{Provider: "meta", Model: "llama-4-scout", Pricing: Pricing{InputPer1M: 0.1, OutputPer1M: 0.3}},
		// Mistral
		{Provider: "mistral", Model: "mistral-large", Pricing: Pricing{InputPer1M: 2, OutputPer1M: 6}},
		{Provider: "mistral", Model: "mistral-medium", Pricing: Pricing{InputPer1M: 0.4, OutputPer1M: 2}},
	}
}

// FindModel looks up pricing for a specific model.
func FindModel(name string) (*ModelPricing, bool) {
	name = strings.ToLower(name)
	for _, mp := range Catalog() {
		if strings.ToLower(mp.Model) == name || strings.ToLower(mp.Provider+"/"+mp.Model) == name {
			return &mp, true
		}
	}
	return nil, false
}

// Estimate calculates the cost for a given token usage.
func Estimate(model string, inputTokens, outputTokens int64) (*EstimateResult, error) {
	mp, ok := FindModel(model)
	if !ok {
		return nil, fmt.Errorf("model %q not found in pricing catalog", model)
	}

	inputCost := float64(inputTokens) / 1_000_000 * mp.Pricing.InputPer1M
	outputCost := float64(outputTokens) / 1_000_000 * mp.Pricing.OutputPer1M

	return &EstimateResult{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		InputCost:    inputCost,
		OutputCost:   outputCost,
		TotalCost:    inputCost + outputCost,
	}, nil
}

// Compare estimates costs across all models for given token usage.
func Compare(inputTokens, outputTokens int64) []EstimateResult {
	var results []EstimateResult

	for _, mp := range Catalog() {
		inputCost := float64(inputTokens) / 1_000_000 * mp.Pricing.InputPer1M
		outputCost := float64(outputTokens) / 1_000_000 * mp.Pricing.OutputPer1M

		results = append(results, EstimateResult{
			Model:        mp.Provider + "/" + mp.Model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			InputCost:    inputCost,
			OutputCost:   outputCost,
			TotalCost:    inputCost + outputCost,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalCost < results[j].TotalCost
	})

	return results
}

// FormatCost formats a USD cost for display.
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.6f", cost)
	}
	if cost < 1 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}
