package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/queue"
	"github.com/spf13/cobra"
)

func queueCmd() *cobra.Command {
	var priority int

	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage the task queue",
		Long: `Manage the persistent task queue for The Forge.
Tasks are processed in priority order with automatic retries.

Examples:
  forge queue add "process-data" --priority 2
  forge queue next
  forge queue list
  forge queue stats
  forge queue complete task-1 --result "done"`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "add [type]",
			Short: "Add a task to the queue",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				taskType := args[0]
				payload, _ := cmd.Flags().GetString("payload")

				q := queue.New("")
				task, err := q.Enqueue(taskType, payload, queue.Priority(priority))
				if err != nil {
					return err
				}

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Task %s enqueued", task.ID)))
				fmt.Printf("   Type:     %s\n", task.Type)
				fmt.Printf("   Priority: %d\n", task.Priority)
				return nil
			},
		},
		&cobra.Command{
			Use:   "next",
			Short: "Dequeue the next task",
			RunE: func(cmd *cobra.Command, args []string) error {
				q := queue.New("")
				task, err := q.Dequeue()
				if err != nil {
					return err
				}
				if task == nil {
					fmt.Println("Forge: No pending tasks")
					return nil
				}

				fmt.Println(pretty.InfoLine(fmt.Sprintf("Dequeued: %s", task.ID)))
				fmt.Printf("   Type:     %s\n", task.Type)
				fmt.Printf("   Priority: %d\n", task.Priority)
				fmt.Printf("   Payload:  %s\n", task.Payload)
				return nil
			},
		},
		&cobra.Command{
			Use:   "complete [id]",
			Short: "Mark a task as complete",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				id := args[0]
				result, _ := cmd.Flags().GetString("result")

				q := queue.New("")
				if err := q.Complete(id, result); err != nil {
					return err
				}

				fmt.Println(pretty.SuccessLine(fmt.Sprintf("Task %s completed", id)))
				return nil
			},
		},
		&cobra.Command{
			Use:   "fail [id]",
			Short: "Mark a task as failed",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				id := args[0]
				errMsg, _ := cmd.Flags().GetString("error")

				q := queue.New("")
				if err := q.Fail(id, errMsg); err != nil {
					return err
				}

				fmt.Println(pretty.ErrorLine(fmt.Sprintf("Task %s failed", id)))
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all tasks",
			RunE: func(cmd *cobra.Command, args []string) error {
				q := queue.New("")
				tasks := q.List()

				if len(tasks) == 0 {
					fmt.Println("Forge: No tasks in queue")
					return nil
				}

				fmt.Println(pretty.HeaderLine("Task Queue"))
				for _, t := range tasks {
					fmt.Printf("  %-12s %-10s %-5d %-10s %s\n",
						t.ID, t.Type, t.Priority, t.State, t.Payload)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "stats",
			Short: "Show queue statistics",
			RunE: func(cmd *cobra.Command, args []string) error {
				q := queue.New("")
				stats := q.Stats()

				fmt.Println(pretty.HeaderLine("Queue Statistics"))
				b, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Println(string(b))
				return nil
			},
		},
		&cobra.Command{
			Use:   "purge",
			Short: "Remove completed and cancelled tasks",
			RunE: func(cmd *cobra.Command, args []string) error {
				q := queue.New("")
				if err := q.Purge(); err != nil {
					return err
				}
				fmt.Println(pretty.SuccessLine("Purged completed and cancelled tasks"))
				return nil
			},
		},
	)

	cmd.Commands()[0].Flags().StringP("payload", "p", "", "Task payload (JSON)")
	cmd.Commands()[0].Flags().IntVarP(&priority, "priority", "P", 1, "Priority (0=low, 1=normal, 2=high, 3=urgent)")
	cmd.Commands()[2].Flags().StringP("result", "r", "", "Result message")
	cmd.Commands()[3].Flags().StringP("error", "e", "", "Error message")

	return cmd
}
