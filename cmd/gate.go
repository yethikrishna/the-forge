package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/featuregate"
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Feature gate management",
	Long:  "Manage feature gates with gradual rollout, targeting rules, and kill switches for safe feature deployment.",
}

var (
	gateDir       string
	gateName      string
	gateDesc      string
	gateOwner     string
	gatePct       float64
	gateUserIDs   []string
	gateTags      []string
	gateUserID    string
	gateAgent     string
)

func init() {
	gateCmd.AddCommand(gateListCmd)
	gateCmd.AddCommand(gateShowCmd)
	gateCmd.AddCommand(gateCreateCmd)
	gateCmd.AddCommand(gateEnableCmd)
	gateCmd.AddCommand(gateDisableCmd)
	gateCmd.AddCommand(gateKillCmd)
	gateCmd.AddCommand(gateUnkillCmd)
	gateCmd.AddCommand(gateRolloutCmd)
	gateCmd.AddCommand(gateCheckCmd)

	gateCmd.PersistentFlags().StringVar(&gateDir, "dir", ".forge/gates", "Gate storage directory")
	gateCreateCmd.Flags().StringVar(&gateName, "name", "", "Gate name")
	gateCreateCmd.Flags().StringVar(&gateDesc, "desc", "", "Description")
	gateCreateCmd.Flags().StringVar(&gateOwner, "owner", "", "Owner")
	gateRolloutCmd.Flags().Float64Var(&gatePct, "pct", 0, "Rollout percentage (0-100)")
	gateCheckCmd.Flags().StringVar(&gateUserID, "user", "", "User ID to check")
	gateCheckCmd.Flags().StringVar(&gateAgent, "agent", "", "Agent to check")
	gateCheckCmd.Flags().StringArrayVar(&gateTags, "tag", nil, "Tags for check context")
}

func getGateStore() (*featuregate.Store, error) {
	return featuregate.NewStore(gateDir)
}

var gateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List feature gates",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getGateStore()
		if err != nil {
			return err
		}
		gates := store.List("")
		if len(gates) == 0 {
			fmt.Println("No feature gates found.")
			return nil
		}
		fmt.Printf("Feature Gates (%d):\n", len(gates))
		for _, g := range gates {
			fmt.Printf("  %s [%s] %s — rollout: %.0f%%\n", g.ID, g.Status, g.Name, g.RolloutPct)
			if g.KillSwitch {
				fmt.Printf("    ⚠️ KILL SWITCH ACTIVE\n")
			}
		}
		return nil
	},
}

var gateShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show gate details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getGateStore()
		if err != nil {
			return err
		}
		g, ok := store.Get(args[0])
		if !ok {
			return fmt.Errorf("gate %q not found", args[0])
		}
		fmt.Printf("Gate: %s\n", g.Name)
		fmt.Printf("ID: %s\n", g.ID)
		fmt.Printf("Status: %s\n", g.Status)
		fmt.Printf("Rollout: %.0f%%\n", g.RolloutPct)
		fmt.Printf("Owner: %s\n", g.Owner)
		fmt.Printf("Description: %s\n", g.Description)
		if g.KillSwitch {
			fmt.Println("⚠️ KILL SWITCH ACTIVE")
		}
		return nil
	},
}

var gateCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a feature gate",
	RunE: func(cmd *cobra.Command, args []string) error {
		if gateName == "" {
			return fmt.Errorf("--name is required")
		}
		store, err := getGateStore()
		if err != nil {
			return err
		}
		g := &featuregate.Gate{
			Name:        gateName,
			Description: gateDesc,
			Owner:       gateOwner,
		}
		if err := store.Create(g); err != nil {
			return err
		}
		fmt.Printf("Created gate: %s (id: %s)\n", g.Name, g.ID)
		return nil
	},
}

var gateEnableCmd = &cobra.Command{
	Use:   "enable [id]",
	Short: "Enable a feature gate (100%% rollout)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getGateStore()
		if err != nil {
			return err
		}
		return store.Enable(args[0])
	},
}

var gateDisableCmd = &cobra.Command{
	Use:   "disable [id]",
	Short: "Disable a feature gate",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getGateStore()
		if err != nil {
			return err
		}
		return store.Disable(args[0])
	},
}

var gateKillCmd = &cobra.Command{
	Use:   "kill [id]",
	Short: "Activate kill switch (emergency disable)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getGateStore()
		if err != nil {
			return err
		}
		if err := store.Kill(args[0]); err != nil {
			return err
		}
		fmt.Printf("⚠️ Kill switch activated for gate %s\n", args[0])
		return nil
	},
}

var gateUnkillCmd = &cobra.Command{
	Use:   "unkill [id]",
	Short: "Deactivate kill switch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getGateStore()
		if err != nil {
			return err
		}
		return store.Unkill(args[0])
	},
}

var gateRolloutCmd = &cobra.Command{
	Use:   "rollout [id]",
	Short: "Set rollout percentage for gradual deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getGateStore()
		if err != nil {
			return err
		}
		if err := store.Rollout(args[0], gatePct); err != nil {
			return err
		}
		fmt.Printf("Rollout set to %.0f%% for gate %s\n", gatePct, args[0])
		return nil
	},
}

var gateCheckCmd = &cobra.Command{
	Use:   "check [id]",
	Short: "Check if a gate is open for a user/agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getGateStore()
		if err != nil {
			return err
		}
		result := store.Check(args[0], featuregate.EvaluationContext{
			UserID: gateUserID,
			Agent:  gateAgent,
			Tags:   gateTags,
		})
		if result.Allowed {
			fmt.Printf("✅ Gate %s is OPEN for %s\n", args[0], gateUserID)
		} else {
			fmt.Printf("❌ Gate %s is CLOSED for %s: %s\n", args[0], gateUserID, result.Reason)
		}
		return nil
	},
}
