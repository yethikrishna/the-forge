package cmd

import (
	"fmt"
	"time"

	"github.com/forge/sword/internal/timeline"
	"github.com/spf13/cobra"
)

var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Agent activity timeline",
	Long:  `Record, query, and visualize agent activity over time. See the story, not just the logs.`,
}

var timelineTracker *timeline.Timeline

func getTimelineTracker() *timeline.Timeline {
	if timelineTracker == nil {
		timelineTracker = timeline.NewTimeline(getForgeDir() + "/timeline")
	}
	return timelineTracker
}

func init() {
	timelineCmd.AddCommand(timelineRecordCmd)
	timelineCmd.AddCommand(timelineStartCmd)
	timelineCmd.AddCommand(timelineEndCmd)
	timelineCmd.AddCommand(timelineQueryCmd)
	timelineCmd.AddCommand(timelineSpansCmd)
	timelineCmd.AddCommand(timelineSummaryCmd)
	timelineCmd.AddCommand(timelineStatsCmd)
	timelineCmd.AddCommand(timelineVisualCmd)
}

// timeline record
var timelineRecordCmd = &cobra.Command{
	Use:   "record [agent-id] [type] [name]",
	Short: "Record a timeline event",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		detail, _ := cmd.Flags().GetString("detail")
		tl := getTimelineTracker()
		e := tl.Record(args[0], timeline.EventType(args[1]), args[2], detail)
		fmt.Printf("Recorded: %s (%s)\n", e.ID, e.Type)
		return nil
	},
}

// timeline start
var timelineStartCmd = &cobra.Command{
	Use:   "start [agent-id] [name]",
	Short: "Start a time span",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tl := getTimelineTracker()
		s := tl.StartSpan(args[0], args[1])
		fmt.Printf("Started span: %s (%s)\n", s.ID, s.Name)
		return nil
	},
}

// timeline end
var timelineEndCmd = &cobra.Command{
	Use:   "end [span-id]",
	Short: "End a time span",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getTimelineTracker().EndSpan(args[0])
	},
}

// timeline query
var timelineQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query timeline events",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		eventType, _ := cmd.Flags().GetString("type")
		limit, _ := cmd.Flags().GetInt("limit")

		tl := getTimelineTracker()
		events := tl.Query(agent, time.Time{}, time.Time{}, timeline.EventType(eventType), limit)

		if len(events) == 0 {
			fmt.Println("No events found")
			return nil
		}

		fmt.Printf("%-20s %-12s %-10s %-20s %s\n", "TIME", "AGENT", "TYPE", "NAME", "DETAIL")
		for _, e := range events {
			fmt.Printf("%-20s %-12s %-10s %-20s %s\n",
				e.Timestamp.Format("15:04:05"), e.AgentID, e.Type, e.Name, e.Detail)
		}
		return nil
	},
}

// timeline spans
var timelineSpansCmd = &cobra.Command{
	Use:   "spans",
	Short: "List time spans",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		activeOnly, _ := cmd.Flags().GetBool("active")

		tl := getTimelineTracker()
		spans := tl.Spans(agent, activeOnly)

		if len(spans) == 0 {
			fmt.Println("No spans found")
			return nil
		}

		for _, s := range spans {
			status := "ended"
			if s.Active {
				status = "active"
			}
			fmt.Printf("  %s [%s] %s — %s (%s)\n", s.ID, s.AgentID, s.Name, status, s.Start.Format(time.RFC3339))
		}
		return nil
	},
}

// timeline summary
var timelineSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Time-bucketed event summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket, _ := cmd.Flags().GetString("bucket")
		tl := getTimelineTracker()
		summary := tl.Summary(bucket)

		for k, v := range summary {
			fmt.Printf("  %s: %d events\n", k, v)
		}
		return nil
	},
}

// timeline stats
var timelineStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show timeline statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getTimelineTracker().Stats()
		fmt.Printf("Events: %v\n", stats["events"])
		fmt.Printf("Spans: %v\n", stats["spans"])
		fmt.Printf("Active Spans: %v\n", stats["active_spans"])
		fmt.Printf("Total Duration: %v\n", stats["total_duration"])
		return nil
	},
}

// timeline visual
var timelineVisualCmd = &cobra.Command{
	Use:   "visual",
	Short: "Render ASCII timeline visualization",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		limit, _ := cmd.Flags().GetInt("limit")

		tl := getTimelineTracker()
		events := tl.Query(agent, time.Time{}, time.Time{}, "", limit)
		fmt.Println(timeline.RenderASCII(events, 60))
		return nil
	},
}

func init() {
	timelineRecordCmd.Flags().String("detail", "", "Event detail")
	timelineQueryCmd.Flags().String("agent", "", "Filter by agent ID")
	timelineQueryCmd.Flags().String("type", "", "Filter by event type")
	timelineQueryCmd.Flags().Int("limit", 50, "Max events")
	timelineSpansCmd.Flags().String("agent", "", "Filter by agent ID")
	timelineSpansCmd.Flags().Bool("active", false, "Show active spans only")
	timelineSummaryCmd.Flags().String("bucket", "hour", "Time bucket (hour, day, week)")
	timelineVisualCmd.Flags().String("agent", "", "Filter by agent ID")
	timelineVisualCmd.Flags().Int("limit", 20, "Max events")
}
