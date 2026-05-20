package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/forge/sword/internal/circuit"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func circuitCmd() *cobra.Command {
	var circuitDir string

	cmd := &cobra.Command{
		Use:   "circuit",
		Short: "Circuit breakers for agent calls",
		Long: `Manage circuit breakers for agent and API calls.

When an agent keeps failing, stop calling it. Let it recover.
Prevents cascading failures in multi-agent pipelines.

Examples:
  forge circuit list
  forge circuit create my-agent --threshold 5
  forge circuit status my-agent
  forge circuit trip my-agent
  forge circuit reset my-agent
  forge circuit stats`,
	}

	createCmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a circuit breaker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			threshold, _ := cmd.Flags().GetInt("threshold")
			timeout, _ := cmd.Flags().GetDuration("timeout")

			cfg := circuit.DefaultConfig(name)
			if threshold > 0 {
				cfg.FailureThreshold = threshold
			}
			if timeout > 0 {
				cfg.Timeout = timeout
			}

			dir := getCircuitDir(circuitDir)
			reg := circuit.NewRegistry(dir)
			reg.GetOrCreate(cfg)

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Circuit breaker created: %s", name)))
			fmt.Printf("  Failure threshold: %d\n", cfg.FailureThreshold)
			fmt.Printf("  Timeout:           %s\n", cfg.Timeout)
			return nil
		},
	}
	createCmd.Flags().Int("threshold", 0, "Failure threshold (default: 5)")
	createCmd.Flags().Duration("timeout", 0, "Open state timeout (default: 30s)")

	statusCmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show circuit breaker status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCircuitDir(circuitDir)
			reg := circuit.NewRegistry(dir)

			b, ok := reg.Get(args[0])
			if !ok {
				return fmt.Errorf("circuit breaker %q not found", args[0])
			}

			stats := b.Stats()
			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Circuit Breaker: %s", args[0])))
			fmt.Printf("  State:             %s\n", circuit.FormatState(b.State()))
			fmt.Printf("  Failures:          %v\n", stats["failures"])
			fmt.Printf("  Total calls:       %v\n", stats["total_calls"])
			fmt.Printf("  Total failures:    %v\n", stats["total_failures"])
			fmt.Printf("  Total rejected:    %v\n", stats["total_rejected"])
			fmt.Printf("  Last failure:      %v\n", stats["last_failure"])
			fmt.Printf("  Last state change: %v\n", stats["last_state_change"])
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all circuit breakers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCircuitDir(circuitDir)
			reg := circuit.NewRegistry(dir)

			names := reg.List()
			if len(names) == 0 {
				fmt.Println(pretty.InfoLine("No circuit breakers"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Circuit Breakers"))
			for _, name := range names {
				b, _ := reg.Get(name)
				fmt.Printf("  %s %s\n", circuit.FormatState(b.State()), name)
			}
			return nil
		},
	}

	tripCmd := &cobra.Command{
		Use:   "trip <name>",
		Short: "Manually trip a circuit breaker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCircuitDir(circuitDir)
			reg := circuit.NewRegistry(dir)

			b, ok := reg.Get(args[0])
			if !ok {
				return fmt.Errorf("circuit breaker %q not found", args[0])
			}

			b.Trip()
			fmt.Println(pretty.WarningLine(fmt.Sprintf("Tripped: %s", args[0])))
			return nil
		},
	}

	resetCmd := &cobra.Command{
		Use:   "reset <name>",
		Short: "Reset a circuit breaker to closed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCircuitDir(circuitDir)
			reg := circuit.NewRegistry(dir)

			b, ok := reg.Get(args[0])
			if !ok {
				return fmt.Errorf("circuit breaker %q not found", args[0])
			}

			b.Reset()
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Reset: %s", args[0])))
			return nil
		},
	}

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show stats for all circuit breakers",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getCircuitDir(circuitDir)
			reg := circuit.NewRegistry(dir)

			allStats := reg.AllStats()
			if len(allStats) == 0 {
				fmt.Println(pretty.InfoLine("No circuit breakers"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Circuit Breaker Stats"))
			for _, stats := range allStats {
				name := stats["name"].(string)
				state := stats["state"].(string)
				calls := stats["total_calls"].(int64)
				failures := stats["total_failures"].(int64)
				rejected := stats["total_rejected"].(int64)

				fmt.Printf("  %-20s %-12s calls:%d fails:%d rejected:%d\n",
					name, state, calls, failures, rejected)
			}
			return nil
		},
	}

	cmd.AddCommand(createCmd, statusCmd, listCmd, tripCmd, resetCmd, statsCmd)
	cmd.PersistentFlags().StringVar(&circuitDir, "dir", "", "Circuit breaker directory (default: .forge/circuits)")

	return cmd
}

func getCircuitDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "circuits")
}

// Ensure time is imported
var _ = time.Second
