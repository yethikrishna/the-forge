// Package bigdur parses human-friendly duration strings that Go's
// time.ParseDuration can't handle: "1d", "2w", "3mo", "1y".
// Big durations for big forges.
package bigdur

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Duration represents a potentially large duration.
type Duration struct {
	value time.Duration
	text  string
}

// Parse parses a human-friendly duration string.
// Supports all time.ParseDuration formats plus:
//   - "d" or "day"/"days" → 24 hours
//   - "w" or "week"/"weeks" → 7 days
//   - "mo" or "month"/"months" → 30 days
//   - "y" or "year"/"years" → 365 days
//
// Examples: "1d", "2w3d", "1h30m", "1mo", "6mo", "1y"
func Parse(s string) (Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Duration{}, fmt.Errorf("bigdur: empty string")
	}

	// First try standard Go duration
	if d, err := time.ParseDuration(s); err == nil {
		return Duration{value: d, text: s}, nil
	}

	// Parse extended format: number+unit pairs
	total := time.Duration(0)
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*([a-zA-Z]+)`)
	matches := re.FindAllStringSubmatch(s, -1)

	if len(matches) == 0 {
		return Duration{}, fmt.Errorf("bigdur: invalid duration %q", s)
	}

	for _, m := range matches {
		amount, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return Duration{}, fmt.Errorf("bigdur: invalid number %q in %q", m[1], s)
		}

		unit := strings.ToLower(m[2])
		d, err := unitToDuration(amount, unit)
		if err != nil {
			return Duration{}, fmt.Errorf("bigdur: %w", err)
		}

		total += d
	}

	return Duration{value: total, text: s}, nil
}

// MustParse parses a duration or panics.
func MustParse(s string) Duration {
	d, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return d
}

func unitToDuration(amount float64, unit string) (time.Duration, error) {
	hours := amount

	switch unit {
	case "ns", "nano", "nanos", "nanosecond", "nanoseconds":
		return time.Duration(amount) * time.Nanosecond, nil
	case "us", "micro", "micros", "microsecond", "microseconds", "µs":
		return time.Duration(amount) * time.Microsecond, nil
	case "ms", "milli", "millis", "millisecond", "milliseconds":
		return time.Duration(amount) * time.Millisecond, nil
	case "s", "sec", "secs", "second", "seconds":
		return time.Duration(amount) * time.Second, nil
	case "m", "min", "mins", "minute", "minutes":
		return time.Duration(amount) * time.Minute, nil
	case "h", "hr", "hrs", "hour", "hours":
		return time.Duration(amount) * time.Hour, nil
	case "d", "day", "days":
		hours = amount * 24
	case "w", "wk", "week", "weeks":
		hours = amount * 24 * 7
	case "mo", "month", "months":
		hours = amount * 24 * 30
	case "y", "yr", "year", "years":
		hours = amount * 24 * 365
	default:
		return 0, fmt.Errorf("unknown unit %q", unit)
	}

	return time.Duration(hours * float64(time.Hour)), nil
}

// Duration returns the Go time.Duration value.
func (d Duration) Duration() time.Duration {
	return d.value
}

// String returns the original text representation.
func (d Duration) String() string {
	return d.text
}

// HumanString returns a human-friendly representation.
func (d Duration) HumanString() string {
	v := d.value
	switch {
	case v < time.Minute:
		return fmt.Sprintf("%s", v.Round(time.Second))
	case v < time.Hour:
		m := int(v.Minutes())
		s := int(v.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	case v < 24*time.Hour:
		h := int(v.Hours())
		m := int(v.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", h, m)
	case v < 7*24*time.Hour:
		days := int(v.Hours()) / 24
		h := int(v.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, h)
	case v < 30*24*time.Hour:
		weeks := int(v.Hours()) / (24 * 7)
		days := (int(v.Hours()) / 24) % 7
		return fmt.Sprintf("%dw%dd", weeks, days)
	case v < 365*24*time.Hour:
		months := int(v.Hours()) / (24 * 30)
		return fmt.Sprintf("%dmo", months)
	default:
		years := int(v.Hours()) / (24 * 365)
		return fmt.Sprintf("%dy", years)
	}
}
