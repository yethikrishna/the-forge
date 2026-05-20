package cli_test

import (
	"bytes"
	"testing"

	"github.com/forge/sword/internal/cli"
)

func TestNewSpinner(t *testing.T) {
	s := cli.NewSpinner("loading")
	if s == nil {
		t.Fatal("spinner should not be nil")
	}
}

func TestNewForgeSpinner(t *testing.T) {
	s := cli.NewForgeSpinner("forging")
	if s == nil {
		t.Fatal("spinner should not be nil")
	}
}

func TestSpinnerStartStop(t *testing.T) {
	var buf bytes.Buffer
	s := cli.NewSpinner("test").WithWriter(&buf)
	s.Start()
	s.Stop()
	// Should not panic, output may or may not have content depending on timing
}

func TestStepTracker(t *testing.T) {
	tracker := cli.NewStepTracker([]string{"step1", "step2", "step3"})
	if tracker == nil {
		t.Fatal("tracker should not be nil")
	}
	tracker.Start(0)
	tracker.Done(0)
	tracker.Start(1)
	tracker.Fail(1)
	tracker.Skip(2)
}

func TestStepTrackerOutOfBounds(t *testing.T) {
	tracker := cli.NewStepTracker([]string{"only"})
	// Should not panic
	tracker.Start(-1)
	tracker.Start(5)
	tracker.Done(-1)
	tracker.Done(5)
}
