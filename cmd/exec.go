package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/forge/sword/internal/duration/timer"
	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/sandbox"
	"github.com/spf13/cobra"
)

func execCmd() *cobra.Command {
	var network bool
	var filesystem bool
	var timeout time.Duration
	var workDir string
	var memoryLimit int64

	cmd := &cobra.Command{
		Use:   "exec [command] [args...]",
		Short: "Execute a command in a sandboxed environment",
		Long: `Run a command with process isolation and resource limits.
Supports filesystem and network sandboxing.

Examples:
  forge exec -- go test ./...
  forge exec --network -- go mod download
  forge exec --timeout 30s -- python script.py
  forge exec --fs -- rm -rf /tmp/test`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				cancel()
			}()

			fmt.Println(pretty.InfoLine(fmt.Sprintf("Executing: %v", args)))
			fmt.Println()

			tm := timer.New()

			// Use sandbox for execution
			sb := sandbox.New(sandbox.Config{
				Timeout:     timeout,
				WorkDir:     workDir,
				MemoryLimit: memoryLimit,
				Network:     network,
			})

			// For exec mode, we run as a shell command
			result, err := sb.Execute(ctx, "#!/bin/bash\n"+strings.Join(args, " "))
			if err != nil {
				fmt.Println(pretty.ErrorLine(fmt.Sprintf("Execution failed: %v (%s)", err, tm.String())))
				return err
			}

			if result.Stdout != "" {
				fmt.Print(result.Stdout)
			}
			if result.Stderr != "" {
				fmt.Fprint(os.Stderr, result.Stderr)
			}

			if result.TimedOut {
				fmt.Println(pretty.ErrorLine(fmt.Sprintf("Timed out after %s", timeout)))
			} else if result.ExitCode == 0 {
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Completed (%s)", tm.String())))
			} else {
				fmt.Println(pretty.ErrorLine(fmt.Sprintf("Exit code %d (%s)", result.ExitCode, tm.String())))
			}

			return nil
		},
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "eval [language] [code]",
			Short: "Evaluate code in a sandboxed environment",
			Long: `Evaluate code snippets in various languages.

Examples:
  forge exec eval python 'print("hello")'
  forge exec eval go 'package main; import "fmt"; func main() { fmt.Println("hi") }'
  forge exec eval bash 'echo hello'
  forge exec eval javascript 'console.log("hi")'`,
			Args: cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				langStr := args[0]
				code := args[1]

				lang := sandbox.Language(strings.ToLower(langStr))

				// Validate language
				found := false
				for _, l := range sandbox.SupportedLanguages() {
					if l == lang {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("unsupported language %q. Supported: %v", langStr, sandbox.SupportedLanguages())
				}

				fmt.Println(pretty.InfoLine(fmt.Sprintf("Evaluating %s code", lang)))

				sb := sandbox.New(sandbox.Config{
					Language: lang,
					Timeout:  timeout,
				})

				tm := timer.New()
				result, err := sb.Execute(context.Background(), code)
				if err != nil {
					return fmt.Errorf("execution error: %w", err)
				}

				if result.Stdout != "" {
					fmt.Println(result.Stdout)
				}
				if result.Stderr != "" {
					fmt.Fprint(os.Stderr, result.Stderr)
				}

				if result.TimedOut {
					fmt.Println(pretty.ErrorLine(fmt.Sprintf("Timed out (%s)", tm.String())))
				} else if result.ExitCode == 0 {
					fmt.Println(pretty.SuccessLine(fmt.Sprintf("OK (%s)", tm.String())))
				} else {
					fmt.Println(pretty.ErrorLine(fmt.Sprintf("Exit %d (%s)", result.ExitCode, tm.String())))
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "runtimes",
			Short: "List available language runtimes",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println(pretty.HeaderLine("Available Runtimes"))
				for _, lang := range sandbox.SupportedLanguages() {
					status := "not installed"
					if sandbox.IsAvailable(lang) {
						status = "✓ available"
					}
					fmt.Printf("  %-15s %s\n", lang, status)
				}
				return nil
			},
		},
	)

	cmd.Flags().BoolVar(&network, "network", false, "Enable network access")
	cmd.Flags().BoolVarP(&filesystem, "fs", "f", false, "Enable filesystem sandboxing")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 30*time.Second, "Execution timeout")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "Working directory")
	cmd.Flags().Int64Var(&memoryLimit, "memory", 0, "Memory limit in bytes (0 = unlimited)")

	return cmd
}
