package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/forge/sword/internal/bridge"
	"github.com/forge/sword/internal/auth/identity"
	"github.com/spf13/cobra"
)

func bridgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "Universal protocol bridge (MCP ↔ A2A ↔ ACP)",
		Long: `Translate messages between agent protocols.

Forge speaks every protocol. Bridge them together so agents
using MCP, A2A, or ACP can all communicate seamlessly.

Examples:
  forge bridge translate --from=mcp --to=a2a --method=tools/list
  forge bridge serve --addr :9090
  forge bridge discover
  forge bridge status
  forge bridge rules
  forge bridge identity list
  forge bridge trust grant <fingerprint> --level=trusted`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		bridgeTranslateCmd(),
		bridgeRulesCmd(),
		bridgeLogCmd(),
		bridgeProtocolsCmd(),
		bridgeServeCmd(),
		bridgeDiscoverCmd(),
		bridgeStatusCmd(),
		bridgeIdentityCmd(),
		bridgeTrustCmd(),
	)

	return cmd
}

func getBridge() (*bridge.Bridge, error) {
	return bridge.NewBridge(getForgeDir() + "/bridge")
}

func bridgeTranslateCmd() *cobra.Command {
	var from, to, method, msgType, paramsJSON string

	cmd := &cobra.Command{
		Use:   "translate",
		Short: "Translate a message between protocols",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := getBridge()
			if err != nil {
				return err
			}

			if from == "" || to == "" || method == "" {
				return fmt.Errorf("--from, --to, and --method are required")
			}

			if msgType == "" {
				msgType = "request"
			}

			var params map[string]interface{}
			if paramsJSON != "" {
				if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
					return fmt.Errorf("invalid --params JSON: %w", err)
				}
			} else {
				params = make(map[string]interface{})
			}

			msg := &bridge.Message{
				Source:   bridge.Protocol(from),
				Target:   bridge.Protocol(to),
				Type:     msgType,
				Method:   method,
				Params:   params,
				Metadata: map[string]string{},
			}

			translated, err := b.Translate(msg)
			if err != nil {
				return err
			}

			fmt.Print(bridge.FormatMessage(translated))
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Source protocol (mcp, a2a, acp)")
	cmd.Flags().StringVar(&to, "to", "", "Target protocol (mcp, a2a, acp)")
	cmd.Flags().StringVar(&method, "method", "", "Method to translate")
	cmd.Flags().StringVar(&msgType, "type", "request", "Message type (request, response, notification)")
	cmd.Flags().StringVar(&paramsJSON, "params", "", "Parameters as JSON")

	return cmd
}

func bridgeRulesCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "rules",
		Short: "List conversion rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := getBridge()
			if err != nil {
				return err
			}

			rules := b.ListRules()

			if jsonOutput {
				data, _ := json.MarshalIndent(rules, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(rules) == 0 {
				fmt.Println("No conversion rules defined.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SOURCE\tTARGET\tTYPE\tMAPPING\tDESCRIPTION")
			for _, r := range rules {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Source, r.Target, r.SourceType, r.MethodMap, r.Description)
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func bridgeLogCmd() *cobra.Command {
	var limit int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show translation log",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := getBridge()
			if err != nil {
				return err
			}

			entries := b.GetLog(limit)

			if jsonOutput {
				data, _ := json.MarshalIndent(entries, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(entries) == 0 {
				fmt.Println("No translations logged yet.")
				return nil
			}

			for _, m := range entries {
				fmt.Print(bridge.FormatMessage(m))
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Number of log entries")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func bridgeProtocolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protocols",
		Short: "List supported protocols",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, p := range bridge.AllProtocols() {
				fmt.Printf("  %s\n", p)
			}
			return nil
		},
	}
	return cmd
}

func bridgeServeCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the bridge server with protocol adapters",
		Long: `Start the bridge server that routes messages between protocol adapters.

The bridge server exposes an HTTP API for submitting messages and
inspecting status. It connects to MCP, A2A, and ACP endpoints
and translates messages between them.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			router, err := bridge.NewRouter(getForgeDir() + "/bridge")
			if err != nil {
				return err
			}

			fmt.Printf("Forge Bridge starting on %s\n", addr)
			fmt.Println("  Protocols: MCP, A2A, ACP")
			fmt.Println("  Endpoints:")
			fmt.Printf("    GET  /bridge/status   — Bridge status\n")
			fmt.Printf("    POST /bridge/route    — Route a message\n")
			fmt.Printf("    GET  /bridge/adapters — List adapters\n")
			fmt.Printf("    GET  /bridge/routes   — List routes\n")

			return router.ServeHTTP(addr)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":9090", "Listen address")
	return cmd
}

func bridgeDiscoverCmd() *cobra.Command {
	var jsonOutput bool
	var scanNetwork bool

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover protocol endpoints on the network",
		Long: `Scan for MCP, A2A, and ACP servers on localhost and in config files.

Checks common MCP server config locations (Claude Desktop, Cursor)
and probes known ports for running protocol servers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := bridge.DefaultDiscoveryConfig()
			cfg.ScanNetwork = scanNetwork
			cfg.ConfigDir = getForgeDir() + "/bridge"

			d := bridge.NewDiscoverer(cfg)
			endpoints, err := d.Scan(cmd.Context())
			if err != nil {
				return err
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(endpoints, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(endpoints) == 0 {
				fmt.Println("No protocol endpoints discovered.")
				return nil
			}

			fmt.Printf("Discovered %d endpoint(s):\n\n", len(endpoints))
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPROTOCOL\tADDRESS\tHEALTH\tSOURCE")
			for _, ep := range endpoints {
				health := "unhealthy"
				if ep.Healthy {
					health = "healthy"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ep.Name, ep.Protocol, ep.Address, health, ep.Source)
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&scanNetwork, "network", false, "Scan local network (not just localhost)")
	return cmd
}

func bridgeStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show bridge status and connected adapters",
		RunE: func(cmd *cobra.Command, args []string) error {
			router, err := bridge.NewRouter(getForgeDir() + "/bridge")
			if err != nil {
				return err
			}

			stats := router.Stats()
			adapters := router.ListAdapters()
			routes := router.ListRoutes()

			if jsonOutput {
				data, _ := json.MarshalIndent(map[string]interface{}{
					"stats":    stats,
					"adapters": adapters,
					"routes":   routes,
				}, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println("Forge Bridge Status")
			fmt.Println("===================")
			fmt.Printf("  Routed: %d  Errors: %d  Since: %s\n",
				stats.TotalRouted, stats.TotalErrors,
				stats.StartedAt.Format(time.RFC3339))
			if !stats.LastRoutedAt.IsZero() {
				fmt.Printf("  Last routed: %s\n", stats.LastRoutedAt.Format(time.RFC3339))
			}

			fmt.Printf("\nAdapters (%d):\n", len(adapters))
			for _, a := range adapters {
				fmt.Printf("  %s\n", bridge.FormatAdapterStatus(a))
			}

			fmt.Printf("\nRoutes (%d):\n", len(routes))
			for _, r := range routes {
				enabled := "enabled"
				if !r.Enabled {
					enabled = "disabled"
				}
				fmt.Printf("  %-20s %s→%s  adapter:%s  %s\n", r.Name, r.Source, r.Target, r.Adapter, enabled)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func bridgeIdentityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Manage agent cryptographic identities",
		Long: `Create and manage Ed25519 identities for agents.

Identities are used to sign manifests and verify agent authenticity.
Each identity has a public/private key pair and a fingerprint.`,
	}

	cmd.AddCommand(
		identityGenerateCmd(),
		identityListCmd(),
		identityShowCmd(),
		identityDeleteCmd(),
		identitySignCmd(),
		identityVerifyCmd(),
	)

	return cmd
}

func getKeyStore() (*identity.KeyStore, error) {
	return identity.NewKeyStore(getForgeDir() + "/identity")
}

func identityGenerateCmd() *cobra.Command {
	var labels []string

	cmd := &cobra.Command{
		Use:   "generate <name>",
		Short: "Generate a new agent identity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ks, err := getKeyStore()
			if err != nil {
				return err
			}

			labelMap := make(map[string]string)
			for _, l := range labels {
				parts := splitKV(l)
				if len(parts) == 2 {
					labelMap[parts[0]] = parts[1]
				}
			}

			id, err := ks.Generate(args[0], labelMap)
			if err != nil {
				return err
			}

			fmt.Printf("Generated identity: %s\n", id.Name)
			fmt.Printf("  Fingerprint: %s\n", id.Fingerprint)
			fmt.Printf("  Algorithm:   %s\n", id.Algorithm)
			fmt.Printf("  Created:     %s\n", id.CreatedAt.Format(time.RFC3339))
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&labels, "label", nil, "Labels (key=value)")
	return cmd
}

func identityListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all identities",
		RunE: func(cmd *cobra.Command, args []string) error {
			ks, err := getKeyStore()
			if err != nil {
				return err
			}

			ids := ks.List()

			if jsonOutput {
				data, _ := json.MarshalIndent(ids, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(ids) == 0 {
				fmt.Println("No identities. Use 'forge bridge identity generate <name>' to create one.")
				return nil
			}

			for _, id := range ids {
				fmt.Printf("  %s\n", identity.FormatIdentity(id))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func identityShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <fingerprint>",
		Short: "Show identity details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ks, err := getKeyStore()
			if err != nil {
				return err
			}

			id, ok := ks.Get(args[0])
			if !ok {
				return fmt.Errorf("identity not found: %s", args[0])
			}

			fmt.Printf("Name:        %s\n", id.Name)
			fmt.Printf("Fingerprint: %s\n", id.Fingerprint)
			fmt.Printf("Algorithm:   %s\n", id.Algorithm)
			fmt.Printf("Public Key:  %s\n", id.PublicKey)
			fmt.Printf("Created:     %s\n", id.CreatedAt.Format(time.RFC3339))
			if len(id.Labels) > 0 {
				fmt.Println("Labels:")
				for k, v := range id.Labels {
					fmt.Printf("  %s: %s\n", k, v)
				}
			}
			return nil
		},
	}
	return cmd
}

func identityDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <fingerprint>",
		Short: "Delete an identity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ks, err := getKeyStore()
			if err != nil {
				return err
			}
			return ks.Delete(args[0])
		},
	}
	return cmd
}

func identitySignCmd() *cobra.Command {
	var agentName, version, description string
	var capabilities, protocols, tools, permissions []string

	cmd := &cobra.Command{
		Use:   "sign <fingerprint>",
		Short: "Sign an agent manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ks, err := getKeyStore()
			if err != nil {
				return err
			}

			manifest := identity.Manifest{
				AgentName:    agentName,
				Version:      version,
				Description:  description,
				Capabilities: capabilities,
				Protocols:    protocols,
				Tools:        tools,
				Permissions:  permissions,
			}

			signed, err := ks.Sign(args[0], manifest)
			if err != nil {
				return err
			}

			data, _ := json.MarshalIndent(signed, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&agentName, "name", "", "Agent name")
	cmd.Flags().StringVar(&version, "version", "1.0.0", "Agent version")
	cmd.Flags().StringVar(&description, "desc", "", "Description")
	cmd.Flags().StringArrayVar(&capabilities, "capability", nil, "Agent capabilities")
	cmd.Flags().StringArrayVar(&protocols, "protocol", nil, "Supported protocols")
	cmd.Flags().StringArrayVar(&tools, "tool", nil, "Agent tools")
	cmd.Flags().StringArrayVar(&permissions, "permission", nil, "Required permissions")

	return cmd
}

func identityVerifyCmd() *cobra.Command {
	var manifestFile string

	cmd := &cobra.Command{
		Use:   "verify <fingerprint>",
		Short: "Verify a signed manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ks, err := getKeyStore()
			if err != nil {
				return err
			}

			if manifestFile == "" {
				return fmt.Errorf("--file is required")
			}

			data, err := os.ReadFile(manifestFile)
			if err != nil {
				return err
			}

			var sm identity.SignedManifest
			if err := json.Unmarshal(data, &sm); err != nil {
				return fmt.Errorf("invalid manifest: %w", err)
			}

			if err := ks.Verify(&sm); err != nil {
				fmt.Printf("VERIFICATION FAILED: %v\n", err)
				return nil
			}

			fmt.Println("VERIFIED — manifest signature is valid")
			fmt.Printf("  Signer:  %s\n", sm.SignerID)
			fmt.Printf("  Signed:  %s\n", sm.SignedAt.Format(time.RFC3339))
			fmt.Printf("  Agent:   %s v%s\n", sm.Manifest.AgentName, sm.Manifest.Version)
			return nil
		},
	}

	cmd.Flags().StringVar(&manifestFile, "file", "", "Path to signed manifest JSON")
	return cmd
}

func bridgeTrustCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage agent trust registry",
		Long: `Manage trust levels for agent identities.

Trust levels: unknown, untrusted, limited, trusted, verified

Agents must have at least "limited" trust to route messages through the bridge.`,
	}

	cmd.AddCommand(
		trustGrantCmd(),
		trustRevokeCmd(),
		trustListCmd(),
		trustCheckCmd(),
	)

	return cmd
}

func getTrustRegistry() (*identity.TrustRegistry, error) {
	return identity.NewTrustRegistry(getForgeDir() + "/trust")
}

func trustGrantCmd() *cobra.Command {
	var level, reason, grantedBy string
	var expires string

	cmd := &cobra.Command{
		Use:   "grant <fingerprint> <name>",
		Short: "Grant trust to an agent identity",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tr, err := getTrustRegistry()
			if err != nil {
				return err
			}

			if level == "" {
				level = "limited"
			}

			var expiresAt *time.Time
			if expires != "" {
				t, err := time.Parse("2006-01-02", expires)
				if err != nil {
					return fmt.Errorf("invalid expires date: %w", err)
				}
				expiresAt = &t
			}

			if grantedBy == "" {
				grantedBy = "cli"
			}

			if err := tr.Grant(args[0], args[1], identity.TrustLevel(level), grantedBy, reason, expiresAt); err != nil {
				return err
			}

			fmt.Printf("Granted %s trust to %s (%s)\n", level, args[1], args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&level, "level", "limited", "Trust level (unknown, untrusted, limited, trusted, verified)")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for granting trust")
	cmd.Flags().StringVar(&grantedBy, "by", "", "Identity granting trust (default: cli)")
	cmd.Flags().StringVar(&expires, "expires", "", "Expiration date (YYYY-MM-DD)")

	return cmd
}

func trustRevokeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <fingerprint>",
		Short: "Revoke trust from an agent identity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tr, err := getTrustRegistry()
			if err != nil {
				return err
			}
			return tr.Revoke(args[0])
		},
	}
	return cmd
}

func trustListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List trust entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			tr, err := getTrustRegistry()
			if err != nil {
				return err
			}

			entries := tr.List()

			if jsonOutput {
				data, _ := json.MarshalIndent(entries, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(entries) == 0 {
				fmt.Println("No trust entries. Use 'forge bridge trust grant <fp> <name>' to add one.")
				return nil
			}

			for _, e := range entries {
				fmt.Printf("  %s\n", identity.FormatTrustEntry(e))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func trustCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <fingerprint>",
		Short: "Check trust level for an identity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tr, err := getTrustRegistry()
			if err != nil {
				return err
			}

			level := tr.Check(args[0])
			trusted := tr.IsTrusted(args[0])
			fmt.Printf("Trust level: %s\n", level)
			fmt.Printf("Is trusted:  %v\n", trusted)
			return nil
		},
	}
	return cmd
}

// splitKV splits a "key=value" string.
func splitKV(s string) []string {
	for i := range s {
		if s[i] == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
