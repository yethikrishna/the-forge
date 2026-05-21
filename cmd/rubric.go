package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/quality/rubric"
	"github.com/spf13/cobra"
)

func rubricCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rubric",
		Short: "Rubric-based output grading for agents",
		Long:  `Grade agent output against rubrics. Below-threshold outputs trigger automatic re-runs. Extends forge test with structured quality assessment.`,
	}

	var outputJSON bool
	var storeDir string
	cmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	cmd.PersistentFlags().StringVar(&storeDir, "dir", ".forge/rubrics", "Rubric storage directory")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available rubrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := rubric.NewGrader(storeDir)
			rubrics := g.ListRubrics()

			if outputJSON {
				data, _ := json.MarshalIndent(rubrics, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(rubrics) == 0 {
				fmt.Println("No rubrics found.")
				return nil
			}

			for _, r := range rubrics {
				fmt.Printf("%-20s %-30s threshold: %.0f%%  criteria: %d\n", r.ID, r.Name, r.PassThreshold, len(r.Criteria))
			}
			return nil
		},
	}

	// show
	showCmd := &cobra.Command{
		Use:   "show <rubric-id>",
		Short: "Show rubric details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := rubric.NewGrader(storeDir)
			r, err := g.GetRubric(args[0])
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(r, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(rubric.FormatRubric(r))
			return nil
		},
	}

	// grade
	gradeCmd := &cobra.Command{
		Use:   "grade <rubric-id> <agent-id>",
		Short: "Grade agent output against a rubric",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			g := rubric.NewGrader(storeDir)

			// Build scores from flags
			scoresStr, _ := cmd.Flags().GetStringSlice("scores")
			scores := make(map[string]float64)
			for _, s := range scoresStr {
				parts := rubricSplitKV(s)
				if len(parts) == 2 {
					var val float64
					fmt.Sscanf(parts[1], "%f", &val)
					scores[parts[0]] = val
				}
			}

			sessionID, _ := cmd.Flags().GetString("session")
			output, _ := cmd.Flags().GetString("output")

			grade, err := g.Grade(args[0], args[1], sessionID, output, scores)
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(grade, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(rubric.FormatGrade(grade))
			return nil
		},
	}
	gradeCmd.Flags().StringSlice("scores", nil, "Criterion scores: id=score pairs")
	gradeCmd.Flags().String("session", "", "Session ID")
	gradeCmd.Flags().String("output", "", "Output text being graded")

	// grades
	gradesCmd := &cobra.Command{
		Use:   "grades",
		Short: "List past grades",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := rubric.NewGrader(storeDir)

			agentID, _ := cmd.Flags().GetString("agent")
			rubricID, _ := cmd.Flags().GetString("rubric")

			grades := g.ListGrades(agentID, rubricID)

			if outputJSON {
				data, _ := json.MarshalIndent(grades, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(grades) == 0 {
				fmt.Println("No grades found.")
				return nil
			}

			fmt.Printf("%-15s %-15s %-15s %6s %6s %-6s\n", "GRADE ID", "AGENT", "RUBRIC", "SCORE", "PCT", "PASS")
			for _, grade := range grades {
				fmt.Printf("%-15s %-15s %-15s %5.1f/%-5.1f %5.1f%% %-6v\n",
					grade.ID[:12], grade.AgentID, grade.RubricID,
					grade.TotalScore, grade.MaxScore, grade.Percentage, grade.Passed)
			}
			return nil
		},
	}
	gradesCmd.Flags().String("agent", "", "Filter by agent")
	gradesCmd.Flags().String("rubric", "", "Filter by rubric")

	// stats
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show grading statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := rubric.NewGrader(storeDir)
			stats := g.Stats()

			if outputJSON {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(rubric.FormatStats(stats))
			return nil
		},
	}

	cmd.AddCommand(listCmd, showCmd, gradeCmd, gradesCmd, statsCmd)
	return cmd
}

func rubricSplitKV(s string) []string {
	for i, c := range s {
		if c == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
