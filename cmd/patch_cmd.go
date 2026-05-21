package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/forge/sword/internal/patch"
	"github.com/spf13/cobra"
)

var patchCmd = &cobra.Command{
	Use:   "patch",
	Short: "Intelligent patch generation and application",
	Long:  `Generate, validate, apply, and revert structured code patches with conflict detection and semantic understanding.`,
}

var patchMgr *patch.Manager

func getPatchManager() *patch.Manager {
	if patchMgr == nil {
		patchMgr = patch.NewManager(getForgeDir() + "/patches")
	}
	return patchMgr
}

func init() {
	patchCmd.AddCommand(patchCreateCmd)
	patchCmd.AddCommand(patchListCmd)
	patchCmd.AddCommand(patchShowCmd)
	patchCmd.AddCommand(patchDeleteCmd)
	patchCmd.AddCommand(patchAddFileCmd)
	patchCmd.AddCommand(patchAddMoveCmd)
	patchCmd.AddCommand(patchFinalizeCmd)
	patchCmd.AddCommand(patchValidateCmd)
	patchCmd.AddCommand(patchApplyCmd)
	patchCmd.AddCommand(patchRevertCmd)
	patchCmd.AddCommand(patchDiffCmd)
	patchCmd.AddCommand(patchStatsCmd)
	patchCmd.AddCommand(patchExportCmd)
}

// patch create
var patchCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new patch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		desc, _ := cmd.Flags().GetString("description")
		author, _ := cmd.Flags().GetString("author")
		m := getPatchManager()
		p := m.Create(args[0], desc)
		p.Author = author
		fmt.Printf("Created patch: %s (id: %s)\n", p.Name, p.ID)
		return nil
	},
}

// patch list
var patchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all patches",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getPatchManager()
		list := m.List()
		if len(list) == 0 {
			fmt.Println("No patches found")
			return nil
		}

		fmt.Printf("%-20s %-20s %-10s %-6s %s\n", "ID", "NAME", "STATUS", "FILES", "CREATED")
		for _, p := range list {
			fmt.Printf("%-20s %-20s %-10s %-6d %s\n",
				p.ID, p.Name, p.Status, len(p.Files),
				p.CreatedAt.Format(time.RFC3339))
		}
		return nil
	},
}

// patch show
var patchShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show patch details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getPatchManager()
		p, ok := m.Get(args[0])
		if !ok {
			return fmt.Errorf("patch %q not found", args[0])
		}
		fmt.Println(patch.RenderPatch(p))
		return nil
	},
}

// patch delete
var patchDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a patch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getPatchManager().Delete(args[0])
	},
}

// patch add-file
var patchAddFileCmd = &cobra.Command{
	Use:   "add-file [patch-id] [file]",
	Short: "Add a file change to a patch (reads old from disk, new from --content or stdin)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		newContent, _ := cmd.Flags().GetString("content")

		// Read current file content as "old"
		data, err := os.ReadFile(args[1])
		oldContent := ""
		if err == nil {
			oldContent = string(data)
		}

		// If no --content, use empty (delete) or stdin
		if newContent == "" && oldContent != "" {
			return fmt.Errorf("use --content to provide new content, or --delete to delete the file")
		}

		return getPatchManager().AddFileChange(args[0], args[1], oldContent, newContent)
	},
}

// patch add-move
var patchAddMoveCmd = &cobra.Command{
	Use:   "add-move [patch-id] [old-path] [new-path]",
	Short: "Add a file move/rename to a patch",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getPatchManager().AddFileMove(args[0], args[1], args[2])
	},
}

// patch finalize
var patchFinalizeCmd = &cobra.Command{
	Use:   "finalize [patch-id]",
	Short: "Finalize a patch for application",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := getPatchManager().Finalize(args[0]); err != nil {
			return err
		}
		fmt.Printf("Patch %s is now ready to apply\n", args[0])
		return nil
	},
}

// patch validate
var patchValidateCmd = &cobra.Command{
	Use:   "validate [patch-id]",
	Short: "Validate a patch against the filesystem",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = "."
		}

		m := getPatchManager()
		conflicts, err := m.Validate(args[0], dir)
		if err != nil {
			return err
		}

		if len(conflicts) == 0 {
			fmt.Println("Patch is clean — no conflicts detected")
		} else {
			fmt.Printf("Conflicts detected (%d):\n", len(conflicts))
			for _, c := range conflicts {
				fmt.Printf("  ✗ %s\n", c)
			}
		}
		return nil
	},
}

// patch apply
var patchApplyCmd = &cobra.Command{
	Use:   "apply [patch-id]",
	Short: "Apply a patch to the filesystem",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = "."
		}

		m := getPatchManager()
		if err := m.Apply(args[0], dir); err != nil {
			return err
		}

		fmt.Printf("Patch %s applied successfully\n", args[0])
		return nil
	},
}

// patch revert
var patchRevertCmd = &cobra.Command{
	Use:   "revert [patch-id]",
	Short: "Revert an applied patch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = "."
		}

		m := getPatchManager()
		if err := m.Revert(args[0], dir); err != nil {
			return err
		}

		fmt.Printf("Patch %s reverted successfully\n", args[0])
		return nil
	},
}

// patch diff
var patchDiffCmd = &cobra.Command{
	Use:   "diff [patch-id]",
	Short: "Show patch as a unified diff",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getPatchManager()
		p, ok := m.Get(args[0])
		if !ok {
			return fmt.Errorf("patch %q not found", args[0])
		}
		fmt.Println(patch.RenderDiff(p))
		return nil
	},
}

// patch stats
var patchStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show patch statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats := getPatchManager().Stats()
		fmt.Printf("Total Patches: %v\n", stats["total_patches"])
		fmt.Printf("Total Files: %v\n", stats["total_files"])
		fmt.Printf("Total Hunks: %v\n", stats["total_hunks"])
		if bs, ok := stats["by_status"].(map[patch.PatchStatus]int); ok {
			fmt.Println("\nBy Status:")
			for status, count := range bs {
				fmt.Printf("  %s: %d\n", status, count)
			}
		}
		return nil
	},
}

// patch export
var patchExportCmd = &cobra.Command{
	Use:   "export [patch-id]",
	Short: "Export a patch as JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m := getPatchManager()
		p, ok := m.Get(args[0])
		if !ok {
			return fmt.Errorf("patch %q not found", args[0])
		}

		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}

		outputFile, _ := cmd.Flags().GetString("output")
		if outputFile != "" {
			return os.WriteFile(outputFile, data, 0644)
		}
		fmt.Println(string(data))
		return nil
	},
}

func init() {
	patchCreateCmd.Flags().String("description", "", "Patch description")
	patchCreateCmd.Flags().String("author", "", "Patch author")

	patchAddFileCmd.Flags().String("content", "", "New file content")

	patchValidateCmd.Flags().String("dir", ".", "Root directory to validate against")
	patchApplyCmd.Flags().String("dir", ".", "Root directory to apply to")
	patchRevertCmd.Flags().String("dir", ".", "Root directory to revert in")

	patchExportCmd.Flags().String("output", "", "Output file path")
}
