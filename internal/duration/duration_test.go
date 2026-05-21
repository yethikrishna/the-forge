package duration

import (
	"testing"
	"time"

	"github.com/forge/sword/internal/duration/bigdur"
	"github.com/forge/sword/internal/duration/timer"
)

func TestBigDurIntegration(t *testing.T) {
	tests := []struct {
		input   string
		wantGTE time.Duration // result should be >= this
		wantLT  time.Duration // result should be < this
	}{
		{"1d", 24 * time.Hour, 25 * time.Hour},
		{"2w", 14 * 24 * time.Hour, 15 * 24 * time.Hour},
		{"1mo", 30 * 24 * time.Hour, 31 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour, 366 * 24 * time.Hour},
		{"1h30m", 90 * time.Minute, 91 * time.Minute},
		{"2w3d", 17 * 24 * time.Hour, 18 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := bigdur.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			got := d.Duration()
			if got < tt.wantGTE || got >= tt.wantLT {
				t.Errorf("Parse(%q) = %v, want in [%v, %v)", tt.input, got, tt.wantGTE, tt.wantLT)
			}
		})
	}
}

func TestBigDurInvalid(t *testing.T) {
	_, err := bigdur.Parse("")
	if err == nil {
		t.Error("Parse empty string should error")
	}
	_, err = bigdur.Parse("xyz")
	if err == nil {
		t.Error("Parse garbage should error")
	}
}

func TestBigDurHumanString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1d", "1d0h"},
		{"1w", "1w0d"},
		{"1mo", "1mo"},
		{"1y", "1y"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := bigdur.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			got := d.HumanString()
			if got != tt.want {
				t.Errorf("HumanString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBigDurMustParse(t *testing.T) {
	d := bigdur.MustParse("1d")
	if d.Duration() < 24*time.Hour {
		t.Errorf("MustParse(1d) = %v, want >= 24h", d.Duration())
	}
}

func TestBigDurMustParsePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse with invalid input should panic")
		}
	}()
	bigdur.MustParse("xyz")
}

func TestTimerBasic(t *testing.T) {
	tm := timer.New()
	if !(tm.Elapsed().Seconds() > 0) {
		t.Error("Timer should have positive elapsed time")
	}
	d := tm.Stop()
	if d.Seconds() <= 0 {
		t.Error("Stop() should return positive duration")
	}
}

func TestTimerReset(t *testing.T) {
	tm := timer.New()
	tm.Stop()
	tm.Reset()
	if tm.Elapsed().Seconds() <= 0 {
		t.Error("After Reset(), Elapsed should be positive")
	}
}

func TestTimerTrack(t *testing.T) {
	d := timer.Track(func() {
		time.Sleep(1 * time.Millisecond)
	})
	if d < time.Millisecond {
		t.Error("Track should measure at least 1ms")
	}
}

func TestTimerTrackError(t *testing.T) {
	d, err := timer.TrackError(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("TrackError unexpected error: %v", err)
	}
	if d < 0 {
		t.Error("TrackError duration should be non-negative")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{100 * time.Nanosecond, "100ns"},
		{5 * time.Millisecond, "5.0ms"},
		{1500 * time.Millisecond, "1.50s"},
	}
	for _, tt := range tests {
		got := timer.FormatDuration(tt.d)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestLapTimer(t *testing.T) {
	lt := timer.NewLapTimer()
	time.Sleep(1 * time.Millisecond)
	lap1 := lt.Lap("first")
	if lap1.Duration < time.Millisecond {
		t.Error("Lap should capture elapsed time")
	}
	if lap1.Name != "first" {
		t.Errorf("Lap name = %q, want %q", lap1.Name, "first")
	}

	laps := lt.Laps()
	if len(laps) != 1 {
		t.Errorf("Laps() = %d, want 1", len(laps))
	}

	total := lt.Total()
	if total < time.Millisecond {
		t.Error("Total should be positive")
	}

	s := lt.String()
	if s == "" {
		t.Error("String should not be empty")
	}
}
