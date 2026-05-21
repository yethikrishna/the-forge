package cmd

import (
	"fmt"
	"time"

	"github.com/forge/sword/internal/snapshot"
	"github.com/spf13/cobra"
)

var snapCmd = &cobra.Command{
	Use:   "snap",
	Short: "Project state snapshots",
	Long:  "Create and manage project state snapshots for time-travel debugging and state comparison.",
}

var (
	snapDir     string
	snapType    string
	snapDesc    string
	snapProject string
)

func init() {
	snapCmd.AddCommand(snapCreateCmd)
	snapCmd.AddCommand(snapListCmd)
	snapCmd.AddCommand(snapShowCmd)
	snapCmd.AddCommand(snapDiffCmd)
	snapCmd.AddCommand(snapDeleteCmd)
	snapCmd.AddCommand(snapStatsCmd)

	snapCmd.PersistentFlags().StringVar(&snapDir, "dir", ".forge/snapshots", "Snapshot storage directory")
	snapCreateCmd.Flags().StringVar(&snapType, "type", "manual", "Snapshot type (manual, auto, pre-operation, post-operation, milestone)")
	snapCreateCmd.Flags().StringVar(&snapDesc, "desc", "", "Description")
	snapCreateCmd.Flags().StringVar(&snapProject, "project", ".", "Project directory")
	snapDiffCmd.Flags().String("from", "", "First snapshot ID")
	snapDiffCmd.Flags().String("to", "", "Second snapshot ID")
}

func getSnapStore() (*snapshot.Store, error) {
	return snapshot.NewStore(snapDir)
}

var snapCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSnapStore()
		if err != nil {
			return err
		}
		snap, err := store.Create(args[0], snapshot.Type(snapType), snapProject, snapDesc)
		if err != nil {
			return err
		}
		fmt.Printf("Snapshot: %s (%d files, %d bytes)\n", snap.ID, snap.FileCount, snap.TotalSize)
		return nil
	},
}

var snapListCmd = &cobra.Command{
	Use:   "list",
	Short: "List snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSnapStore()
		if err != nil {
			return err
		}
		snaps := store.List("")
		if len(snaps) == 0 {
			fmt.Println("No snapshots found.")
			return nil
		}
		fmt.Printf("Snapshots (%d):\n", len(snaps))
		for _, s := range snaps {
			fmt.Printf("  %s [%s] %s — %d files (%s)\n", s.Name, s.Type, s.ID, s.FileCount, s.CreatedAt.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var snapShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show snapshot details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSnapStore()
		if err != nil {
			return err
		}
		snap, ok := store.Get(args[0])
		if !ok {
			return fmt.Errorf("snapshot %q not found", args[0])
		}
		fmt.Printf("Snapshot: %s (id: %s)\n", snap.Name, snap.ID)
		fmt.Printf("Type: %s  Created: %s\n", snap.Type, snap.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Files: %d  Size: %d bytes\n", snap.FileCount, snap.TotalSize)
		if snap.GitBranch != "" {
			fmt.Printf("Git: %s@%s (dirty: %v)\n", snap.GitBranch, snap.GitCommit, snap.GitDirty)
		}
		return nil
	},
}

var snapDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Compare two snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromID, _ := cmd.Flags().GetString("from")
		toID, _ := cmd.Flags().GetString("to")
		if fromID == "" || toID == "" {
			return fmt.Errorf("--from and --to are required")
		}
		store, err := getSnapStore()
		if err != nil {
			return err
		}
		diff, err := store.Compare(fromID, toID)
		if err != nil {
			return err
		}
		fmt.Printf("Added: %d  Removed: %d  Modified: %d  Unchanged: %d\n",
			len(diff.Added), len(diff.Removed), len(diff.Modified), diff.Unchanged)
		if len(diff.Added) > 0 {
			fmt.Println("\nAdded:")
			for _, f := range diff.Added {
				fmt.Printf("  + %s\n", f)
			}
		}
		if len(diff.Removed) > 0 {
			fmt.Println("\nRemoved:")
			for _, f := range diff.Removed {
				fmt.Printf("  - %s\n", f)
			}
		}
		if len(diff.Modified) > 0 {
			fmt.Println("\nModified:")
			for _, f := range diff.Modified {
				fmt.Printf("  ~ %s\n", f)
			}
		}
		return nil
	},
}

var snapDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSnapStore()
		if err != nil {
			return err
		}
		return store.Delete(args[0])
	},
}

var snapStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Snapshot statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getSnapStore()
		if err != nil {
			return err
		}
		stats := store.Stats()
		fmt.Printf("Snapshots: %d  Files: %d  Size: %d bytes\n", stats.TotalSnapshots, stats.TotalFiles, stats.TotalSize)
		for t, count := range stats.ByType {
			fmt.Printf("  %s: %d\n", t, count)
		}
		return nil
	},
}
