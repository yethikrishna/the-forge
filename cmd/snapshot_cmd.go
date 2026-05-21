package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/forge/sword/internal/snapshot"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Time-travel filesystem snapshots",
	Long:  `Capture, browse, diff, and restore project snapshots without git. Every moment, preserved.`,
}

var snapshotMgr *snapshot.Manager

func getSnapshotManager() *snapshot.Manager {
	if snapshotMgr == nil {
		snapshotMgr = snapshot.NewManager(getForgeDir() + "/snapshots")
	}
	return snapshotMgr
}

func init() {
	snapshotCmd.AddCommand(snapshotCaptureCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotShowCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
	snapshotCmd.AddCommand(snapshotDiffCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotStatsCmd)
}

// snapshot capture
var snapshotCaptureCmd = &cobra.Command{
	Use:   "capture [dir]",
	Short: "Capture a snapshot of a directory",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")

		m := getSnapshotManager()
		snap, err := m.Capture(dir, name, desc, snapshot.IgnorePatterns())
		if err != nil {
			return err
		}

		fmt.Printf("Captured snapshot: %s (id: %s)\n", snap.Name, snap.ID)
		fmt.Printf("Files: %d | Size: %d bytes\n", snap.FileCount, snap.TotalSize)
		return nil
	},
}

// snapshot list
var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getSnapshotManager()
		list := m.List()
		if len(list) == 0 {
			fmt.Println("No snapshots found")
			return nil
		}

		fmt.Printf("%-20s %-15s %-8s %-10s %s\n", "ID", "NAME", "FILES", "SIZE", "CREATED")
		for _, s := range list {
			size := fmt.Sprintf("%d", s.TotalSize)
			if s.TotalSize > 1024*1024 {
				size = fmt.Sprintf("%.1fMB", float64(s.TotalSize)/(1024*1024))
			} else if s.TotalSize > 1024 {
				size = fmt.Sprintf("%.1fKB", float64(s.TotalSize)/1024)
			}
			fmt.Printf("%-20s %-15s %-8d %-10s %s\n",
				s.ID, s.Name, s.FileCount, size,
				s.CreatedAt.Format(time.RFC3339))
		}
		return nil
	},
}

// snapshot show
var snapshotShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show snapshot details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getSnapshotManager()
		snap, ok := m.Get(args[0])
		if !ok {
			return fmt.Errorf("snapshot %q not found", args[0])
		}
		fmt.Println(snapshot.RenderSnapshot(snap))
		return nil
	},
}

// snapshot delete
var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getSnapshotManager().Delete(args[0])
	},
}

// snapshot diff
var snapshotDiffCmd = &cobra.Command{
	Use:   "diff [old-id] [new-id]",
	Short: "Compare two snapshots",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getSnapshotManager()
		diff, err := m.Diff(args[0], args[1])
		if err != nil {
			return err
		}

		outputFormat, _ := cmd.Flags().GetString("output")
		switch outputFormat {
		case "json":
			data, _ := json.MarshalIndent(diff, "", "  ")
			fmt.Println(string(data))
		default:
			fmt.Println(snapshot.RenderDiff(diff))
		}
		return nil
	},
}

// snapshot restore
var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore [snapshot-id]",
	Short: "Restore files from a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = "."
		}
		files, _ := cmd.Flags().GetStringSlice("files")

		m := getSnapshotManager()
		if err := m.RestoreFiles(args[0], dir, files); err != nil {
			return err
		}

		fmt.Printf("Restored %d files from snapshot %s\n", len(files), args[0])
		return nil
	},
}

// snapshot stats
var snapshotStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show snapshot statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getSnapshotManager().Stats()
		fmt.Printf("Total Snapshots: %v\n", stats["total_snapshots"])
		fmt.Printf("Total Files: %v\n", stats["total_files"])
		fmt.Printf("Total Size: %v bytes\n", stats["total_size"])
		return nil
	},
}

func init() {
	snapshotCaptureCmd.Flags().String("name", "unnamed", "Snapshot name")
	snapshotCaptureCmd.Flags().String("description", "", "Snapshot description")

	snapshotDiffCmd.Flags().String("output", "text", "Output format (text, json)")

	snapshotRestoreCmd.Flags().String("dir", ".", "Target directory for restore")
	snapshotRestoreCmd.Flags().StringSlice("files", nil, "Specific files to restore")
}
