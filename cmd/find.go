package cmd

import (
	"strings"

	"github.com/forge/sword/internal/find"
	"github.com/spf13/cobra"
)

func findCmd() *cobra.Command {
	var asJSON bool
	var typeFilter []string
	var limit int

	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Global search across memory, sessions, pipelines, templates, codebase",
		Long: `Search everywhere in Forge:
  • Memory files (daily notes, long-term memory)
  • Session transcripts
  • Pipeline configurations
  • Templates and prompts
  • Config files
  • Source code in workspace

Examples:
  forge find "gin framework"
  forge find "gpt-4" --type=config
  forge find "TODO" --type=code --limit=5`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")

			var types []find.ResultType
			for _, tf := range typeFilter {
				types = append(types, find.ResultType(tf))
			}

			searcher := find.NewSearcher("", "")
			results, err := searcher.Search(query, types, limit)
			if err != nil {
				return err
			}

			if asJSON || getOutputFormat() == "json" {
				s, err := find.FormatResultsJSON(results)
				if err != nil {
					return err
				}
				cmd.Println(s)
				return nil
			}

			cmd.Print(find.FormatResults(results, query))
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	cmd.Flags().StringSliceVar(&typeFilter, "type", nil, "Filter by type (memory,session,pipeline,template,file,config,code)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")

	return cmd
}
