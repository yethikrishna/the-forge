package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/forge/sword/internal/duration/timer"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func runCmd() *cobra.Command {
	var listTasks bool

	cmd := &cobra.Command{
		Use:   "run [task]",
		Short: "Execute tasks defined in Forgefile",
		Long: `Run tasks defined in the project's Forgefile.
If no task is specified, runs the default task.

Examples:
  forge run
  forge run test
  forge run build
  forge run --list`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			forgefilePath := "Forgefile"
			if _, err := os.Stat(forgefilePath); err != nil {
				return fmt.Errorf("Forgefile not found in current directory")
			}

			tasks, err := parseForgefile(forgefilePath)
			if err != nil {
				return fmt.Errorf("parse Forgefile: %w", err)
			}

			if listTasks {
				fmt.Println(pretty.HeaderLine("Available Tasks"))
				if len(tasks) == 0 {
					fmt.Println("  No tasks defined")
				}
				for name, task := range tasks {
					fmt.Printf("  %-15s %s\n", name, task)
				}
				return nil
			}

			taskName := "default"
			if len(args) > 0 {
				taskName = args[0]
			}

			command, ok := tasks[taskName]
			if !ok {
				return fmt.Errorf("task %q not found in Forgefile", taskName)
			}

			fmt.Println(pretty.InfoLine(fmt.Sprintf("Running task: %s", taskName)))
			fmt.Printf("   Command: %s\n", command)
			fmt.Println()

			tm := timer.New()
			ctx := context.Background()

			shellCmd := exec.CommandContext(ctx, "sh", "-c", command)
			shellCmd.Stdout = os.Stdout
			shellCmd.Stderr = os.Stderr
			shellCmd.Stdin = os.Stdin

			if err := shellCmd.Run(); err != nil {
				fmt.Println(pretty.ErrorLine(fmt.Sprintf("Task %q failed (%v)", taskName, tm.String())))
				return err
			}

			fmt.Println()
			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Task %q completed (%s)", taskName, tm.String())))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&listTasks, "list", "l", false, "List available tasks")

	return cmd
}

// parseForgefile parses a simple Forgefile and extracts tasks.
func parseForgefile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tasks := make(map[string]string)
	inTasksSection := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for [tasks] section
		if line == "[tasks]" {
			inTasksSection = true
			continue
		}

		// Check for other sections
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inTasksSection = false
			continue
		}

		// Parse task definition: name = "command"
		if inTasksSection {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, "\"")
				tasks[name] = value
			}
		}
	}

	return tasks, nil
}
