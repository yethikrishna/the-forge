package cmd

import (
	"fmt"
	"time"

	"github.com/forge/sword/internal/replay"
	"github.com/spf13/cobra"
)

var replayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Session replay and time-travel debugging",
	Long: `Replay agent sessions with time-travel debugging. Step through
conversations forward/backward, branch from any point, compare
alternative execution paths.`,
}

var (
	replayDir string
)

func init() {
	replayCmd.AddCommand(replayListCmd)
	replayCmd.AddCommand(replayShowCmd)
	replayCmd.AddCommand(replayStepCmd)
	replayCmd.AddCommand(replaySummaryCmd)
	replayCmd.AddCommand(replayCompareCmd)
	replayCmd.AddCommand(replayDeleteCmd)

	replayCmd.PersistentFlags().StringVar(&replayDir, "dir", ".forge/replay", "Replay storage directory")
}

func getReplayStore() *replay.Store {
	return replay.NewStore(replayDir)
}

var replayListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getReplayStore()
		sessions := store.List()
		if len(sessions) == 0 {
			fmt.Println("No recorded sessions.")
			return nil
		}

		fmt.Printf("Sessions (%d):\n", len(sessions))
		for _, s := range sessions {
			duration := ""
			if !s.EndedAt.IsZero() {
				duration = fmt.Sprintf(" (%v)", s.EndedAt.Sub(s.StartedAt).Round(time.Second))
			}
			branch := ""
			if s.BranchFrom != "" {
				branch = fmt.Sprintf(" [branched from %s@%d]", s.BranchFrom, s.BranchAt)
			}
			fmt.Printf("  %s  %s  events=%d  tokens=%d  cost=$%.4f%s%s\n",
				s.ID, s.Name, len(s.Events), s.TotalTokens, s.TotalCost, duration, branch)
		}
		return nil
	},
}

var replayShowCmd = &cobra.Command{
	Use:   "show [session-id]",
	Short: "Show session events",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getReplayStore()
		session, err := store.Get(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("═══ Session: %s ═══\n", session.Name)
		fmt.Printf("ID: %s\n", session.ID)
		fmt.Printf("Agent: %s\n", session.AgentID)
		fmt.Printf("Status: %s\n", session.Status)
		fmt.Printf("Started: %s\n", session.StartedAt.Format(time.RFC3339))
		if !session.EndedAt.IsZero() {
			fmt.Printf("Ended: %s\n", session.EndedAt.Format(time.RFC3339))
		}
		fmt.Printf("Total: %d events, %d tokens, $%.4f\n",
			len(session.Events), session.TotalTokens, session.TotalCost)

		fmt.Println("\n── Events ──")
		for _, e := range session.Events {
			icon := "→"
			switch e.Type {
			case replay.EventPrompt:
				icon = "▶"
			case replay.EventResponse:
				icon = "◀"
			case replay.EventToolCall:
				icon = "🔧"
			case replay.EventToolResult:
				icon = "↩"
			case replay.EventError:
				icon = "✗"
			case replay.EventRetry:
				icon = "↻"
			}

			cost := ""
			if e.CostUSD > 0 {
				cost = fmt.Sprintf(" $%.4f", e.CostUSD)
			}
			dur := ""
			if e.Duration > 0 {
				dur = fmt.Sprintf(" (%v)", e.Duration.Round(time.Millisecond))
			}

			fmt.Printf("%s #%d [%s]%s%s\n", icon, e.Sequence, e.Type, cost, dur)
			if e.AgentID != "" {
				fmt.Printf("   Agent: %s", e.AgentID)
				if e.Model != "" {
					fmt.Printf("  Model: %s", e.Model)
				}
				fmt.Println()
			}
			// Show data summary
			if prompt, ok := e.Data["prompt"]; ok {
				fmt.Printf("   %s\n", truncStr(fmt.Sprintf("%v", prompt), 80))
			}
			if response, ok := e.Data["response"]; ok {
				fmt.Printf("   %s\n", truncStr(fmt.Sprintf("%v", response), 80))
			}
			if tool, ok := e.Data["tool"]; ok {
				fmt.Printf("   Tool: %v\n", tool)
			}
			if errMsg, ok := e.Data["error"]; ok {
				fmt.Printf("   Error: %v\n", errMsg)
			}
		}
		return nil
	},
}

var replayStepCmd = &cobra.Command{
	Use:   "step [session-id]",
	Short: "Step through session events interactively",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getReplayStore()
		session, err := store.Get(args[0])
		if err != nil {
			return err
		}

		player := replay.NewPlayer(session)

		fmt.Printf("Session: %s (%d events)\n", session.Name, len(session.Events))
		fmt.Println("Use 'next', 'prev', 'rewind', 'summary' subcommands or see 'forge replay show' for full listing.")
		_ = player

		// Show first event
		e := player.Next()
		if e != nil {
			fmt.Printf("\n#%d [%s] %s\n", e.Sequence, e.Type, e.Timestamp.Format(time.RFC3339))
		}
		return nil
	},
}

var replaySummaryCmd = &cobra.Command{
	Use:   "summary [session-id]",
	Short: "Show session summary statistics",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getReplayStore()
		session, err := store.Get(args[0])
		if err != nil {
			return err
		}

		player := replay.NewPlayer(session)
		summary := player.Summary()

		fmt.Printf("Session Summary: %s\n", summary.Name)
		fmt.Printf("  ID: %s\n", summary.SessionID)
		fmt.Printf("  Events: %d\n", summary.TotalEvents)
		fmt.Printf("  Duration: %v\n", summary.Duration.Round(time.Second))
		fmt.Printf("  Tokens: %d\n", summary.TotalTokens)
		fmt.Printf("  Cost: $%.4f\n", summary.TotalCost)
		fmt.Println("\n  By Type:")
		for typ, count := range summary.ByType {
			fmt.Printf("    %s: %d\n", typ, count)
		}
		return nil
	},
}

var replayCompareCmd = &cobra.Command{
	Use:   "compare [session-a] [session-b]",
	Short: "Compare two sessions",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getReplayStore()
		cmp, err := store.Compare(args[0], args[1])
		if err != nil {
			return err
		}

		fmt.Println("═══ Session Comparison ═══")
		fmt.Printf("  A: %s (%d events, %d tokens, $%.4f)\n", cmp.SessionA, cmp.EventsA, cmp.TokensA, cmp.CostA)
		fmt.Printf("  B: %s (%d events, %d tokens, $%.4f)\n", cmp.SessionB, cmp.EventsB, cmp.TokensB, cmp.CostB)
		fmt.Printf("\n  Token Diff: %+.1f%%\n", cmp.TokenDiff)
		fmt.Printf("  Cost Diff:  %+.1f%%\n", cmp.CostDiff)
		fmt.Printf("  Common Prefix: %d events\n", cmp.CommonPrefix)
		return nil
	},
}

var replayDeleteCmd = &cobra.Command{
	Use:   "delete [session-id]",
	Short: "Delete a recorded session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getReplayStore()
		return store.Delete(args[0])
	},
}
