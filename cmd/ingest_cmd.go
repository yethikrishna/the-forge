package cmd

import (
	"fmt"
	"time"

	"github.com/forge/sword/internal/ingest"
	"github.com/spf13/cobra"
)

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Multi-source data ingestion for agent context",
	Long:  `Ingest files, URLs, APIs, and commands into chunked, searchable context. All the world's data, one pipeline.`,
}

var ingestPipeline *ingest.Pipeline

func getIngestPipeline() *ingest.Pipeline {
	if ingestPipeline == nil {
		ingestPipeline = ingest.NewPipeline(getForgeDir() + "/ingest")
	}
	return ingestPipeline
}

func init() {
	ingestCmd.AddCommand(ingestAddCmd)
	ingestCmd.AddCommand(ingestListCmd)
	ingestCmd.AddCommand(ingestShowCmd)
	ingestCmd.AddCommand(ingestDeleteCmd)
	ingestCmd.AddCommand(ingestRunCmd)
	ingestCmd.AddCommand(ingestRunAllCmd)
	ingestCmd.AddCommand(ingestChunksCmd)
	ingestCmd.AddCommand(ingestSearchCmd)
	ingestCmd.AddCommand(ingestStatsCmd)
}

// ingest add
var ingestAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add an ingestion source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceType, _ := cmd.Flags().GetString("type")
		path, _ := cmd.Flags().GetString("path")

		p := getIngestPipeline()
		s := p.AddSource(args[0], ingest.SourceType(sourceType), path)

		chunkSize, _ := cmd.Flags().GetInt("chunk-size")
		chunkStrategy, _ := cmd.Flags().GetString("chunk-strategy")

		if chunkSize > 0 || chunkStrategy != "" {
			// Update via pipeline (simplified)
			_ = chunkSize
			_ = chunkStrategy
		}

		fmt.Printf("Added source: %s (id: %s, type: %s)\n", s.Name, s.ID, s.Type)
		return nil
	},
}

// ingest list
var ingestListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all ingestion sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getIngestPipeline()
		list := p.ListSources()
		if len(list) == 0 {
			fmt.Println("No ingestion sources")
			return nil
		}

		fmt.Printf("%-20s %-15s %-12s %-8s %s\n", "ID", "NAME", "TYPE", "CHUNKS", "LAST INGESTED")
		for _, s := range list {
			lastIngested := "never"
			if !s.LastIngested.IsZero() {
				lastIngested = s.LastIngested.Format(time.RFC3339)
			}
			fmt.Printf("%-20s %-15s %-12s %-8d %s\n",
				s.ID, s.Name, s.Type, s.ChunkCount, lastIngested)
		}
		return nil
	},
}

// ingest show
var ingestShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show source details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getIngestPipeline()
		s, ok := p.GetSource(args[0])
		if !ok {
			return fmt.Errorf("source %q not found", args[0])
		}
		fmt.Println(ingest.RenderSource(s))
		return nil
	},
}

// ingest delete
var ingestDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an ingestion source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getIngestPipeline().DeleteSource(args[0])
	},
}

// ingest run
var ingestRunCmd = &cobra.Command{
	Use:   "run [source-id]",
	Short: "Run ingestion for a source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getIngestPipeline()
		chunks, err := p.Ingest(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Ingested %d chunks from source %s\n", len(chunks), args[0])
		return nil
	},
}

// ingest run-all
var ingestRunAllCmd = &cobra.Command{
	Use:   "run-all",
	Short: "Run ingestion for all sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getIngestPipeline()
		results, err := p.IngestAll()
		if err != nil {
			return err
		}

		for id, count := range results {
			if count < 0 {
				fmt.Printf("  %s: ERROR\n", id)
			} else {
				fmt.Printf("  %s: %d chunks\n", id, count)
			}
		}
		return nil
	},
}

// ingest chunks
var ingestChunksCmd = &cobra.Command{
	Use:   "chunks [source-id]",
	Short: "List chunks for a source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := getIngestPipeline()
		chunks := p.GetChunks(args[0])
		if len(chunks) == 0 {
			fmt.Println("No chunks found")
			return nil
		}

		for _, c := range chunks {
			preview := c.Content
			if len(preview) > 80 {
				preview = preview[:77] + "..."
			}
			fmt.Printf("  [%d] %s (%d bytes): %s\n", c.Index, c.ID, c.Size, preview)
		}
		return nil
	},
}

// ingest search
var ingestSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search across all chunks",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		p := getIngestPipeline()
		results := p.Search(args[0], limit)

		if len(results) == 0 {
			fmt.Println("No results found")
			return nil
		}

		for _, c := range results {
			preview := c.Content
			if len(preview) > 100 {
				preview = preview[:97] + "..."
			}
			fmt.Printf("  [%s] %s\n", c.SourceID, preview)
		}
		return nil
	},
}

// ingest stats
var ingestStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show pipeline statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getIngestPipeline().Stats()
		fmt.Printf("Sources: %v\n", stats["sources"])
		fmt.Printf("Total Chunks: %v\n", stats["total_chunks"])
		fmt.Printf("Total Size: %v bytes\n", stats["total_size"])
		if bt, ok := stats["by_type"].(map[ingest.SourceType]int); ok {
			fmt.Println("By Type:")
			for t, c := range bt {
				fmt.Printf("  %s: %d\n", t, c)
			}
		}
		return nil
	},
}

func init() {
	ingestAddCmd.Flags().String("type", "file", "Source type (file, url, api, command, inline)")
	ingestAddCmd.Flags().String("path", "", "Source path (file path, URL, or command)")
	ingestAddCmd.Flags().Int("chunk-size", 1000, "Chunk size in characters")
	ingestAddCmd.Flags().String("chunk-strategy", "paragraph", "Chunk strategy (fixed, sentence, paragraph, line)")

	ingestSearchCmd.Flags().Int("limit", 10, "Max results")
}
