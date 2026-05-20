package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/progressive"
	"github.com/spf13/cobra"
)

func levelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "level",
		Short: "Progressive complexity ladder — track your Forge mastery",
		Long: `Track your progression from Forge beginner to master.
Level 0 (Curious) → Level 5 (Master) with guided milestones.

Each level unlocks new capabilities and teaches you the next set of features.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		levelShowCmd(),
		levelPathCmd(),
		levelCompleteCmd(),
		levelNextCmd(),
		levelStatsCmd(),
	)

	return cmd
}

func getLadder() *progressive.Ladder {
	return progressive.NewLadder(getForgeDir() + "/ladder.json")
}

func levelShowCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show your current level and progress",
		RunE: func(cmd *cobra.Command, args []string) error {
			ladder := getLadder()

			if jsonOutput {
				data, _ := json.MarshalIndent(ladder.Stats(), "", "  ")
				fmt.Println(string(data))
				return nil
			}

			level := ladder.CurrentLevel()
			progress := ladder.Progress()

			fmt.Printf("Forge Level: %d — %s %s\n", level, level.Icon(), level)
			fmt.Printf("XP: %d\n\n", ladder.XP)

			fmt.Println("Progress by Level:")
			for l := progressive.Level0; l <= progressive.Level5; l++ {
				p := progress[l]
				bar := progressBar(p.Pct, 20)
				current := "  "
				if l == level {
					current = "→ "
				}
				fmt.Printf("  %s Level %d %-10s %s %5.1f%% (%d/%d)\n",
					current, l, l, bar, p.Pct, p.Completed, p.Total)
			}

			fmt.Printf("\nOverall: %.1f%% complete\n", ladder.OverallProgress())
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func levelPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show the full learning path",
		RunE: func(cmd *cobra.Command, args []string) error {
			ladder := getLadder()
			fmt.Println(ladder.Path())
			return nil
		},
	}
	return cmd
}

func levelCompleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete <milestone-id>",
		Short: "Mark a milestone as completed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ladder := getLadder()
			m, err := ladder.Complete(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("✅ Milestone completed: %s\n", m.Name)
			fmt.Printf("   %s\n", m.Description)

			level := ladder.CurrentLevel()
			fmt.Printf("\nLevel: %d — %s %s | XP: %d\n", level, level.Icon(), level, ladder.XP)
			return nil
		},
	}
	return cmd
}

func levelNextCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show recommended next steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			ladder := getLadder()
			steps := ladder.NextSteps()

			if len(steps) == 0 {
				fmt.Println("🎉 You've completed all milestones! You're a Forge Master!")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(steps, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Recommended Next Steps\n\n")
			for i, s := range steps {
				cmd := ""
				if s.Command != "" {
					cmd = fmt.Sprintf(" → %s", s.Command)
				}
				fmt.Printf("  %d. %s%s\n", i+1, s.Description, cmd)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func levelStatsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show detailed ladder statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			ladder := getLadder()
			stats := ladder.Stats()

			if jsonOutput {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Level Statistics\n")
			fmt.Printf("================\n")
			fmt.Printf("Current Level: %d — %s %s\n", stats["level_number"], stats["level"], progressive.Level(stats["level_number"].(int)).Icon())
			fmt.Printf("XP: %v\n", stats["xp"])
			fmt.Printf("Milestones: %v/%v completed\n", stats["completed_milestones"], stats["total_milestones"])
			fmt.Printf("Overall Progress: %.1f%%\n", stats["overall_pct"])
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func progressBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
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
