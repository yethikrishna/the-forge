package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/sessiontag"
	"github.com/spf13/cobra"
)

var tagMgr = sessiontag.NewManager("")

func sessiontagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Session tags & organization",
		Long: `Tag sessions, filter by tags, auto-tag based on content.
Organize your sessions for quick retrieval.`,
	}

	cmd.AddCommand(
		tagListCmd(),
		tagCreateCmd(),
		tagDeleteCmd(),
		tagAddCmd(),
		tagRemoveCmd(),
		tagFindCmd(),
		tagAutoCmd(),
	)

	return cmd
}

func tagListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			tags := tagMgr.ListTags()
			if len(tags) == 0 {
				fmt.Println("No tags")
				return nil
			}
			for _, t := range tags {
				fmt.Printf("  %-15s [%s] (%d sessions)\n", t.Name, t.Color, t.Count)
			}
			return nil
		},
	}
}

func tagCreateCmd() *cobra.Command {
	var color string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if color == "" {
				color = "gray"
			}
			return tagMgr.CreateTag(args[0], sessiontag.Color(color))
		},
	}

	cmd.Flags().StringVar(&color, "color", "", "Tag color")
	return cmd
}

func tagDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a tag",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tagMgr.DeleteTag(args[0])
		},
	}
}

func tagAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <session-id> <tag...>",
		Short: "Add tags to a session",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tagMgr.TagSession(args[0], args[1:])
		},
	}
}

func tagRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <session-id> <tag...>",
		Short: "Remove tags from a session",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tagMgr.UntagSession(args[0], args[1:])
		},
	}
}

func tagFindCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "find <tag...>",
		Short: "Find sessions by tags",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions := tagMgr.FindByTags(args)
			if len(sessions) == 0 {
				fmt.Println("No sessions found")
				return nil
			}
			for _, s := range sessions {
				fmt.Println(sessiontag.FormatSession(&s))
			}
			return nil
		},
	}
}

func tagAutoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auto <session-id> <title>",
		Short: "Auto-tag a session based on title",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			applied := tagMgr.AutoTag(args[0], args[1])
			if len(applied) == 0 {
				fmt.Println("No auto-tags applied")
			} else {
				fmt.Printf("Applied tags: %v\n", applied)
			}
			return nil
		},
	}
}
