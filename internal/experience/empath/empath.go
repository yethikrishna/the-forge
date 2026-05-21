// Package empath detects user frustration from message patterns
// and suggests adaptive response strategies.
//
// Read the room. Read the user.
package empath

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// FrustrationLevel represents how frustrated a user seems.
type FrustrationLevel string

const (
	LevelCalm       FrustrationLevel = "calm"
	LevelNeutral    FrustrationLevel = "neutral"
	LevelAnnoyed    FrustrationLevel = "annoyed"
	LevelFrustrated FrustrationLevel = "frustrated"
	LevelAngry      FrustrationLevel = "angry"
)

// Signal represents a detected frustration signal.
type Signal struct {
	Type    string  `json:"type"`    // "keyword", "pattern", "caps", "punctuation", "repetition", "urgency"
	Match   string  `json:"match"`   // what was matched
	Weight  float64 `json:"weight"`  // 0-1, how strong
	Message string  `json:"message"` // human-readable description
}

// Analysis is the result of analyzing a message for frustration.
type Analysis struct {
	Level      FrustrationLevel `json:"level"`
	Score      float64          `json:"score"`      // 0-100
	Signals    []Signal         `json:"signals"`
	Confidence float64          `json:"confidence"` // 0-1
	Strategy   Strategy         `json:"strategy"`
}

// Strategy is the recommended response strategy.
type Strategy struct {
	Tone          string   `json:"tone"`           // "empathetic", "calm", "direct", "supportive"
	Avoid         []string `json:"avoid"`          // things to avoid saying
	Suggestions   []string `json:"suggestions"`    // response suggestions
	Escalate      bool     `json:"escalate"`       // whether to escalate to human
	SlowDown      bool     `json:"slow_down"`      // take extra care in response
	Acknowledge   bool     `json:"acknowledge"`    // explicitly acknowledge frustration
	MaxWords      int      `json:"max_words"`      // suggested response length (0 = no limit)
}

// Analyzer detects frustration in messages.
type Analyzer struct {
	history   []messageEntry
	maxHistory int
}

type messageEntry struct {
	Text      string
	Timestamp time.Time
	Score     float64
}

// NewAnalyzer creates a frustration analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		maxHistory: 50,
	}
}

// Analyze analyzes a single message for frustration signals.
func (a *Analyzer) Analyze(message string) Analysis {
	var signals []Signal

	// 1. Keyword detection
	signals = append(signals, a.detectKeywords(message)...)

	// 2. CAPS detection (shouting)
	signals = append(signals, a.detectCaps(message)...)

	// 3. Excessive punctuation
	signals = append(signals, a.detectPunctuation(message)...)

	// 4. Repetition patterns
	signals = append(signals, a.detectRepetition(message)...)

	// 5. Urgency indicators
	signals = append(signals, a.detectUrgency(message)...)

	// 6. Negative sentiment patterns
	signals = append(signals, a.detectNegativity(message)...)

	// Calculate score
	score := calculateScore(signals)

	// Determine level
	level := scoreToLevel(score)

	// Calculate confidence based on signal count
	confidence := math.Min(float64(len(signals))/3.0, 1.0)
	if len(signals) == 0 {
		confidence = 0.5 // neutral confidence for calm messages
	}

	// Generate strategy
	strategy := generateStrategy(level, score, signals)

	// Store in history
	a.history = append(a.history, messageEntry{
		Text:      message,
		Timestamp: time.Now(),
		Score:     score,
	})
	if len(a.history) > a.maxHistory {
		a.history = a.history[len(a.history)-a.maxHistory:]
	}

	return Analysis{
		Level:      level,
		Score:      score,
		Signals:    signals,
		Confidence: confidence,
		Strategy:   strategy,
	}
}

// Trend returns the frustration trend over recent messages.
func (a *Analyzer) Trend() string {
	if len(a.history) < 3 {
		return "stable"
	}

	recent := a.history
	if len(a.history) > 5 {
		recent = a.history[len(a.history)-5:]
	}

	// Simple trend: compare first half avg to second half avg
	mid := len(recent) / 2
	firstHalf := avg(recent[:mid])
	secondHalf := avg(recent[mid:])

	diff := secondHalf - firstHalf
	if diff > 15 {
		return "escalating"
	}
	if diff < -15 {
		return "deescalating"
	}
	return "stable"
}

// History returns recent frustration scores.
func (a *Analyzer) History() []float64 {
	scores := make([]float64, len(a.history))
	for i, h := range a.history {
		scores[i] = h.Score
	}
	return scores
}

