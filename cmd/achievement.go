package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/achievement"
	"github.com/spf13/cobra"
)

func achievementCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "achievement",
		Short: "Track milestones and developer achievements",
		Long: `Track your journey with Forge. Earn achievements for milestones
like your first chat, first pipeline, first orchestration, and more.

Gamification that drives adoption.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		achievementListCmd(),
		achievementUnlockCmd(),
		achievementStatusCmd(),
	)

	return cmd
}

func getAchievementTracker() *achievement.Tracker {
	return achievement.NewTrackerSimple(getForgeDir() + "/achievement")
}

func achievementListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all achievements and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			tracker := getAchievementTracker()

			if jsonOutput {
				data, _ := json.MarshalIndent(tracker.All(), "", "  ")
				fmt.Println(string(data))
				return nil
			}

			achievements := tracker.All()
			unlocked := 0
			for _, a := range achievements {
				if a.Unlocked {
					unlocked++
				}
			}

			fmt.Printf("Achievements: %d/%d unlocked\n\n", unlocked, len(achievements))
			for _, a := range achievements {
				status := "🔒"
				if a.Unlocked {
					status = "✅"
				}
				fmt.Printf("  %s %-30s %s\n", status, a.ID, a.Description)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func achievementUnlockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock <achievement-id>",
		Short: "Unlock an achievement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tracker := getAchievementTracker()

			a, ok := tracker.UnlockSimple(args[0])
			if !ok {
				return fmt.Errorf("achievement not found or already unlocked: %s", args[0])
			}

			fmt.Printf("🏆 Achievement unlocked: %s\n", a.ID)
			fmt.Printf("   %s\n", a.Description)
			return nil
		},
	}
	return cmd
}

func achievementStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show achievement progress summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			tracker := getAchievementTracker()
			achievements := tracker.All()
			unlocked := 0
			var levels map[string]int

			for _, a := range achievements {
				if a.Unlocked {
					unlocked++
				}
			}

			pct := 0.0
			if len(achievements) > 0 {
				pct = float64(unlocked) / float64(len(achievements)) * 100
			}

			fmt.Printf("Achievement Progress\n")
			fmt.Printf("====================\n")
			fmt.Printf("Unlocked: %d/%d (%.0f%%)\n", unlocked, len(achievements), pct)
			fmt.Printf("Level: %d\n", achievement.LevelForCount(unlocked))

			_ = levels
			return nil
		},
	}
	return cmd
}
