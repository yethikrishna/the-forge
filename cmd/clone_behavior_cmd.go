package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/forge/sword/internal/clonebehavior"
	"github.com/spf13/cobra"
)

var cloneBehaviorCmd = &cobra.Command{
	Use:   "clone-behavior",
	Short: "Record human tasks and generate agent configurations",
	Long: `Record a human performing a task, analyze the patterns,
and generate an agent configuration that can repeat it automatically.

Workflow:
  1. forge clone-behavior record "task name"     — Start recording
  2. forge clone-behavior command "go build"     — Record a command
  3. forge clone-behavior read main.go           — Record file read
  4. forge clone-behavior write output.txt       — Record file write
  5. forge clone-behavior decision "Use X" --reason "..." — Record decision
  6. forge clone-behavior stop                   — Stop recording
  7. forge clone-behavior analyze [recording-id] — Analyze patterns
  8. forge clone-behavior generate [recording-id] — Generate agent config`,
}

var (
	cloneOutput      string
	cloneReason      string
	cloneDescription string
)

func init() {
	cloneBehaviorCmd.AddCommand(cloneRecordCmd)
	cloneBehaviorCmd.AddCommand(cloneCommandActionCmd)
	cloneBehaviorCmd.AddCommand(cloneReadActionCmd)
	cloneBehaviorCmd.AddCommand(cloneWriteActionCmd)
	cloneBehaviorCmd.AddCommand(cloneEditActionCmd)
	cloneBehaviorCmd.AddCommand(cloneDecisionActionCmd)
	cloneBehaviorCmd.AddCommand(cloneSearchActionCmd)
	cloneBehaviorCmd.AddCommand(clonePauseCmd)
	cloneBehaviorCmd.AddCommand(cloneResumeCmd)
	cloneBehaviorCmd.AddCommand(cloneStopCmd)
	cloneBehaviorCmd.AddCommand(cloneAnalyzeCmd)
	cloneBehaviorCmd.AddCommand(cloneGenerateCmd)
	cloneBehaviorCmd.AddCommand(cloneListCmd)
	cloneBehaviorCmd.AddCommand(cloneShowCmd)

	cloneRecordCmd.Flags().StringVar(&cloneDescription, "desc", "", "task description")
	cloneDecisionActionCmd.Flags().StringVar(&cloneReason, "reason", "", "reasoning for the decision")
	cloneCommandActionCmd.Flags().Duration("duration", 0, "command duration")
	cloneEditActionCmd.Flags().String("old", "", "old text being replaced")
	cloneEditActionCmd.Flags().String("new", "", "new text being inserted")
}

var cloneRecordCmd = &cobra.Command{
	Use:   "record [name]",
	Short: "Start recording a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		desc := cloneDescription
		if desc == "" {
			desc = args[0]
		}
		rec := clonebehavior.NewRecorder(args[0], desc)
		setActiveRecorder(rec)
		fmt.Printf("Recording started: %s\n", args[0])
		fmt.Println("Use clone-behavior subcommands to record actions. Stop with 'forge clone-behavior stop'.")
		return nil
	},
}

var cloneCommandActionCmd = &cobra.Command{
	Use:   "command [command]",
	Short: "Record a command execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording. Start with 'forge clone-behavior record'")
		}
		duration, _ := cmd.Flags().GetDuration("duration")
		err := rec.RecordCommand(args[0], duration)
		if err != nil {
			return err
		}
		fmt.Printf("Command recorded: %s\n", args[0])
		return nil
	},
}

var cloneReadActionCmd = &cobra.Command{
	Use:   "read [file-path]",
	Short: "Record a file read",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording")
		}
		err := rec.RecordFileRead(args[0], "")
		if err != nil {
			return err
		}
		fmt.Printf("File read recorded: %s\n", args[0])
		return nil
	},
}

