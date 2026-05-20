package slog_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	forgeslog "github.com/forge/sword/internal/slog"
)

func TestDefaultLogger(t *testing.T) {
	if forgeslog.Default == nil {
		t.Fatal("Default logger should not be nil")
	}
}

func TestNewTextLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := forgeslog.New(
		forgeslog.WithWriter(&buf),
		forgeslog.WithLevel(slog.LevelDebug),
	)

	logger.Info("test message", "key", "value")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log to contain 'test message', got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "key=value") {
		t.Errorf("expected log to contain 'key=value', got: %s", buf.String())
	}
}

func TestNewJSONLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := forgeslog.New(
		forgeslog.WithWriter(&buf),
		forgeslog.WithJSON(),
		forgeslog.WithLevel(slog.LevelInfo),
	)

	logger.Info("json test", "status", "ok")
	output := buf.String()
	if !strings.Contains(output, `"msg":"json test"`) {
		t.Errorf("expected JSON log with msg, got: %s", output)
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	logger := forgeslog.New(
		forgeslog.WithWriter(&buf),
		forgeslog.WithLevel(slog.LevelInfo),
	)

	contextLogger := logger.With("component", "forge")
	contextLogger.Info("contextual")

	output := buf.String()
	if !strings.Contains(output, "component=forge") {
		t.Errorf("expected contextual attribute, got: %s", output)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := forgeslog.New(
		forgeslog.WithWriter(&buf),
		forgeslog.WithLevel(slog.LevelWarn),
	)

	logger.Debug("should not appear")
	logger.Info("should not appear either")
	logger.Warn("should appear")
	logger.Error("should also appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Errorf("debug/info should be filtered out, got: %s", output)
	}
	if !strings.Contains(output, "should appear") {
		t.Errorf("warn should be present, got: %s", output)
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	// Just verify they don't panic
	forgeslog.Debug("debug")
	forgeslog.Info("info")
	forgeslog.Warn("warn")
	forgeslog.Error("error")
}
