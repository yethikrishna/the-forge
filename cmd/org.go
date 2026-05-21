package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/forge/sword/internal/org"
	"github.com/spf13/cobra"
)

func orgCmd() *cobra.Command {
	var dataPath string
	cmd := &cobra.Command{
		Use:   "org",
		Short: "Organization management",
		Long:  "Create and manage your AI organization: divisions, agents, goals, standups, and restructuring.",
	}
	cmd.PersistentFlags().StringVar(&dataPath, "data", ".forge/org.json", "Path to org data file")

	// init
	cmd.AddCommand(&cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new organization",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			o := org.New(args[0], "human", dataPath)
			status := o.GetStatus()
			fmt.Printf("Initialized organization: %s (version %d)\n", status.OrgName, status.Version)
		},
	})

	// status
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show organization status",
		Run: func(cmd *cobra.Command, args []string) {
			o := mustLoadOrg(dataPath)
			status := o.GetStatus()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "Organization:\t%s\n", status.OrgName)
			fmt.Fprintf(w, "Version:\t%d\n", status.Version)
			fmt.Fprintf(w, "Agents:\t%d active / %d total\n", status.ActiveAgents, status.TotalAgents)
			fmt.Fprintf(w, "Divisions:\t%d active / %d total\n", status.ActiveDivisions, status.TotalDivisions)
			fmt.Fprintf(w, "Active Goals:\t%d\n", status.ActiveGoals)
			fmt.Fprintf(w, "Open Escalations:\t%d\n", status.OpenEscalations)
			fmt.Fprintf(w, "Pending Handoffs:\t%d\n", status.PendingHandoffs)
			fmt.Fprintf(w, "Running Experiments:\t%d\n", status.RunningExperiments)
			w.Flush()
		},
	})

	// standup
	cmd.AddCommand(&cobra.Command{
		Use:   "standup",
		Short: "Show latest standup report",
		Run: func(cmd *cobra.Command, args []string) {
			o := mustLoadOrg(dataPath)
			report := o.GetLatestStandup()
			if report == nil {
				fmt.Println("No standup reports yet.")
				return
			}
			fmt.Printf("Standup for %s\n\n", report.Date.Format("2006-01-02"))
			for _, entry := range report.Entries {
				fmt.Printf("  %s:\n", entry.AgentID)
				for _, d := range entry.Done {
					fmt.Printf("    ✓ %s\n", d)
				}
				for _, d := range entry.Doing {
					fmt.Printf("    → %s\n", d)
				}
				for _, d := range entry.Blocked {
					fmt.Printf("    ✗ %s\n", d)
				}
			}
			if len(report.Blockers) > 0 {
				fmt.Printf("\nBlockers (%d):\n", len(report.Blockers))
				for _, b := range report.Blockers {
					fmt.Printf("  - %s\n", b)
				}
			}
		},
	})

	// goals
	cmd.AddCommand(&cobra.Command{
		Use:   "goals",
		Short: "List organization goals",
		Run: func(cmd *cobra.Command, args []string) {
			o := mustLoadOrg(dataPath)
			goals := o.ListGoals("", "")
			if len(goals) == 0 {
				fmt.Println("No goals set.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tTITLE\tSTATUS\tPROGRESS\tOWNER\n")
			for _, g := range goals {
				fmt.Fprintf(w, "%s\t%s\t%s\t%.0f%%\t%s\n", shortID(g.ID), g.Title, g.Status, g.Progress, g.Owner)
			}
			w.Flush()
		},
	})

	// escalations
	cmd.AddCommand(&cobra.Command{
		Use:   "escalations",
		Short: "List open escalations",
		Run: func(cmd *cobra.Command, args []string) {
			o := mustLoadOrg(dataPath)
			escs := o.ListOpenEscalations("")
			if len(escs) == 0 {
				fmt.Println("No open escalations.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tAGENT\tSEVERITY\tREASON\tTARGET\n")
			for _, e := range escs {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", shortID(e.ID), e.AgentID, e.Severity, orgTruncate(e.Reason, 40), e.TargetID)
			}
			w.Flush()
		},
	})

	// experiments
	cmd.AddCommand(&cobra.Command{
		Use:   "experiments",
		Short: "List experiments",
		Run: func(cmd *cobra.Command, args []string) {
			o := mustLoadOrg(dataPath)
			exps := o.ListExperiments("")
			if len(exps) == 0 {
				fmt.Println("No experiments.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tTITLE\tSTATUS\tDIVISION\n")
			for _, e := range exps {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", shortID(e.ID), orgTruncate(e.Title, 30), e.Status, e.DivisionID)
			}
			w.Flush()
		},
	})

	return cmd
}

func mustLoadOrg(path string) *org.Org {
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		fmt.Fprintln(os.Stderr, "No org found. Run 'forge org init <name>' first.")
		os.Exit(1)
	}
	var info struct {
		Org struct {
			Name    string `json:"name"`
			OwnerID string `json:"owner_id"`
		} `json:"org"`
	}
	json.Unmarshal(data, &info)
	o := org.New(info.Org.Name, info.Org.OwnerID, path)
	return o
}

func orgTruncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
