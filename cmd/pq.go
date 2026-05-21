package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/persistentqueue"
	"github.com/spf13/cobra"
)

var pqCmd = &cobra.Command{
	Use:   "pq",
	Short: "Persistent task queue",
	Long:  "Manage persistent task queues with priority ordering, deduplication, and TTL support. Tasks survive restarts.",
}

var (
	pqPath     string
	pqQueue    string
	pqPriority int
	pqLimit    int
	pqPayload  string
	pqMaxRetry int
)

func init() {
	pqCmd.AddCommand(pqEnqueueCmd)
	pqCmd.AddCommand(pqDequeueCmd)
	pqCmd.AddCommand(pqListCmd)
	pqCmd.AddCommand(pqCompleteCmd)
	pqCmd.AddCommand(pqFailCmd)
	pqCmd.AddCommand(pqCancelCmd)
	pqCmd.AddCommand(pqStatsCmd)
	pqCmd.AddCommand(pqPurgeCmd)
	pqCmd.AddCommand(pqReclaimCmd)

	pqCmd.PersistentFlags().StringVar(&pqPath, "db", ".forge/queue.db", "Queue database path")
	pqEnqueueCmd.Flags().StringVar(&pqQueue, "queue", "default", "Queue name")
	pqEnqueueCmd.Flags().IntVar(&pqPriority, "priority", 1, "Priority (0=low, 1=normal, 2=high, 3=critical)")
	pqEnqueueCmd.Flags().StringVar(&pqPayload, "payload", "", "Task payload")
	pqEnqueueCmd.Flags().IntVar(&pqMaxRetry, "max-retries", 3, "Max retries")
	pqDequeueCmd.Flags().StringVar(&pqQueue, "queue", "default", "Queue name")
	pqListCmd.Flags().StringVar(&pqQueue, "queue", "", "Filter by queue")
	pqListCmd.Flags().IntVar(&pqLimit, "limit", 20, "Max tasks to list")
	pqStatsCmd.Flags().StringVar(&pqQueue, "queue", "default", "Queue name")
	pqPurgeCmd.Flags().StringVar(&pqDuration, "older-than", "24h", "Purge tasks older than this duration")
}

var pqDuration string

func getPQ() (*persistentqueue.Queue, error) {
	return persistentqueue.NewQueue(pqPath)
}

var pqEnqueueCmd = &cobra.Command{
	Use:   "enqueue",
	Short: "Enqueue a task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pqPayload == "" {
			return fmt.Errorf("--payload is required")
		}
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()

		task := &persistentqueue.Task{
			Queue:      pqQueue,
			Payload:    pqPayload,
			Priority:   persistentqueue.Priority(pqPriority),
			MaxRetries: pqMaxRetry,
		}
		if err := q.Enqueue(task); err != nil {
			return err
		}
		fmt.Printf("Enqueued: %s (queue: %s, priority: %d)\n", task.ID, task.Queue, task.Priority)
		return nil
	},
}

var pqDequeueCmd = &cobra.Command{
	Use:   "dequeue",
	Short: "Dequeue next task",
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()

		task, err := q.Dequeue(pqQueue)
		if err != nil {
			return err
		}
		if task == nil {
			fmt.Println("No tasks available.")
			return nil
		}
		printJSON(map[string]interface{}{
			"id":       task.ID,
			"queue":    task.Queue,
			"priority": task.Priority,
			"status":   task.Status,
			"payload":  task.PayloadJSON,
		})
		return nil
	},
}

var pqListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()

		statusFilter := persistentqueue.TaskStatus("")
		if len(args) > 0 {
			statusFilter = persistentqueue.TaskStatus(args[0])
		}

		tasks, err := q.List(pqQueue, statusFilter, pqLimit)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}
		fmt.Printf("Tasks (%d):\n", len(tasks))
		for _, t := range tasks {
			fmt.Printf("  %s [%s] priority:%d queue:%s\n", t.ID, t.Status, t.Priority, t.Queue)
		}
		return nil
	},
}

var pqCompleteCmd = &cobra.Command{
	Use:   "complete [task-id]",
	Short: "Mark task as completed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()
		return q.Complete(args[0], nil)
	},
}

var pqFailCmd = &cobra.Command{
	Use:   "fail [task-id]",
	Short: "Mark task as failed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()
		return q.Fail(args[0], fmt.Errorf("manually failed"))
	},
}

var pqCancelCmd = &cobra.Command{
	Use:   "cancel [task-id]",
	Short: "Cancel a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()
		return q.Cancel(args[0])
	},
}

var pqStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Queue statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()

		stats, err := q.Stats(pqQueue)
		if err != nil {
			return err
		}
		fmt.Printf("Queue: %s\n", stats.Queue)
		fmt.Printf("  Pending: %d  Running: %d  Completed: %d  Failed: %d  Dead: %d\n",
			stats.Pending, stats.Running, stats.Completed, stats.Failed, stats.Dead)
		fmt.Printf("  Total: %d\n", stats.Total)
		return nil
	},
}

var pqPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge completed/failed/dead tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()

		purged, err := q.Purge(0) // purge all completed immediately for CLI simplicity
		if err != nil {
			return err
		}
		fmt.Printf("Purged %d tasks\n", purged)
		return nil
	},
}

var pqReclaimCmd = &cobra.Command{
	Use:   "reclaim",
	Short: "Reclaim stuck running tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		q, err := getPQ()
		if err != nil {
			return err
		}
		defer q.Close()

		reclaimed, err := q.ReclaimRunning(0) // reclaim all immediately for CLI simplicity
		if err != nil {
			return err
		}
		fmt.Printf("Reclaimed %d tasks\n", reclaimed)
		return nil
	},
}
