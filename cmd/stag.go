package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/forge/sword/internal/sessiontag"
)

var stagCmd = &cobra.Command{
	Use:   "stag",
	Short: "Session tags and organization",
	Long:  "Manage session tags for organizing, filtering, and auto-tagging agent sessions.",
}

var (
	stagDir    string
	stagColor  string
	stagTags   []string
)

func init() {
	stagCmd.AddCommand(stagListCmd)
	stagCmd.AddCommand(stagCreateCmd)
	stagCmd.AddCommand(stagTagCmd)
	stagCmd.AddCommand(stagUntagCmd)
	stagCmd.AddCommand(stagFindCmd)
	stagCmd.AddCommand(stagAutoTagCmd)

	stagCmd.PersistentFlags().StringVar(&stagDir, "dir", ".forge/tags", "Tag storage directory")
	stagCreateCmd.Flags().StringVar(&stagColor, "color", "", "Tag color (hex)")
	stagTagCmd.Flags().StringArrayVar(&stagTags, "tag", nil, "Tags to apply")
	stagUntagCmd.Flags().StringArrayVar(&stagTags, "tag", nil, "Tags to remove")
	stagFindCmd.Flags().StringArrayVar(&stagTags, "tag", nil, "Tags to search")
}

func getStagStore() (*sessiontag.Store, error) {
	return sessiontag.NewStore(stagDir)
}

var stagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getStagStore()
		if err != nil {
			return err
		}
		tags := store.ListTags()
		if len(tags) == 0 {
			fmt.Println("No tags found.")
			return nil
		}
		fmt.Printf("Tags (%d):\n", len(tags))
		for _, t := range tags {
			auto := ""
			if t.AutoTag {
				auto = " (auto)"
			}
			fmt.Printf("  %s%s — %d sessions\n", t.Name, auto, t.Count)
		}
		return nil
	},
}

var stagCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getStagStore()
		if err != nil {
			return err
		}
		return store.CreateTag(args[0], stagColor)
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
		store, err := getStagStore()
		if err != nil {
			return err
		}
		return store.TagSession(args[0], stagTags)
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
		store, err := getStagStore()
		if err != nil {
			return err
		}
		return store.UntagSession(args[0], stagTags)
	},
}

var stagFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Find sessions by tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(stagTags) == 0 {
			return fmt.Errorf("--tag is required")
		}
		store, err := getStagStore()
		if err != nil {
			return err
		}
		sessions := store.FindSessions(stagTags)
		if len(sessions) == 0 {
			fmt.Println("No matching sessions found.")
			return nil
		}
		fmt.Printf("Found %d sessions:\n", len(sessions))
		for _, s := range sessions {
			fmt.Printf("  %s\n", s)
		}
		return nil
	},
}

var stagAutoTagCmd = &cobra.Command{
	Use:   "auto-tag [session-id]",
	Short: "Auto-tag a session based on content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := getStagStore()
		if err != nil {
			return err
		}
		prompt, _ := cmd.Flags().GetString("prompt")
		output, _ := cmd.Flags().GetString("output")
		tags := store.AutoTag(args[0], prompt, output)
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
	stagAutoTagCmd.Flags().String("output", "", "Session output for auto-tagging")
}
