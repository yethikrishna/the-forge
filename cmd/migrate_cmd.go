package cmd

import (
	"fmt"
	"time"

	"github.com/forge/sword/internal/migrate"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database schema migration manager",
	Long:  `Track, apply, and rollback schema migrations. Version your database evolution. Never lose data.`,
}

var migrateManager *migrate.Manager

func getMigrateManager() *migrate.Manager {
	if migrateManager == nil {
		migrateManager = migrate.NewManager(getForgeDir() + "/migrate")
	}
	return migrateManager
}

func init() {
	migrateCmd.AddCommand(migrateCreateCmd)
	migrateCmd.AddCommand(migrateApplyCmd)
	migrateCmd.AddCommand(migrateRollbackCmd)
	migrateCmd.AddCommand(migrateListCmd)
	migrateCmd.AddCommand(migrateShowCmd)
	migrateCmd.AddCommand(migratePendingCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
}

// migrate create
var migrateCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new migration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getMigrateManager()
		mig, err := m.Create(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Created migration: %s (v%d, id: %s)\n", mig.Name, mig.Version, mig.ID)
		return nil
	},
}

// migrate apply
var migrateApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetInt("target")
		m := getMigrateManager()
		applied, err := m.Apply(target)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			fmt.Println("No pending migrations")
		} else {
			for _, mig := range applied {
				fmt.Printf("✓ Applied: %s (v%d)\n", mig.Name, mig.Version)
			}
			fmt.Printf("\n%d migrations applied (now at v%d)\n", len(applied), m.CurrentVersion())
		}
		return nil
	},
}

// migrate rollback
var migrateRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback applied migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetInt("target")
		m := getMigrateManager()
		rolled, err := m.Rollback(target)
		if err != nil {
			return err
		}
		if len(rolled) == 0 {
			fmt.Println("No migrations to rollback")
		} else {
			for _, mig := range rolled {
				fmt.Printf("↩ Rolled back: %s (v%d)\n", mig.Name, mig.Version)
			}
		}
		return nil
	},
}

// migrate list
var migrateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getMigrateManager()
		list := m.List()
		if len(list) == 0 {
			fmt.Println("No migrations")
			return nil
		}

		fmt.Printf("%-8s %-30s %-12s %-20s %s\n", "VERSION", "NAME", "STATUS", "APPLIED", "CHECKSUM")
		for _, mig := range list {
			applied := "—"
			if !mig.AppliedAt.IsZero() {
				applied = mig.AppliedAt.Format(time.RFC3339)
			}
			fmt.Printf("%-8d %-30s %-12s %-20s %s\n",
				mig.Version, mig.Name, mig.Status, applied, mig.Checksum[:8])
		}
		return nil
	},
}

// migrate show
var migrateShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show migration details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getMigrateManager()
		mig, ok := m.Get(args[0])
		if !ok {
			return fmt.Errorf("migration %q not found", args[0])
		}
		fmt.Println(migrate.RenderMigration(mig))
		return nil
	},
}

// migrate pending
var migratePendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "Show pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getMigrateManager()
		pending := m.Pending()
		if len(pending) == 0 {
			fmt.Println("No pending migrations")
		} else {
			for _, mig := range pending {
				fmt.Printf("  v%d: %s\n", mig.Version, mig.Name)
			}
		}
		return nil
	},
}

// migrate status
var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getMigrateManager().Stats()
		fmt.Printf("Total: %v\n", stats["total"])
		fmt.Printf("Current Version: %v\n", stats["current_version"])
		if byStatus, ok := stats["by_status"].(map[migrate.Status]int); ok {
			fmt.Println("By Status:")
			for status, count := range byStatus {
				fmt.Printf("  %s: %d\n", status, count)
			}
		}
		return nil
	},
}

func init() {
	migrateApplyCmd.Flags().Int("target", 0, "Target version (0 = apply all)")
	migrateRollbackCmd.Flags().Int("target", 0, "Rollback to this version (0 = rollback one)")
}
