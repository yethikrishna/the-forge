package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forge/sword/internal/stress"
	"github.com/spf13/cobra"
)

var stressCmd = &cobra.Command{
	Use:   "stress",
	Short: "Agent load and stress testing",
	Long:  `Simulate concurrent agent sessions, measure throughput, latency, error rates, and resource consumption under load. Know your limits before your users do.`,
}

var stressRunner *stress.Runner

func getStressRunner() *stress.Runner {
	if stressRunner == nil {
		stressRunner = stress.NewRunner(getForgeDir() + "/stress")
	}
	return stressRunner
}

func init() {
	stressCmd.AddCommand(stressCreateCmd)
	stressCmd.AddCommand(stressListCmd)
	stressCmd.AddCommand(stressShowCmd)
	stressCmd.AddCommand(stressDeleteCmd)
	stressCmd.AddCommand(stressRunCmd)
	stressCmd.AddCommand(stressReportCmd)
	stressCmd.AddCommand(stressReportListCmd)
}

// stress create
var stressCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a stress test configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		testType, _ := cmd.Flags().GetString("type")
		r := getStressRunner()
		cfg := r.CreateConfig(args[0], stress.TestType(testType))

		// Apply flags
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		duration, _ := cmd.Flags().GetDuration("duration")
		maxRPS, _ := cmd.Flags().GetFloat64("max-rps")
		errorRate, _ := cmd.Flags().GetFloat64("error-rate")

		r.UpdateConfig(cfg.ID, func(c *stress.TestConfig) {
			if concurrency > 0 {
				c.Concurrency = concurrency
			}
			if duration > 0 {
				c.Duration = duration
			}
			if maxRPS > 0 {
				c.MaxRPS = maxRPS
			}
			if errorRate > 0 {
				c.ErrorRate = errorRate
			}
		})

		fmt.Printf("Created stress test config: %s (id: %s)\n", args[0], cfg.ID)
		return nil
	},
}

// stress list
var stressListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stress test configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getStressRunner()
		list := r.ListConfigs()
		if len(list) == 0 {
			fmt.Println("No stress test configurations")
			return nil
		}

		fmt.Printf("%-25s %-20s %-12s %-12s %s\n", "ID", "NAME", "TYPE", "CONCURRENCY", "DURATION")
		for _, c := range list {
			fmt.Printf("%-25s %-20s %-12s %-12d %s\n",
				c.ID, c.Name, c.Type, c.Concurrency, c.Duration)
		}
		return nil
	},
}

// stress show
var stressShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show stress test configuration details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getStressRunner()
		cfg, ok := r.GetConfig(args[0])
		if !ok {
			return fmt.Errorf("config %q not found", args[0])
		}
		fmt.Println(stress.RenderConfig(cfg))
		return nil
	},
}

// stress delete
var stressDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a stress test configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getStressRunner().DeleteConfig(args[0])
	},
}

// stress run
var stressRunCmd = &cobra.Command{
	Use:   "run [config-id]",
	Short: "Run a stress test",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getStressRunner()

		fmt.Printf("Running stress test %s...\n", args[0])
		report, err := r.Run(args[0])
		if err != nil {
			return err
		}

		fmt.Println(stress.RenderReport(report))
		return nil
	},
}

// stress report
var stressReportCmd = &cobra.Command{
	Use:   "report [config-id]",
	Short: "Show the latest stress test report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getStressRunner()
		report, ok := r.GetReport(args[0])
		if !ok {
			return fmt.Errorf("no report found for config %q", args[0])
		}

		outputFormat, _ := cmd.Flags().GetString("output")
		switch outputFormat {
		case "json":
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(data))
		default:
			fmt.Println(stress.RenderReport(report))
		}
		return nil
	},
}

// stress report-list
var stressReportListCmd = &cobra.Command{
	Use:   "report-list",
	Short: "List all stress test reports",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := getStressRunner()
		reports := r.ListReports()
		if len(reports) == 0 {
			fmt.Println("No stress test reports")
			return nil
		}

		fmt.Printf("%-25s %-20s %-8s %-8s %-8s %s\n",
			"CONFIG", "NAME", "TOTAL", "OK", "FAIL", "RPS")
		for _, rep := range reports {
			fmt.Printf("%-25s %-20s %-8d %-8d %-8d %.1f\n",
				rep.ConfigID, rep.ConfigName, rep.TotalSessions,
				rep.SuccessCount, rep.FailureCount, rep.ThroughputRPS)
		}
		return nil
	},
}

func init() {
	stressCreateCmd.Flags().String("type", "sustained", "Test type (ramp-up, sustained, spike, wave, custom)")
	stressCreateCmd.Flags().Int("concurrency", 10, "Concurrent sessions")
	stressCreateCmd.Flags().Duration("duration", 30*time.Second, "Test duration")
	stressCreateCmd.Flags().Float64("max-rps", 0, "Max requests per second (0 = unlimited)")
	stressCreateCmd.Flags().Float64("error-rate", 0.02, "Simulated error rate (0-1)")

	stressReportCmd.Flags().String("output", "text", "Output format (text, json)")
}
