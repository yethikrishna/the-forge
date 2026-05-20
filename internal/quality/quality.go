// Package quality provides multi-dimensional agent output quality scoring.
// Score agent responses on correctness, style, security, cost, and completeness.
//
// Measure twice, deploy once.
package quality

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Dimension is a quality scoring axis.
type Dimension string

const (
	DimCorrectness  Dimension = "correctness"
	DimCompleteness Dimension = "completeness"
	DimStyle        Dimension = "style"
	DimSecurity     Dimension = "security"
	DimClarity      Dimension = "clarity"
	DimEfficiency   Dimension = "efficiency"
	DimSafety       Dimension = "safety"
)

// AllDimensions returns all scoring dimensions.
func AllDimensions() []Dimension {
	return []Dimension{
		DimCorrectness, DimCompleteness, DimStyle,
		DimSecurity, DimClarity, DimEfficiency, DimSafety,
	}
}

// Score is a single dimension score (0-100).
type Score struct {
	Dimension Dimension `json:"dimension"`
	Value     float64   `json:"value"`     // 0-100
	Weight    float64   `json:"weight"`    // relative weight
	Reason    string    `json:"reason"`    // why this score
}

// Report is a full quality assessment.
type Report struct {
	ID          string    `json:"id"`
	Agent       string    `json:"agent,omitempty"`
	Model       string    `json:"model,omitempty"`
	Prompt      string    `json:"prompt,omitempty"`
	Response    string    `json:"response,omitempty"`
	Scores      []Score   `json:"scores"`
	WeightedAvg float64   `json:"weighted_avg"`
	Composite   float64   `json:"composite"` // alias for WeightedAvg
	Grade       Grade     `json:"grade"`
	Timestamp   time.Time `json:"timestamp"`
	Duration    string    `json:"duration,omitempty"`
	TokensIn    int       `json:"tokens_in,omitempty"`
	TokensOut   int       `json:"tokens_out,omitempty"`
}

// Grade is a letter grade (A-F).
type Grade string

const (
	GradeAPlus Grade = "A+"
	GradeA     Grade = "A"
	GradeAMinus Grade = "A-"
	GradeBPlus Grade = "B+"
	GradeB     Grade = "B"
	GradeBMinus Grade = "B-"
	GradeCPlus Grade = "C+"
	GradeC     Grade = "C"
	GradeD     Grade = "D"
	GradeF     Grade = "F"
)

// ScoreToGrade converts a 0-100 score to a letter grade.
func ScoreToGrade(score float64) Grade {
	switch {
	case score >= 97:
		return GradeAPlus
	case score >= 93:
		return GradeA
	case score >= 90:
		return GradeAMinus
	case score >= 87:
		return GradeBPlus
	case score >= 83:
		return GradeB
	case score >= 80:
		return GradeBMinus
	case score >= 77:
		return GradeCPlus
	case score >= 70:
		return GradeC
	case score >= 60:
		return GradeD
	default:
		return GradeF
	}
}

// Scorer evaluates agent responses across multiple dimensions.
type Scorer struct {
	Weights map[Dimension]float64
}

// NewScorer creates a scorer with default weights.
func NewScorer() *Scorer {
	return &Scorer{
		Weights: map[Dimension]float64{
			DimCorrectness:  0.25,
			DimCompleteness: 0.20,
			DimStyle:        0.10,
			DimSecurity:     0.15,
			DimClarity:      0.15,
			DimEfficiency:   0.10,
			DimSafety:       0.05,
		},
	}
}

// SetWeight changes the weight for a dimension.
func (s *Scorer) SetWeight(dim Dimension, w float64) {
	s.Weights[dim] = w
}

// EvaluateInput is the input for the Evaluate method (compatible with abtest package).
type EvaluateInput struct {
	Prompt   string
	Response string
	Model    string
	Agent    string
}

// Evaluate scores a prompt-response pair and returns a Report (error-compatible signature).
func (s *Scorer) Evaluate(input EvaluateInput) (*Report, error) {
	report := s.Score(input.Prompt, input.Response)
	report.Model = input.Model
	report.Agent = input.Agent
	return report, nil
}

