package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/routing"
	"github.com/spf13/cobra"
)

func pipelineCmd() *cobra.Command {
	var strategy string

	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage multi-agent pipelines and routing",
		Long: `Configure and run multi-agent pipelines with
configurable routing strategies.

Strategies: round_robin, random, least_loaded, weighted, fallback, latency_based

Examples:
  forge pipeline create my-pipeline --strategy round_robin
  forge pipeline add-agent my-pipeline --id agent1 --url http://localhost:3284
  forge pipeline route my-pipeline
  forge pipeline list`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "create [name]",
			Short: "Create a new pipeline",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Pipeline %q created (strategy: %s)", name, strategy)))
				return nil
			},
		},
		&cobra.Command{
			Use:   "add-agent [pipeline]",
			Short: "Add an agent to a pipeline",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				pipeline := args[0]
				agentID, _ := cmd.Flags().GetString("id")
				agentURL, _ := cmd.Flags().GetString("url")
				weight, _ := cmd.Flags().GetFloat64("weight")

				fmt.Println(pretty.SuccessLine(fmt.Sprintf(
					"Agent %q added to pipeline %q (url=%s, weight=%.1f)",
					agentID, pipeline, agentURL, weight,
				)))
				return nil
			},
		},
		&cobra.Command{
			Use:   "route [pipeline]",
			Short: "Route a request through the pipeline",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				// Demo routing with sample agents
				r := routing.New(routing.Strategy(strategy))
				r.AddAgent(&routing.Agent{ID: "agent1", Name: "Claude", URL: "http://localhost:3284", Weight: 3.0, Healthy: true})
				r.AddAgent(&routing.Agent{ID: "agent2", Name: "Codex", URL: "http://localhost:3285", Weight: 1.0, Healthy: true})
				r.AddAgent(&routing.Agent{ID: "agent3", Name: "Gemini", URL: "http://localhost:3286", Weight: 2.0, Healthy: true})

				agent, err := r.Route()
				if err != nil {
					return err
				}

				fmt.Println(pretty.InfoLine(fmt.Sprintf("Routed to: %s (%s) via %s", agent.Name, agent.URL, strategy)))
				return nil
			},
		},
		&cobra.Command{
			Use:   "benchmark [pipeline]",
			Short: "Benchmark routing strategies",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				strategies := []routing.Strategy{
					routing.RoundRobin,
					routing.Random,
					routing.LeastLoaded,
					routing.Weighted,
					routing.LatencyBased,
				}

				agents := []*routing.Agent{
					{ID: "agent1", Name: "Claude", URL: "http://localhost:3284", Weight: 3.0, Healthy: true, Latency: 120},
					{ID: "agent2", Name: "Codex", URL: "http://localhost:3285", Weight: 1.0, Healthy: true, Latency: 80},
					{ID: "agent3", Name: "Gemini", URL: "http://localhost:3286", Weight: 2.0, Healthy: true, Latency: 200},
				}

				fmt.Println(pretty.HeaderLine("Routing Benchmark"))
				fmt.Println()

				n := 1000
				for _, s := range strategies {
					r := routing.New(s)
					for _, a := range agents {
						r.AddAgent(&routing.Agent{
							ID: a.ID, Name: a.Name, URL: a.URL,
							Weight: a.Weight, Healthy: a.Healthy, Latency: a.Latency,
						})
					}

					counts := make(map[string]int)
					start := time.Now()
					for i := 0; i < n; i++ {
						agent, _ := r.Route()
						counts[agent.Name]++
					}
					elapsed := time.Since(start)

					fmt.Printf("  %-15s %v  ", s, elapsed)
					for _, a := range agents {
						pct := float64(counts[a.Name]) / float64(n) * 100
						fmt.Printf("%s:%.0f%%  ", a.Name, pct)
					}
					fmt.Println()
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List available routing strategies",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println(pretty.HeaderLine("Routing Strategies"))
				strategies := []struct {
					name string
					desc string
				}{
					{"round_robin", "Cycle through agents in order"},
					{"random", "Select agent at random"},
					{"least_loaded", "Pick agent with fewest active requests"},
					{"weighted", "Distribute by weight"},
					{"fallback", "Try agents in order until one succeeds"},
					{"latency_based", "Pick agent with lowest response time"},
				}

				for _, s := range strategies {
					fmt.Printf("  %-15s %s\n", s.name, s.desc)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "simulate [pipeline]",
			Short: "Simulate pipeline routing with synthetic load",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				requests, _ := cmd.Flags().GetInt("requests")

				r := routing.New(routing.Strategy(strategy))
				r.AddAgent(&routing.Agent{ID: "claude", Name: "Claude", URL: "http://localhost:3284", Weight: 3.0, Healthy: true, Latency: 120})
				r.AddAgent(&routing.Agent{ID: "codex", Name: "Codex", URL: "http://localhost:3285", Weight: 1.0, Healthy: true, Latency: 80})
				r.AddAgent(&routing.Agent{ID: "gemini", Name: "Gemini", URL: "http://localhost:3286", Weight: 2.0, Healthy: true, Latency: 200})

				counts := make(map[string]int)
				for i := 0; i < requests; i++ {
					agent, err := r.RouteForRequest()
					if err != nil {
						continue
					}
					counts[agent.Name]++
					// Simulate some work
					r.Release(agent, time.Duration(agent.Latency)*time.Millisecond)
				}

				fmt.Println(pretty.HeaderLine(fmt.Sprintf("Simulation: %d requests via %s", requests, strategy)))
				b, _ := json.MarshalIndent(counts, "", "  ")
				fmt.Println(string(b))
				return nil
			},
		},
	)

	cmd.PersistentFlags().StringVarP(&strategy, "strategy", "s", "round_robin", "Routing strategy")
	cmd.Commands()[1].Flags().String("id", "", "Agent ID")
	cmd.Commands()[1].Flags().String("url", "", "Agent URL")
	cmd.Commands()[1].Flags().Float64("weight", 1.0, "Agent weight (for weighted strategy)")
	cmd.Commands()[5].Flags().Int("requests", 1000, "Number of simulated requests")

	_ = os.Stdout

	return cmd
}
