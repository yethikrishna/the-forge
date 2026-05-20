package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/schedule"
	"github.com/spf13/cobra"
)

func scheduleCmd() *cobra.Command {
	var scheduleDir string
	var agent string
	var tags []string
	var enabled bool

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Schedule recurring agent tasks with cron expressions",
		Long: `The forge strikes at appointed hours, unbidden.

Schedule agents to run on cron expressions — hourly checks,
daily reports, weekly reviews. Never miss a beat.

Examples:
  forge schedule add "daily-check" "@daily" --agent sentinel --task "check-email"
  forge schedule add "hourly-monitor" "@hourly" --agent watcher --task "monitor"
  forge schedule add "every-5m" "@every 5m" --agent scout --task "scan"
  forge schedule list
  forge schedule run sched-123456789
  forge schedule disable sched-123456789`,
	}

	cmd.PersistentFlags().StringVar(&scheduleDir, "dir", ".forge/schedule", "Schedule storage directory")
	cmd.PersistentFlags().StringVar(&agent, "agent", "", "Agent name or ID")
	cmd.PersistentFlags().StringSliceVar(&tags, "tags", nil, "Tags for the schedule")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "add [name] [cron]",
			Short: "Add a new scheduled task",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := scheduleStorePath(scheduleDir)
				task := "run"
				s := schedule.NewScheduler(store, defaultScheduleRunner())

				sched, err := s.Add(args[0], args[1], agent, task,
					schedule.WithTags(tags...),
				)
				if err != nil {
					return err
				}

				fmt.Println(pretty.HeaderLine("Schedule Created"))
				fmt.Print(schedule.FormatSchedule(sched))
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all schedules",
			RunE: func(cmd *cobra.Command, args []string) error {
				store := scheduleStorePath(scheduleDir)
				s := schedule.NewScheduler(store, defaultScheduleRunner())

				list := s.List()
				if len(list) == 0 {
					fmt.Println("No schedules found. Use 'forge schedule add' to create one.")
					return nil
				}

				fmt.Println(pretty.HeaderLine("Schedules"))
				fmt.Printf("%-20s %-15s %-10s %-12s %-8s %s\n",
					"ID", "Name", "Cron", "Agent", "Status", "Next Run")
				fmt.Println(string(make([]byte, 90)))

				for _, sched := range list {
					status := "enabled"
					if !sched.Enabled {
						status = "disabled"
					}
					nextRun := "never"
					if !sched.NextRun.IsZero() {
						nextRun = sched.NextRun.Format("2006-01-02 15:04")
					}
					fmt.Printf("%-20s %-15s %-10s %-12s %-8s %s\n",
						sched.ID, sched.Name, sched.Cron, sched.Agent, status, nextRun)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "show [id]",
			Short: "Show schedule details",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := scheduleStorePath(scheduleDir)
				s := schedule.NewScheduler(store, defaultScheduleRunner())

				sched, err := s.Get(args[0])
				if err != nil {
					return err
				}

				fmt.Print(schedule.FormatSchedule(sched))
				return nil
			},
		},
		&cobra.Command{
			Use:   "run [id]",
			Short: "Execute a schedule immediately",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := scheduleStorePath(scheduleDir)
				s := schedule.NewScheduler(store, defaultScheduleRunner())

				log, err := s.RunNow(context.Background(), args[0])
				if err != nil {
					return err
				}

				fmt.Printf("Status:    %s\n", log.Status)
				fmt.Printf("Started:   %s\n", log.StartedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("Finished:  %s\n", log.FinishedAt.Format("2006-01-02 15:04:05"))
				if log.Output != "" {
					fmt.Printf("Output:    %s\n", log.Output)
				}
				if log.Error != "" {
					fmt.Printf("Error:     %s\n", log.Error)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "enable [id]",
			Short: "Enable a schedule",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := scheduleStorePath(scheduleDir)
				s := schedule.NewScheduler(store, defaultScheduleRunner())
				return s.Enable(args[0])
			},
		},
		&cobra.Command{
			Use:   "disable [id]",
			Short: "Disable a schedule",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := scheduleStorePath(scheduleDir)
				s := schedule.NewScheduler(store, defaultScheduleRunner())
				return s.Disable(args[0])
			},
		},
		&cobra.Command{
			Use:   "delete [id]",
			Short: "Delete a schedule",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := scheduleStorePath(scheduleDir)
				s := schedule.NewScheduler(store, defaultScheduleRunner())
				if err := s.Delete(args[0]); err != nil {
					return err
				}
				fmt.Printf("Schedule %s deleted.\n", args[0])
				return nil
			},
		},
		&cobra.Command{
			Use:   "logs [id]",
			Short: "Show run logs for a schedule",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				store := scheduleStorePath(scheduleDir)
				s := schedule.NewScheduler(store, defaultScheduleRunner())

				logs := s.Logs(20)
				if len(logs) == 0 {
					fmt.Println("No run logs found.")
					return nil
				}

				fmt.Println(pretty.HeaderLine("Run Logs"))
				fmt.Printf("%-20s %-10s %-20s %s\n", "Schedule", "Status", "Started", "Duration")
				for _, log := range logs {
					if len(args) > 0 && log.ScheduleID != args[0] {
						continue
					}
					duration := log.FinishedAt.Sub(log.StartedAt)
					fmt.Printf("%-20s %-10s %-20s %v\n",
						log.ScheduleID, log.Status,
						log.StartedAt.Format("2006-01-02 15:04:05"),
						duration)
				}
				return nil
			},
		},
	)

	_ = enabled

	return cmd
}

func scheduleStorePath(dir string) string {
	return filepath.Join(dir, "schedule.json")
}

func defaultScheduleRunner() func(ctx context.Context, sched *schedule.Schedule) (string, error) {
	return func(_ context.Context, sched *schedule.Schedule) (string, error) {
		return fmt.Sprintf("executed %s/%s (simulated)", sched.Agent, sched.Task), nil
	}
}

// silence unused
var _ = os.ReadFile
