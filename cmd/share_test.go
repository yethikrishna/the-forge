package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestShareCmdHTML(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := shareCmd()
	cmd.SetArgs([]string{"--format", "html", "--title", "Test Session"})
	_ = cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Test Session") {
		t.Error("expected title in HTML output")
	}
	if !strings.Contains(output, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
}

func TestShareCmdMarkdown(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := shareCmd()
	cmd.SetArgs([]string{"--format", "markdown", "--title", "MD Test"})
	_ = cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "# MD Test") {
		t.Error("expected title in markdown output")
	}
}

func TestShareCmdOutputFile(t *testing.T) {
	tmpFile := "/tmp/forge-share-test.html"
	defer os.Remove(tmpFile)

	cmd := shareCmd()
	cmd.SetArgs([]string{"--format", "html", "--output", tmpFile})
	_ = cmd.Execute()

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "<!DOCTYPE html>") {
		t.Error("expected HTML in output file")
	}
}

func TestShareCmdInvalidFormat(t *testing.T) {
	cmd := shareCmd()
	cmd.SetArgs([]string{"--format", "pdf"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestDemoSession(t *testing.T) {
	s := demoSession()
	if s.Title != "Forge Demo Session" {
		t.Error("demo session should have a title")
	}
	if len(s.Entries) == 0 {
		t.Error("demo session should have entries")
	}
}
