package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/forge/sword/internal/org"
	"github.com/spf13/cobra"
)

func divisionCmd() *cobra.Command {
	var dataPath string
	cmd := &cobra.Command{
		Use:   "division",
		Short: "Division management",
		Long:  "Create, list, and manage divisions in your organization.",
	}
	cmd.PersistentFlags().StringVar(&dataPath, "data", ".forge/org.json", "Path to org data file")

	// list
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all divisions",
		Run: func(cmd *cobra.Command, args []string) {
			o := mustLoadOrg(dataPath)
			divs := o.ListDivisions(false)
			if len(divs) == 0 {
				fmt.Println("No divisions. Create one with 'forge division create <name> <type>'.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tNAME\tTYPE\tAGENTS\tHEAD\tACTIVE\n")
			for _, d := range divs {
				head := d.HeadAgentID
				if len(head) > 12 {
					head = head[:12]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%v\n",
					shortID(d.ID), d.Name, d.Type, len(d.Agents), head, d.Active)
			}
			w.Flush()
		},
	})

	// create
	cmd.AddCommand(&cobra.Command{
		Use:   "create [name] [type]",
		Short: "Create a new division",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			o := mustLoadOrg(dataPath)
			div, err := o.CreateDivision(args[0], org.DivisionType(args[1]), 0)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("Created division: %s (%s) [%s]\n", div.Name, div.Type, div.ID)
		},
	})

	// deactivate
	cmd.AddCommand(&cobra.Command{
		Use:   "deactivate [division-id]",
		Short: "Deactivate a division",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			o := mustLoadOrg(dataPath)
			err := o.DeactivateDivision(args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("Deactivated division: %s\n", args[0])
		},
	})

	return cmd
}
