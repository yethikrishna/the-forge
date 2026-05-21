package cmd

import (
	"fmt"
	"os"

	"github.com/forge/sword/internal/diffx"
	"github.com/spf13/cobra"
)

var diffxCmd = &cobra.Command{
	Use:   "diffx",
	Short: "Semantic code diff",
	Long:  `Compare code files with structural understanding. Detects moved, renamed, and reformatted code — not just line-level changes.`,
}

func init() {
	diffxCmd.AddCommand(diffxFileCmd)
	diffxCmd.AddCommand(diffxStatsCmd)
}

// diffx file
var diffxFileCmd = &cobra.Command{
	Use:   "file [old-file] [new-file]",
	Short: "Semantic diff between two files",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldData, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("reading %s: %w", args[0], err)
		}

		newData, err := os.ReadFile(args[1])
		if err != nil {
			return fmt.Errorf("reading %s: %w", args[1], err)
		}

		lang := diffx.DetectLanguage(args[0])
		if flagLang, _ := cmd.Flags().GetString("lang"); flagLang != "" {
			lang = diffx.Language(flagLang)
		}

		d := diffx.NewDiffer(lang)
		result := d.Diff(string(oldData), string(newData))
		result.OldFile = args[0]
		result.NewFile = args[1]

		outputFormat, _ := cmd.Flags().GetString("output")
		switch outputFormat {
		case "json":
			// JSON output
			fmt.Printf("{\"language\":\"%s\",\"stats\":%v}\n", result.Language, result.Stats)
		default:
			fmt.Println(diffx.RenderDiff(result))
		}

		return nil
	},
}

// diffx stats
var diffxStatsCmd = &cobra.Command{
	Use:   "stats [old-file] [new-file]",
	Short: "Show diff statistics only",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldData, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		newData, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}

		lang := diffx.DetectLanguage(args[0])
		d := diffx.NewDiffer(lang)
		result := d.Diff(string(oldData), string(newData))

		fmt.Printf("Language: %s\n", result.Language)
		fmt.Printf("Added: %d\n", result.Stats.Added)
		fmt.Printf("Removed: %d\n", result.Stats.Removed)
		fmt.Printf("Modified: %d\n", result.Stats.Modified)
		fmt.Printf("Moved: %d\n", result.Stats.Moved)
		fmt.Printf("Renamed: %d\n", result.Stats.Renamed)
		fmt.Printf("Reformatted: %d\n", result.Stats.Reformatted)
		fmt.Printf("Unchanged: %d\n", result.Stats.Unchanged)
		return nil
	},
}

func init() {
	diffxFileCmd.Flags().String("lang", "", "Override language detection (go, python, typescript, rust, java)")
	diffxFileCmd.Flags().String("output", "text", "Output format (text, json)")
}