// Score evaluates a prompt-response pair and returns a quality report.
func (s *Scorer) Score(prompt, response string) *Report {
	report := &Report{
		ID:        fmt.Sprintf("q-%d", time.Now().UnixNano()),
		Prompt:    prompt,
		Response:  response,
		Timestamp: time.Now(),
	}

	// Run each dimension scorer
	report.Scores = []Score{
		s.scoreCorrectness(prompt, response),
		s.scoreCompleteness(prompt, response),
		s.scoreStyle(response),
		s.scoreSecurity(response),
		s.scoreClarity(response),
		s.scoreEfficiency(prompt, response),
		s.scoreSafety(response),
	}

	// Calculate weighted average
	totalWeight := 0.0
	totalScore := 0.0
	for _, sc := range report.Scores {
		w, ok := s.Weights[sc.Dimension]
		if !ok {
			w = 1.0 / float64(len(report.Scores))
		}
		totalScore += sc.Value * w
		totalWeight += w
	}
	if totalWeight > 0 {
		report.WeightedAvg = math.Round(totalScore/totalWeight*100) / 100
	}
	report.Grade = ScoreToGrade(report.WeightedAvg)
	report.Composite = report.WeightedAvg

	return report
}

// scoreCorrectness evaluates whether the response seems to address the prompt.
func (s *Scorer) scoreCorrectness(prompt, response string) Score {
	score := 50.0 // neutral baseline
	reason := "basic relevance"

	_ = strings.ToLower(prompt) // promptLower kept for future use
	responseLower := strings.ToLower(response)

	// Check if key prompt words appear in response
	promptWords := extractKeywords(prompt)
	if len(promptWords) == 0 {
		return Score{Dimension: DimCorrectness, Value: 70, Weight: s.Weights[DimCorrectness], Reason: "no specific keywords to match"}
	}

	matched := 0
	for _, w := range promptWords {
		if strings.Contains(responseLower, w) {
			matched++
		}
	}

	ratio := float64(matched) / float64(len(promptWords))
	score = 30 + ratio*60 // 30-90 range based on keyword overlap

	// Bonus for structured response (numbered lists, headers)
	if strings.Contains(response, "1.") || strings.Contains(response, "##") {
		score += 5
	}

	// Penalty for very short response to a complex prompt
	if len(promptWords) > 5 && len(response) < 100 {
		score -= 15
		reason = "response too short for complex prompt"
	} else {
		reason = fmt.Sprintf("%d/%d prompt keywords addressed (%.0f%%)", matched, len(promptWords), ratio*100)
	}

	score = clamp(score)
	return Score{Dimension: DimCorrectness, Value: score, Weight: s.Weights[DimCorrectness], Reason: reason}
}

// scoreCompleteness checks if the response covers the prompt thoroughly.
func (s *Scorer) scoreCompleteness(prompt, response string) Score {
	score := 60.0
	reason := "adequate coverage"

	// Count question words and check if they're answered
	questionIndicators := []string{"how", "what", "why", "when", "where", "which", "who"}
	questions := 0
	addressed := 0

	sentences := strings.Split(prompt, ".")
	for _, sent := range sentences {
		sentLower := strings.ToLower(sent)
		for _, q := range questionIndicators {
			if strings.Contains(sentLower, q) {
				questions++
				// Check if response addresses this
				words := strings.Fields(sent)
				keyFound := false
				for _, w := range words {
					if len(w) > 3 && strings.Contains(strings.ToLower(response), strings.ToLower(w)) {
						keyFound = true
						break
					}
				}
				if keyFound {
					addressed++
				}
				break
			}
		}
	}

	if questions > 0 {
		ratio := float64(addressed) / float64(questions)
		score = 30 + ratio*60
		reason = fmt.Sprintf("%d/%d questions addressed", addressed, questions)
	}

	// Length-based assessment
	if len(response) > 500 {
		score += 5
	}
	if len(response) > 1000 {
		score += 5
	}
	if len(response) < 50 && len(prompt) > 50 {
		score -= 20
		reason = "response too brief for the question"
	}

	score = clamp(score)
	return Score{Dimension: DimCompleteness, Value: score, Weight: s.Weights[DimCompleteness], Reason: reason}
}

