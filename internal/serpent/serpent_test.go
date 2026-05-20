package serpent_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/forge/sword/internal/serpent"
	"github.com/spf13/cobra"
)

func TestEnhanceCommand(t *testing.T) {
	cmd := &cobra.Command{Use: "test", Short: "A test command"}
	ce := serpent.Enhance(cmd)
	if ce == nil {
		t.Fatal("enhancer should not be nil")
	}
}

func TestValidatedCommand(t *testing.T) {
	cmd := serpent.ValidatedCommand("test <arg>", "test",
		func(cmd *cobra.Command, args []string) error {
			return nil
		},
		serpent.RequireArgs(1),
	)

	if err := cmd.Execute(); err == nil {
		t.Error("should fail with no args")
	}
}

func TestRequireArgs(t *testing.T) {
	v := serpent.RequireArgs(2)
	cmd := &cobra.Command{}
	if err := v(cmd, []string{"one"}); err == nil {
		t.Error("should fail with 1 arg when 2 required")
	}
	if err := v(cmd, []string{"one", "two"}); err != nil {
		t.Errorf("should pass with 2 args: %v", err)
	}
}

func TestRequireExactArgs(t *testing.T) {
	v := serpent.RequireExactArgs(1)
	cmd := &cobra.Command{}
	if err := v(cmd, []string{}); err == nil {
		t.Error("should fail with 0 args")
	}
	if err := v(cmd, []string{"one"}); err != nil {
		t.Errorf("should pass with 1 arg: %v", err)
	}
	if err := v(cmd, []string{"one", "two"}); err == nil {
		t.Error("should fail with 2 args")
	}
}

func TestCommandList(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.AddCommand(
		&cobra.Command{Use: "beta", Short: "Beta command", Run: func(*cobra.Command, []string){}},
		&cobra.Command{Use: "alpha", Short: "Alpha command", Run: func(*cobra.Command, []string){}},
		&cobra.Command{Use: "gamma", Short: "Gamma command", Run: func(*cobra.Command, []string){}},
	)

	list := serpent.CommandList(root)
	if len(list) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(list))
	}
	if list[0] != "alpha" || list[1] != "beta" || list[2] != "gamma" {
		t.Errorf("expected sorted list, got %v", list)
	}
}

func TestPrintVersion(t *testing.T) {
	var buf bytes.Buffer
	serpent.PrintVersion(&buf, "forge", "3.0.0", "2024-01-01", "abc123")

	output := buf.String()
	if !strings.Contains(output, "forge 3.0.0") {
		t.Errorf("expected version in output: %s", output)
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("expected commit in output: %s", output)
	}
}

func TestPrintCustomHelp(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "forge [command]",
		Short: "The Forge - AI Agent Orchestration",
	}
	cmd.Flags().StringP("model", "m", "default", "Model to use")

	var buf bytes.Buffer
	serpent.PrintCustomHelp(&buf, cmd, nil, []serpent.Example{
		{Description: "Start a server", Command: "forge serve -- claude"},
	})

	output := buf.String()
	if !strings.Contains(output, "Examples:") {
		t.Errorf("expected examples section: %s", output)
	}
}
