package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestDoctorCmdRuns(t *testing.T) {
	// Doctor writes to stdout via fmt.Println, so we capture that
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := doctorCmd()
	cmd.SetArgs([]string{})
	_ = cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Summary") {
		t.Errorf("expected summary in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Go toolchain") {
		t.Errorf("expected Go toolchain check in output, got:\n%s", output)
	}
}

func TestDoctorVerbose(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := doctorCmd()
	cmd.SetArgs([]string{"--verbose"})
	_ = cmd.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Summary") {
		t.Errorf("expected summary in verbose output, got:\n%s", output)
	}
}

func TestDoctorDetectsAPIKeys(t *testing.T) {
	origKey := os.Getenv("OPENAI_API_KEY")
	os.Setenv("OPENAI_API_KEY", "sk-test-1234567890abcdef")
	defer os.Setenv("OPENAI_API_KEY", origKey)

	results := checkAPIKeys()
	found := false
	for _, r := range results {
		if strings.Contains(r.message, "OpenAI API key: configured") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected OpenAI key to be detected as configured")
	}
}

func TestDoctorGoVersion(t *testing.T) {
	results := checkGoVersion()
	if len(results) == 0 {
		t.Fatal("expected at least one Go version check result")
	}
	if results[0].status == statusFail {
		t.Errorf("Go should be available, got: %s", results[0].message)
	}
}

func TestDoctorOSArch(t *testing.T) {
	results := checkOSArch()
	if len(results) != 1 {
		t.Fatalf("expected 1 OS/arch result, got %d", len(results))
	}
	if results[0].status != statusPass {
		t.Errorf("OS/arch check should pass, got: %s", results[0].message)
	}
}

func TestDoctorForgeBinary(t *testing.T) {
	results := checkForgeBinary()
	if len(results) != 1 {
		t.Fatalf("expected 1 binary check result, got %d", len(results))
	}
	if results[0].status != statusPass {
		t.Errorf("forge binary check should pass, got: %s", results[0].message)
	}
	if !strings.Contains(results[0].message, "Forge version") {
		t.Errorf("expected version in message, got: %s", results[0].message)
	}
}

func TestDoctorGit(t *testing.T) {
	results := checkGit()
	if len(results) == 0 {
		t.Fatal("expected at least one git check result")
	}
}

func TestDoctorCheckResultTypes(t *testing.T) {
	p := pass("all good")
	if p.status != statusPass {
		t.Error("pass() should have statusPass")
	}
	w := warn("maybe bad", "fix it")
	if w.status != statusWarn {
		t.Error("warn() should have statusWarn")
	}
	f := fail("broken", "fix now")
	if f.status != statusFail {
		t.Error("fail() should have statusFail")
	}
}

func TestDoctorNoAPIKeysWarning(t *testing.T) {
	// Temporarily clear all keys
	keys := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_AI_API_KEY", "XAI_API_KEY", "GROQ_API_KEY"}
	saved := make(map[string]string)
	for _, k := range keys {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	defer func() {
		for k, v := range saved {
			os.Setenv(k, v)
		}
	}()

	results := checkAPIKeys()
	// Should have a "No LLM API keys configured" fail
	foundFail := false
	for _, r := range results {
		if r.status == statusFail && strings.Contains(r.message, "No LLM API keys") {
			foundFail = true
			break
		}
	}
	if !foundFail {
		t.Error("expected 'No LLM API keys configured' failure when no keys set")
	}
}
