package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/forge/sword/internal/bridge"
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
  forge bridge rules
  forge bridge log
  forge bridge protocols`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		bridgeTranslateCmd(),
		bridgeRulesCmd(),
		bridgeLogCmd(),
		bridgeProtocolsCmd(),
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