// scoreStyle evaluates formatting and readability.
func (s *Scorer) scoreStyle(response string) Score {
	score := 70.0
	reason := "acceptable style"

	// Good: uses formatting
	hasHeadings := strings.Contains(response, "#") || strings.Contains(response, "**")
	hasLists := strings.Contains(response, "- ") || strings.Contains(response, "1.")
	hasCodeBlocks := strings.Contains(response, "```")

	features := 0
	if hasHeadings { features++ }
	if hasLists { features++ }
	if hasCodeBlocks { features++ }

	score += float64(features) * 7 // up to +21

	// Bad: excessive repetition
	if hasExcessiveRepetition(response) {
		score -= 15
		reason = "contains repetitive content"
	}

	// Bad: very long paragraphs (no line breaks)
	paragraphs := strings.Split(response, "\n\n")
	maxParaLen := 0
	for _, p := range paragraphs {
		if len(p) > maxParaLen {
			maxParaLen = len(p)
		}
	}
	if maxParaLen > 1000 {
		score -= 10
		reason = "very long unbroken paragraphs"
	}

	if reason == "acceptable style" && features > 0 {
		reason = fmt.Sprintf("uses %d formatting features (headings/lists/code)", features)
	}

	score = clamp(score)
	return Score{Dimension: DimStyle, Value: score, Weight: s.Weights[DimStyle], Reason: reason}
}

// scoreSecurity checks for security anti-patterns.
func (s *Scorer) scoreSecurity(response string) Score {
	score := 90.0
	reason := "no security issues detected"

	securityIssues := []struct {
		pattern string
		desc    string
		penalty float64
	}{
		{"password =", "hardcoded password", 20},
		{"api_key =", "hardcoded API key", 25},
		{"secret =", "hardcoded secret", 20},
		{"token =", "hardcoded token", 20},
		{"Authorization: Basic", "basic auth in code", 15},
		{"eval(", "eval usage", 15},
		{"exec(", "exec usage", 10},
		{"os.system(", "shell execution", 15},
		{"rm -rf", "dangerous rm command", 20},
		{"sudo ", "sudo usage", 5},
		{"chmod 777", "insecure permissions", 15},
		{"INSERT INTO", "raw SQL (possible injection)", 5},
		{"<script>", "script injection risk", 20},
	}

	lower := strings.ToLower(response)
	issueCount := 0
	for _, issue := range securityIssues {
		if strings.Contains(lower, strings.ToLower(issue.pattern)) {
			score -= issue.penalty
			issueCount++
		}
	}

	if issueCount > 0 {
		reason = fmt.Sprintf("%d security concern(s) detected", issueCount)
	}

	score = clamp(score)
	return Score{Dimension: DimSecurity, Value: score, Weight: s.Weights[DimSecurity], Reason: reason}
}

// scoreClarity evaluates how clear and understandable the response is.
func (s *Scorer) scoreClarity(response string) Score {
	score := 70.0
	reason := "acceptable clarity"

	// Short sentences are clearer
	sentences := splitSentences(response)
	if len(sentences) == 0 {
		return Score{Dimension: DimClarity, Value: 50, Weight: s.Weights[DimClarity], Reason: "no content to evaluate"}
	}

	avgLen := 0
	for _, s := range sentences {
		avgLen += len(s)
	}
	avgLen /= len(sentences)

	// Sweet spot: 15-25 words per sentence
	if avgLen < 150 {
		score += 10
	} else if avgLen > 400 {
		score -= 15
		reason = "very long sentences reduce clarity"
	}

	// Transitions and connective words improve clarity
	connectives := []string{"however", "therefore", "for example", "in addition", "furthermore", "specifically", "note that"}
	connCount := 0
	lower := strings.ToLower(response)
	for _, c := range connectives {
		if strings.Contains(lower, c) {
			connCount++
		}
	}
	if connCount > 0 {
		score += float64(min(connCount, 3)) * 3
		reason = fmt.Sprintf("uses %d connective phrases", connCount)
	}

	score = clamp(score)
	return Score{Dimension: DimClarity, Value: score, Weight: s.Weights[DimClarity], Reason: reason}
}

// scoreEfficiency evaluates token efficiency.
func (s *Scorer) scoreEfficiency(prompt, response string) Score {
	score := 70.0
	reason := "reasonable efficiency"

	promptLen := len(prompt)
	responseLen := len(response)

	// Ratio of response to prompt
	ratio := 0.0
	if promptLen > 0 {
		ratio = float64(responseLen) / float64(promptLen)
	}

	// Ideal ratio: 0.5x to 3x the prompt length
	if ratio >= 0.5 && ratio <= 3.0 {
		score = 85
		reason = fmt.Sprintf("response/prompt ratio %.1fx (ideal range)", ratio)
	} else if ratio > 3.0 && ratio <= 6.0 {
		score = 65
		reason = fmt.Sprintf("response/prompt ratio %.1fx (slightly verbose)", ratio)
	} else if ratio > 6.0 {
		score = 40
		reason = fmt.Sprintf("response/prompt ratio %.1fx (excessively verbose)", ratio)
	} else if ratio < 0.5 && ratio > 0 {
		score = 60
		reason = fmt.Sprintf("response/prompt ratio %.1fx (may be too terse)", ratio)
	}

	// Penalty for empty response
	if responseLen == 0 {
		score = 0
		reason = "empty response"
	}

	score = clamp(score)
	return Score{Dimension: DimEfficiency, Value: score, Weight: s.Weights[DimEfficiency], Reason: reason}
}

