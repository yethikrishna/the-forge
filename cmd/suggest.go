package cmd

import (
	"fmt"
	"strings"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/suggest"
	"github.com/spf13/cobra"
)

func suggestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Context-aware agent and model suggestions",
		Long: `Suggest the best agent and model for the current context.

Analyzes the file path, language, error type, and task type
to recommend the optimal agent/model combination with alternatives.

Examples:
  forge suggest --file auth/login.go --task fix
  forge suggest --file auth/login.go --error "nil pointer dereference"
  forge suggest --task review
  forge suggest --file handlers_test.go
  forge suggest --language python --task test`,
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, _ := cmd.Flags().GetString("file")
			language, _ := cmd.Flags().GetString("language")
			taskType, _ := cmd.Flags().GetString("task")
			errorMsg, _ := cmd.Flags().GetString("error")
			errorType, _ := cmd.Flags().GetString("error-type")

			ctx := suggest.Context{
				FilePath:     filePath,
				Language:     language,
				TaskType:     taskType,
				ErrorMessage: errorMsg,
				ErrorType:    errorType,
				HasError:     errorMsg != "" || errorType != "",
				IsTest: strings.Contains(strings.ToLower(filePath), "test") ||
					strings.Contains(strings.ToLower(filePath), "_test") ||
					strings.Contains(strings.ToLower(filePath), ".spec."),
				IsConfig: strings.HasSuffix(strings.ToLower(filePath), ".yaml") ||
					strings.HasSuffix(strings.ToLower(filePath), ".yml") ||
					strings.HasSuffix(strings.ToLower(filePath), ".json") ||
					strings.HasSuffix(strings.ToLower(filePath), ".toml"),
			}

			s := suggest.Suggest(ctx)

			fmt.Println(pretty.HeaderLine("Agent Suggestion"))
			fmt.Print(suggest.FormatSuggestion(s))
			return nil
		},
	}

	cmd.Flags().String("file", "", "File path for context")
	cmd.Flags().String("language", "", "Programming language (auto-detected from file)")
	cmd.Flags().String("task", "", "Task type (fix, test, review, refactor, explain)")
	cmd.Flags().String("error", "", "Error message")
	cmd.Flags().String("error-type", "", "Error type (compile, runtime, test, etc.)")

	return cmd
}
