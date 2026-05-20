package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/forge/sword/internal/archaeologist"
	"github.com/spf13/cobra"
)

func archaeologistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archaeologist",
		Short: "AI-powered git forensics",
		Long: `Analyze git history to understand why code was written,
detect dead code, identify hotspots, and surface patterns.

Every line has a story. Dig it up.

Examples:
  forge archaeologist blame internal/bridge/bridge.go
  forge archaeologist log internal/bridge/bridge.go
  forge archaeologist hotspots
  forge archaeologist dead-code
  forge archaeologist why internal/bridge/bridge.go:42`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		archaeologistBlameCmd(),
		archaeologistLogCmd(),
		archaeologistHotspotsCmd(),
		archaeologistDeadCodeCmd(),
		archaeologistWhyCmd(),
	)

	return cmd
}

func getArchaeologist() (*archaeologist.Archaeologist, error) {
	return archaeologist.New("."), nil
}

func archaeologistBlameCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "blame <file>",
		Short: "Git blame with archaeologist context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := getArchaeologist()
			if err != nil {
				return err
			}

			entries, err := a.Blame(args[0])
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(archaeologist.MarshalJSON(entries))
				return nil
			}

			if len(entries) == 0 {
				fmt.Println("No blame entries found.")
				return nil
			}

			fmt.Print(archaeologist.FormatBlame(entries))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func archaeologistLogCmd() *cobra.Command {
	var jsonOutput bool
	var limit int

	cmd := &cobra.Command{
		Use:   "log <file>",
		Short: "File history with churn analysis",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := getArchaeologist()
			if err != nil {
				return err
			}

			history, err := a.FileLog(args[0], limit)
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(archaeologist.MarshalJSON(history))
				return nil
			}

			fmt.Println(archaeologist.FormatHistory(history))
			fmt.Println()

			if len(history.Commits) > 0 {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "HASH\tAUTHOR\tDATE\tMESSAGE")
				for _, c := range history.Commits {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Hash[:8], c.Author, c.Date.Format("2006-01-02"), c.Message)
				}
				w.Flush()
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max commits to show")
	return cmd
}

func archaeologistHotspotsCmd() *cobra.Command {
	var jsonOutput bool
	var limit int

	cmd := &cobra.Command{
		Use:   "hotspots",
		Short: "Find high-churn files (bug risk indicators)",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := getArchaeologist()
			if err != nil {
				return err
			}

			hotspots, err := a.Hotspots(limit)
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(archaeologist.MarshalJSON(hotspots))
				return nil
			}

			if len(hotspots) == 0 {
				fmt.Println("No hotspots found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "FILE\tCHURN\tAUTHORS\tCOMMITS\tRISK\tLAST CHANGED")
			for _, h := range hotspots {
				fmt.Fprintf(w, "%s\t%.0f\t%d\t%d\t%s\t%s\n",
					h.File, h.ChurnScore, h.NumAuthors, h.NumCommits, h.RiskLevel, h.LastChanged)
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max hotspots to show")
	return cmd
}

func archaeologistDeadCodeCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "dead-code",
		Short: "Find potentially dead code",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := getArchaeologist()
			if err != nil {
				return err
			}

			candidates, err := a.DeadCode(nil)
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(archaeologist.MarshalJSON(candidates))
				return nil
			}

			if len(candidates) == 0 {
				fmt.Println("No dead code candidates found.")
				return nil
			}

			fmt.Printf("Found %d potential dead code locations:\n\n", len(candidates))
			for _, c := range candidates {
				fmt.Printf("  %s\n", archaeologist.FormatDeadCode(c))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func archaeologistWhyCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "why <file:line>",
		Short: "Why was this line written? (git forensics)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse file:line
			parts := splitFileLine(args[0])
			if len(parts) != 2 {
				return fmt.Errorf("format: file:line (e.g., main.go:42)")
			}

			a, err := getArchaeologist()
			if err != nil {
				return err
			}

			info, err := a.WhyWasThisWritten(fmt.Sprintf("%v", parts[0]), int(parts[1].(float64)))
			if err != nil {
				return err
			}

			if jsonOutput {
				fmt.Println(archaeologist.MarshalJSON(info))
				return nil
			}

			fmt.Printf("Commit:  %s\n", info.Hash)
			fmt.Printf("Author:  %s\n", info.Author)
			fmt.Printf("Date:    %s\n", info.Date.Format("2006-01-02 15:04:05"))
			fmt.Printf("Message: %s\n", info.Message)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

// splitFileLine splits "file.go:42" into ["file.go", 42].
func splitFileLine(s string) [2]interface{} {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			lineNum := 0
			fmt.Sscanf(s[i+1:], "%d", &lineNum)
			return [2]interface{}{s[:i], lineNum}
		}
	}
	return [2]interface{}{s, 0}
}
