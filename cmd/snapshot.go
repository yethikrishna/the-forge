package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/snapshot"
	"github.com/spf13/cobra"
)

func snapshotCmd() *cobra.Command {
	var snapshotDir string
	var tags []string
	var description string
	var captureEnv bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Environment checkpoints",
		Long: `Create and manage environment snapshots.

Capture full workspace state: files, git status, environment variables.
Name snapshots, restore them, diff between them.

Git only tracks committed changes. Agents make uncommitted changes.
Snapshots fill the gap.

Examples:
  forge snapshot create before-refactor
  forge snapshot list
  forge snapshot restore before-refactor
  forge snapshot diff snap-a snap-b
  forge snapshot delete old-snapshot`,
	}

	createCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new snapshot",
		Long: `Capture the current environment state into a named snapshot.

Records git state (branch, commit, dirty flag), environment variables,
and archives all project files. Snapshots can be restored later.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, workDir := getSnapshotDirs(snapshotDir)

			name := ""
			if len(args) > 0 {
				name = args[0]
			}

			opts := []snapshot.CreateOption{
				snapshot.WithCaptureEnv(captureEnv),
				snapshot.WithTags(tags),
			}
			if description != "" {
				opts = append(opts, snapshot.WithAgent(description))
			}

			if dryRun {
				fmt.Println(pretty.InfoLine("Dry run — would create snapshot"))
				fmt.Printf("  Name:    %s\n", nameOrDefault(name))
				fmt.Printf("  WorkDir: %s\n", workDir)
				fmt.Printf("  Env:     %v\n", captureEnv)
				if len(tags) > 0 {
					fmt.Printf("  Tags:    %s\n", strings.Join(tags, ", "))
				}
				return nil
			}

			store := snapshot.NewStore(dir, workDir)
			cp, err := store.Create(name, opts...)
			if err != nil {
				return fmt.Errorf("failed to create snapshot: %w", err)
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Snapshot created: %s", cp.NameOrID())))
			fmt.Printf("  ID:      %s\n", cp.ID)
			fmt.Printf("  Files:   %d\n", cp.FileCount)
			fmt.Printf("  Size:    %s\n", formatSize(cp.TotalSize))
			fmt.Printf("  Branch:  %s\n", cp.GitBranch)
			fmt.Printf("  Commit:  %s\n", cp.GitCommit)
			if cp.GitDirty {
				fmt.Printf("  Dirty:   yes\n")
			}
			if len(cp.Tags) > 0 {
				fmt.Printf("  Tags:    %s\n", strings.Join(cp.Tags, ", "))
			}
			return nil
		},
	}
	createCmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags for the snapshot")
	createCmd.Flags().StringVar(&description, "desc", "", "Description of the snapshot")
	createCmd.Flags().BoolVar(&captureEnv, "env", true, "Capture environment variables")
	createCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without doing it")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := getSnapshotDirs(snapshotDir)
			store := snapshot.NewStore(dir, "")

			checkpoints, err := store.List()
			if err != nil {
				return fmt.Errorf("failed to list snapshots: %w", err)
			}

			if len(checkpoints) == 0 {
				fmt.Println(pretty.InfoLine("No snapshots found"))
				fmt.Println("  Create one with: forge snapshot create <name>")
				return nil
			}

			fmt.Println(pretty.HeaderLine("Environment Snapshots"))
			fmt.Println()

			for _, cp := range checkpoints {
				status := "●"
				switch cp.Status {
				case snapshot.StatusActive:
					status = pretty.Sprint(pretty.Success, "●")
				case snapshot.StatusRestored:
					status = pretty.Sprint(pretty.Info, "↩")
				case snapshot.StatusDeleted:
					status = pretty.Sprint(pretty.DimF, "✗")
				}

				ts := cp.Timestamp.Format("Jan 02 15:04:05")
				name := cp.NameOrID()

				fmt.Printf("  %s %-14s %-20s %s  %s  %d files  %s\n",
					status,
					pretty.Sprint(pretty.DimF, ts),
					pretty.Sprint(pretty.Info, name),
					pretty.Sprint(pretty.DimF, cp.GitBranch),
					pretty.Sprint(pretty.DimF, cp.GitCommit),
					cp.FileCount,
					formatSize(cp.TotalSize),
				)

				if len(cp.Tags) > 0 {
					fmt.Printf("    tags: %s\n", pretty.Sprint(pretty.DimF, strings.Join(cp.Tags, ", ")))
				}
			}

			fmt.Println()
			fmt.Printf("  %d snapshot(s)\n", len(checkpoints))
			return nil
		},
	}

	restoreCmd := &cobra.Command{
		Use:   "restore <id-or-name>",
		Short: "Restore a snapshot",
		Long: `Restore the working directory to a previous snapshot state.

This overwrites current files with the snapshot contents.
Use with caution — uncommitted changes will be lost.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, workDir := getSnapshotDirs(snapshotDir)
			store := snapshot.NewStore(dir, workDir)

			cp, err := store.Restore(args[0])
			if err != nil {
				return fmt.Errorf("failed to restore snapshot: %w", err)
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Restored snapshot: %s", cp.NameOrID())))
			fmt.Printf("  Files:   %d\n", cp.FileCount)
			fmt.Printf("  Branch:  %s\n", cp.GitBranch)
			fmt.Printf("  Commit:  %s\n", cp.GitCommit)
			return nil
		},
	}

	diffCmd := &cobra.Command{
		Use:   "diff <snapshot-a> <snapshot-b>",
		Short: "Compare two snapshots",
		Long: `Show differences between two snapshots.

Displays added, deleted, and modified files, plus git state changes.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := getSnapshotDirs(snapshotDir)
			store := snapshot.NewStore(dir, "")

			diff, err := store.Diff(args[0], args[1])
			if err != nil {
				return fmt.Errorf("failed to diff snapshots: %w", err)
			}

			fmt.Print(diff)
			return nil
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete <id-or-name>",
		Short: "Delete a snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := getSnapshotDirs(snapshotDir)
			store := snapshot.NewStore(dir, "")

			if err := store.Delete(args[0]); err != nil {
				return fmt.Errorf("failed to delete snapshot: %w", err)
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Deleted snapshot: %s", args[0])))
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, restoreCmd, diffCmd, deleteCmd)
	cmd.PersistentFlags().StringVar(&snapshotDir, "dir", "", "Snapshot directory (default: .forge/snapshots)")

	return cmd
}

func getSnapshotDirs(flagDir string) (string, string) {
	workDir, _ := os.Getwd()
	dir := flagDir
	if dir == "" {
		dir = filepath.Join(workDir, ".forge", "snapshots")
	}
	return dir, workDir
}

func nameOrDefault(name string) string {
	if name == "" {
		return "(auto)"
	}
	return name
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
