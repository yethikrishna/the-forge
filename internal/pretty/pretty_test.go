package pretty_test

import (
	"strings"
	"testing"

	"github.com/forge/sword/internal/pretty"
)

func TestColorFunc(t *testing.T) {
	fn := pretty.Color(pretty.Red, pretty.Bold)
	result := fn("test")
	if !strings.Contains(result, "test") {
		t.Errorf("expected 'test' in output, got: %s", result)
	}
}

func TestSprint(t *testing.T) {
	result := pretty.Sprint(pretty.GreenF, "hello")
	if !strings.Contains(result, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", result)
	}
}

func TestSprintf(t *testing.T) {
	result := pretty.Sprintf(pretty.BoldF, "count: %d", 42)
	if !strings.Contains(result, "count: 42") {
		t.Errorf("expected 'count: 42' in output, got: %s", result)
	}
}

func TestStatusLines(t *testing.T) {
	lines := []struct {
		fn   func(string) string
		text string
	}{
		{pretty.SuccessLine, "done"},
		{pretty.ErrorLine, "failed"},
		{pretty.WarningLine, "caution"},
		{pretty.InfoLine, "note"},
		{pretty.HeaderLine, "title"},
	}
	for _, l := range lines {
		result := l.fn(l.text)
		if !strings.Contains(result, l.text) {
			t.Errorf("expected '%s' in output, got: %s", l.text, result)
		}
	}
}

func TestTable(t *testing.T) {
	result := pretty.Table(
		[]string{"Name", "Status"},
		[][]string{{"forge", "ready"}, {"sword", "sharp"}},
	)
	if !strings.Contains(result, "Name") || !strings.Contains(result, "forge") {
		t.Errorf("table missing expected content: %s", result)
	}
}

func TestBox(t *testing.T) {
	result := pretty.Box("Test", "content line")
	if !strings.Contains(result, "Test") || !strings.Contains(result, "content line") {
		t.Errorf("box missing expected content: %s", result)
	}
	if !strings.Contains(result, "┌") || !strings.Contains(result, "└") {
		t.Errorf("box missing border characters: %s", result)
	}
}

func TestProgressBar(t *testing.T) {
	result := pretty.ProgressBar(50, 100, 20)
	if !strings.Contains(result, "50%") {
		t.Errorf("progress bar missing percentage: %s", result)
	}
}

func TestProgressBarZeroTotal(t *testing.T) {
	result := pretty.ProgressBar(0, 0, 10)
	if !strings.Contains(result, "[") {
		t.Errorf("progress bar should still render: %s", result)
	}
}
