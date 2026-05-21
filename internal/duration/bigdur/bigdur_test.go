package bigdur_test

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/duration/bigdur"
)

func TestParseStandardDurations(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1h", time.Hour},
		{"30m", 30 * time.Minute},
		{"1h30m", 90 * time.Minute},
		{"500ms", 500 * time.Millisecond},
	}
	for _, tt := range tests {
		d, err := bigdur.Parse(tt.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.input, err)
			continue
		}
		if d.Duration() != tt.want {
			t.Errorf("Parse(%q) = %v, want %v", tt.input, d.Duration(), tt.want)
		}
	}
}

func TestParseExtendedDurations(t *testing.T) {
	tests := []struct {
		input    string
		minimum  time.Duration
	}{
		{"1d", 24 * time.Hour},
		{"2d", 48 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"1mo", 30 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"6mo", 6 * 30 * 24 * time.Hour},
	}
	for _, tt := range tests {
		d, err := bigdur.Parse(tt.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.input, err)
			continue
		}
		if d.Duration() < tt.minimum {
			t.Errorf("Parse(%q) = %v, want >= %v", tt.input, d.Duration(), tt.minimum)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := bigdur.Parse("")
	if err == nil {
		t.Error("expected error for empty string")
	}
	_, err = bigdur.Parse("abc")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestMustParse(t *testing.T) {
	d := bigdur.MustParse("1d")
	if d.Duration() < 24*time.Hour {
		t.Errorf("MustParse(1d) = %v, want >= 24h", d.Duration())
	}
}

func TestHumanString(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"1h"}, {"1d"}, {"1w"}, {"1mo"}, {"1y"},
	}
	for _, tt := range tests {
		d, err := bigdur.Parse(tt.input)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.input, err)
			continue
		}
		s := d.HumanString()
		if s == "" {
			t.Errorf("HumanString() for %q should not be empty", tt.input)
		}
	}
}
