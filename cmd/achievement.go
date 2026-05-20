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
	return achievement.NewTracker(getForgeDir() + "/achievement.json")
}

func achievementListCmd() *cobra.Command {
	var jsonOutput bool
	var showAll bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all achievements and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			tracker := getAchievementTracker()

			var achievements []achievement.Achievement
			if showAll {
				achievements = tracker.ListAll()
			} else {
				achievements = tracker.List()
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(achievements, "", "  ")
				fmt.Println(string(data))
				return nil
			}

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
	cmd.Flags().BoolVar(&showAll, "all", false, "Show hidden achievements too")
	return cmd
}

func achievementUnlockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock <achievement-id>",
		Short: "Unlock an achievement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tracker := getAchievementTracker()

			a, err := tracker.Unlock(args[0])
			if err != nil {
				return fmt.Errorf("achievement error: %w", err)
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
			stats := tracker.Stats()

			pct := 0.0
			if stats.Total > 0 {
				pct = float64(stats.UnlockedTotal) / float64(stats.Total) * 100
			}

			fmt.Printf("Achievement Progress\n")
			fmt.Printf("====================\n")
			fmt.Printf("Unlocked: %d/%d (%.0f%%)\n", stats.UnlockedTotal, stats.Total, pct)
			fmt.Printf("Level: %s\n", levelForCount(stats.UnlockedTotal))

			fmt.Printf("\nBy Tier:\n")
			for _, tier := range []achievement.Tier{
				achievement.TierCommon,
				achievement.TierUncommon,
				achievement.TierRare,
				achievement.TierEpic,
				achievement.TierLegendary,
			} {
				total := stats.Tiers[tier]
				unlocked := stats.Unlocked[tier]
				if total > 0 {
					fmt.Printf("  %s: %d/%d\n", tier, unlocked, total)
				}
			}
			return nil
		},
	}
	return cmd
}

// levelForCount returns a level title based on unlock count.
func levelForCount(count int) string {
	switch {
	case count >= 17:
		return "Forge Master 👑"
	case count >= 12:
		return "Veteran ⚔️"
	case count >= 8:
		return "Adventurer 🗡️"
	case count >= 4:
		return "Apprentice 📖"
	case count >= 1:
		return "Novice 🌱"
	default:
		return "Uninitiated"
	}
}
