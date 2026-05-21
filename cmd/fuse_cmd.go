package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/fuse"
	"github.com/spf13/cobra"
)

var fuseCmd = &cobra.Command{
	Use:   "fuse",
	Short: "Multi-agent knowledge fusion",
	Long:  `Fuse outputs from multiple agents into coherent results. Deduplication, conflict resolution, confidence-weighted merging. Many minds, one answer.`,
}

var fuseEngine *fuse.Fuse

func getFuseEngine() *fuse.Fuse {
	if fuseEngine == nil {
		fuseEngine = fuse.NewFuse()
	}
	return fuseEngine
}

func init() {
	fuseCmd.AddCommand(fuseContributeCmd)
	fuseCmd.AddCommand(fuseMergeCmd)
	fuseCmd.AddCommand(fuseTopicsCmd)
	fuseCmd.AddCommand(fuseConflictsCmd)
	fuseCmd.AddCommand(fuseStatsCmd)
}

// fuse contribute
var fuseContributeCmd = &cobra.Command{
	Use:   "contribute [agent-id] [topic] [content]",
	Short: "Add a contribution from an agent",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		confidence, _ := cmd.Flags().GetFloat64("confidence")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		f := getFuseEngine()
		c := f.Contribute(args[0], args[1], args[2], confidence, tags)
		fmt.Printf("Contributed: %s (confidence: %.2f)\n", c.ID, c.Confidence)
		return nil
	},
}

// fuse merge
var fuseMergeCmd = &cobra.Command{
	Use:   "merge [topic]",
	Short: "Merge contributions for a topic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		strategy, _ := cmd.Flags().GetString("strategy")

		f := getFuseEngine()
		result, err := f.Merge(args[0], fuse.MergeStrategy(strategy))
		if err != nil {
			return err
		}

		fmt.Printf("Fused Result: %s\n", result.ID)
		fmt.Printf("Topic: %s\n", result.Topic)
		fmt.Printf("Strategy: %s\n", result.Strategy)
		fmt.Printf("Contributions: %d\n", result.Contributions)
		fmt.Printf("Confidence: %.2f\n", result.Confidence)
		fmt.Printf("Agreement: %.1f%%\n", result.Agreement*100)
		fmt.Printf("Conflicts: %d\n", result.ConflictCount)
		fmt.Printf("\nContent:\n%s\n", result.Content)
		return nil
	},
}

// fuse topics
var fuseTopicsCmd = &cobra.Command{
	Use:   "topics",
	Short: "List all topics with contributions",
	RunE: func(cmd *cobra.Command, args []string) error {
		f := getFuseEngine()
		topics := f.ListTopics()
		if len(topics) == 0 {
			fmt.Println("No topics")
			return nil
		}
		for _, t := range topics {
			contribs := f.GetContributions(t)
			fmt.Printf("  %s (%d contributions)\n", t, len(contribs))
		}
		return nil
	},
}

// fuse conflicts
var fuseConflictsCmd = &cobra.Command{
	Use:   "conflicts [topic]",
	Short: "Show conflicts for a topic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := getFuseEngine()
		conflicts := f.Conflicts(args[0])
		if len(conflicts) == 0 {
			fmt.Println("No conflicts")
			return nil
		}
		for i, c := range conflicts {
			fmt.Printf("Conflict %d:\n", i+1)
			for j, agentID := range c.AgentIDs {
				fmt.Printf("  %s: %s\n", agentID, c.Contents[j])
			}
		}
		return nil
	},
}

// fuse stats
var fuseStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show fusion statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getFuseEngine().Stats()
		fmt.Printf("Topics: %v\n", stats["topics"])
		fmt.Printf("Total Contributions: %v\n", stats["total_contributions"])
		fmt.Printf("Fused Results: %v\n", stats["fused_results"])
		return nil
	},
}

func init() {
	fuseContributeCmd.Flags().Float64("confidence", 0.8, "Confidence score (0-1)")
	fuseContributeCmd.Flags().StringSlice("tags", nil, "Tags for the contribution")

	fuseMergeCmd.Flags().String("strategy", "weighted", "Merge strategy (vote, weighted, concat, best, consensus)")
}