var cloneWriteActionCmd = &cobra.Command{
	Use:   "write [file-path]",
	Short: "Record a file write",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording")
		}
		err := rec.RecordFileWrite(args[0], "")
		if err != nil {
			return err
		}
		fmt.Printf("File write recorded: %s\n", args[0])
		return nil
	},
}

var cloneEditActionCmd = &cobra.Command{
	Use:   "edit [file-path]",
	Short: "Record a file edit",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording")
		}
		oldText, _ := cmd.Flags().GetString("old")
		newText, _ := cmd.Flags().GetString("new")
		err := rec.RecordFileEdit(args[0], oldText, newText)
		if err != nil {
			return err
		}
		fmt.Printf("File edit recorded: %s\n", args[0])
		return nil
	},
}

var cloneDecisionActionCmd = &cobra.Command{
	Use:   "decision [decision]",
	Short: "Record a decision point",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording")
		}
		err := rec.RecordDecision(args[0], cloneReason)
		if err != nil {
			return err
		}
		fmt.Printf("Decision recorded: %s\n", args[0])
		return nil
	},
}

var cloneSearchActionCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Record a search query",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording")
		}
		err := rec.RecordSearch(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Search recorded: %s\n", args[0])
		return nil
	},
}

var clonePauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the active recording",
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording")
		}
		rec.Pause()
		fmt.Println("Recording paused.")
		return nil
	},
}

var cloneResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused recording",
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording")
		}
		rec.Resume()
		fmt.Println("Recording resumed.")
		return nil
	},
}

var cloneStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the active recording",
	RunE: func(cmd *cobra.Command, args []string) error {
		rec := getActiveRecorder()
		if rec == nil {
			return fmt.Errorf("no active recording")
		}
		result := rec.Stop()
		getCloneStore().SaveRecording(result)
		clearActiveRecorder()
		fmt.Printf("Recording stopped: %s (%d actions, %v)\n",
			result.Name, len(result.Actions), result.EndTime.Sub(result.StartTime).Round(time.Second))
		fmt.Printf("ID: %s\n", result.ID)
		return nil
	},
}

