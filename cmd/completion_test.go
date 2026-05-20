package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionCmdRegistered(t *testing.T) {
	cmd := completionCmd()
	if cmd == nil {
		t.Fatal("completionCmd() returned nil")
	}
	if cmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}
}

func TestCompletionValidArgs(t *testing.T) {
	cmd := completionCmd()
	expected := []string{"bash", "zsh", "fish", "powershell"}
	if len(cmd.ValidArgs) != len(expected) {
		t.Fatalf("expected %d valid args, got %d", len(expected), len(cmd.ValidArgs))
	}
	for i, arg := range expected {
		if cmd.ValidArgs[i] != arg {
			t.Errorf("ValidArgs[%d] = %q, want %q", i, cmd.ValidArgs[i], arg)
		}
	}
}

func TestCompletionRequiresArg(t *testing.T) {
	cmd := completionCmd()
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("expected error when no args provided")
	}
}

func TestCompletionRejectsInvalidShell(t *testing.T) {
	cmd := completionCmd()
	err := cmd.Args(cmd, []string{"invalid-shell"})
	if err == nil {
		t.Error("expected error for invalid shell type")
	}
}

func TestCompletionAcceptsValidShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		cmd := completionCmd()
		err := cmd.Args(cmd, []string{shell})
		if err != nil {
			t.Errorf("expected %q to be accepted, got: %v", shell, err)
		}
	}
}

func TestCompletionNoDescFlag(t *testing.T) {
	cmd := completionCmd()
	f := cmd.Flags().Lookup("no-descriptions")
	if f == nil {
		t.Fatal("expected --no-descriptions flag")
	}
}

func TestCompletionBashNoDesc(t *testing.T) {
	// Build a minimal root with completion registered
	root := &cobra.Command{Use: "forge", SilenceUsage: true}
	root.AddCommand(completionCmd())
	
	compCmd, _, _ := root.Find([]string{"completion", "bash"})
	if compCmd == nil {
		t.Fatal("completion command not found")
	}
}
