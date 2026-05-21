package empath

import (
	"strings"
	"testing"
)

func TestAnalyzeCalm(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("What's the weather today?")

	if result.Level != LevelCalm {
		t.Errorf("expected calm, got %s (score: %.1f)", result.Level, result.Score)
	}
	if result.Score > 10 {
		t.Errorf("calm message should have low score, got %.1f", result.Score)
	}
}

func TestAnalyzeFrustrated(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("This is SO FRUSTRATING! I've tried everything and it still doesn't work!!!")

	if result.Level != LevelFrustrated && result.Level != LevelAngry {
		t.Errorf("expected frustrated/angry, got %s (score: %.1f)", result.Level, result.Score)
	}
	if len(result.Signals) == 0 {
		t.Error("expected frustration signals")
	}
}

func TestAnalyzeAngry(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("THIS IS COMPLETE GARBAGE!!! I HATE THIS STUPID THING!!! WTF!!!")

	if result.Level != LevelAngry {
		t.Errorf("expected angry, got %s (score: %.1f)", result.Level, result.Score)
	}
	if !result.Strategy.Escalate {
		t.Error("angry user should trigger escalation")
	}
	if !result.Strategy.Acknowledge {
		t.Error("should acknowledge frustration")
	}
}

func TestAnalyzeKeywordDetection(t *testing.T) {
	a := NewAnalyzer()

	keywords := []string{"frustrated", "annoyed", "angry", "hate this", "terrible", "useless", "broken", "wtf"}
	for _, kw := range keywords {
		result := a.Analyze("I am " + kw)
		found := false
		for _, sig := range result.Signals {
			if sig.Type == "keyword" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected keyword signal for: %s", kw)
		}
	}
}

func TestAnalyzeCaps(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("THIS DOES NOT WORK AT ALL")

	found := false
	for _, sig := range result.Signals {
		if sig.Type == "caps" {
			found = true
		}
	}
	if !found {
		t.Error("expected caps detection")
	}
}

func TestAnalyzePunctuation(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("Why doesn't this work???!!!")

	found := false
	for _, sig := range result.Signals {
		if sig.Type == "punctuation" {
			found = true
		}
	}
	if !found {
		t.Error("expected punctuation detection")
	}
}

func TestAnalyzeUrgency(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("I need this fixed ASAP, it's urgent!")

	found := false
	for _, sig := range result.Signals {
		if sig.Type == "urgency" {
			found = true
		}
	}
	if !found {
		t.Error("expected urgency detection")
	}
}

func TestAnalyzeNegativity(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("Why does it keep failing? I can't figure this out.")

	found := false
	for _, sig := range result.Signals {
		if sig.Type == "pattern" {
			found = true
		}
	}
	if !found {
		t.Error("expected negative pattern detection")
	}
}

func TestAnalyzeRepetition(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("tried tried tried tried tried again")

	found := false
	for _, sig := range result.Signals {
		if sig.Type == "repetition" {
			found = true
		}
	}
	if !found {
		t.Error("expected repetition detection")
	}
}

func TestStrategyAngry(t *testing.T) {
	s := generateStrategy(LevelAngry, 80, nil)
	if s.Tone != "empathetic" {
		t.Errorf("expected empathetic tone, got %s", s.Tone)
	}
	if !s.Escalate {
		t.Error("should escalate")
	}
	if !s.Acknowledge {
		t.Error("should acknowledge")
	}
	if s.MaxWords == 0 {
		t.Error("should have word limit")
	}
}

func TestStrategyFrustrated(t *testing.T) {
	s := generateStrategy(LevelFrustrated, 55, nil)
	if s.Tone != "supportive" {
		t.Errorf("expected supportive tone, got %s", s.Tone)
	}
	if !s.SlowDown {
		t.Error("should slow down")
	}
}

func TestStrategyAnnoyed(t *testing.T) {
	s := generateStrategy(LevelAnnoyed, 30, nil)
	if s.Tone != "calm" {
		t.Errorf("expected calm tone, got %s", s.Tone)
	}
}

func TestStrategyNeutral(t *testing.T) {
	s := generateStrategy(LevelCalm, 0, nil)
	if s.Tone != "direct" {
		t.Errorf("expected direct tone, got %s", s.Tone)
	}
}

func TestTrendEscalating(t *testing.T) {
	a := NewAnalyzer()
	a.Analyze("hello")
	a.Analyze("this is a bit annoying")
	a.Analyze("I'm getting frustrated now")
	a.Analyze("THIS IS TERRIBLE")
	a.Analyze("I HATE THIS SO MUCH!!!")

	trend := a.Trend()
	if trend != "escalating" {
		t.Errorf("expected escalating, got %s", trend)
	}
}

func TestTrendDeescalating(t *testing.T) {
	a := NewAnalyzer()
	a.Analyze("I HATE THIS!!!")
	a.Analyze("ok still frustrated")
	a.Analyze("maybe it's ok")
	a.Analyze("thanks that helped")
	a.Analyze("great, works now")

	trend := a.Trend()
	if trend != "deescalating" {
		t.Errorf("expected deescalating, got %s", trend)
	}
}

func TestTrendStable(t *testing.T) {
	a := NewAnalyzer()
	a.Analyze("hello")
	a.Analyze("how are you")
	a.Analyze("thanks")

	trend := a.Trend()
	if trend != "stable" {
		t.Errorf("expected stable, got %s", trend)
	}
}

func TestHistory(t *testing.T) {
	a := NewAnalyzer()
	a.Analyze("hello")
	a.Analyze("this is fine")
	a.Analyze("WHY ISN'T THIS WORKING!!!")

	history := a.History()
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}
}

func TestHistoryMaxLimit(t *testing.T) {
	a := NewAnalyzer()
	a.maxHistory = 5

	for i := 0; i < 10; i++ {
		a.Analyze("message")
	}

	history := a.History()
	if len(history) > 5 {
		t.Errorf("history should be capped at %d, got %d", 5, len(history))
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score   float64
		level   FrustrationLevel
	}{
		{0, LevelCalm},
		{5, LevelCalm},
		{30, LevelAnnoyed},
		{55, LevelFrustrated},
		{80, LevelAngry},
	}
	for _, tt := range tests {
		got := scoreToLevel(tt.score)
		if got != tt.level {
			t.Errorf("score %.0f: expected %s, got %s", tt.score, tt.level, got)
		}
	}
}

func TestIsAllUpper(t *testing.T) {
	if !isAllUpper("HELLO") {
		t.Error("HELLO should be all upper")
	}
	if isAllUpper("Hello") {
		t.Error("Hello should not be all upper")
	}
}

func TestIsNormalWord(t *testing.T) {
	if !isNormalWord("API") {
		t.Error("API should be normal")
	}
	if isNormalWord("HELLO") {
		t.Error("HELLO should not be normal")
	}
}

func TestFormatAnalysis(t *testing.T) {
	a := NewAnalyzer()
	result := a.Analyze("This is really frustrating!!!")

	s := FormatAnalysis(result)
	if !strings.Contains(s, "Level:") {
		t.Error("should contain level")
	}
	if !strings.Contains(s, "Score:") {
		t.Error("should contain score")
	}
}

func TestCalculateScoreEmpty(t *testing.T) {
	score := calculateScore(nil)
	if score != 0 {
		t.Errorf("empty signals should score 0, got %.1f", score)
	}
}
