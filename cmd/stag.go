package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/sessiontag"
	"github.com/spf13/cobra"
)

var stagCmd = &cobra.Command{
	Use:   "stag",
	Short: "Session tags and organization",
	Long:  "Manage session tags for organizing, filtering, and auto-tagging agent sessions.",
}

var (
	stagDir  string
	stagTags []string
)

func init() {
	stagCmd.AddCommand(stagListCmd)
	stagCmd.AddCommand(stagCreateCmd)
	stagCmd.AddCommand(stagTagCmd)
	stagCmd.AddCommand(stagUntagCmd)
	stagCmd.AddCommand(stagFindCmd)
	stagCmd.AddCommand(stagAutoTagCmd)

	stagCmd.PersistentFlags().StringVar(&stagDir, "dir", ".forge/tags", "Tag storage directory")
	stagTagCmd.Flags().StringArrayVar(&stagTags, "tag", nil, "Tags to apply")
	stagUntagCmd.Flags().StringArrayVar(&stagTags, "tag", nil, "Tags to remove")
	stagFindCmd.Flags().StringArrayVar(&stagTags, "tag", nil, "Tags to search")
}

func getStagMgr() *sessiontag.Manager {
	return sessiontag.NewManager(stagDir)
}

var stagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := getStagMgr()
		tags := mgr.ListTags()
		if len(tags) == 0 {
			fmt.Println("No tags found.")
			return nil
		}
		fmt.Printf("Tags (%d):\n", len(tags))
		for _, t := range tags {
			fmt.Printf("  %s — %d sessions\n", t.Name, t.Count)
		}
		return nil
	},
}

var stagCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := getStagMgr()
		return mgr.CreateTag(args[0], sessiontag.Color(""))
	},
}

var stagTagCmd = &cobra.Command{
	Use:   "tag [session-id]",
	Short: "Tag a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(stagTags) == 0 {
			return fmt.Errorf("--tag is required")
		}
		mgr := getStagMgr()
		return mgr.TagSession(args[0], stagTags)
	},
}

var stagUntagCmd = &cobra.Command{
	Use:   "untag [session-id]",
	Short: "Remove tags from a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(stagTags) == 0 {
			return fmt.Errorf("--tag is required")
		}
		mgr := getStagMgr()
		return mgr.UntagSession(args[0], stagTags)
	},
}

var stagFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Find sessions by tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(stagTags) == 0 {
			return fmt.Errorf("--tag is required")
		}
		mgr := getStagMgr()
		sessions := mgr.FindByTags(stagTags)
		if len(sessions) == 0 {
			fmt.Println("No matching sessions found.")
			return nil
		}
		fmt.Printf("Found %d sessions:\n", len(sessions))
		for _, s := range sessions {
			fmt.Printf("  %s\n", s.ID)
		}
		return nil
	},
}

var stagAutoTagCmd = &cobra.Command{
	Use:   "auto-tag [session-id]",
	Short: "Auto-tag a session based on content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := getStagMgr()
		prompt, _ := cmd.Flags().GetString("prompt")
		tags := mgr.AutoTag(args[0], prompt)
		if len(tags) == 0 {
			fmt.Println("No auto-tags generated.")
		} else {
			fmt.Printf("Auto-tagged: %v\n", tags)
		}
		return nil
	},
}

func init() {
	stagAutoTagCmd.Flags().String("prompt", "", "Session prompt for auto-tagging")
}
