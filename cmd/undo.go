package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/safety/undo"
	"github.com/spf13/cobra"
)

func undoCmd() *cobra.Command {
	var limit int
	var all bool
	var last bool
	var journalDir string

	cmd := &cobra.Command{
		Use:   "undo [snapshot-id]",
		Short: "Undo agent actions",
		Long: `Revert actions performed by agents.
Shows recent changes and allows selective or full undo.

The forge remembers every action. Every strike can be reversed.

Examples:
  forge undo                    # show recent actions
  forge undo --last             # undo the most recent action
  forge undo snap-123456        # undo a specific action
  forge undo --all              # undo all tracked actions
  forge undo --last --dry-run   # preview what would be undone`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if journalDir == "" {
				home, _ := os.UserHomeDir()
				journalDir = filepath.Join(home, ".forge", "undo")
			}

			j := undo.NewJournal(journalDir)

			// Specific snapshot ID
			if len(args) > 0 {
				id := args[0]
				fmt.Println(pretty.InfoLine(fmt.Sprintf("Undoing %s...", id)))
				if err := j.Undo(id); err != nil {
					return fmt.Errorf("undo failed: %w", err)
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Reverted %s", id)))
				return nil
			}

			// Undo all
			if all {
				fmt.Println(pretty.WarningLine("Undoing ALL tracked actions..."))
				count, err := j.UndoAll()
				if err != nil {
					fmt.Printf("  %s Reverted %d action(s) before error: %v\n",
						pretty.Sprint(pretty.Warning, "!"), count, err)
					return nil
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Reverted %d action(s)", count)))
				return nil
			}

			// Undo last
			if last {
				snap, err := j.UndoLast()
				if err != nil {
					return fmt.Errorf("undo failed: %w", err)
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Reverted %s: %s %s",
					snap.ID, snap.Action, snap.Path)))
				return nil
			}

			// Default: show recent actions
			snaps, err := j.List(limit)
			if err != nil {
				return fmt.Errorf("failed to load journal: %w", err)
			}

			if len(snaps) == 0 {
				fmt.Println(pretty.InfoLine("No tracked actions found"))
				fmt.Println("  Actions are tracked when agents use the undo journal.")
				return nil
			}

			fmt.Println(pretty.HeaderLine("Recent Agent Actions"))
			fmt.Println()

			for _, snap := range snaps {
				status := pretty.Sprint(pretty.DimF, "○")
				if snap.Reverted {
					status = pretty.Sprint(pretty.DimF, "↩")
				}

				ts := snap.Timestamp.Format("Jan 02 15:04:05")
				agent := snap.Agent
				if agent == "" {
					agent = "unknown"
				}

				fmt.Printf("  %s %-14s %-12s %s  %s\n",
					status,
					pretty.Sprint(pretty.DimF, ts),
					pretty.Sprint(pretty.Info, string(snap.Action)),
					shortenPath(snap.Path),
					pretty.Sprintf(pretty.DimF, "(%s)", agent),
				)

				if snap.Reverted {
					fmt.Printf("    %s\n", pretty.Sprint(pretty.DimF, "↩ reverted"))
				}
			}

			fmt.Println()
			fmt.Printf("  %d action(s) shown. Use 'forge undo <id>' to revert.\n", len(snaps))
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Number of recent actions to show")
	cmd.Flags().BoolVar(&all, "all", false, "Undo all tracked actions")
	cmd.Flags().BoolVarP(&last, "last", "l", false, "Undo the most recent action")
	cmd.Flags().StringVar(&journalDir, "journal", "", "Journal directory (default: ~/.forge/undo)")

	return cmd
}

func shortenPath(path string) string {
	if path == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	// Shorten to last 3 components
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 3 {
		return "..." + string(filepath.Separator) + strings.Join(parts[len(parts)-3:], string(filepath.Separator))
	}
	return path
}
