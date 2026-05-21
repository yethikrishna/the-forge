package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/forge/sword/internal/civilization"
	"github.com/spf13/cobra"
)

func civCmd() *cobra.Command {
	var dataPath string
	cmd := &cobra.Command{
		Use:   "civ",
		Short: "Civilization management",
		Long:  "Manage inter-civilization communication, marketplace, treaties, reputation, and procurement.",
	}
	cmd.PersistentFlags().StringVar(&dataPath, "data", ".forge/civilization.json", "Path to civilization data")

	// init
	cmd.AddCommand(&cobra.Command{
		Use:   "init [name]",
		Short: "Initialize civilization identity",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := &civilization.OrgIdentity{
				ID:           fmt.Sprintf("civ-%d", time.Now().UnixNano()),
				Name:         args[0],
				Capabilities: []string{},
				FoundedAt:    time.Now().UTC(),
				Version:      civilization.ProtocolVersion,
			}
			hub := civilization.NewHub(id, dataPath)
			fmt.Printf("Initialized civilization: %s [%s]\n", hub.GetIdentity().Name, hub.GetIdentity().ID)
		},
	})

	// identity
	cmd.AddCommand(&cobra.Command{
		Use:   "identity",
		Short: "Show this org's civilization identity",
		Run: func(cmd *cobra.Command, args []string) {
			hub := mustLoadCivHub(dataPath)
			id := hub.GetIdentity()
			fmt.Printf("ID:           %s\n", id.ID)
			fmt.Printf("Name:         %s\n", id.Name)
			fmt.Printf("Endpoint:     %s\n", id.Endpoint)
			fmt.Printf("Capabilities: %v\n", id.Capabilities)
			fmt.Printf("Verified:     %v\n", id.Verified)
		},
	})

	// peers
	cmd.AddCommand(&cobra.Command{
		Use:   "peers",
		Short: "List known peer organizations",
		Run: func(cmd *cobra.Command, args []string) {
			hub := mustLoadCivHub(dataPath)
			peers := hub.ListPeers()
			if len(peers) == 0 {
				fmt.Println("No known peers.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tNAME\tCAPABILITIES\n")
			for _, p := range peers {
				fmt.Fprintf(w, "%s\t%s\t%v\n", shortID(p.ID), p.Name, p.Capabilities)
			}
			w.Flush()
		},
	})

	// treaties
	cmd.AddCommand(&cobra.Command{
		Use:   "treaties",
		Short: "List treaties",
		Run: func(cmd *cobra.Command, args []string) {
			hub := mustLoadCivHub(dataPath)
			treaties := hub.ListTreaties("")
			if len(treaties) == 0 {
				fmt.Println("No treaties.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tTITLE\tTYPE\tSTATUS\tPARTIES\n")
			for _, t := range treaties {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n",
					shortID(t.ID), orgTruncate(t.Title, 25), t.Type, t.Status, t.Parties)
			}
			w.Flush()
		},
	})

	// marketplace
	cmd.AddCommand(&cobra.Command{
		Use:   "marketplace",
		Short: "List marketplace offerings",
		Run: func(cmd *cobra.Command, args []string) {
			hub := mustLoadCivHub(dataPath)
			offerings := hub.ListOfferings("")
			if len(offerings) == 0 {
				fmt.Println("No offerings on marketplace.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tNAME\tCATEGORY\tPRICE\tORG\n")
			for _, o := range offerings {
				fmt.Fprintf(w, "%s\t%s\t%s\t%.0f %s\t%s\n",
					shortID(o.ID), orgTruncate(o.Name, 20), o.Category, o.PriceAmount, o.Currency, shortID(o.OrgID))
			}
			w.Flush()
		},
	})

	// federations
	cmd.AddCommand(&cobra.Command{
		Use:   "federations",
		Short: "List federations",
		Run: func(cmd *cobra.Command, args []string) {
			hub := mustLoadCivHub(dataPath)
			feds := hub.ListFederations()
			if len(feds) == 0 {
				fmt.Println("Not a member of any federation.")
				return
			}
			for _, f := range feds {
				fmt.Printf("  %s (%s) — %d members\n", f.Name, shortID(f.ID), len(f.Members))
			}
		},
	})

	// reputation
	cmd.AddCommand(&cobra.Command{
		Use:   "reputation [org-id]",
		Short: "Show reputation score",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			hub := mustLoadCivHub(dataPath)
			score := hub.GetReputation(args[0])
			fmt.Printf("Organization: %s\n", score.OrgID)
			fmt.Printf("Overall Score: %.1f/100\n", score.Overall)
			fmt.Printf("Entries: %d\n", score.EntryCount)
			if len(score.Categories) > 0 {
				fmt.Println("\nBy Category:")
				for cat, val := range score.Categories {
					fmt.Printf("  %s: %.1f\n", cat, val)
				}
			}
		},
	})

	return cmd
}

func mustLoadCivHub(path string) *civilization.Hub {
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		fmt.Fprintln(os.Stderr, "No civilization data. Run 'forge civ init <name>' first.")
		os.Exit(1)
	}
	var wrapper struct {
		Identity *civilization.OrgIdentity `json:"identity"`
	}
	json.Unmarshal(data, &wrapper)
	if wrapper.Identity == nil {
		fmt.Fprintln(os.Stderr, "No identity found in civilization data.")
		os.Exit(1)
	}
	return civilization.NewHub(wrapper.Identity, path)
}
