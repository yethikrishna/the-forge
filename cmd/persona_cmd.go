package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/persona"
)

var personaCmd = &cobra.Command{
	Use:   "persona",
	Short: "Manage agent personas",
	Long:  "Create and manage persistent agent personas with style preferences, trust scores, and system prompts.",
}

var (
	personaDir       string
	personaName      string
	personaDesc      string
	personaTone      string
	personaVerbosity string
	personaScope     string
	personaMaxCost   float64
	personaPrefKey   string
	personaPrefValue string
)

func init() {
	personaCmd.AddCommand(personaListCmd)
	personaCmd.AddCommand(personaShowCmd)
	personaCmd.AddCommand(personaCreateCmd)
	personaCmd.AddCommand(personaDeleteCmd)
	personaCmd.AddCommand(personaPromptCmd)
	personaCmd.AddCommand(personaTrustCmd)
	personaCmd.AddCommand(personaPrefCmd)
	personaCmd.AddCommand(personaDefaultsCmd)

	personaCmd.PersistentFlags().StringVar(&personaDir, "dir", ".forge/personas", "Persona storage directory")
	personaCreateCmd.Flags().StringVar(&personaName, "name", "", "Persona name")
	personaCreateCmd.Flags().StringVar(&personaDesc, "desc", "", "Description")
	personaCreateCmd.Flags().StringVar(&personaTone, "tone", "technical", "Tone (technical, casual, formal, friendly)")
	personaCreateCmd.Flags().StringVar(&personaVerbosity, "verbosity", "moderate", "Verbosity (concise, moderate, detailed)")
	personaCreateCmd.Flags().StringVar(&personaScope, "scope", "full", "Scope (read-only, src-only, sandbox, full)")
	personaCreateCmd.Flags().Float64Var(&personaMaxCost, "max-cost", 0, "Max cost per session")
	personaTrustCmd.Flags().Float64("delta", 0, "Trust score delta")
	personaPrefCmd.Flags().StringVar(&personaPrefKey, "key", "", "Preference key")
	personaPrefCmd.Flags().StringVar(&personaPrefValue, "value", "", "Preference value")
}

func getPersonaStore() (*persona.Store, error) {
	return persona.NewStore(personaDir)
}

var personaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List personas",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPersonaStore()
		if err != nil {
			return err
		}
		list := store.List()
		if len(list) == 0 {
			fmt.Println("No personas found. Use 'forge persona defaults' to create built-in ones.")
			return nil
		}
		fmt.Printf("Personas (%d):\n", len(list))
		for _, p := range list {
			fmt.Printf("  %s [%s] trust:%.0f uses:%d — %s\n",
				p.Name, p.TrustLevel, p.TrustScore, p.UseCount, p.Description)
		}
		return nil
	},
}

var personaShowCmd = &cobra.Command{
	Use:   "show [name-or-id]",
	Short: "Show persona details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPersonaStore()
		if err != nil {
			return err
		}
		p, ok := store.Get(args[0])
		if !ok {
			p, ok = store.GetByName(args[0])
		}
		if !ok {
			return fmt.Errorf("persona %q not found", args[0])
		}

		fmt.Printf("Persona: %s\n", p.Name)
		fmt.Printf("ID: %s\n", p.ID)
		fmt.Printf("Description: %s\n", p.Description)
		fmt.Printf("Trust: %s (%.0f/100)\n", p.TrustLevel, p.TrustScore)
		fmt.Printf("Uses: %d\n", p.UseCount)
		fmt.Printf("Style: %s, %s\n", p.Style.Tone, p.Style.Verbosity)
		if p.Scope != "" {
			fmt.Printf("Scope: %s\n", p.Scope)
		}
		if p.MaxCost > 0 {
			fmt.Printf("Max Cost: $%.2f/session\n", p.MaxCost)
		}
		if len(p.Preferences) > 0 {
			fmt.Println("\nPreferences:")
			for _, pref := range p.Preferences {
				fmt.Printf("  %s = %s (priority: %d)\n", pref.Key, pref.Value, pref.Priority)
			}
		}
		return nil
	},
}

var personaCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a persona",
	RunE: func(cmd *cobra.Command, args []string) error {
		if personaName == "" {
			return fmt.Errorf("--name is required")
		}
		store, err := getPersonaStore()
		if err != nil {
			return err
		}
		p := &persona.Persona{
			Name:        personaName,
			Description: personaDesc,
			Style: persona.Style{
				Tone:      personaTone,
				Verbosity: personaVerbosity,
			},
			Scope:   personaScope,
			MaxCost: personaMaxCost,
		}
		if err := store.Create(p); err != nil {
			return err
		}
		fmt.Printf("Created persona: %s (id: %s)\n", p.Name, p.ID)
		return nil
	},
}

var personaDeleteCmd = &cobra.Command{
	Use:   "delete [name-or-id]",
	Short: "Delete a persona",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPersonaStore()
		if err != nil {
			return err
		}
		p, ok := store.Get(args[0])
		if !ok {
			p, ok = store.GetByName(args[0])
		}
		if !ok {
			return fmt.Errorf("persona %q not found", args[0])
		}
		return store.Delete(p.ID)
	},
}

var personaPromptCmd = &cobra.Command{
	Use:   "prompt [name-or-id]",
	Short: "Generate system prompt from persona",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPersonaStore()
		if err != nil {
			return err
		}
		p, ok := store.Get(args[0])
		if !ok {
			p, ok = store.GetByName(args[0])
		}
		if !ok {
			return fmt.Errorf("persona %q not found", args[0])
		}
		fmt.Println(persona.FormatSystemPrompt(p))
		return nil
	},
}

var personaTrustCmd = &cobra.Command{
	Use:   "trust [name-or-id]",
	Short: "Update persona trust score",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		delta, _ := cmd.Flags().GetFloat64("delta")
		if delta == 0 {
			return fmt.Errorf("--delta is required")
		}
		store, err := getPersonaStore()
		if err != nil {
			return err
		}
		p, ok := store.Get(args[0])
		if !ok {
			p, ok = store.GetByName(args[0])
		}
		if !ok {
			return fmt.Errorf("persona %q not found", args[0])
		}
		return store.UpdateTrust(p.ID, delta)
	},
}

var personaPrefCmd = &cobra.Command{
	Use:   "pref [name-or-id]",
	Short: "Set a persona preference",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if personaPrefKey == "" || personaPrefValue == "" {
			return fmt.Errorf("--key and --value are required")
		}
		store, err := getPersonaStore()
		if err != nil {
			return err
		}
		p, ok := store.Get(args[0])
		if !ok {
			p, ok = store.GetByName(args[0])
		}
		if !ok {
			return fmt.Errorf("persona %q not found", args[0])
		}
		return store.SetPreference(p.ID, personaPrefKey, personaPrefValue, 3)
	},
}

var personaDefaultsCmd = &cobra.Command{
	Use:   "defaults",
	Short: "Create built-in default personas",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getPersonaStore()
		if err != nil {
			return err
		}
		defaults := persona.DefaultPersonas()
		created := 0
		for i := range defaults {
			p := defaults[i]
			if _, ok := store.GetByName(p.Name); ok {
				continue // already exists
			}
			if err := store.Create(&p); err != nil {
				continue
			}
			created++
		}
		fmt.Printf("Created %d default personas\n", created)
		return nil
	},
}