// scoreSafety checks for harmful content patterns.
func (s *Scorer) scoreSafety(response string) Score {
	score := 95.0
	reason := "no safety concerns"

	harmfulPatterns := []struct {
		pattern string
		desc    string
		penalty float64
	}{
		{"ignore previous instructions", "prompt injection attempt", 50},
		{"disregard all above", "prompt injection attempt", 50},
		{"you are now", "identity manipulation", 20},
		{"bypass", "bypass instruction", 15},
		{"jailbreak", "jailbreak reference", 30},
	}

	lower := strings.ToLower(response)
	for _, p := range harmfulPatterns {
		if strings.Contains(lower, p.pattern) {
			score -= p.penalty
			reason = fmt.Sprintf("safety concern: %s", p.desc)
		}
	}

	score = clamp(score)
	return Score{Dimension: DimSafety, Value: score, Weight: s.Weights[DimSafety], Reason: reason}
}

// Compare compares two quality reports and returns differences.
func Compare(a, b *Report) Comparison {
	return Comparison{
		A:         a,
		B:         b,
		ScoreDiff: b.WeightedAvg - a.WeightedAvg,
		GradeDiff: string(b.Grade) + " vs " + string(a.Grade),
		Winner:    winner(a.WeightedAvg, b.WeightedAvg),
	}
}

// Comparison holds the result of comparing two reports.
type Comparison struct {
	A         *Report  `json:"a"`
	B         *Report  `json:"b"`
	ScoreDiff float64  `json:"score_diff"`
	GradeDiff string   `json:"grade_diff"`
	Winner    string   `json:"winner"` // "a", "b", or "tie"
}

// helpers

func clamp(v float64) float64 {
	if v < 0 { return 0 }
	if v > 100 { return 100 }
	return math.Round(v*100) / 100
}

func extractKeywords(text string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"through": true, "during": true, "before": true, "after": true,
		"and": true, "but": true, "or": true, "nor": true, "not": true,
		"so": true, "yet": true, "both": true, "either": true, "neither": true,
		"each": true, "every": true, "all": true, "any": true, "few": true,
		"more": true, "most": true, "other": true, "some": true, "such": true,
		"no": true, "only": true, "own": true, "same": true, "than": true,
		"too": true, "very": true, "just": true, "because": true,
		"it": true, "its": true, "this": true, "that": true, "these": true,
		"those": true, "i": true, "me": true, "my": true, "we": true,
		"you": true, "your": true, "he": true, "she": true, "they": true,
		"what": true, "how": true, "if": true, "then": true,
	}

	var keywords []string
	for _, w := range strings.Fields(strings.ToLower(text)) {
		w = strings.Trim(w, ".,!?;:\"'()[]{}")
		if len(w) >= 3 && !stopWords[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

func hasExcessiveRepetition(text string) bool {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 20 {
		return false
	}

	// Check if any 3-word sequence appears 3+ times
	counts := make(map[string]int)
	for i := 0; i <= len(words)-3; i++ {
		seq := strings.Join(words[i:i+3], " ")
		counts[seq]++
		if counts[seq] >= 3 {
			return true
		}
	}
	return false
}

func splitSentences(text string) []string {
	var sentences []string
	current := ""
	for _, r := range text {
		current += string(r)
		if r == '.' || r == '!' || r == '?' || r == '\n' {
			s := strings.TrimSpace(current)
			if s != "" {
				sentences = append(sentences, s)
			}
			current = ""
		}
	}
	if s := strings.TrimSpace(current); s != "" {
		sentences = append(sentences, s)
	}
	return sentences
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

func winner(a, b float64) string {
	diff := b - a
	if diff > 2 { return "b" }
	if diff < -2 { return "a" }
	return "tie"
}
