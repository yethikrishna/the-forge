package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/auth"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage API keys and authentication",
		Long: `Manage API keys for LLM providers, agents, and admin access.
Keys are stored securely in ~/.forge/keys/.

Examples:
  forge auth add openai --provider openai
  forge auth add anthropic --provider anthropic
  forge auth list
  forge auth delete openai
  forge auth generate`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "add [name]",
			Short: "Add a new API key",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				keyValue, _ := cmd.Flags().GetString("key")
				provider, _ := cmd.Flags().GetString("provider")
				keyType, _ := cmd.Flags().GetString("type")

				if keyValue == "" {
					keyValue = auth.GenerateKey("forge_")
				}

				ks := auth.NewKeyStore("")
				key, err := ks.Store(name, auth.KeyType(keyType), provider, keyValue)
				if err != nil {
					return err
				}

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Key %q added (%s)", name, key.Type)))
				fmt.Printf("   ID:      %s\n", key.ID)
				fmt.Printf("   Prefix:  %s\n", key.Prefix)
				if keyValue[:6] == "forge_" {
					fmt.Printf("   Key:     %s\n", keyValue)
					fmt.Println("   ⚠️  Save this key — it won't be shown again")
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List stored API keys",
			RunE: func(cmd *cobra.Command, args []string) error {
				ks := auth.NewKeyStore("")
				keys, err := ks.List()
				if err != nil {
					return err
				}

				if len(keys) == 0 {
					fmt.Println("Forge: No API keys stored")
					fmt.Println("  Use 'forge auth add <name> --key <value>' to add a key")
					return nil
				}

				fmt.Println(pretty.HeaderLine("API Keys"))
				fmt.Printf("  %-15s %-10s %-15s %-12s %s\n", "Name", "Type", "Provider", "Prefix", "Created")
				for _, k := range keys {
					fmt.Printf("  %-15s %-10s %-15s %-12s %s\n",
						k.Name, k.Type, k.Provider, k.Prefix,
						k.CreatedAt.Format("2006-01-02"))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "delete [name]",
			Short: "Delete a stored API key",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name := args[0]
				ks := auth.NewKeyStore("")
				if err := ks.Delete(name); err != nil {
					return err
				}
				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Key %q deleted", name)))
				return nil
			},
		},
		&cobra.Command{
			Use:   "generate",
			Short: "Generate a new random API key",
			RunE: func(cmd *cobra.Command, args []string) error {
				prefix, _ := cmd.Flags().GetString("prefix")
				key := auth.GenerateKey(prefix)
				fmt.Println(key)
				return nil
			},
		},
	)

	// add flags
	cmd.Commands()[0].Flags().String("key", "", "API key value (will prompt if not provided)")
	cmd.Commands()[0].Flags().String("provider", "", "Provider name (openai, anthropic, google, etc.)")
	cmd.Commands()[0].Flags().String("type", "provider", "Key type (provider|agent|admin)")
	cmd.Commands()[3].Flags().String("prefix", "forge_", "Key prefix")

	return cmd
}