func (a *Analyzer) detectKeywords(msg string) []Signal {
	var signals []Signal
	lower := strings.ToLower(msg)

	keywords := map[string]float64{
		"frustrated":    0.8,
		"frustrating":   0.8,
		"annoyying":     0.7,
		"annoyed":       0.7,
		"angry":         0.9,
		"hate":          0.7,
		"stupid":        0.8,
		"useless":       0.8,
		"terrible":      0.8,
		"horrible":      0.8,
		"worst":         0.7,
		"garbage":       0.8,
		"broken":        0.6,
		"doesn't work":  0.6,
		"doesn't help":  0.7,
		"not working":   0.6,
		"again":         0.5,
		"still":         0.4,
		"ugh":           0.6,
		"argh":          0.7,
		"wtf":           0.8,
		"give up":       0.9,
		"waste of time": 0.8,
		"no help":       0.7,
		"this sucks":    0.8,
		"unacceptable":  0.8,
	}

	for kw, weight := range keywords {
		if strings.Contains(lower, kw) {
			signals = append(signals, Signal{
				Type:    "keyword",
				Match:   kw,
				Weight:  weight,
				Message: "Frustration keyword detected: " + kw,
			})
		}
	}

	return signals
}

func (a *Analyzer) detectCaps(msg string) []Signal {
	var signals []Signal

	words := strings.Fields(msg)
	if len(words) == 0 {
		return nil
	}

	capsCount := 0
	for _, w := range words {
		if len(w) > 2 && isAllUpper(w) && !isNormalWord(w) {
			capsCount++
		}
	}

	ratio := float64(capsCount) / float64(len(words))
	if ratio > 0.5 && capsCount > 0 {
		signals = append(signals, Signal{
			Type:    "caps",
			Match:   "SHOUTING",
			Weight:  math.Min(ratio, 1.0),
			Message: "Excessive capitalization detected (shouting)",
		})
	}

	return signals
}

func (a *Analyzer) detectPunctuation(msg string) []Signal {
	var signals []Signal

	// Multiple exclamation marks
	re := regexp.MustCompile(`!{3,}`)
	if matches := re.FindAllString(msg, -1); len(matches) > 0 {
		signals = append(signals, Signal{
			Type:    "punctuation",
			Match:   matches[0],
			Weight:  math.Min(float64(len(matches[0]))/5.0, 0.8),
			Message: "Multiple exclamation marks",
		})
	}

	// Multiple question marks
	re2 := regexp.MustCompile(`\?{3,}`)
	if matches := re2.FindAllString(msg, -1); len(matches) > 0 {
		signals = append(signals, Signal{
			Type:    "punctuation",
			Match:   matches[0],
			Weight:  0.5,
			Message: "Multiple question marks (confusion/urgency)",
		})
	}

	return signals
}

func (a *Analyzer) detectRepetition(msg string) []Signal {
	var signals []Signal
	lower := strings.ToLower(msg)
	words := strings.Fields(lower)

	if len(words) < 3 {
		return nil
	}

	// Check for repeated phrases
	wordCount := make(map[string]int)
	for _, w := range words {
		wordCount[w]++
	}

	maxRepeat := 0
	repeatWord := ""
	for w, c := range wordCount {
		if c > maxRepeat && len(w) > 3 {
			maxRepeat = c
			repeatWord = w
		}
	}

	if maxRepeat >= 3 {
		signals = append(signals, Signal{
			Type:    "repetition",
			Match:   repeatWord,
			Weight:  math.Min(float64(maxRepeat)/5.0, 0.7),
			Message: "Word repetition detected (emphasis/frustration)",
		})
	}

	return signals
}

func (a *Analyzer) detectUrgency(msg string) []Signal {
	var signals []Signal
	lower := strings.ToLower(msg)

	urgencyPatterns := map[string]float64{
		"asap":       0.7,
		"urgent":     0.8,
		"emergency":  0.9,
		"now":        0.6,
		"immediately": 0.8,
		"right now":  0.7,
		"hurry":      0.7,
		"quickly":    0.5,
		"help me":    0.6,
		"help":       0.3,
	}

	for pattern, weight := range urgencyPatterns {
		if strings.Contains(lower, pattern) {
			signals = append(signals, Signal{
				Type:    "urgency",
				Match:   pattern,
				Weight:  weight,
				Message: "Urgency indicator: " + pattern,
			})
		}
	}

	return signals
}

