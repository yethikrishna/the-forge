package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/forge/sword/internal/integration"
	"github.com/spf13/cobra"
)

func integrationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integration",
		Short: "Manage project management integrations",
		Long: `Connect to Jira, Linear, Notion, and other task tracking tools.
Agents can read and update tasks across all connected providers.

Examples:
  forge integration connect jira --name=MyJira --url=https://myorg.atlassian.net --token=$JIRA_TOKEN
  forge integration connect linear --name=Engineering --token=$LINEAR_TOKEN
  forge integration tasks <connection-id>
  forge integration create-task <connection-id> --title="Fix bug" --priority=high
  forge integration list`,
		SilenceUsage: true,
	}

	cmd.AddCommand(
		integrationConnectCmd(),
		integrationListCmd(),
		integrationDisconnectCmd(),
		integrationTasksCmd(),
		integrationCreateTaskCmd(),
		integrationUpdateTaskCmd(),
		integrationCommentCmd(),
	)

	return cmd
}

func getIntegrationManager() (*integration.Manager, error) {
	return integration.NewManager(getForgeDir() + "/integrations")
}

func integrationConnectCmd() *cobra.Command {
	var name, url, token, email, project, workspace string

	cmd := &cobra.Command{
		Use:   "connect <provider>",
		Short: "Connect to a project management tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getIntegrationManager()
			if err != nil {
				return err
			}

			provider := integration.Provider(args[0])
			switch provider {
			case integration.ProviderJira, integration.ProviderLinear,
				integration.ProviderNotion, integration.ProviderGitHub, integration.ProviderGeneric:
				// valid
			default:
				return fmt.Errorf("unsupported provider %q (use: jira, linear, notion, github, generic)", args[0])
			}

			if name == "" {
				name = args[0]
			}

			c, err := mgr.Connect(integration.Config{
				Provider:  provider,
				Name:      name,
				BaseURL:   url,
				APIToken:  token,
				Email:     email,
				Project:   project,
				Workspace: workspace,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Connected to %s (%s)\n", c.Config.Name, c.ID)
			fmt.Printf("  Provider: %s\n", c.Config.Provider)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Connection name")
	cmd.Flags().StringVar(&url, "url", "", "Base URL")
	cmd.Flags().StringVar(&token, "token", "", "API token")
	cmd.Flags().StringVar(&email, "email", "", "Email (Jira)")
	cmd.Flags().StringVar(&project, "project", "", "Project key")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace (Notion)")

	return cmd
}

func integrationListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List integration connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getIntegrationManager()
			if err != nil {
				return err
			}

			conns := mgr.List()
			if len(conns) == 0 {
				fmt.Println("No integrations connected. Use: forge integration connect <provider>")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(conns, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPROVIDER\tSTATUS\tID")
			for _, c := range conns {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Config.Name, c.Config.Provider, c.Status, c.ID)
			}
			w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func integrationDisconnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect <id>",
		Short: "Disconnect an integration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getIntegrationManager()
			if err != nil {
				return err
			}
			if err := mgr.Disconnect(args[0]); err != nil {
				return err
			}
			fmt.Printf("Disconnected %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func integrationTasksCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "tasks <connection-id>",
		Short: "List tasks from a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getIntegrationManager()
			if err != nil {
				return err
			}

			tasks, err := mgr.FetchTasks(args[0])
			if err != nil {
				return err
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks found.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(tasks, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			for _, t := range tasks {
				fmt.Print(integration.FormatTask(t))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func integrationCreateTaskCmd() *cobra.Command {
	var title, description, priority, assignee, key string
	var labels []string

	cmd := &cobra.Command{
		Use:   "create-task <connection-id>",
		Short: "Create a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getIntegrationManager()
			if err != nil {
				return err
			}

			if title == "" {
				return fmt.Errorf("--title is required")
			}

			task := &integration.Task{
				ProviderKey: key,
				Title:       title,
				Description: description,
				Status:      "open",
				Priority:    priority,
				Assignee:    assignee,
				Labels:      labels,
			}
			if priority == "" {
				task.Priority = "medium"
			}

			if err := mgr.CreateTask(args[0], task); err != nil {
				return err
			}

			fmt.Printf("Task created: %s (%s)\n", task.ProviderKey, task.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Task title")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&priority, "priority", "medium", "Priority (critical, high, medium, low)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee")
	cmd.Flags().StringVar(&key, "key", "", "Provider key (e.g., PROJ-123)")
	cmd.Flags().StringSliceVar(&labels, "labels", nil, "Labels")

	return cmd
}

func integrationUpdateTaskCmd() *cobra.Command {
	var status, priority, assignee string

	cmd := &cobra.Command{
		Use:   "update-task <connection-id> <task-id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getIntegrationManager()
			if err != nil {
				return err
			}

			err = mgr.UpdateTask(args[0], args[1], func(t *integration.Task) error {
				if status != "" {
					t.Status = status
				}
				if priority != "" {
					t.Priority = priority
				}
				if assignee != "" {
					t.Assignee = assignee
				}
				return nil
			})
			if err != nil {
				return err
			}

			fmt.Printf("Task %s updated.\n", args[1])
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "New status")
	cmd.Flags().StringVar(&priority, "priority", "", "New priority")
	cmd.Flags().StringVar(&assignee, "assignee", "", "New assignee")

	return cmd
}

func integrationCommentCmd() *cobra.Command {
	var author string

	cmd := &cobra.Command{
		Use:   "comment <connection-id> <task-id> <body>",
		Short: "Add a comment to a task",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := getIntegrationManager()
			if err != nil {
				return err
			}
			if author == "" {
				author = "forge"
			}
			return mgr.AddComment(args[0], args[1], author, args[2])
		},
	}

	cmd.Flags().StringVar(&author, "author", "forge", "Comment author")
	return cmd
}
