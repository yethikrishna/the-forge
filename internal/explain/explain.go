// Package explain generates human-readable explanations of agent decisions.
// Understanding why the sword struck is as important as the strike itself.
package explain

import (
	"fmt"
	"strings"
	"time"
)

// Decision represents an agent decision to be explained.
type Decision struct {
	Agent      string            `json:"agent"`
	Model      string            `json:"model"`
	Action     string            `json:"action"`
	Reason     string            `json:"reason"`
	Inputs     []string          `json:"inputs,omitempty"`
	Outputs    []string          `json:"outputs,omitempty"`
	FilesRead  []string          `json:"files_read,omitempty"`
	FilesWrite []string          `json:"files_write,omitempty"`
	Cost       float64           `json:"cost,omitempty"`
	Duration   string            `json:"duration,omitempty"`
	Confidence float64           `json:"confidence,omitempty"`
	Alternatives []Alternative   `json:"alternatives,omitempty"`
	Context    map[string]string `json:"context,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
}

// Alternative is a considered-but-not-chosen option.
type Alternative struct {
	Option    string  `json:"option"`
	Reason    string  `json:"reason"`
	Score     float64 `json:"score"`
	Rejected  bool    `json:"rejected"`
}

// Explanation is a formatted explanation of a decision.
type Explanation struct {
	Decision   Decision `json:"decision"`
	Summary    string   `json:"summary"`
	Reasoning  []string `json:"reasoning"`
	Trace      []string `json:"trace"`
	Warnings   []string `json:"warnings,omitempty"`
	Confidence string   `json:"confidence"`
}

// Explainer generates explanations for agent decisions.
type Explainer struct {
	modelAliases map[string]string
}

// NewExplainer creates a new explainer.
func NewExplainer() *Explainer {
	return &Explainer{
		modelAliases: map[string]string{
			"anthropic/claude-sonnet-4-20250514": "Claude Sonnet",
			"anthropic/claude-opus-4-20250514":   "Claude Opus",
			"openai/gpt-4o":                     "GPT-4o",
			"openai/gpt-5-mini":                 "GPT-5 Mini",
			"google/gemini-2.5-pro":             "Gemini Pro",
		},
	}
}

// Explain generates an explanation for a decision.
func (e *Explainer) Explain(d Decision) Explanation {
	exp := Explanation{
		Decision: d,
	}

	// Generate summary
	exp.Summary = e.generateSummary(d)

	// Generate reasoning chain
	exp.Reasoning = e.generateReasoning(d)

	// Generate trace
	exp.Trace = e.generateTrace(d)

	// Generate warnings
	exp.Warnings = e.generateWarnings(d)

	// Confidence level
	switch {
	case d.Confidence >= 0.9:
		exp.Confidence = "high"
	case d.Confidence >= 0.7:
		exp.Confidence = "medium"
	case d.Confidence >= 0.4:
		exp.Confidence = "low"
	default:
		exp.Confidence = "very low"
	}

	return exp
}

// FormatHuman formats an explanation for human reading.
func FormatHuman(exp Explanation) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("🔍 Decision Explanation: %s\n", exp.Decision.Action))
	b.WriteString(fmt.Sprintf("   Agent:  %s\n", exp.Decision.Agent))
	b.WriteString(fmt.Sprintf("   Model:  %s\n", exp.Decision.Model))
	b.WriteString(fmt.Sprintf("   Confidence: %s (%.0f%%)\n\n", exp.Confidence, exp.Decision.Confidence*100))

	b.WriteString("Summary:\n")
	b.WriteString(fmt.Sprintf("  %s\n\n", exp.Summary))

	if len(exp.Reasoning) > 0 {
		b.WriteString("Reasoning:\n")
		for i, r := range exp.Reasoning {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, r))
		}
		b.WriteString("\n")
	}

	if len(exp.Trace) > 0 {
		b.WriteString("Trace:\n")
		for _, t := range exp.Trace {
			b.WriteString(fmt.Sprintf("  → %s\n", t))
		}
		b.WriteString("\n")
	}

	if len(exp.Warnings) > 0 {
		b.WriteString("⚠ Warnings:\n")
		for _, w := range exp.Warnings {
			b.WriteString(fmt.Sprintf("  • %s\n", w))
		}
		b.WriteString("\n")
	}

	if len(exp.Decision.Alternatives) > 0 {
		b.WriteString("Alternatives considered:\n")
		for _, alt := range exp.Decision.Alternatives {
			status := "chosen"
			if alt.Rejected {
				status = fmt.Sprintf("rejected (%s)", alt.Reason)
			}
			b.WriteString(fmt.Sprintf("  • %s — %s\n", alt.Option, status))
		}
	}

	return b.String()
}

// FormatJSON formats an explanation as JSON-like text.
func FormatJSON(exp Explanation) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`{"action": "%s", "agent": "%s", "model": "%s", "confidence": "%s", "summary": "%s"`,
		exp.Decision.Action, exp.Decision.Agent, exp.Decision.Model, exp.Confidence, exp.Summary))
	return b.String()
}

func (e *Explainer) generateSummary(d Decision) string {
	model := e.resolveModel(d.Model)

	if d.Reason != "" {
		return fmt.Sprintf("Agent %s (%s) chose to %s because: %s", d.Agent, model, d.Action, d.Reason)
	}

	return fmt.Sprintf("Agent %s (%s) performed action: %s", d.Agent, model, d.Action)
}

func (e *Explainer) generateReasoning(d Decision) []string {
	var reasoning []string

	if d.Reason != "" {
		reasoning = append(reasoning, d.Reason)
	}

	// Model choice reasoning
	if d.Model != "" {
		model := e.resolveModel(d.Model)
		reasoning = append(reasoning, fmt.Sprintf("Used model %s for this task", model))
	}

	// File-based reasoning
	for _, f := range d.FilesRead {
		reasoning = append(reasoning, fmt.Sprintf("Read file %s for context", f))
	}

	// Cost reasoning
	if d.Cost > 0 {
		reasoning = append(reasoning, fmt.Sprintf("This action cost $%.4f", d.Cost))
	}

	// Alternative reasoning
	for _, alt := range d.Alternatives {
		if alt.Rejected {
			reasoning = append(reasoning, fmt.Sprintf("Considered %s but rejected: %s", alt.Option, alt.Reason))
		}
	}

	return reasoning
}

func (e *Explainer) generateTrace(d Decision) []string {
	var trace []string

	if d.Agent != "" {
		trace = append(trace, fmt.Sprintf("Agent %s activated", d.Agent))
	}

	for _, input := range d.Inputs {
		trace = append(trace, fmt.Sprintf("Received input: %s", input))
	}

	for _, f := range d.FilesRead {
		trace = append(trace, fmt.Sprintf("Read: %s", f))
	}

	trace = append(trace, fmt.Sprintf("Executed: %s", d.Action))

	for _, f := range d.FilesWrite {
		trace = append(trace, fmt.Sprintf("Modified: %s", f))
	}

	for _, output := range d.Outputs {
		trace = append(trace, fmt.Sprintf("Produced output: %s", output))
	}

	if d.Duration != "" {
		trace = append(trace, fmt.Sprintf("Completed in %s", d.Duration))
	}

	return trace
}

func (e *Explainer) generateWarnings(d Decision) []string {
	var warnings []string

	if d.Confidence < 0.5 {
		warnings = append(warnings, "Low confidence — result may be unreliable")
	}

	if d.Cost > 0.10 {
		warnings = append(warnings, fmt.Sprintf("High cost: $%.4f", d.Cost))
	}

	if len(d.FilesWrite) > 3 {
		warnings = append(warnings, fmt.Sprintf("Modified %d files — review changes carefully", len(d.FilesWrite)))
	}

	return warnings
}

func (e *Explainer) resolveModel(model string) string {
	if alias, ok := e.modelAliases[model]; ok {
		return alias
	}
	return model
}
