package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/forge/sword/internal/mcp2/discover"
	"github.com/spf13/cobra"
)

func mcpDiscoverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Auto-discover MCP servers on the network",
		Long: `Scan for MCP servers in config files, running processes,
and local network ports. Find every server automatically.

Examples:
  forge mcp discover
  forge mcp discover --json
  forge mcp discover --source=config
  forge mcp discover --source=network`,
		SilenceUsage: true,
	}

	var jsonOutput bool
	var source string

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source (config, network, process)")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		d := discover.NewDiscoverer()
		result := d.Discover()

		// Filter by source if specified
		if source != "" {
			filtered := make([]*discover.DiscoveredServer, 0)
			for _, s := range result.Servers {
				if s.Source == source {
					filtered = append(filtered, s)
				}
			}
			result.Servers = filtered
		}

		if jsonOutput {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		if len(result.Servers) == 0 {
			fmt.Println("No MCP servers discovered.")
			fmt.Println("\nChecked:")
			fmt.Println("  - Config files (~/.config/mcp, ~/.mcp)")
			fmt.Println("  - Running processes")
			fmt.Println("  - Local ports (3000, 8080, 8765, 9090)")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTRANSPORT\tSTATUS\tSOURCE\tDETAIL")
		for _, s := range result.Servers {
			detail := s.Address
			if detail == "" {
				detail = s.Command
			}
			if len(detail) > 40 {
				detail = detail[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Transport, s.Status, s.Source, detail)
		}
		w.Flush()

		fmt.Printf("\nDiscovered %d server(s) in %s\n", len(result.Servers), result.Duration.Round(1000000))
		return nil
	}

	return cmd
}
