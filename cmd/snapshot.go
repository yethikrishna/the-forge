package cmd

import (
	"fmt"
	"os"

	"github.com/forge/sword/internal/snapshot"
	"github.com/spf13/cobra"
)

func snapshotCmd() *cobra.Command {
	var storeDir string
	var overwrite bool
	var tags []string
	var notes string

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Create and manage environment checkpoints",
		Long: `Before every great swing, mark where you stand.

Snapshots capture the current state of your project — files, git state,
environment variables — so you can return to any point in time.

Examples:
  forge snapshot create "before-refactor"
  forge snapshot list
  forge snapshot restore snap-123456789
  forge snapshot diff snap-111 snap-222`,
	}

	cmd.PersistentFlags().StringVar(&storeDir, "store", ".forge/snapshots", "Snapshot storage directory")
	cmd.PersistentFlags().StringSliceVar(&tags, "tags", nil, "Tags for the snapshot")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "create [label]",
			Short: "Create a new snapshot",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := snapshot.NewStore(storeDir)
				label := "untitled"
				if len(args) > 0 {
					label = args[0]
				}

				opts := []snapshot.CreateOption{
					snapshot.WithTags(tags...),
					snapshot.WithNotes(notes),
				}

				snap, err := store.Create(label, opts...)
				if err != nil {
					return err
				}

				fmt.Printf("Snapshot created: %s\n", snap.ID)
				fmt.Printf("  Label:    %s\n", snap.Label)
				fmt.Printf("  Git:      %s (%s)\n", snap.GitCommit[:8], snap.GitBranch)
				fmt.Printf("  Dirty:    %v\n", snap.GitDirty)
				fmt.Printf("  Files:    %d\n", len(snap.Files))
				fmt.Printf("  Size:     %d bytes\n", snap.Size)
				if len(snap.Tags) > 0 {
					fmt.Printf("  Tags:     %v\n", snap.Tags)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all snapshots",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := snapshot.NewStore(storeDir)
				snaps, err := store.List()
				if err != nil {
					return err
				}

				if len(snaps) == 0 {
					fmt.Println("No snapshots found.")
					return nil
				}

				fmt.Printf("%-20s %-20s %-8s %-10s %s\n", "ID", "Label", "Files", "Git", "Created")
				fmt.Println(string(make([]byte, 80)))
				for _, s := range snaps {
					commit := "none"
					if len(s.GitCommit) >= 8 {
						commit = s.GitCommit[:8]
					}
					fmt.Printf("%-20s %-20s %-8d %-10s %s\n",
						s.ID, s.Label, len(s.Files), commit,
						s.CreatedAt.Format("2006-01-02 15:04"))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "show [id]",
			Short: "Show snapshot details",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := snapshot.NewStore(storeDir)
				snap, err := store.Get(args[0])
				if err != nil {
					return err
				}

				fmt.Printf("ID:        %s\n", snap.ID)
				fmt.Printf("Label:     %s\n", snap.Label)
				fmt.Printf("Created:   %s\n", snap.CreatedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("Git:       %s (%s)\n", snap.GitCommit, snap.GitBranch)
				fmt.Printf("Dirty:     %v\n", snap.GitDirty)
				fmt.Printf("Files:     %d (%d bytes)\n", len(snap.Files), snap.Size)
				if len(snap.Tags) > 0 {
					fmt.Printf("Tags:      %v\n", snap.Tags)
				}
				if snap.Notes != "" {
					fmt.Printf("Notes:     %s\n", snap.Notes)
				}
				fmt.Println("\nFiles:")
				for path := range snap.Files {
					fmt.Printf("  %s\n", path)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "restore [id]",
			Short: "Restore a snapshot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := snapshot.NewStore(storeDir)

				opts := []snapshot.RestoreOption{
					snapshot.WithOverwrite(overwrite),
				}

				if err := store.Restore(args[0], opts...); err != nil {
					return err
				}

				fmt.Printf("Snapshot %s restored.\n", args[0])
				return nil
			},
		},
		&cobra.Command{
			Use:   "diff [id1] [id2]",
			Short: "Compare two snapshots",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := snapshot.NewStore(storeDir)
				diffs, err := store.Diff(args[0], args[1])
				if err != nil {
					return err
				}

				if len(diffs) == 0 {
					fmt.Println("No differences.")
					return nil
				}

				fmt.Printf("Differences between %s and %s:\n\n", args[0], args[1])
				for path, d := range diffs {
					fmt.Printf("  %-10s %s\n", d.Status, path)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "delete [id]",
			Short: "Delete a snapshot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := snapshot.NewStore(storeDir)
				if err := store.Delete(args[0]); err != nil {
					return err
				}
				fmt.Printf("Snapshot %s deleted.\n", args[0])
				return nil
			},
		},
	)

	cmd.AddCommand(&cobra.Command{
		Use:   "create [label]",
		Short: "Create a new snapshot",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = notes
			_ = os.ReadFile
			return nil
		},
		Hidden: true,
	})

	return cmd
}
