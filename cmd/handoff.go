package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/agenthandoff"
	"github.com/spf13/cobra"
)

var handoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Agent handoff protocol",
	Long:  "Manage agent handoffs — transfer context, artifacts, and confidence between agents during handoff.",
}

var (
	handoffDir      string
	handoffFrom     string
	handoffTo       string
	handoffTask     string
	handoffStatus   string
	handoffProgress float64
)

func init() {
	handoffCmd.AddCommand(handoffListCmd)
	handoffCmd.AddCommand(handoffShowCmd)
	handoffCmd.AddCommand(handoffCreateCmd)
	handoffCmd.AddCommand(handoffAcceptCmd)
	handoffCmd.AddCommand(handoffRejectCmd)
	handoffCmd.AddCommand(handoffContextCmd)

	handoffCmd.PersistentFlags().StringVar(&handoffDir, "dir", ".forge/handoffs", "Handoff storage directory")
	handoffListCmd.Flags().StringVar(&handoffFrom, "from", "", "Filter by from agent")
	handoffListCmd.Flags().StringVar(&handoffTo, "to", "", "Filter by to agent")
	handoffListCmd.Flags().StringVar(&handoffStatus, "status", "", "Filter by status")
	handoffCreateCmd.Flags().StringVar(&handoffFrom, "from", "", "Source agent")
	handoffCreateCmd.Flags().StringVar(&handoffTo, "to", "", "Target agent")
	handoffCreateCmd.Flags().StringVar(&handoffTask, "task", "", "Task description")
	handoffCreateCmd.Flags().Float64Var(&handoffProgress, "progress", 0, "Progress (0-1)")
}

func getHandoffStore() (*agenthandoff.Store, error) {
	return agenthandoff.NewStore(handoffDir)
}

var handoffListCmd = &cobra.Command{
	Use:   "list",
	Short: "List handoffs",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHandoffStore()
		if err != nil {
			return err
		}

		handoffs := store.List(handoffFrom, handoffTo, agenthandoff.HandoffStatus(handoffStatus))
		if len(handoffs) == 0 {
			fmt.Println("No handoffs found.")
			return nil
		}

		fmt.Printf("Handoffs (%d):\n", len(handoffs))
		for _, h := range handoffs {
			fmt.Printf("  %s [%s] %s → %s: %s (%.0f%%)\n",
				h.ID, h.Status, h.FromAgent, h.ToAgent, h.Task, h.Progress*100)
		}
		return nil
	},
}

var handoffShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show handoff details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHandoffStore()
		if err != nil {
			return err
		}

		h, ok := store.Get(args[0])
		if !ok {
			return fmt.Errorf("handoff %q not found", args[0])
		}

		fmt.Printf("Handoff: %s\n", h.ID)
		fmt.Printf("From: %s → To: %s\n", h.FromAgent, h.ToAgent)
		fmt.Printf("Status: %s | Progress: %.0f%%\n", h.Status, h.Progress*100)
		fmt.Printf("Task: %s\n", h.Task)
		if h.Summary != "" {
			fmt.Printf("Summary: %s\n", h.Summary)
		}
		fmt.Printf("Confidence: %.0f%%\n", h.Confidence.Overall*100)

		if len(h.Artifacts) > 0 {
			fmt.Println("\nArtifacts:")
			for _, a := range h.Artifacts {
				fmt.Printf("  [%s] %s: %s\n", a.Type, a.Name, a.Summary)
			}
		}
		if len(h.PendingItems) > 0 {
			fmt.Println("\nPending:")
			for _, p := range h.PendingItems {
				fmt.Printf("  - %s\n", p)
			}
		}
		return nil
	},
}

var handoffCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a handoff request",
	RunE: func(cmd *cobra.Command, args []string) error {
		if handoffFrom == "" || handoffTo == "" || handoffTask == "" {
			return fmt.Errorf("--from, --to, and --task are required")
		}

		store, err := getHandoffStore()
		if err != nil {
			return err
		}

		req := agenthandoff.AutoGenerate(handoffFrom, handoffTo, handoffTask, handoffProgress)
		if err := store.Create(req); err != nil {
			return err
		}

		fmt.Printf("Created handoff: %s (%s → %s)\n", req.ID, req.FromAgent, req.ToAgent)
		return nil
	},
}

var handoffAcceptCmd = &cobra.Command{
	Use:   "accept [id]",
	Short: "Accept a handoff",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHandoffStore()
		if err != nil {
			return err
		}
		if err := store.Accept(args[0], "current-agent", nil); err != nil {
			return err
		}
		fmt.Printf("Accepted handoff: %s\n", args[0])
		return nil
	},
}

var handoffRejectCmd = &cobra.Command{
	Use:   "reject [id]",
	Short: "Reject a handoff",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHandoffStore()
		if err != nil {
			return err
		}
		if err := store.Reject(args[0], "current-agent", "rejected"); err != nil {
			return err
		}
		fmt.Printf("Rejected handoff: %s\n", args[0])
		return nil
	},
}

var handoffContextCmd = &cobra.Command{
	Use:   "context [id]",
	Short: "Generate formatted handoff context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getHandoffStore()
		if err != nil {
			return err
		}
		h, ok := store.Get(args[0])
		if !ok {
			return fmt.Errorf("handoff %q not found", args[0])
		}
		fmt.Println(agenthandoff.BuildContext(h))
		return nil
	},
}
