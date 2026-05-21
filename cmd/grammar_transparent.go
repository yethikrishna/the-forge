package cmd

import (
	"fmt"
	"strings"

	"github.com/forge/sword/internal/grammar"
	"github.com/forge/sword/internal/transparent"
	"github.com/spf13/cobra"
)

var grammarAuditor = grammar.NewAuditor()

func grammarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grammar",
		Short: "Audit unified command grammar (forge <noun> <verb>)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				violations := grammarAuditor.AuditAll(args)
				if len(violations) == 0 {
					fmt.Println("All commands follow unified grammar.")
				} else {
					fmt.Print(grammarAuditor.Report(args))
				}
				return nil
			}
			// List all patterns
			fmt.Println("Registered command patterns:\n")
			for _, noun := range grammarAuditor.Nouns() {
				verbs := grammarAuditor.VerbsFor(noun)
				fmt.Printf("  %-12s %s\n", noun, strings.Join(verbs, ", "))
			}
			return nil
		},
	}

	cmd.AddCommand(grammarAuditCmd())
	return cmd
}

func grammarAuditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit <commands...>",
		Short: "Audit specific commands for grammar violations",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(grammarAuditor.Report(args))
			return nil
		},
	}
}

func transparentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transparent",
		Short: "Real-time visibility into agent operations",
		Long: `Show model selection, token counts, costs, tool calls,
file access, and network requests in real-time.`,
	}

	cmd.AddCommand(
		transparentStatsCmd(),
		transparentExportCmd(),
	)

	return cmd
}

func transparentStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show session transparency stats",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Demo stats
			tr := transparent.NewTracker("demo", "agent-1", true)
			tr.Record(transparent.EventModelSelect, transparent.WithModel("gpt-4"))
			tr.Record(transparent.EventTokenCount, transparent.WithTokens(100, 50))
			tr.Record(transparent.EventCost, transparent.WithCost(0.03, 0.06, "$"))
			stats := tr.Stats()
			fmt.Print(transparent.FormatStats(&stats))
			return nil
		},
	}
}

func transparentExportCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export transparency events",
		RunE: func(cmd *cobra.Command, args []string) error {
			tr := transparent.NewTracker("export", "agent-1", true)
			tr.Record(transparent.EventModelSelect, transparent.WithModel("gpt-4"))

			var data []byte
			var err error
			if format == "stats" {
				data, err = tr.ExportStatsJSON()
			} else {
				data, err = tr.ExportJSON()
			}
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "events", "Export format: events, stats")
	return cmd
}
