// Package serpent enhances Cobra commands with rich terminal output,
// flag grouping, and validation. Sharpen the CLI into a serpent's fang.
package serpent

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// FlagGroup groups related flags together for display.
type FlagGroup struct {
	Name   string
	Flags  []*pflag.Flag
	Hidden bool
}

// CommandEnhancer wraps a Cobra command with additional features.
type CommandEnhancer struct {
	cmd        *cobra.Command
	flagGroups []FlagGroup
	examples   []Example
}

// Enhance wraps a Cobra command for enhancement.
func Enhance(cmd *cobra.Command) *CommandEnhancer {
	return &CommandEnhancer{cmd: cmd}
}

// AddFlagGroup adds a named group of flags.
func (ce *CommandEnhancer) AddFlagGroup(name string, flagNames ...string) *CommandEnhancer {
	var flags []*pflag.Flag
	for _, name := range flagNames {
		if f := ce.cmd.Flags().Lookup(name); f != nil {
			flags = append(flags, f)
		}
	}
	ce.flagGroups = append(ce.flagGroups, FlagGroup{Name: name, Flags: flags})
	return ce
}

// AddExample adds a usage example.
func (ce *CommandEnhancer) AddExample(description, command string) *CommandEnhancer {
	ce.examples = append(ce.examples, Example{Description: description, Command: command})
	return ce
}

// SetCustomHelp sets a custom help function.
func (ce *CommandEnhancer) SetCustomHelp() *CommandEnhancer {
	ce.cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		PrintCustomHelp(os.Stdout, cmd, ce.flagGroups, ce.examples)
	})
	return ce
}

// Example represents a command usage example.
type Example struct {
	Description string
	Command     string
}

// PrintCustomHelp prints a beautifully formatted help message.
func PrintCustomHelp(w io.Writer, cmd *cobra.Command, groups []FlagGroup, examples []Example) {
	fmt.Fprintf(w, "%s\n\n", cmd.Short)

	if cmd.Long != "" && cmd.Long != cmd.Short {
		fmt.Fprintf(w, "%s\n\n", cmd.Long)
	}

	// Usage
	fmt.Fprintf(w, "Usage:\n  %s\n\n", cmd.UseLine())

	// Examples
	if len(examples) > 0 {
		fmt.Fprintf(w, "Examples:\n")
		for _, ex := range examples {
			fmt.Fprintf(w, "  # %s\n  %s\n\n", ex.Description, ex.Command)
		}
	}

	// Subcommands
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(w, "Commands:\n")
		maxLen := 0
		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() && len(sub.Name()) > maxLen {
				maxLen = len(sub.Name())
			}
		}
		for _, sub := range cmd.Commands() {
			if !sub.IsAvailableCommand() {
				continue
			}
			fmt.Fprintf(w, "  %-*s  %s\n", maxLen, sub.Name(), sub.Short)
		}
		fmt.Fprintln(w)
	}

	// Flag groups
	if len(groups) > 0 {
		for _, group := range groups {
			if group.Hidden {
				continue
			}
			fmt.Fprintf(w, "%s:\n", group.Name)
			maxLen := 0
			for _, f := range group.Flags {
				if f.Hidden {
					continue
				}
				l := flagDisplayName(f)
				if len(l) > maxLen {
					maxLen = len(l)
				}
			}
			for _, f := range group.Flags {
				if f.Hidden {
					continue
				}
				fmt.Fprintf(w, "  %-*s  %s\n", maxLen, flagDisplayName(f), f.Usage)
			}
			fmt.Fprintln(w)
		}
	} else {
		// Default flag display
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			fmt.Fprintf(w, "  %-20s  %s\n", flagDisplayName(f), f.Usage)
		})
	}
}

func flagDisplayName(f *pflag.Flag) string {
	parts := []string{}
	if f.Shorthand != "" {
		parts = append(parts, "-"+f.Shorthand)
	}
	parts = append(parts, "--"+f.Name)
	return strings.Join(parts, ", ")
}

// ValidateFunc validates command input.
type ValidateFunc func(cmd *cobra.Command, args []string) error

// ValidatedCommand creates a command with input validation.
func ValidatedCommand(use, short string, runFunc func(cmd *cobra.Command, args []string) error, validators ...ValidateFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, v := range validators {
				if err := v(cmd, args); err != nil {
					return err
				}
			}
			return runFunc(cmd, args)
		},
	}
	return cmd
}

// RequireArgs validates that the required number of arguments is provided.
func RequireArgs(min int) ValidateFunc {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < min {
			return fmt.Errorf("requires at least %d argument(s), received %d", min, len(args))
		}
		return nil
	}
}

// RequireExactArgs validates that exactly n arguments are provided.
func RequireExactArgs(n int) ValidateFunc {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("requires exactly %d argument(s), received %d", n, len(args))
		}
		return nil
	}
}

// RequireFlag validates that a flag is set.
func RequireFlag(name string) ValidateFunc {
	return func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed(name) {
			return fmt.Errorf("required flag --%s not set", name)
		}
		return nil
	}
}

// CommandList returns a sorted list of command names.
func CommandList(cmd *cobra.Command) []string {
	var names []string
	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			names = append(names, sub.Name())
		}
	}
	sort.Strings(names)
	return names
}

// PrintVersion prints version info with optional build details.
func PrintVersion(w io.Writer, name, version, buildTime, gitCommit string) {
	fmt.Fprintf(w, "%s %s\n", name, version)
	if buildTime != "" && buildTime != "unknown" {
		fmt.Fprintf(w, "  built:   %s\n", buildTime)
	}
	if gitCommit != "" && gitCommit != "unknown" {
		fmt.Fprintf(w, "  commit:  %s\n", gitCommit)
	}
}
