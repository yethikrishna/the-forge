package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/rollback"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Operation rollback and undo",
	Long:  "Manage reversible operations with state snapshots and rollback support.",
}

var (
	rbDir string
)

func init() {
	rollbackCmd.AddCommand(rbSnapshotCmd)
	rollbackCmd.AddCommand(rbBeginCmd)
	rollbackCmd.AddCommand(rbCompleteCmd)
	rollbackCmd.AddCommand(rbUndoCmd)
	rollbackCmd.AddCommand(rbHistoryCmd)
	rollbackCmd.AddCommand(rbStatsCmd)

	rollbackCmd.PersistentFlags().StringVar(&rbDir, "dir", ".forge/rollback", "Rollback storage directory")
}

func getRbMgr() (*rollback.Manager, error) {
	return rollback.NewManager(rbDir)
}

var rbSnapshotCmd = &cobra.Command{
	Use:   "snapshot [description]",
	Short: "Save a state snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getRbMgr()
		if err != nil {
			return err
		}
		state, err := mgr.SaveState(args[0], "cli", map[string]string{"source": "cli"})
		if err != nil {
			return err
		}
		fmt.Printf("Snapshot: %s\n", state.ID)
		return nil
	},
}

var rbBeginCmd = &cobra.Command{
	Use:   "begin [description]",
	Short: "Begin a reversible operation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getRbMgr()
		if err != nil {
			return err
		}
		op, err := mgr.BeginOperation(rollback.OpCustom, args[0], "cli", nil)
		if err != nil {
			return err
		}
		fmt.Printf("Operation: %s\n", op.ID)
		return nil
	},
}

var rbCompleteCmd = &cobra.Command{
	Use:   "complete [operation-id]",
	Short: "Complete an operation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getRbMgr()
		if err != nil {
			return err
		}
		postState, _ := mgr.SaveState("post-state", "cli", map[string]string{"source": "cli"})
		return mgr.CompleteOperation(args[0], postState)
	},
}

var rbUndoCmd = &cobra.Command{
	Use:   "undo [operation-id]",
	Short: "Rollback an operation",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getRbMgr()
		if err != nil {
			return err
		}
		if len(args) > 0 {
			state, err := mgr.Rollback(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Rolled back to: %s\n", state.ID)
		} else {
			state, err := mgr.RollbackLast("cli")
			if err != nil {
				return err
			}
			fmt.Printf("Rolled back to: %s\n", state.ID)
		}
		return nil
	},
}

var rbHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show operation history",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getRbMgr()
		if err != nil {
			return err
		}
		ops := mgr.ListOperations("", "")
		if len(ops) == 0 {
			fmt.Println("No operations recorded.")
			return nil
		}
		fmt.Printf("Operations (%d):\n", len(ops))
		for _, op := range ops {
			status := "active"
			if op.RolledBack {
				status = "rolled-back"
			}
			fmt.Printf("  %s [%s] %s %s\n", op.ID, op.Type, op.Description, status)
		}
		return nil
	},
}

var rbStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Rollback statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := getRbMgr()
		if err != nil {
			return err
		}
		stats := mgr.Stats()
		fmt.Printf("Operations: %d (active: %d, rolled back: %d)\n", stats.TotalOps, stats.Active, stats.RolledBack)
		fmt.Printf("States: %d\n", stats.TotalStates)
		return nil
	},
}
