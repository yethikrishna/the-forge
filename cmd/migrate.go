package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/migrate"
	"github.com/spf13/cobra"
)

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate agents between models with A/B comparison",
		Long: `Seamlessly migrate running agents from one model to another.
Preserves context, memory, and tool state. Includes A/B comparison
to validate the migration improves quality and/or reduces cost.`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		migrateStartCmd(),
		migrateCompleteCmd(),
		migrateRollbackCmd(),
		migrateListCmd(),
		migrateShowCmd(),
		migrateABTestCmd(),
		migrateStatsCmd(),
	)

	return cmd
}

func getMigrateManager() *migrate.Manager {
	return migrate.NewManager(getForgeDir() + "/migrations")
}

func migrateStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <agent-id> <from-model> <to-model>",
		Short: "Start a model migration",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getMigrateManager()
			mig, err := mgr.StartMigration(args[0], args[1], args[2])
			if err != nil {
				return err
			}

			fmt.Printf("Migration started: %s\n", mig.ID)
			fmt.Printf("  Agent: %s | %s → %s\n", args[0], args[1], args[2])
			fmt.Printf("  Context: %d tokens | Memory: %d entries\n", mig.ContextTokens, mig.MemoryEntries)
			return nil
		},
	}
	return cmd
}

func migrateCompleteCmd() *cobra.Command {
	var cost float64
	var quality float64

	cmd := &cobra.Command{
		Use:   "complete <migration-id>",
		Short: "Complete a migration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getMigrateManager()
			if err := mgr.CompleteMigration(args[0], cost, quality); err != nil {
				return err
			}

			mig, _ := mgr.GetMigration(args[0])
			fmt.Printf("Migration completed: %s\n", mig.ID)
			fmt.Println(migrate.MigrationReport(mig))
			return nil
		},
	}

	cmd.Flags().Float64Var(&cost, "cost", 0, "Cost after migration")
	cmd.Flags().Float64Var(&quality, "quality", 0, "Quality score after migration (0-100)")
	return cmd
}

func migrateRollbackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback <migration-id>",
		Short: "Rollback a completed migration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getMigrateManager()
			rollback, err := mgr.RollbackMigration(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Migration rolled back: %s\n", rollback.ID)
			fmt.Printf("  Reverted: %s → %s\n", rollback.FromModel, rollback.ToModel)
			return nil
		},
	}
	return cmd
}

func migrateListCmd() *cobra.Command {
	var jsonOutput bool
	var agentID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getMigrateManager()

			var list []*migrate.Migration
			if agentID != "" {
				list = mgr.ListByAgent(agentID)
			} else {
				list = mgr.ListMigrations()
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(list, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(list) == 0 {
				fmt.Println("No migrations found.")
				return nil
			}

			fmt.Printf("Migrations (%d)\n\n", len(list))
			for _, mig := range list {
				icon := "🔄"
				switch mig.Status {
				case migrate.StatusCompleted:
					icon = "✅"
				case migrate.StatusFailed:
					icon = "❌"
				case migrate.StatusRolledBack:
					icon = "⏪"
				}
				fmt.Printf("  %s %-15s %-20s → %-20s [%s]\n",
					icon, mig.AgentID, mig.FromModel, mig.ToModel, mig.Status)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVarP(&agentID, "agent", "a", "", "Filter by agent ID")
	return cmd
}

func migrateShowCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show <migration-id>",
		Short: "Show migration details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getMigrateManager()
			mig, ok := mgr.GetMigration(args[0])
			if !ok {
				return fmt.Errorf("migration %s not found", args[0])
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(mig, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(migrate.MigrationReport(mig))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func migrateABTestCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "abtest <agent-id> <model-a> <model-b>",
		Short: "Run A/B comparison between two models",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getMigrateManager()
			test := mgr.StartABTest(args[0], args[1], args[2], "Compare model quality and cost")

			if jsonOutput {
				data, _ := json.MarshalIndent(test, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(migrate.ABTestReport(test))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func migrateStatsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show migration statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := getMigrateManager()
			stats := mgr.Stats()

			if jsonOutput {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Migration Statistics\n")
			fmt.Printf("====================\n")
			fmt.Printf("Total: %v | Completed: %v | Failed: %v | Rolled Back: %v\n",
				stats["total_migrations"], stats["completed"], stats["failed"], stats["rolled_back"])
			fmt.Printf("A/B Tests: %v\n", stats["ab_tests"])
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}
