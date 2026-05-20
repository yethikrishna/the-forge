package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/debate"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func debateCmd() *cobra.Command {
	var debateDir string

	cmd := &cobra.Command{
		Use:   "debate",
		Short: "Multi-agent debate for decision making",
		Long: `Run structured debates between agents.

Multiple agents argue different positions, a judge evaluates,
and the best argument wins. Truth emerges from disagreement.

Examples:
  forge debate start "Should we use microservices?" --debaters "pro:for,con:against"
  forge debate argue <id> --debater pro --claim "Better scalability"
  forge debate argue <id> --debater con --claim "More complexity" --rebuttal
  forge debate judge <id>
  forge debate list
  forge debate show <id>`,
	}

	startCmd := &cobra.Command{
		Use:   "start <topic>",
		Short: "Start a new debate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDebateDir(debateDir)
			store := debate.NewStore(dir)

			topic := args[0]
			desc, _ := cmd.Flags().GetString("description")
			maxRounds, _ := cmd.Flags().GetInt("rounds")
			debatersFlag, _ := cmd.Flags().GetString("debaters")

			var debaters []debate.Debater
			if debatersFlag != "" {
				for i, d := range strings.Split(debatersFlag, ",") {
					parts := strings.SplitN(d, ":", 2)
					name := parts[0]
					pos := debate.PositionFor
					if len(parts) > 1 {
						switch strings.ToLower(parts[1]) {
						case "against":
							pos = debate.PositionAgainst
						case "neutral":
							pos = debate.PositionNeutral
						case "expert":
							pos = debate.PositionExpert
						default:
							pos = debate.PositionFor
						}
					}
					debaters = append(debaters, debate.Debater{
						ID:       fmt.Sprintf("d%d", i+1),
						Name:     name,
						Position: pos,
						Agent:    name,
					})
				}
			}

			if len(debaters) < 2 {
				debaters = []debate.Debater{
					{ID: "d1", Name: "pro", Position: debate.PositionFor, Agent: "pro"},
					{ID: "d2", Name: "con", Position: debate.PositionAgainst, Agent: "con"},
				}
			}

			d, err := store.Create(topic, desc, debaters, maxRounds)
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Debate started: %s", d.ID)))
			fmt.Printf("  Topic:    %s\n", topic)
			fmt.Printf("  Debaters: %d\n", len(debaters))
			fmt.Printf("  Rounds:   %d\n", maxRounds)
			return nil
		},
	}
	startCmd.Flags().String("description", "", "Debate description")
	startCmd.Flags().Int("rounds", 3, "Maximum number of rounds")
	startCmd.Flags().String("debaters", "", "Debaters as name:position pairs (e.g., pro:for,con:against)")

	argueCmd := &cobra.Command{
		Use:   "argue <debate-id>",
		Short: "Add an argument to a debate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDebateDir(debateDir)
			store := debate.NewStore(dir)

			debaterID, _ := cmd.Flags().GetString("debater")
			claim, _ := cmd.Flags().GetString("claim")
			evidence, _ := cmd.Flags().GetString("evidence")
			reasoning, _ := cmd.Flags().GetString("reasoning")
			rebuttalTo, _ := cmd.Flags().GetString("rebuttal")

			if claim == "" {
				return fmt.Errorf("--claim is required")
			}

			d, err := store.Get(args[0])
			if err != nil {
				return err
			}

			// Find debater's position
			position := debate.PositionNeutral
			for _, db := range d.Debaters {
				if db.ID == debaterID || db.Name == debaterID {
					position = db.Position
					debaterID = db.ID
					break
				}
			}

			arg := debate.Argument{
				DebaterID:  debaterID,
				Position:   position,
				Claim:      claim,
				Evidence:   evidence,
				Reasoning:  reasoning,
				RebuttalTo: rebuttalTo,
			}

			updated, err := store.AddArgument(d.ID, arg)
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine("Argument added"))
			fmt.Printf("  Round: %d/%d\n", updated.Rounds, updated.MaxRounds)
			if updated.Status == "concluded" {
				fmt.Println(pretty.InfoLine("Debate concluded — use 'forge debate judge' to render verdict"))
			}
			return nil
		},
	}
	argueCmd.Flags().String("debater", "", "Debater ID or name")
	argueCmd.Flags().String("claim", "", "The claim (required)")
	argueCmd.Flags().String("evidence", "", "Supporting evidence")
	argueCmd.Flags().String("reasoning", "", "Logical reasoning")
	argueCmd.Flags().String("rebuttal", "", "ID of argument being rebutted")

	judgeCmd := &cobra.Command{
		Use:   "judge <debate-id>",
		Short: "Render a verdict on a debate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDebateDir(debateDir)
			store := debate.NewStore(dir)

			d, err := store.Get(args[0])
			if err != nil {
				return err
			}

			if len(d.Arguments) == 0 {
				return fmt.Errorf("no arguments to judge")
			}

			// Evaluate arguments
			debate.EvaluateArguments(d)

			// Find winner by highest score
			var bestArg debate.Argument
			for _, arg := range d.Arguments {
				if arg.Score > bestArg.Score {
					bestArg = arg
				}
			}

			verdict := debate.Verdict{
				Winner:     bestArg.DebaterID,
				Reasoning:  fmt.Sprintf("Strongest argument: %s", bestArg.Claim),
				Confidence: bestArg.Score / 100.0,
				Consensus:  false,
			}

			// Check for consensus
			positions := make(map[debate.Position]int)
			for _, arg := range d.Arguments {
				positions[arg.Position]++
			}
			verdict.Consensus = len(positions) <= 1

			concluded, err := store.Conclude(d.ID, verdict)
			if err != nil {
				return err
			}

			fmt.Println(pretty.HeaderLine("Verdict"))
			fmt.Print(debate.FormatDebate(concluded))
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all debates",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDebateDir(debateDir)
			store := debate.NewStore(dir)

			debates, err := store.List()
			if err != nil {
				return err
			}

			if len(debates) == 0 {
				fmt.Println(pretty.InfoLine("No debates found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Debates"))
			for _, d := range debates {
				status := d.Status
				icon := "●"
				if d.Status == "concluded" {
					icon = "✓"
				}
				args := len(d.Arguments)
				fmt.Printf("  %s %-20s %-12s %d arg(s) %d/%d rounds\n",
					icon, d.Topic, status, args, d.Rounds, d.MaxRounds)
			}
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <debate-id>",
		Short: "Show debate details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getDebateDir(debateDir)
			store := debate.NewStore(dir)

			d, err := store.Get(args[0])
			if err != nil {
				return err
			}

			debate.EvaluateArguments(d)
			fmt.Print(debate.FormatDebate(d))
			return nil
		},
	}

	cmd.AddCommand(startCmd, argueCmd, judgeCmd, listCmd, showCmd)
	cmd.PersistentFlags().StringVar(&debateDir, "dir", "", "Debate directory (default: .forge/debates)")

	return cmd
}

func getDebateDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "debates")
}
