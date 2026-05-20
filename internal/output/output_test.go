package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    Format
		wantErr bool
	}{
		{"json", FormatJSON, false},
		{"j", FormatJSON, false},
		{"JSON", FormatJSON, false},
		{"quiet", FormatQuiet, false},
		{"q", FormatQuiet, false},
		{"silent", FormatQuiet, false},
		{"verbose", FormatVerbose, false},
		{"v", FormatVerbose, false},
		{"default", FormatDefault, false},
		{"human", FormatDefault, false},
		{"", FormatDefault, false},
		{"bogus", FormatDefault, true},
	}

	for _, tt := range tests {
		got, err := ParseFormat(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("ParseFormat(%q) expected error, got none", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ParseFormat(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestOutputManager_JSON(t *testing.T) {
	var buf bytes.Buffer
	o := NewWithWriters(FormatJSON, &buf, &buf)

	o.Success("it worked")

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if result["status"] != "success" {
		t.Errorf("expected status=success, got %v", result["status"])
	}
}

func TestOutputManager_Quiet(t *testing.T) {
	var buf bytes.Buffer
	o := NewWithWriters(FormatQuiet, &buf, &buf)

	o.Println("should not appear")
	o.Success("should not appear")
	o.Header("should not appear")
	o.Progress(1, 10, "loading")

	if buf.Len() > 0 {
		t.Errorf("quiet mode should produce no stdout/stderr output, got: %q", buf.String())
	}
}

func TestOutputManager_Verbose(t *testing.T) {
	var buf bytes.Buffer
	o := NewWithWriters(FormatVerbose, &buf, &buf)

	o.Verbose("detail message")

	if !strings.Contains(buf.String(), "detail message") {
		t.Errorf("verbose mode should contain message, got: %q", buf.String())
	}
}

func TestOutputManager_DefaultVerbose(t *testing.T) {
	var buf bytes.Buffer
	o := NewWithWriters(FormatDefault, &buf, &buf)

	o.Verbose("should not appear in default mode")

	if strings.Contains(buf.String(), "should not appear") {
		t.Errorf("default mode should not show verbose messages, got: %q", buf.String())
	}
}

func TestOutputManager_Error(t *testing.T) {
	var stdout, stderr bytes.Buffer
	o := NewWithWriters(FormatDefault, &stdout, &stderr)

	o.Error("something failed")

	if !strings.Contains(stderr.String(), "something failed") {
		t.Errorf("error should go to stderr, got: %q", stderr.String())
	}
}

func TestOutputManager_ErrorJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	o := NewWithWriters(FormatJSON, &stdout, &stderr)

	o.Error("something failed")

	var result map[string]interface{}
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON error: %v", err)
	}
	if result["error"] != "something failed" {
		t.Errorf("expected error in JSON, got: %v", result)
	}
}

func TestOutputManager_Table(t *testing.T) {
	var buf bytes.Buffer
	o := NewWithWriters(FormatDefault, &buf, &buf)

	o.Table(
		[]string{"Name", "Status"},
		[][]string{
			{"agent-1", "running"},
			{"agent-2", "stopped"},
		},
	)

	output := buf.String()
	if !strings.Contains(output, "Name") || !strings.Contains(output, "agent-1") {
		t.Errorf("table output missing expected content: %q", output)
	}
}

func TestOutputManager_TableJSON(t *testing.T) {
	var buf bytes.Buffer
	o := NewWithWriters(FormatJSON, &buf, &buf)

	o.Table(
		[]string{"Name", "Status"},
		[][]string{
			{"agent-1", "running"},
			{"agent-2", "stopped"},
		},
	)

	var result []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON table: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result))
	}
	if result[0]["Name"] != "agent-1" {
		t.Errorf("expected Name=agent-1, got %s", result[0]["Name"])
	}
}

func TestOutputManager_KeyValue(t *testing.T) {
	var buf bytes.Buffer
	o := NewWithWriters(FormatDefault, &buf, &buf)

	o.KeyValue(map[string]string{
		"Version": "1.1.0",
		"Status":  "healthy",
	})

	output := buf.String()
	if !strings.Contains(output, "1.1.0") || !strings.Contains(output, "healthy") {
		t.Errorf("key-value output missing expected content: %q", output)
	}
}

func TestOutputManager_Result(t *testing.T) {
	var buf bytes.Buffer
	o := NewWithWriters(FormatJSON, &buf, &buf)

	data := map[string]string{"key": "value"}
	o.Result(data, func() string { return "human output" })

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON result: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got: %v", result)
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"\x1b[32mgreen\x1b[0m", "green"},
		{"\x1b[1;31mred bold\x1b[0m text", "red bold text"},
		{"no ansi here", "no ansi here"},
	}

	for _, tt := range tests {
		got := StripANSI(tt.input)
		if got != tt.want {
			t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCommandResult(t *testing.T) {
	r := NewCommandResult(map[string]int{"count": 42})
	if !r.Success {
		t.Error("expected success=true")
	}

	e := NewCommandError("failed")
	if e.Success {
		t.Error("expected success=false")
	}
	if e.Error != "failed" {
		t.Errorf("expected error=failed, got %s", e.Error)
	}
}

func TestOutputManager_Format(t *testing.T) {
	o := New(FormatJSON)
	if !o.IsJSON() {
		t.Error("expected IsJSON=true")
	}
	if o.IsQuiet() {
		t.Error("expected IsQuiet=false")
	}
	if o.IsVerbose() {
		t.Error("expected IsVerbose=false")
	}

	o2 := New(FormatQuiet)
	if !o2.IsQuiet() {
		t.Error("expected IsQuiet=true")
	}

	o3 := New(FormatVerbose)
	if !o3.IsVerbose() {
		t.Error("expected IsVerbose=true")
	}
}
