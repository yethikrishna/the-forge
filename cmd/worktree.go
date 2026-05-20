package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/worktree"
	"github.com/spf13/cobra"
)

func worktreeCmd() *cobra.Command {
	var wtDir string

	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Git worktree management for parallel agents",
		Long: `Manage git worktrees for parallel agent execution.

Each agent gets its own worktree, avoiding merge conflicts
between concurrent agents working on the same repository.

Examples:
  forge worktree create . --agent coder-1 --branch fix-auth
  forge worktree list
  forge worktree merge <id>
  forge worktree remove <id>`,
	}

	createCmd := &cobra.Command{
		Use:   "create <repo-path>",
		Short: "Create a worktree for an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWTDir(wtDir)
			mgr := worktree.NewManager(dir)

			agentID, _ := cmd.Flags().GetString("agent")
			branchSuffix, _ := cmd.Flags().GetString("branch")

			if agentID == "" {
				return fmt.Errorf("--agent is required")
			}

			wt, err := mgr.Create(args[0], agentID, branchSuffix)
			if err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Created worktree for %s", agentID)))
			fmt.Print(worktree.FormatWorktree(wt))
			return nil
		},
	}
	createCmd.Flags().String("agent", "", "Agent ID")
	createCmd.Flags().String("branch", "", "Branch suffix (default: auto-generated)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List worktrees",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWTDir(wtDir)
			mgr := worktree.NewManager(dir)

			worktrees, err := mgr.List()
			if err != nil {
				return err
			}

			if len(worktrees) == 0 {
				fmt.Println(pretty.InfoLine("No worktrees found"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Git Worktrees"))
			for _, wt := range worktrees {
				fmt.Printf("  %-20s %-15s %-25s %s\n", wt.ID, wt.AgentID, wt.Branch, wt.Status)
			}
			return nil
		},
	}

	mergeCmd := &cobra.Command{
		Use:   "merge <id>",
		Short: "Merge a worktree's branch back",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWTDir(wtDir)
			mgr := worktree.NewManager(dir)

			if err := mgr.Merge(args[0]); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Merged worktree %s", args[0])))
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWTDir(wtDir)
			mgr := worktree.NewManager(dir)

			if err := mgr.Remove(args[0]); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Removed worktree %s", args[0])))
			return nil
		},
	}

	cmd.AddCommand(createCmd, listCmd, mergeCmd, removeCmd)
	cmd.PersistentFlags().StringVar(&wtDir, "dir", "", "Worktree data directory (default: .forge/worktrees)")

	return cmd
}

func getWTDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, ".forge", "worktrees")
}
