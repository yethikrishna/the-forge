package flog_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/forge/sword/internal/flog"
)

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := flog.NewWithWriter(&buf)
	logger.SetLevel(flog.LevelDebug)

	logger.Debugf("debug %s", "msg")
	logger.Infof("info %s", "msg")
	logger.Warnf("warn %s", "msg")
	logger.Errorf("error %s", "msg")

	output := buf.String()
	if !strings.Contains(output, "debug msg") {
		t.Errorf("expected debug message in output: %s", output)
	}
	if !strings.Contains(output, "info msg") {
		t.Errorf("expected info message in output: %s", output)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := flog.NewWithWriter(&buf)
	logger.SetLevel(flog.LevelWarn)

	logger.Debugf("should not appear")
	logger.Infof("should not appear either")
	logger.Warnf("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Errorf("debug/info should be filtered: %s", output)
	}
	if !strings.Contains(output, "should appear") {
		t.Errorf("warn should be present: %s", output)
	}
}

func TestPrefix(t *testing.T) {
	var buf bytes.Buffer
	logger := flog.NewWithWriter(&buf)
	logger.SetPrefix("forge: ")

	logger.Infof("test")
	output := buf.String()
	if !strings.Contains(output, "forge: test") {
		t.Errorf("expected prefix in output: %s", output)
	}
}

func TestSilentLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := flog.NewWithWriter(&buf)
	logger.SetLevel(flog.LevelSilent)

	logger.Errorf("should not appear")
	if buf.Len() > 0 {
		t.Errorf("silent level should suppress all output: %s", buf.String())
	}
}