var cloneAnalyzeCmd = &cobra.Command{
	Use:   "analyze [recording-id]",
	Short: "Analyze a recording and extract patterns",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var recording *clonebehavior.Recording
		var err error
		if len(args) > 0 {
			recording, err = getCloneStore().GetRecording(args[0])
			if err != nil {
				return err
			}
		} else {
			rec := getActiveRecorder()
			if rec == nil {
				return fmt.Errorf("specify a recording ID or have an active recording")
			}
			recording = rec.GetRecording()
		}
		analyzer := clonebehavior.NewAnalyzer()
		patterns, err := analyzer.Analyze(recording)
		if err != nil {
			return err
		}
		if cloneOutput == "json" {
			data, _ := json.MarshalIndent(patterns, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Printf("═══ Pattern Analysis: %s ═══\n", recording.Name)
		fmt.Printf("Actions analyzed: %d\n", len(recording.Actions))
		fmt.Printf("Patterns found: %d\n\n", len(patterns))
		for i, p := range patterns {
			fmt.Printf("Pattern %d: %s\n", i+1, p.Name)
			fmt.Printf("  Description: %s\n", p.Description)
			fmt.Printf("  Frequency: %d  Confidence: %.0f%%\n", p.Frequency, p.Confidence*100)
			fmt.Printf("  Steps: %d\n", len(p.Actions))
			if len(p.DecisionPoints) > 0 {
				fmt.Printf("  Decision points: %d\n", len(p.DecisionPoints))
			}
			fmt.Println()
		}
		return nil
	},
}

var cloneGenerateCmd = &cobra.Command{
	Use:   "generate [recording-id]",
	Short: "Generate an agent configuration from a recording",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var recording *clonebehavior.Recording
		var err error
		if len(args) > 0 {
			recording, err = getCloneStore().GetRecording(args[0])
			if err != nil {
				return err
			}
		} else {
			rec := getActiveRecorder()
			if rec == nil {
				return fmt.Errorf("specify a recording ID or have an active recording")
			}
			recording = rec.GetRecording()
		}
		analyzer := clonebehavior.NewAnalyzer()
		patterns, err := analyzer.Analyze(recording)
		if err != nil {
			return err
		}
		generator := clonebehavior.NewGenerator()
		config := generator.Generate(recording, patterns)
		getCloneStore().SaveConfig(config)
		if cloneOutput == "json" {
			data, _ := json.MarshalIndent(config, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Printf("═══ Generated Agent: %s ═══\n", config.Name)
		fmt.Printf("Description: %s\n", config.Description)
		fmt.Printf("Confidence: %.0f%%\n", config.Confidence*100)
		fmt.Printf("Model: %s  Temperature: %.1f\n", config.Model, config.Temperature)
		fmt.Printf("Tools: %v\n", config.Tools)
		fmt.Printf("Parameters: %d\n", len(config.Parameters))
		fmt.Printf("Patterns: %d\n\n", len(config.Patterns))
		fmt.Println("── Instructions ──")
		fmt.Println(config.Instructions)
		return nil
	},
}

var cloneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all recordings and generated configs",
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getCloneStore()
		recordings := store.ListRecordings()
		configs := store.ListConfigs()
		if cloneOutput == "json" {
			data, _ := json.MarshalIndent(map[string]any{
				"recordings": recordings,
				"configs":    configs,
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Println("═══ Recordings ═══")
		if len(recordings) == 0 {
			fmt.Println("  (none)")
		}
		for _, r := range recordings {
			fmt.Printf("  %s  %s  actions=%d  status=%s\n",
				r.ID, r.Name, len(r.Actions), r.Status)
		}
		fmt.Println("\n═══ Generated Agents ═══")
		if len(configs) == 0 {
			fmt.Println("  (none)")
		}
		for _, c := range configs {
			fmt.Printf("  %s  confidence=%.0f%%  tools=%v\n",
				c.Name, c.Confidence*100, c.Tools)
		}
		return nil
	},
}

var cloneShowCmd = &cobra.Command{
	Use:   "show [recording-id]",
	Short: "Show recording details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := getCloneStore()
		rec, err := store.GetRecording(args[0])
		if err != nil {
			return err
		}
		if cloneOutput == "json" {
			data, _ := json.MarshalIndent(rec, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Printf("═══ Recording: %s ═══\n", rec.Name)
		fmt.Printf("ID: %s\n", rec.ID)
		fmt.Printf("Description: %s\n", rec.Description)
		fmt.Printf("Status: %s\n", rec.Status)
		fmt.Printf("Duration: %v\n", rec.EndTime.Sub(rec.StartTime).Round(time.Second))
		fmt.Printf("Actions: %d\n\n", len(rec.Actions))
		for i, act := range rec.Actions {
			fmt.Printf("  %d. [%s] ", i+1, act.Type)
			switch act.Type {
			case clonebehavior.ActionCommand:
				fmt.Printf("%s", act.Command)
			case clonebehavior.ActionFileRead, clonebehavior.ActionFileWrite, clonebehavior.ActionFileEdit:
				fmt.Printf("%s", act.FilePath)
			case clonebehavior.ActionDecision:
				fmt.Printf("%s (%s)", act.Decision, act.Reasoning)
			case clonebehavior.ActionSearch:
				fmt.Printf("%s", act.Query)
			}
			fmt.Println()
		}
		return nil
	},
}

// Global state for the active recording session
var (
	activeRecorder *clonebehavior.Recorder
	cloneStoreInst = clonebehavior.NewStore()
)

func setActiveRecorder(r *clonebehavior.Recorder) { activeRecorder = r }
func getActiveRecorder() *clonebehavior.Recorder  { return activeRecorder }
func clearActiveRecorder()                        { activeRecorder = nil }
func getCloneStore() *clonebehavior.Store         { return cloneStoreInst }
