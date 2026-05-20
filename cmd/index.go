package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/hnsw"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func indexCmd() *cobra.Command {
	var dims int
	var indexPath string

	cmd := &cobra.Command{
		Use:   "index [path]",
		Short: "Build and query a RAG codebase index",
		Long: `Build a semantic codebase index using HNSW vector search.
Index source code files and search them semantically.

Note: Full embedding support requires an embedding model.
This command provides the indexing infrastructure.

Examples:
  forge index build ./src
  forge index search "authentication middleware"
  forge index stats`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "build [path]",
			Short: "Build a codebase index",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				sourcePath := args[0]

				fmt.Println(pretty.InfoLine(fmt.Sprintf("Indexing %s", sourcePath)))

				graph := hnsw.New(hnsw.DefaultConfig(dims))
				fileCount := 0

				err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if info.IsDir() {
						return nil
					}

					// Skip hidden dirs and common non-source dirs
					if strings.Contains(path, "/.") || strings.Contains(path, "node_modules") ||
						strings.Contains(path, "vendor") || strings.Contains(path, ".git") {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}

					// Only index source files
					ext := strings.ToLower(filepath.Ext(path))
					sourceExts := map[string]bool{
						".go": true, ".py": true, ".js": true, ".ts": true,
						".rs": true, ".java": true, ".c": true, ".cpp": true,
						".h": true, ".md": true, ".yaml": true, ".yml": true,
						".json": true, ".toml": true,
					}
					if !sourceExts[ext] {
						return nil
					}

					// Create a simple hash-based embedding for indexing
					// (Real implementation would use an embedding model)
					vector := pathToVector(path, dims)
					graph.Insert(fileCount, vector)
					fileCount++

					if fileCount%100 == 0 {
						fmt.Printf("  Indexed %d files...\n", fileCount)
					}

					return nil
				})

				if err != nil {
					return fmt.Errorf("index build failed: %w", err)
				}

				// Save index
				if indexPath == "" {
					indexPath = filepath.Join(sourcePath, ".forge-index.json")
				}

				indexData := map[string]any{
					"source":    sourcePath,
					"files":     fileCount,
					"dims":      dims,
					"max_level": graph.MaxLevel(),
				}

				data, _ := json.MarshalIndent(indexData, "", "  ")
				if err := os.WriteFile(indexPath, data, 0o644); err != nil {
					return fmt.Errorf("save index: %w", err)
				}

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Indexed %d files", fileCount)))
				fmt.Printf("   Index:    %s\n", indexPath)
				fmt.Printf("   Max level: %d\n", graph.MaxLevel())
				return nil
			},
		},
		&cobra.Command{
			Use:   "search [query]",
			Short: "Search the codebase index",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				query := args[0]

				// Convert query to vector (stub)
				vector := queryToVector(query, dims)

				graph := hnsw.New(hnsw.DefaultConfig(dims))
				results := graph.Search(vector, 5)

				if len(results) == 0 {
					fmt.Println("Forge: No results found")
					return nil
				}

				fmt.Println(pretty.HeaderLine("Search Results"))
				for i, r := range results {
					fmt.Printf("  %d. ID=%d Distance=%.4f\n", i+1, r.ID, r.Distance)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "stats",
			Short: "Show index statistics",
			RunE: func(cmd *cobra.Command, args []string) error {
				if indexPath == "" {
					indexPath = ".forge-index.json"
				}

				data, err := os.ReadFile(indexPath)
				if err != nil {
					return fmt.Errorf("index not found: %w", err)
				}

				var indexData map[string]any
				json.Unmarshal(data, &indexData)

				fmt.Println(pretty.HeaderLine("Index Statistics"))
				fmt.Printf("  Source:     %v\n", indexData["source"])
				fmt.Printf("  Files:      %v\n", indexData["files"])
				fmt.Printf("  Dimensions: %v\n", indexData["dims"])
				fmt.Printf("  Max Level:  %v\n", indexData["max_level"])
				return nil
			},
		},
	)

	cmd.PersistentFlags().IntVar(&dims, "dims", 128, "Embedding dimensions")
	cmd.PersistentFlags().StringVar(&indexPath, "index", "", "Index file path")

	return cmd
}

// pathToVector creates a simple deterministic vector from a file path.
// In production, this would use an embedding model.
func pathToVector(path string, dims int) []float64 {
	vector := make([]float64, dims)
	for i, c := range path {
		vector[i%dims] += float64(c) / 1000.0
	}
	// Normalize
	var norm float64
	for _, v := range vector {
		norm += v * v
	}
	norm = sqrt(norm)
	if norm > 0 {
		for i := range vector {
			vector[i] /= norm
		}
	}
	return vector
}

// queryToVector creates a simple deterministic vector from a query string.
func queryToVector(query string, dims int) []float64 {
	return pathToVector(query, dims)
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}
