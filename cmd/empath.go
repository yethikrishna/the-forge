package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/experience/empath"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

var empathAnalyzer = empath.NewAnalyzer()

func empathCmd() *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "empath",
		Short: "User frustration detection and adaptive response",
		Long: `Analyze messages for frustration signals and get adaptive
response strategies.

Read the room. Read the user.`,
	}

	cmd.AddCommand(
		empathAnalyzeCmd(&asJSON),
		empathTrendCmd(&asJSON),
		empathHistoryCmd(&asJSON),
	)

	return cmd
}

func empathAnalyzeCmd(asJSON *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze <message>",
		Short: "Analyze a message for frustration",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			message := args[0]
			for i := 1; i < len(args); i++ {
				message += " " + args[i]
			}

			result := empathAnalyzer.Analyze(message)

			if *asJSON {
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Frustration Analysis"))

			levelStr := levelColor(result.Level)
			fmt.Printf("  Level:      %s\n", levelStr)
			fmt.Printf("  Score:      %.1f/100\n", result.Score)
			fmt.Printf("  Confidence: %.0f%%\n", result.Confidence*100)

			if len(result.Signals) > 0 {
				fmt.Printf("\n  Signals (%d):\n", len(result.Signals))
				for _, sig := range result.Signals {
					fmt.Printf("    [%s] %s\n", sig.Type, sig.Message)
				}
			}

			fmt.Printf("\n  Strategy:   %s\n", result.Strategy.Tone)
			if len(result.Strategy.Suggestions) > 0 {
				fmt.Println("  Suggestions:")
				for _, s := range result.Strategy.Suggestions {
					fmt.Printf("    → %s\n", s)
				}
			}
			if len(result.Strategy.Avoid) > 0 {
				fmt.Println("  Avoid:")
				for _, a := range result.Strategy.Avoid {
					fmt.Printf("    ✗ %s\n", a)
				}
			}
			if result.Strategy.Acknowledge {
				fmt.Println(pretty.WarningLine("Acknowledge frustration before responding"))
			}
			if result.Strategy.Escalate {
				fmt.Println(pretty.Sprintf(pretty.RedF, "  ⚠ Consider escalation"))
			}

			return nil
		},
	}

	return cmd
}

func empathTrendCmd(asJSON *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Show frustration trend",
		RunE: func(cmd *cobra.Command, args []string) error {
			trend := empathAnalyzer.Trend()
			if *asJSON {
				fmt.Printf(`{"trend":"%s"}`+"\n", trend)
				return nil
			}

			fmt.Printf("Trend: %s\n", trend)
			return nil
		},
	}

	return cmd
}

func empathHistoryCmd(asJSON *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show frustration history",
		RunE: func(cmd *cobra.Command, args []string) error {
			history := empathAnalyzer.History()

			if *asJSON {
				data, _ := json.MarshalIndent(history, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(history) == 0 {
				fmt.Println(pretty.InfoLine("No analysis history"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Frustration History"))
			for i, score := range history {
				bar := scoreBar(score)
				fmt.Printf("  #%d  %s  %.1f\n", i+1, bar, score)
			}

			return nil
		},
	}

	return cmd
}

func levelColor(level empath.FrustrationLevel) string {
	switch level {
	case empath.LevelCalm:
		return pretty.Sprint(pretty.GreenF, string(level))
	case empath.LevelNeutral:
		return string(level)
	case empath.LevelAnnoyed:
		return pretty.Sprint(pretty.YellowF, string(level))
	case empath.LevelFrustrated:
		return pretty.Sprint(pretty.CyanF, string(level))
	case empath.LevelAngry:
		return pretty.Sprint(pretty.RedF, string(level))
	default:
		return string(level)
	}
}

func scoreBar(score float64) string {
	const width = 20
	filled := int(score / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}
