package pair_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/forge/sword/internal/pair"
)

func TestNewPair(t *testing.T) {
	p := pair.NewPair("test", pair.ModeNavigate, nil)
	sess := p.Session()

	if sess.Name != "test" {
		t.Errorf("expected 'test', got %s", sess.Name)
	}
	if sess.Mode != pair.ModeNavigate {
		t.Errorf("expected navigate, got %s", sess.Mode)
	}
	if sess.Status != "active" {
		t.Errorf("expected active, got %s", sess.Status)
	}
}

func TestAddTurns(t *testing.T) {
	p := pair.NewPair("test", pair.ModeDrive, nil)

	p.AddHumanTurn("Write a hello world function")
	p.AddAgentTurn("Here's the function:\n```go\nfunc hello() string { return \"hello\" }\n```")

	history := p.History()
	if len(history) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(history))
	}

	if history[0].Role != pair.RoleHuman {
		t.Errorf("expected human, got %s", history[0].Role)
	}
	if history[1].Role != pair.RoleAgent {
		t.Errorf("expected agent, got %s", history[1].Role)
	}
}

func TestLastTurn(t *testing.T) {
	p := pair.NewPair("test", pair.ModeDrive, nil)

	if last := p.LastTurn(); last != nil {
		t.Error("should be nil for empty session")
	}

	p.AddHumanTurn("hello")
	if last := p.LastTurn(); last == nil || last.Content != "hello" {
		t.Error("should return last turn")
	}
}

func TestContext(t *testing.T) {
	p := pair.NewPair("test", pair.ModeDrive, nil)

	p.AddHumanTurn("hello")
	p.AddAgentTurn("world")

	ctx := p.Context()
	if !strings.Contains(ctx, "hello") || !strings.Contains(ctx, "world") {
		t.Error("context should contain both turns")
	}
}

func TestExport(t *testing.T) {
	p := pair.NewPair("test", pair.ModeDrive, nil)
	p.AddHumanTurn("hello")

	data, err := p.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Error("export should contain turn content")
	}
}

func TestSetMode(t *testing.T) {
	var buf bytes.Buffer
	p := pair.NewPair("test", pair.ModeDrive, nil)
	p.WithIO(strings.NewReader(""), &buf)

	p.SetMode(pair.ModeObserve)
	sess := p.Session()
	if sess.Mode != pair.ModeObserve {
		t.Errorf("expected observe, got %s", sess.Mode)
	}
}

func TestInteractiveQuit(t *testing.T) {
	var buf bytes.Buffer
	input := strings.NewReader("/quit\n")

	p := pair.NewPair("test", pair.ModeDrive, func(_ context.Context, _ []pair.Turn) (string, error) {
		return "response", nil
	})
	p.WithIO(input, &buf)

	err := p.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	sess := p.Session()
	if sess.Status != "completed" {
		t.Errorf("expected completed, got %s", sess.Status)
	}
}

func TestInteractiveHelp(t *testing.T) {
	var buf bytes.Buffer
	input := strings.NewReader("/help\n/quit\n")

	p := pair.NewPair("test", pair.ModeDrive, nil)
	p.WithIO(input, &buf)

	p.Start(context.Background())

	if !strings.Contains(buf.String(), "Commands:") {
		t.Error("should show help")
	}
}

func TestInteractiveAgent(t *testing.T) {
	var buf bytes.Buffer
	input := strings.NewReader("write a function\n/quit\n")

	callCount := 0
	p := pair.NewPair("test", pair.ModeDrive, func(_ context.Context, _ []pair.Turn) (string, error) {
		callCount++
		return "here's the function", nil
	})
	p.WithIO(input, &buf)

	p.Start(context.Background())

	if callCount != 1 {
		t.Errorf("expected 1 agent call, got %d", callCount)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pair.json"

	p := pair.NewPair("persist-test", pair.ModeDrive, nil)
	p.WithStore(path)
	p.AddHumanTurn("hello")

	// Verify file was written
	data, err := readPairFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(data, "hello") {
		t.Error("should persist turns")
	}
}

func TestModes(t *testing.T) {
	modes := []string{pair.ModeDrive, pair.ModeNavigate, pair.ModeObserve}
	for _, mode := range modes {
		p := pair.NewPair("test", mode, nil)
		if p.Session().Mode != mode {
			t.Errorf("expected %s, got %s", mode, p.Session().Mode)
		}
	}
}

func readPairFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
