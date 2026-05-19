package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func searchCmd() *cobra.Command {
	var dimensions int
	var efSearch int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Semantic code search using HNSW vector index",
		Long: `Search your codebase semantically using Hierarchical Navigable
Small World graphs (github.com/coder/hnsw).

Requires embedding model to be configured.
Not yet fully wired - placeholder for integration.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Forge: Searching: %s\n", args[0])
			fmt.Printf("   Dimensions: %d\n", dimensions)
			fmt.Printf("   efSearch:   %d\n", efSearch)
			fmt.Println()
			fmt.Println("   (hnsw search integration pending - requires embedding model)")
			fmt.Println("   The HNSW library (github.com/coder/hnsw) provides:")
			fmt.Println("   - Approximate nearest neighbor search")
			fmt.Println("   - O(log n) search complexity")
			fmt.Println("   - Configurable recall/accuracy tradeoff")
			return nil
		},
	}

	cmd.Flags().IntVar(&dimensions, "dims", 1536, "Embedding dimensions")
	cmd.Flags().IntVar(&efSearch, "ef", 100, "HNSW efSearch parameter")
	return cmd
}
