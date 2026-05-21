package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/forge/sword/internal/learn"
	"github.com/spf13/cobra"
)

func learnCmdFn() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "learn",
		Short: "Interactive tutorial system",
		Long: `Hands-on lessons to learn Forge — step by step, with verification.

Progressive lessons from beginner to advanced. Each lesson has
concrete steps, commands to run, and explanations of why it matters.

Examples:
  forge learn list
  forge learn start your-first-agent
  forge learn next
  forge learn done
  forge learn skip
  forge learn hint
  forge learn progress
  forge learn stats
  forge learn show your-first-agent`,
	}

	var learnDir string
	cmd.PersistentFlags().StringVar(&learnDir, "dir", "", "learn data directory (default .forge/learn)")

	getStore := func() (*learn.Store, error) {
		dir := learnDir
		if dir == "" {
			dir = filepath.Join(".forge", "learn")
		}
		return learn.NewStore(dir)
	}

	// --- list ---
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List available lessons",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			filters := make(map[string]string)
			for _, k := range []string{"difficulty", "category", "tag"} {
				if v, _ := cmd.Flags().GetString(k); v != "" {
					filters[k] = v
				}
			}

			lessons, err := store.ListLessons(filters)
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(lessons, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Lessons (%d)\n\n", len(lessons))
				progress := store.GetAllProgress()
				w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tTITLE\tDIFFICULTY\tDURATION\tSTATUS\tSTEPS\n")
				for _, l := range lessons {
					status := "new"
					done := 0
					if p, ok := progress[l.ID]; ok {
						status = p.Status
						done = p.StepsDone
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d/%d\n",
						l.ID, l.Title, l.Difficulty, l.Duration, status, done, len(l.Steps))
				}
				w.Flush()
			}
			return nil
		},
	}
	listCmd.Flags().String("difficulty", "", "Filter by difficulty")
	listCmd.Flags().String("category", "", "Filter by category")
	listCmd.Flags().String("tag", "", "Filter by tag")
	cmd.AddCommand(listCmd)

	// --- show ---
	showCmd := &cobra.Command{
		Use:   "show <lesson-id>",
		Short: "Show lesson details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			l, err := store.GetLesson(args[0])
			if err != nil {
				return err
			}

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(l, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Lesson: %s\n", l.Title)
				fmt.Printf("  ID:           %s\n", l.ID)
				fmt.Printf("  Difficulty:   %s\n", l.Difficulty)
				fmt.Printf("  Category:     %s\n", l.Category)
				fmt.Printf("  Duration:     %s\n", l.Duration)
				fmt.Printf("  Description:  %s\n", l.Description)
				if len(l.Prerequisites) > 0 {
					fmt.Printf("  Prerequisites: %v\n", l.Prerequisites)
				}
				fmt.Println()
				for _, step := range l.Steps {
					icon := "○"
					switch step.Status {
					case learn.StepCompleted:
						icon = "✓"
					case learn.StepInProgress:
						icon = "→"
					case learn.StepSkipped:
						icon = "⊘"
					}
					fmt.Printf("  %s %d. %s\n", icon, step.Order, step.Title)
					if step.Instruction != "" {
						fmt.Printf("     %s\n", step.Instruction)
					}
					if step.Command != "" {
						fmt.Printf("     $ %s\n", step.Command)
					}
				}
			}
			return nil
		},
	}
	cmd.AddCommand(showCmd)

	// --- start ---
	startCmd := &cobra.Command{
		Use:   "start <lesson-id>",
		Short: "Start a lesson",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			l, p, err := store.StartLesson(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("📚 Started: %s\n", l.Title)
			fmt.Printf("   Difficulty: %s | Duration: %s | %d steps\n\n", l.Difficulty, l.Duration, len(l.Steps))

			if p.CurrentStep > 0 && p.CurrentStep <= len(l.Steps) {
				step := l.Steps[p.CurrentStep-1]
				printStep(step)
			}
			return nil
		},
	}
	cmd.AddCommand(startCmd)

	// --- next / current ---
	nextCmd := &cobra.Command{
		Use:   "next <lesson-id>",
		Short: "Show current step in a lesson",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			step, err := store.GetCurrentStep(args[0])
			if err != nil {
				return err
			}
			printStep(*step)

			p, _ := store.GetProgress(args[0])
			fmt.Printf("\nProgress: step %d | %d completed | status: %s\n", p.CurrentStep, p.StepsDone, p.Status)
			return nil
		},
	}
	cmd.AddCommand(nextCmd)

	// --- done ---
	doneCmd := &cobra.Command{
		Use:   "done <lesson-id>",
		Short: "Mark current step as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			step, err := store.GetCurrentStep(args[0])
			if err != nil {
				return err
			}

			completed, p, err := store.CompleteStep(args[0], step.ID)
			if err != nil {
				return err
			}

			fmt.Printf("✓ Completed: %s\n", completed.Title)
			if completed.VerifyMsg != "" {
				fmt.Printf("  %s\n", completed.VerifyMsg)
			}

			if p.Status == "completed" {
				fmt.Printf("\n🎉 Lesson complete! Score: %d/100\n", p.Score)
			} else {
				// Show next step.
				nextStep, err := store.GetCurrentStep(args[0])
				if err == nil {
					fmt.Println()
					printStep(*nextStep)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(doneCmd)

	// --- skip ---
	skipCmd := &cobra.Command{
		Use:   "skip <lesson-id>",
		Short: "Skip current step",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			step, err := store.GetCurrentStep(args[0])
			if err != nil {
				return err
			}

			p, err := store.SkipStep(args[0], step.ID)
			if err != nil {
				return err
			}

			fmt.Printf("⊘ Skipped: %s\n", step.Title)

			if p.Status == "completed" {
				fmt.Printf("\nLesson complete! Score: %d/100\n", p.Score)
			} else {
				nextStep, err := store.GetCurrentStep(args[0])
				if err == nil {
					fmt.Println()
					printStep(*nextStep)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(skipCmd)

	// --- hint ---
	hintCmd := &cobra.Command{
		Use:   "hint <lesson-id>",
		Short: "Show hint for current step",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			step, err := store.GetCurrentStep(args[0])
			if err != nil {
				return err
			}

			if step.Hint == "" {
				fmt.Println("No hint available for this step.")
			} else {
				fmt.Printf("💡 Hint: %s\n", step.Hint)
			}
			return nil
		},
	}
	cmd.AddCommand(hintCmd)

	// --- progress ---
	progressCmd := &cobra.Command{
		Use:   "progress [lesson-id]",
		Short: "Show learning progress",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			if len(args) > 0 {
				p, err := store.GetProgress(args[0])
				if err != nil {
					return err
				}
				output, _ := cmd.Flags().GetString("output")
				if output == "json" {
					data, _ := json.MarshalIndent(p, "", "  ")
					fmt.Println(string(data))
				} else {
					fmtProgress(p)
				}
			} else {
				all := store.GetAllProgress()
				output, _ := cmd.Flags().GetString("output")
				if output == "json" {
					data, _ := json.MarshalIndent(all, "", "  ")
					fmt.Println(string(data))
				} else {
					if len(all) == 0 {
						fmt.Println("No progress yet. Start a lesson with: forge learn start <lesson-id>")
					} else {
						fmt.Printf("Progress (%d lessons)\n\n", len(all))
						w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
						fmt.Fprintf(w, "LESSON\tSTATUS\tSTEPS\tSCORE\n")
						for id, p := range all {
							fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", id, p.Status, p.StepsDone, p.Score)
						}
						w.Flush()
					}
				}
			}
			return nil
		},
	}
	cmd.AddCommand(progressCmd)

	// --- reset ---
	resetCmd := &cobra.Command{
		Use:   "reset <lesson-id>",
		Short: "Reset progress for a lesson",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			if err := store.ResetProgress(args[0]); err != nil {
				return err
			}
			fmt.Printf("Progress reset for %s\n", args[0])
			return nil
		},
	}
	cmd.AddCommand(resetCmd)

	// --- stats ---
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show learning statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			stats := store.GetStats()

			output, _ := cmd.Flags().GetString("output")
			if output == "json" {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Println("Learning Statistics")
				fmt.Println("===================")
				fmt.Printf("  Total Lessons:  %d\n", stats.TotalLessons)
				fmt.Printf("  Completed:      %d\n", stats.CompletedCount)
				fmt.Printf("  In Progress:    %d\n", stats.InProgressCount)
				fmt.Printf("  Not Started:    %d\n", stats.NotStartedCount)
				fmt.Printf("  Avg Score:      %.0f/100\n", stats.AvgScore)
				fmt.Printf("  Steps Done:     %d/%d\n", stats.StepsCompleted, stats.TotalSteps)
				fmt.Println()
				fmt.Println("  By Difficulty:")
				for d, c := range stats.ByDifficulty {
					fmt.Printf("    %-16s %d\n", d, c)
				}
				fmt.Println()
				fmt.Println("  By Category:")
				for c, n := range stats.ByCategory {
					fmt.Printf("    %-20s %d\n", c, n)
				}
			}
			return nil
		},
	}
	cmd.AddCommand(statsCmd)

	// --- create ---
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new lesson",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}

			title, _ := cmd.Flags().GetString("title")
			desc, _ := cmd.Flags().GetString("desc")
			difficulty, _ := cmd.Flags().GetString("difficulty")
			category, _ := cmd.Flags().GetString("category")
			duration, _ := cmd.Flags().GetString("duration")
			stepsStr, _ := cmd.Flags().GetString("steps")

			var steps []learn.Step
			if stepsStr != "" {
				for i, s := range strings.Split(stepsStr, ",") {
					s = strings.TrimSpace(s)
					if s != "" {
						steps = append(steps, learn.Step{Title: s, Instruction: fmt.Sprintf("Complete: %s", s), Order: i + 1})
					}
				}
			}

			l, err := store.CreateLesson(learn.Lesson{
				Title:       title,
				Description: desc,
				Difficulty:  learn.Difficulty(difficulty),
				Category:    category,
				Duration:    duration,
				Steps:       steps,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Created lesson: %s (%d steps)\n", l.ID, len(l.Steps))
			return nil
		},
	}
	createCmd.Flags().String("title", "", "Lesson title")
	createCmd.Flags().String("desc", "", "Description")
	createCmd.Flags().String("difficulty", "beginner", "Difficulty: beginner, intermediate, advanced")
	createCmd.Flags().String("category", "custom", "Category")
	createCmd.Flags().String("duration", "5 min", "Estimated duration")
	createCmd.Flags().String("steps", "", "Comma-separated step titles")
	cmd.AddCommand(createCmd)

	// --- delete ---
	deleteCmd := &cobra.Command{
		Use:   "delete <lesson-id>",
		Short: "Delete a lesson",
		Args:  cobra.ExactArgs(1),
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			if err := store.DeleteLesson(args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted lesson: %s\n", args[0])
			return nil
		},
	}
	cmd.AddCommand(deleteCmd)

	// --- export ---
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export progress as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore()
			if err != nil {
				return err
			}
			data, err := store.ExportProgress()
			if err != nil {
				return err
			}
			outFile, _ := cmd.Flags().GetString("output-file")
			if outFile != "" {
				return os.WriteFile(outFile, data, 0o644)
			}
			fmt.Println(string(data))
			return nil
		},
	}
	exportCmd.Flags().String("output-file", "", "Write to file")
	cmd.AddCommand(exportCmd)

	return cmd
}

func printStep(step learn.Step) {
	fmt.Printf("Step %d: %s\n", step.Order, step.Title)
	fmt.Printf("  %s\n", step.Instruction)
	if step.Command != "" {
		fmt.Printf("  $ %s\n", step.Command)
	}
	if step.Explanation != "" {
		fmt.Printf("  Why: %s\n", step.Explanation)
	}
}

func fmtProgress(p *learn.Progress) {
	fmt.Printf("Lesson: %s\n", p.LessonID)
	fmt.Printf("  Status:     %s\n", p.Status)
	fmt.Printf("  Steps:      %d done, current: %d\n", p.StepsDone, p.CurrentStep)
	if p.Score > 0 {
		fmt.Printf("  Score:      %d/100\n", p.Score)
	}
	if p.StartedAt != nil {
		fmt.Printf("  Started:    %s\n", p.StartedAt.Format(time.RFC3339))
	}
	if p.CompletedAt != nil {
		fmt.Printf("  Completed:  %s\n", p.CompletedAt.Format(time.RFC3339))
	}
}
