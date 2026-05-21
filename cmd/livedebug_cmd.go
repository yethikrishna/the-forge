package cmd

import (
	"fmt"
	"time"

	"github.com/forge/sword/internal/livedebug"
	"github.com/spf13/cobra"
)

var livedebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Real-time collaborative debugging with AI assistance",
	Long: `Debug commands in real-time with an AI agent watching the
terminal output, analyzing errors, and providing suggestions.

Examples:
  forge debug sessions                  # List debug sessions
  forge debug show dbg-123              # Show session details
  forge debug suggest dbg-123           # Get suggestions
  forge debug apply dbg-123 sug-456     # Apply a suggestion
  forge debug ask dbg-123 "why did it fail?"
  forge debug stop dbg-123              # Stop a session`,
}

var (
	debugOutput string
)

func init() {
	livedebugCmd.AddCommand(debugSessionsCmd)
	livedebugCmd.AddCommand(debugShowCmd)
	livedebugCmd.AddCommand(debugSuggestCmd)
	livedebugCmd.AddCommand(debugApplyCmd)
	livedebugCmd.AddCommand(debugAskCmd)
	livedebugCmd.AddCommand(debugStopCmd)

	livedebugCmd.PersistentFlags().StringVarP(&debugOutput, "output", "o", "text", "output format: text, json")
}

var debugSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List debug sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getDebugEngine()
		sessions := engine.ListSessions()

		if len(sessions) == 0 {
			fmt.Println("No debug sessions.")
			return nil
		}

		for _, s := range sessions {
			status := s.State.String()
			if s.ExitCode != 0 {
				status = fmt.Sprintf("exit:%d", s.ExitCode)
			}
			fmt.Printf("%s  %s  %s  output=%d\n",
				s.ID, s.Command, status, len(s.Output))
		}
		return nil
	},
}

var debugShowCmd = &cobra.Command{
	Use:   "show [session-id]",
	Short: "Show debug session details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getDebugEngine()
		session, err := engine.GetSession(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("═══ Debug Session: %s ═══\n", session.ID)
		fmt.Printf("Command: %s\n", session.Command)
		fmt.Printf("Working Dir: %s\n", session.WorkingDir)
		fmt.Printf("State: %s\n", session.State)
		fmt.Printf("Duration: %v\n", session.EndTime.Sub(session.StartTime).Round(time.Second))
		fmt.Printf("Exit Code: %d\n", session.ExitCode)
		fmt.Printf("Output Lines: %d\n", len(session.Output))
		fmt.Printf("Suggestions: %d\n", len(session.Suggestions))

		if session.RootCause != "" {
			fmt.Printf("\nRoot Cause: %s\n", session.RootCause)
		}
		if session.FixApplied != "" {
			fmt.Printf("Fix Applied: %s\n", session.FixApplied)
		}

		if len(session.Output) > 0 {
			fmt.Println("\n── Recent Output ──")
			start := 0
			if len(session.Output) > 20 {
				start = len(session.Output) - 20
				fmt.Printf("  ... (%d earlier lines omitted)\n", start)
			}
			for _, line := range session.Output[start:] {
				prefix := "  "
				if line.Stream == "stderr" {
					prefix = "  ✗ "
				}
				fmt.Printf("%s%s\n", prefix, line.Content)
			}
		}

		return nil
	},
}

var debugSuggestCmd = &cobra.Command{
	Use:   "suggest [session-id]",
	Short: "Get debugging suggestions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getDebugEngine()
		suggestions, err := engine.GetSuggestions(args[0])
		if err != nil {
			return err
		}

		if len(suggestions) == 0 {
			fmt.Println("No suggestions available.")
			return nil
		}

		fmt.Println("═══ Debugging Suggestions ═══")
		for i, s := range suggestions {
			autoTag := ""
			if s.AutoApply {
				autoTag = " [auto-apply]"
			}
			fmt.Printf("\n%d. [%s] %s%s (confidence: %.0f%%)\n",
				i+1, s.Category, s.Title, autoTag, s.Confidence*100)
			fmt.Printf("   %s\n", s.Description)
			fmt.Printf("   Action: %s\n", s.Action)
		}
		return nil
	},
}

var debugApplyCmd = &cobra.Command{
	Use:   "apply [session-id] [suggestion-id]",
	Short: "Apply a debugging suggestion",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getDebugEngine()
		err := engine.ApplySuggestion(args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Suggestion %s applied to session %s.\n", args[1], args[0])
		return nil
	},
}

var debugAskCmd = &cobra.Command{
	Use:   "ask [session-id] [question]",
	Short: "Ask a question about the debug session",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getDebugEngine()
		question := ""
		for i, a := range args[1:] {
			if i > 0 {
				question += " "
			}
			question += a
		}

		suggestion, err := engine.AskQuestion(args[0], question)
		if err != nil {
			return err
		}

		if suggestion == nil {
			fmt.Println("No answer available.")
			return nil
		}

		fmt.Printf("[%s] %s\n", suggestion.Category, suggestion.Title)
		fmt.Printf("%s\n", suggestion.Description)
		if suggestion.Action != "" {
			fmt.Printf("Suggested action: %s\n", suggestion.Action)
		}
		return nil
	},
}

var debugStopCmd = &cobra.Command{
	Use:   "stop [session-id]",
	Short: "Stop a debug session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engine := getDebugEngine()
		err := engine.StopSession(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Session %s stopped.\n", args[0])
		return nil
	},
}

var globalDebugEngine = livedebug.NewEngine()

func getDebugEngine() *livedebug.Engine {
	return globalDebugEngine
}
