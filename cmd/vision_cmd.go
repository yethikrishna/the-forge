package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/dream"
)

var visionCmd = &cobra.Command{
	Use:   "vision",
	Short: "Agent vision system — background simulation and insight generation",
	Long: `Agents simulate future scenarios while idle, generating insights
and pre-computed solutions. Like dreaming but with purpose.

Vision types: scenario, hypothesis, stress, creative, consolidate, precompute`,
}

var visionDir string

func init() {
	visionCmd.AddCommand(visionSubmitCmd)
	visionCmd.AddCommand(visionListCmd)
	visionCmd.AddCommand(visionShowCmd)
	visionCmd.AddCommand(visionInsightsCmd)
	visionCmd.AddCommand(visionStatsCmd)
	visionCmd.AddCommand(visionInterruptCmd)

	visionCmd.PersistentFlags().StringVar(&visionDir, "dir", ".forge/visions", "Vision storage directory")
	visionSubmitCmd.Flags().Int("priority", 3, "Priority (1-10)")
	visionSubmitCmd.Flags().String("agent", "default", "Agent ID")
	visionInsightsCmd.Flags().Bool("actionable", false, "Show only actionable insights")
}

func getVisionEngine() *dream.Engine {
	return dream.NewEngine(visionDir)
}

var visionSubmitCmd = &cobra.Command{
	Use:   "submit [type] [prompt]",
	Short: "Submit a new vision",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getVisionEngine()

		var dreamType dream.DreamType
		switch args[0] {
		case "scenario":
			dreamType = dream.DreamScenario
		case "hypothesis":
			dreamType = dream.DreamHypothesis
		case "stress":
			dreamType = dream.DreamStress
		case "creative":
			dreamType = dream.DreamCreative
		case "consolidate":
			dreamType = dream.DreamConsolidate
		case "precompute":
			dreamType = dream.DreamPrecompute
		default:
			return fmt.Errorf("unknown type: %s (use: scenario, hypothesis, stress, creative, consolidate, precompute)", args[0])
		}

		priority, _ := cmd.Flags().GetInt("priority")
		agentID, _ := cmd.Flags().GetString("agent")

		d := engine.Submit(dreamType, agentID, args[1], nil, priority)
		fmt.Printf("Vision submitted: %s [%s]\n", d.ID, d.Type)
		return nil
	},
}

var visionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List visions",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getVisionEngine()
		dreams := engine.List()
		if len(dreams) == 0 {
			fmt.Println("No visions found.")
			return nil
		}

		fmt.Printf("Visions (%d):\n", len(dreams))
		for _, d := range dreams {
			insights := ""
			if len(d.Insights) > 0 {
				insights = fmt.Sprintf(" insights=%d", len(d.Insights))
			}
			fmt.Printf("  %s [%s] %s — %s%s\n",
				d.ID, d.Status, d.Type, truncStr(d.Prompt, 50), insights)
		}
		return nil
	},
}

var visionShowCmd = &cobra.Command{
	Use:   "show [vision-id]",
	Short: "Show vision details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getVisionEngine()
		d, err := engine.Get(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("═══ Vision: %s ═══\n", d.ID)
		fmt.Printf("Type: %s\n", d.Type)
		fmt.Printf("Agent: %s\n", d.AgentID)
		fmt.Printf("Prompt: %s\n", d.Prompt)
		fmt.Printf("Status: %s\n", d.Status)
		fmt.Printf("Priority: %d  Confidence: %.2f  Relevance: %.2f\n", d.Priority, d.Confidence, d.Relevance)
		fmt.Printf("Created: %s\n", d.CreatedAt.Format(time.RFC3339))
		if !d.CompletedAt.IsZero() {
			fmt.Printf("Duration: %v  Tokens: %d\n", d.Duration.Round(time.Millisecond), d.TokensUsed)
		}

		if len(d.Insights) > 0 {
			fmt.Println("\n── Insights ──")
			for i, ins := range d.Insights {
				actionable := ""
				if ins.Actionable {
					actionable = " ✓ actionable"
				}
				fmt.Printf("%d. [%s] %s (%.0f%% conf, %.0f%% impact)%s\n",
					i+1, ins.Type, ins.Title, ins.Confidence*100, ins.Impact*100, actionable)
				if ins.Action != "" {
					fmt.Printf("   → %s\n", ins.Action)
				}
			}
		}
		return nil
	},
}

var visionInsightsCmd = &cobra.Command{
	Use:   "insights",
	Short: "Show all insights from completed visions",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getVisionEngine()

		actionableOnly, _ := cmd.Flags().GetBool("actionable")

		var insights []dream.Insight
		if actionableOnly {
			insights = engine.GetActionableInsights()
		} else {
			insights = engine.GetInsights()
		}

		if len(insights) == 0 {
			fmt.Println("No insights found.")
			return nil
		}

		fmt.Printf("Insights (%d):\n", len(insights))
		for i, ins := range insights {
			actionable := ""
			if ins.Actionable {
				actionable = " ✓"
			}
			fmt.Printf("%d. [%s] %s (%.0f%% conf, %.0f%% impact)%s\n",
				i+1, ins.Type, ins.Title, ins.Confidence*100, ins.Impact*100, actionable)
			if ins.Action != "" {
				fmt.Printf("   → %s\n", ins.Action)
			}
		}
		return nil
	},
}

var visionStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show vision engine statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getVisionEngine()
		stats := engine.Stats()

		fmt.Println("Vision Engine Statistics")
		fmt.Println("========================")
		fmt.Printf("Total Visions: %d\n", stats.TotalDreams)
		fmt.Printf("Total Insights: %d (actionable: %d)\n", stats.TotalInsights, stats.ActionableInsights)
		fmt.Printf("Avg Relevance: %.2f\n", stats.AvgRelevance)
		fmt.Printf("Tokens Used: %d (budget: %d)\n", stats.TokensUsed, stats.BudgetRemaining)

		fmt.Println("\nBy Type:")
		for typ, count := range stats.ByType {
			fmt.Printf("  %s: %d\n", typ, count)
		}
		fmt.Println("\nBy Status:")
		for status, count := range stats.ByStatus {
			fmt.Printf("  %s: %d\n", status, count)
		}
		return nil
	},
}

var visionInterruptCmd = &cobra.Command{
	Use:   "interrupt [vision-id]",
	Short: "Interrupt a running vision",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getVisionEngine()
		err := engine.Interrupt(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Vision %s interrupted.\n", args[0])
		return nil
	},
}