func (a *Analyzer) detectNegativity(msg string) []Signal {
	var signals []Signal
	lower := strings.ToLower(msg)

	negativePatterns := []struct {
		regex  string
		weight float64
		label  string
	}{
		{`(never|not|no)\s+(works?|helps?|good)`, 0.7, "negative assessment"},
		{`i\s+(can't|cannot|don't)\s+(get|make|figure|understand)`, 0.7, "negative capability"},
		{`(why|how)\s+(does\s+)?(it\s+)?(keep|always|still)`, 0.6, "complaint pattern"},
		{`(tried|attempted)\s+\d+\s+times`, 0.7, "repeated failure"},
	}

	for _, p := range negativePatterns {
		re := regexp.MustCompile(p.regex)
		if re.MatchString(lower) {
			signals = append(signals, Signal{
				Type:    "pattern",
				Match:   p.label,
				Weight:  p.weight,
				Message: "Negative pattern: " + p.label,
			})
		}
	}

	return signals
}

func calculateScore(signals []Signal) float64 {
	if len(signals) == 0 {
		return 0
	}

	var totalWeight float64
	for _, s := range signals {
		totalWeight += s.Weight
	}

	// Normalize: more signals = higher score, but with diminishing returns
	raw := totalWeight / (1 + totalWeight*0.2)
	score := raw * 50 // scale to 0-100ish

	return math.Min(math.Round(score*100)/100, 100)
}

func scoreToLevel(score float64) FrustrationLevel {
	switch {
	case score >= 75:
		return LevelAngry
	case score >= 50:
		return LevelFrustrated
	case score >= 25:
		return LevelAnnoyed
	case score > 5:
		return LevelNeutral
	default:
		return LevelCalm
	}
}

func generateStrategy(level FrustrationLevel, score float64, signals []Signal) Strategy {
	switch level {
	case LevelAngry:
		return Strategy{
			Tone:        "empathetic",
			Avoid:       []string{"defensiveness", "minimizing", "technical jargon", "long explanations"},
			Suggestions: []string{"Acknowledge the frustration directly", "Offer a concrete solution or escalation path", "Keep response short and action-oriented"},
			Escalate:    true,
			SlowDown:    true,
			Acknowledge: true,
			MaxWords:    50,
		}
	case LevelFrustrated:
		return Strategy{
			Tone:        "supportive",
			Avoid:       []string{"long explanations", "blame", "generic advice"},
			Suggestions: []string{"Acknowledge the difficulty", "Provide a direct solution", "Offer to try a different approach"},
			Escalate:    score > 60,
			SlowDown:    true,
			Acknowledge: true,
			MaxWords:    100,
		}
	case LevelAnnoyed:
		return Strategy{
			Tone:        "calm",
			Avoid:       []string{"repeating previous answers", "unnecessary detail"},
			Suggestions: []string{"Be direct and concise", "Focus on the solution", "Skip the explanation if not asked"},
			Acknowledge: score > 30,
			MaxWords:    150,
		}
	default:
		return Strategy{
			Tone:        "direct",
			Avoid:       []string{},
			Suggestions: []string{"Normal response mode"},
			MaxWords:    0,
		}
	}
}

func isAllUpper(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsUpper(r) {
			return false
		}
	}
	return len(s) > 0
}

func isNormalWord(s string) bool {
	// Words that are naturally all-caps
	normals := map[string]bool{"I": true, "A": true, "OK": true, "YES": true, "NO": true, "API": true, "URL": true, "HTTP": true, "JSON": true, "CLI": true, "AI": true, "FAQ": true, "TODO": true}
	return normals[s]
}

func avg(entries []messageEntry) float64 {
	if len(entries) == 0 {
		return 0
	}
	var sum float64
	for _, e := range entries {
		sum += e.Score
	}
	return sum / float64(len(entries))
}

// FormatAnalysis formats an analysis for display.
func FormatAnalysis(a Analysis) string {
	s := fmt.Sprintf("Level:       %s\n", a.Level)
	s += fmt.Sprintf("Score:       %.1f/100\n", a.Score)
	s += fmt.Sprintf("Confidence:  %.0f%%\n", a.Confidence*100)
	s += fmt.Sprintf("Strategy:    %s\n", a.Strategy.Tone)

	if len(a.Signals) > 0 {
		s += fmt.Sprintf("\nSignals (%d):\n", len(a.Signals))
		for _, sig := range a.Signals {
			s += fmt.Sprintf("  [%s] %s (weight: %.1f)\n", sig.Type, sig.Message, sig.Weight)
		}
	}

	if a.Strategy.Acknowledge {
		s += "\n→ Acknowledge frustration before responding\n"
	}
	if a.Strategy.Escalate {
		s += "→ Consider escalating to human support\n"
	}

	return s
}
