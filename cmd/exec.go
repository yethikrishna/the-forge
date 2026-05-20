package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forge/sword/internal/boundary"
	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/timer"
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

			// Build isolation config
			level := boundary.IsolationNone
			if network && filesystem {
				level = boundary.IsolationFull
			} else if network {
				level = boundary.IsolationNetwork
			} else if filesystem {
				level = boundary.IsolationFileSystem
			}

			config := boundary.Config{
				Level:       level,
				WorkDir:     workDir,
				MemoryLimit: memoryLimit,
			}

			if timeout > 0 {
				config.Timeout = timeout
			}

			fmt.Println(pretty.InfoLine(fmt.Sprintf("Executing: %v", args)))
			fmt.Printf("   Isolation: %s\n", levelName(level))
			if timeout > 0 {
				fmt.Printf("   Timeout:   %s\n", timeout)
			}
			fmt.Println()

			iso := boundary.NewIsolator()
			tm := timer.New()

			var proc *boundary.Process
			var err error

			if timeout > 0 {
				proc, err = iso.RunWithTimeout(args[0], args[1:], config, timeout)
			} else {
				proc, err = iso.Run(ctx, args[0], args[1:], config)
			}

			if err != nil {
				fmt.Println(pretty.ErrorLine(fmt.Sprintf("Execution failed: %v", err)))
				return err
			}

			// Wait a bit for the process
			time.Sleep(100 * time.Millisecond)

			exitCode := proc.ExitCode()
			elapsed := tm.String()

			if exitCode == 0 {
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Completed (%s)", elapsed)))
			} else {
				fmt.Println(pretty.ErrorLine(fmt.Sprintf("Exit code %d (%s)", exitCode, elapsed)))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&network, "network", false, "Enable network sandboxing")
	cmd.Flags().BoolVarP(&filesystem, "fs", "f", false, "Enable filesystem sandboxing")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 0, "Execution timeout (0 = unlimited)")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "Working directory")
	cmd.Flags().Int64Var(&memoryLimit, "memory", 0, "Memory limit in bytes (0 = unlimited)")

	return cmd
}

func levelName(level boundary.IsolationLevel) string {
	switch level {
	case boundary.IsolationNone:
		return "none"
	case boundary.IsolationFileSystem:
		return "filesystem"
	case boundary.IsolationNetwork:
		return "network"
	case boundary.IsolationFull:
		return "full (fs+net)"
	default:
		return "unknown"
	}
}
