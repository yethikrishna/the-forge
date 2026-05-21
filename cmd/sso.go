package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forge/sword/internal/auth/sso"
	"github.com/spf13/cobra"
)

func ssoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sso",
		Short: "Single Sign-On management (OIDC, SAML, API keys)",
		Long:  `Manage SSO providers, sessions, and API keys. OIDC and SAML for enterprise, API keys for automation.`,
	}

	var outputJSON bool
	var storeDir string
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().StringVar(&storeDir, "dir", ".forge/sso", "SSO storage directory")

	// providers
	providersCmd := &cobra.Command{
		Use:   "providers",
		Short: "List SSO providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := sso.NewSSOManager(storeDir)
			providers := m.ListProviders()

			if outputJSON {
				data, _ := json.MarshalIndent(providers, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(providers) == 0 {
				fmt.Println("No SSO providers configured.")
				return nil
			}

			for _, p := range providers {
				fmt.Printf("%-20s %s\n", p.Name, p.Type)
			}
			return nil
		},
	}

	// register-oidc
	registerOIDCCmd := &cobra.Command{
		Use:   "register-oidc <name>",
		Short: "Register an OIDC provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := sso.NewSSOManager(storeDir)

			issuer, _ := cmd.Flags().GetString("issuer")
			clientID, _ := cmd.Flags().GetString("client-id")
			clientSecret, _ := cmd.Flags().GetString("client-secret")
			redirectURL, _ := cmd.Flags().GetString("redirect-url")

			config := sso.OIDCConfig{
				Issuer:       issuer,
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
			}

			return m.RegisterOIDC(args[0], config)
		},
	}
	registerOIDCCmd.Flags().String("issuer", "", "OIDC issuer URL")
	registerOIDCCmd.Flags().String("client-id", "", "Client ID")
	registerOIDCCmd.Flags().String("client-secret", "", "Client secret")
	registerOIDCCmd.Flags().String("redirect-url", "", "Redirect URL")

	// register-saml
	registerSAMLSetCmd := &cobra.Command{
		Use:   "register-saml <name>",
		Short: "Register a SAML provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := sso.NewSSOManager(storeDir)

			entityID, _ := cmd.Flags().GetString("entity-id")
			ssoURL, _ := cmd.Flags().GetString("sso-url")
			cert, _ := cmd.Flags().GetString("certificate")

			config := sso.SAMLConfig{
				EntityID:    entityID,
				SSOURL:      ssoURL,
				Certificate: cert,
			}

			return m.RegisterSAML(args[0], config)
		},
	}
	registerSAMLSetCmd.Flags().String("entity-id", "", "SAML Entity ID")
	registerSAMLSetCmd.Flags().String("sso-url", "", "SSO URL")
	registerSAMLSetCmd.Flags().String("certificate", "", "X.509 Certificate")

	// create-api-key
	createKeyCmd := &cobra.Command{
		Use:   "create-api-key <name>",
		Short: "Create an API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := sso.NewSSOManager(storeDir)

			userID, _ := cmd.Flags().GetString("user")
			scopes, _ := cmd.Flags().GetStringSlice("scopes")
			ttl, _ := cmd.Flags().GetString("ttl")

			var expiresAt *time.Time
			if ttl != "" {
				d, err := time.ParseDuration(ttl)
				if err != nil {
					return fmt.Errorf("invalid TTL: %v", err)
				}
				t := time.Now().UTC().Add(d)
				expiresAt = &t
			}

			key, err := m.CreateAPIKey(args[0], userID, scopes, expiresAt)
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(key, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("API Key created: %s\n", key.Key)
			fmt.Printf("  ID:     %s\n", key.ID)
			fmt.Printf("  Prefix: %s...\n", key.Prefix)
			fmt.Println("\n⚠️  Save this key — it won't be shown again!")
			return nil
		},
	}
	createKeyCmd.Flags().String("user", "", "User ID for the key")
	createKeyCmd.Flags().StringSlice("scopes", []string{"read"}, "Key scopes")
	createKeyCmd.Flags().String("ttl", "", "Key TTL (e.g., 24h, 30d)")

	// list-api-keys
	listKeysCmd := &cobra.Command{
		Use:   "list-api-keys",
		Short: "List API keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := sso.NewSSOManager(storeDir)
			userID, _ := cmd.Flags().GetString("user")
			keys := m.ListAPIKeys(userID)

			if outputJSON {
				data, _ := json.MarshalIndent(keys, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(keys) == 0 {
				fmt.Println("No API keys found.")
				return nil
			}

			for _, k := range keys {
				fmt.Println(sso.FormatAPIKey(k))
			}
			return nil
		},
	}
	listKeysCmd.Flags().String("user", "", "Filter by user ID")

	// revoke-api-key
	revokeKeyCmd := &cobra.Command{
		Use:   "revoke-api-key <key-id>",
		Short: "Revoke an API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := sso.NewSSOManager(storeDir)
			return m.RevokeAPIKey(args[0])
		},
	}

	// sessions
	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "List active sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := sso.NewSSOManager(storeDir)
			userID, _ := cmd.Flags().GetString("user")
			sessions := m.ListSessions(userID)

			if outputJSON {
				data, _ := json.MarshalIndent(sessions, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(sessions) == 0 {
				fmt.Println("No active sessions.")
				return nil
			}

			for _, s := range sessions {
				fmt.Println(sso.FormatSession(s))
			}
			return nil
		},
	}
	sessionsCmd.Flags().String("user", "", "Filter by user ID")

	// stats
	ssoStatsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show SSO statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := sso.NewSSOManager(storeDir)
			stats := m.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(sso.FormatStats(stats))
			return nil
		},
	}

	cmd.AddCommand(providersCmd, registerOIDCCmd, registerSAMLSetCmd,
		createKeyCmd, listKeysCmd, revokeKeyCmd, sessionsCmd, ssoStatsCmd)
	return cmd
}
