package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yethikrishna/the-forge/internal/tenant"
)

func tenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Manage multi-tenant workspaces",
		Long: `Manage tenants for forge serve with isolated agents, memory, cost tracking,
and resource quotas. Each tenant gets their own API keys and usage limits.

Examples:
  forge tenant create acme-corp --max-agents=10
  forge tenant list
  forge tenant get tn-abc123
  forge tenant key create tn-abc123 --name=ci-key --scopes=read,write
  forge tenant usage tn-abc123`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		tenantCreateCmd(),
		tenantListCmd(),
		tenantGetCmd(),
		tenantDeleteCmd(),
		tenantKeyCmd(),
		tenantUsageCmd(),
		tenantSuspendCmd(),
		tenantResumeCmd(),
		tenantUpdateCmd(),
	)

	return cmd
}

func getTenantStore() (*tenant.Store, error) {
	dir := filepath.Join(getForgeDir(), "tenants")
	return tenant.NewStore(dir)
}

func tenantCreateCmd() *cobra.Command {
	var displayName string
	var maxAgents int
	var maxConcurrent int
	var maxCostDay float64
	var maxCostMonth float64
	var maxRequests int
	var maxSessions int
	var dataResidency string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			quota := tenant.Quota{
				MaxAgents:       maxAgents,
				MaxConcurrent:   maxConcurrent,
				MaxCostPerDay:   maxCostDay,
				MaxCostPerMonth: maxCostMonth,
				MaxRequests:     maxRequests,
				MaxSessions:     maxSessions,
				DataResidency:   dataResidency,
			}

			t, err := store.Create(args[0], quota)
			if err != nil {
				return err
			}

			if displayName != "" {
				store.Update(t.ID, func(tt *tenant.Tenant) error {
					tt.DisplayName = displayName
					return nil
				})
			}

			// Create default API key
			key, err := store.CreateAPIKey(t.ID, "default", []string{"read", "write"})
			if err != nil {
				return fmt.Errorf("failed to create API key: %w", err)
			}

			fmt.Printf("Tenant created: %s (%s)\n", t.Name, t.ID)
			fmt.Printf("API Key: %s\n", key.Key)
			fmt.Println("\n⚠️  Save this API key — it won't be shown again.")
			fmt.Printf("\nSet it in requests:\n")
			fmt.Printf("  Authorization: Bearer %s\n", key.Key)
			fmt.Printf("  X-Forge-API-Key: %s\n", key.Key)
			return nil
		},
	}

	cmd.Flags().StringVar(&displayName, "display-name", "", "Human-readable name")
	cmd.Flags().IntVar(&maxAgents, "max-agents", 0, "Max agents (0=unlimited)")
	cmd.Flags().IntVar(&maxConcurrent, "max-concurrent", 0, "Max concurrent runs (0=unlimited)")
	cmd.Flags().Float64Var(&maxCostDay, "max-cost-day", 0, "Max cost per day in USD (0=unlimited)")
	cmd.Flags().Float64Var(&maxCostMonth, "max-cost-month", 0, "Max cost per month in USD (0=unlimited)")
	cmd.Flags().IntVar(&maxRequests, "max-requests", 0, "Max requests per minute (0=unlimited)")
	cmd.Flags().IntVar(&maxSessions, "max-sessions", 0, "Max sessions (0=unlimited)")
	cmd.Flags().StringVar(&dataResidency, "data-residency", "", "Data residency policy (us-only, eu-only)")

	return cmd
}

func tenantListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tenants",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			tenants := store.List()
			if jsonOutput {
				data, _ := json.MarshalIndent(tenants, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(tenants) == 0 {
				fmt.Println("No tenants found. Create one with: forge tenant create <name>")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tID\tSTATUS\tAGENTS\tCOST/DAY\tKEYS")
			for _, t := range tenants {
				agents := fmt.Sprintf("%d", t.Usage.ActiveAgents)
				if t.Quota.MaxAgents > 0 {
					agents = fmt.Sprintf("%d/%d", t.Usage.ActiveAgents, t.Quota.MaxAgents)
				}
				costDay := "unlimited"
				if t.Quota.MaxCostPerDay > 0 {
					costDay = fmt.Sprintf("$%.2f/$%.2f", t.Usage.CostToday, t.Quota.MaxCostPerDay)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
					t.Name, t.ID, t.Status, agents, costDay, len(t.APIKeys))
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func tenantGetCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get tenant details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			// Try as ID first, then as name
			t, err := store.Get(args[0])
			if err != nil {
				t, err = store.GetByName(args[0])
				if err != nil {
					return err
				}
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(t, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Print(tenant.FormatTenant(t))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func tenantDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Archive a tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			if err := store.Delete(args[0]); err != nil {
				return err
			}

			fmt.Printf("Tenant %s archived.\n", args[0])
			return nil
		},
	}

	return cmd
}

func tenantUpdateCmd() *cobra.Command {
	var displayName string
	var maxAgents int
	var maxConcurrent int
	var maxCostDay float64
	var maxCostMonth float64
	var maxRequests int
	var dataResidency string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update tenant configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			err = store.Update(args[0], func(t *tenant.Tenant) error {
				if displayName != "" {
					t.DisplayName = displayName
				}
				if cmd.Flags().Changed("max-agents") {
					t.Quota.MaxAgents = maxAgents
				}
				if cmd.Flags().Changed("max-concurrent") {
					t.Quota.MaxConcurrent = maxConcurrent
				}
				if cmd.Flags().Changed("max-cost-day") {
					t.Quota.MaxCostPerDay = maxCostDay
				}
				if cmd.Flags().Changed("max-cost-month") {
					t.Quota.MaxCostPerMonth = maxCostMonth
				}
				if cmd.Flags().Changed("max-requests") {
					t.Quota.MaxRequests = maxRequests
				}
				if cmd.Flags().Changed("data-residency") {
					t.Quota.DataResidency = dataResidency
				}
				return nil
			})
			if err != nil {
				return err
			}

			t, _ := store.Get(args[0])
			fmt.Printf("Tenant %s updated.\n", t.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&displayName, "display-name", "", "Human-readable name")
	cmd.Flags().IntVar(&maxAgents, "max-agents", 0, "Max agents (0=unlimited)")
	cmd.Flags().IntVar(&maxConcurrent, "max-concurrent", 0, "Max concurrent runs (0=unlimited)")
	cmd.Flags().Float64Var(&maxCostDay, "max-cost-day", 0, "Max cost per day in USD (0=unlimited)")
	cmd.Flags().Float64Var(&maxCostMonth, "max-cost-month", 0, "Max cost per month in USD (0=unlimited)")
	cmd.Flags().IntVar(&maxRequests, "max-requests", 0, "Max requests per minute (0=unlimited)")
	cmd.Flags().StringVar(&dataResidency, "data-residency", "", "Data residency policy")

	return cmd
}

func tenantKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage tenant API keys",
	}

	cmd.AddCommand(
		tenantKeyCreateCmd(),
		tenantKeyListCmd(),
		tenantKeyRevokeCmd(),
	)

	return cmd
}

func tenantKeyCreateCmd() *cobra.Command {
	var scopes []string

	cmd := &cobra.Command{
		Use:   "create <tenant-id>",
		Short: "Create a new API key for a tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			name := "generated"
			key, err := store.CreateAPIKey(args[0], name, scopes)
			if err != nil {
				return err
			}

			fmt.Printf("API Key created: %s\n", key.Key)
			fmt.Printf("Key ID: %s\n", key.ID)
			fmt.Printf("Scopes: %v\n", key.Scopes)
			fmt.Println("\n⚠️  Save this API key — it won't be shown again.")
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&scopes, "scopes", []string{"read", "write"}, "Key scopes (read, write, admin)")

	return cmd
}

func tenantKeyListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <tenant-id>",
		Short: "List API keys for a tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			t, err := store.Get(args[0])
			if err != nil {
				t, err = store.GetByName(args[0])
				if err != nil {
					return err
				}
			}

			if len(t.APIKeys) == 0 {
				fmt.Println("No API keys found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSCOPES\tCREATED\tLAST USED")
			for _, k := range t.APIKeys {
				lastUsed := "never"
				if k.LastUsed != nil {
					lastUsed = k.LastUsed.Format("2006-01-02 15:04")
				}
				fmt.Fprintf(w, "%s\t%s\t%v\t%s\t%s\n",
					k.ID, k.Name, k.Scopes, k.CreatedAt.Format("2006-01-02"), lastUsed)
			}
			w.Flush()
			return nil
		},
	}

	return cmd
}

func tenantKeyRevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <tenant-id> <key-id>",
		Short: "Revoke an API key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			if err := store.RevokeAPIKey(args[0], args[1]); err != nil {
				return err
			}

			fmt.Printf("API key %s revoked.\n", args[1])
			return nil
		},
	}

	return cmd
}

func tenantUsageCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "usage <id>",
		Short: "Show tenant resource usage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			usage, err := store.GetUsage(args[0])
			if err != nil {
				return err
			}

			t, _ := store.Get(args[0])

			if jsonOutput {
				data, _ := json.MarshalIndent(map[string]interface{}{
					"tenant_id": args[0],
					"usage":     usage,
					"quota":     t.Quota,
				}, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Usage for %s (%s):\n\n", t.Name, t.ID)
			fmt.Printf("  Active Agents:    %d", usage.ActiveAgents)
			if t.Quota.MaxAgents > 0 {
				fmt.Printf(" / %d", t.Quota.MaxAgents)
			}
			fmt.Println()

			fmt.Printf("  Active Sessions:  %d", usage.ActiveSessions)
			if t.Quota.MaxSessions > 0 {
				fmt.Printf(" / %d", t.Quota.MaxSessions)
			}
			fmt.Println()

			fmt.Printf("  Concurrent Runs:  %d", usage.ConcurrentRuns)
			if t.Quota.MaxConcurrent > 0 {
				fmt.Printf(" / %d", t.Quota.MaxConcurrent)
			}
			fmt.Println()

			fmt.Printf("  Cost Today:       $%.4f", usage.CostToday)
			if t.Quota.MaxCostPerDay > 0 {
				fmt.Printf(" / $%.2f", t.Quota.MaxCostPerDay)
			}
			fmt.Println()

			fmt.Printf("  Cost This Month:  $%.4f", usage.CostThisMonth)
			if t.Quota.MaxCostPerMonth > 0 {
				fmt.Printf(" / $%.2f", t.Quota.MaxCostPerMonth)
			}
			fmt.Println()

			fmt.Printf("  Total Cost:       $%.4f\n", usage.TotalCost)
			fmt.Printf("  Total Requests:   %d\n", usage.TotalRequests)

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func tenantSuspendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suspend <id>",
		Short: "Suspend a tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			err = store.Update(args[0], func(t *tenant.Tenant) error {
				t.Status = "suspended"
				return nil
			})
			if err != nil {
				return err
			}

			fmt.Printf("Tenant %s suspended.\n", args[0])
			return nil
		},
	}

	return cmd
}

func tenantResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume <id>",
		Short: "Resume a suspended tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getTenantStore()
			if err != nil {
				return err
			}

			err = store.Update(args[0], func(t *tenant.Tenant) error {
				t.Status = "active"
				return nil
			})
			if err != nil {
				return err
			}

			fmt.Printf("Tenant %s resumed.\n", args[0])
			return nil
		},
	}

	return cmd
}
