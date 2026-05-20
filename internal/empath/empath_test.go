package empath

import (
	"testing"
)

func TestAnalyzeNeutral(t *testing.T) {
	d := NewDetector(t.TempDir())
	state := d.Analyze("Can you help me write a function to sort an array?")

	if state.Level != FrustrationNone {
		t.Errorf("expected none, got %s", state.Level)
	}
	if state.MessageCount != 1 {
		t.Errorf("expected 1 message, got %d", state.MessageCount)
	}
}

func TestAnalyzeCaps(t *testing.T) {
	d := NewDetector(t.TempDir())
	state := d.Analyze("WHY IS THIS NOT WORKING???")

	if state.Score == 0 {
		t.Error("expected non-zero score for caps")
	}
}

func TestAnalyzeImpatience(t *testing.T) {
	d := NewDetector(t.TempDir())
	state := d.Analyze("This is so frustrating, why isn't it working?")

	if state.Score == 0 {
		t.Error("expected non-zero score for impatience")
	}
}

func TestAnalyzeShortResponse(t *testing.T) {
	d := NewDetector(t.TempDir())
	d.Analyze("Hello, can you help me?")
	d.Analyze("I need a sorting function")
	d.Analyze("no")

	state := d.State()
	if state.ShortResponseCount == 0 {
		t.Error("expected short response count > 0")
	}
}

func TestAnalyzeRepeat(t *testing.T) {
	d := NewDetector(t.TempDir())
	d.Analyze("Sort the array in descending order")
	state := d.Analyze("Sort the array in descending order")

	if state.RepeatCount == 0 {
		t.Error("expected repeat count > 0")
	}
}

func TestRecordError(t *testing.T) {
	d := NewDetector(t.TempDir())
	d.RecordError("connection timeout")

	state := d.State()
	if state.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", state.ErrorCount)
	}
}

func TestEscalatingFrustration(t *testing.T) {
	d := NewDetector(t.TempDir())

	d.Analyze("Can you help me with this?")
	if d.State().Level != FrustrationNone {
		t.Error("expected none at start")
	}

	for i := 0; i < 5; i++ {
		d.Analyze("THIS IS SO FRUSTRATING AND BROKEN AND TERRIBLE")
		d.RecordError("error")
	}

	state := d.State()
	if state.Level == FrustrationNone {
		t.Errorf("expected escalated frustration, got %s (score: %.1f)", state.Level, state.Score)
	}
}

func TestAdaptiveConfig(t *testing.T) {
	d := NewDetector(t.TempDir())

	cfg := d.GetAdaptiveConfig()
	if cfg.ResponseStyle != "normal" {
		t.Errorf("expected normal, got %s", cfg.ResponseStyle)
	}

	// Trigger significant frustration
	for i := 0; i < 5; i++ {
		d.Analyze("THIS IS TERRIBLE AND BROKEN AND FRUSTRATING")
		d.RecordError("error")
	}

	cfg = d.GetAdaptiveConfig()
	if cfg.ResponseStyle == "normal" {
		t.Errorf("expected adapted style, got %s (score: %.1f)", cfg.ResponseStyle, d.State().Score)
	}
}

func TestCriticalFrustration(t *testing.T) {
	d := NewDetector(t.TempDir())

	// Heavy frustration signals — need many to reach critical
	for i := 0; i < 30; i++ {
		d.Analyze("THIS IS RIDICULOUS AND TERRIBLE AND FRUSTRATING AND BROKEN AND USELESS")
		d.RecordError("error")
	}

	state := d.State()
	if state.Level != FrustrationCritical {
		t.Errorf("expected critical, got %s (score: %.1f)", state.Level, state.Score)
	}

	cfg := d.GetAdaptiveConfig()
	if cfg.ResponseStyle != "handoff" {
		t.Errorf("expected handoff, got %s", cfg.ResponseStyle)
	}
}

func TestReset(t *testing.T) {
	d := NewDetector(t.TempDir())
	d.Analyze("THIS IS SO FRUSTRATING!!!")
	d.RecordError("error")

	d.Reset()
	state := d.State()

	if state.Level != FrustrationNone {
		t.Errorf("expected none after reset, got %s", state.Level)
	}
	if state.Score != 0 {
		t.Errorf("expected score 0 after reset, got %.1f", state.Score)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	d := NewDetector(dir)
	d.Analyze("This is frustrating")
	d.RecordError("error")

	if err := d.Save(); err != nil {
		t.Fatal(err)
	}

	d2 := NewDetector(dir)
	if err := d2.Load(); err != nil {
		t.Fatal(err)
	}

	state := d2.State()
	if state.MessageCount != 1 {
		t.Errorf("expected 1 message after load, got %d", state.MessageCount)
	}
}

func TestFormatState(t *testing.T) {
	state := State{Level: FrustrationLow, Score: 15.3, MessageCount: 5, ErrorCount: 1}
	output := FormatState(state)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFormatConfig(t *testing.T) {
	cfg := AdaptiveConfig{ResponseStyle: "supportive", MaxRetries: 2, ShowProgress: true, OfferAlternatives: true}
	output := FormatConfig(cfg)
	if output == "" {
		t.Error("expected non-empty output")
	}
}

func TestSimilarity(t *testing.T) {
	if similarity("sort the array", "sort the array") != 1.0 {
		t.Error("expected 1.0 for identical")
	}
	if similarity("hello world", "completely different") > 0.3 {
		t.Error("expected low similarity")
	}
}
