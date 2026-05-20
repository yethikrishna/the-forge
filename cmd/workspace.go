package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/workspace"
	"github.com/spf13/cobra"
)

func workspaceCmd() *cobra.Command {
	var workspaceDir string

	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Multi-repo context management",
		Long: `Manage multi-repo workspaces for cross-project agent workflows.

Define a workspace of multiple git repos, clone them all,
build cross-repo indexes, and coordinate changes across boundaries.

Real projects span repos. Forge handles them as one.

Examples:
  forge workspace init my-project --repo https://github.com/org/api --repo https://github.com/org/client
  forge workspace clone my-project
  forge workspace status my-project
  forge workspace diff my-project
  forge workspace plan my-project`,
	}

	initCmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Initialize a new workspace",
		Long: `Create a workspace with one or more repos.

Repos are specified with --repo flags (URL with optional @branch suffix):
  --repo https://github.com/org/api@main
  --repo https://github.com/org/client@develop`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWorkspaceDir(workspaceDir)
			repoURLs, _ := cmd.Flags().GetStringSlice("repo")
			desc, _ := cmd.Flags().GetString("desc")

			if len(repoURLs) == 0 {
				return fmt.Errorf("at least one --repo is required")
			}

			var repos []workspace.Repo
			for _, r := range repoURLs {
				url, branch := parseRepoRef(r)
				repos = append(repos, workspace.Repo{
					URL:    url,
					Branch: branch,
				})
			}

			mgr := workspace.NewManager(dir)
			ws, err := mgr.Create(args[0], desc, repos)
			if err != nil {
				return fmt.Errorf("failed to create workspace: %w", err)
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Workspace created: %s", ws.Name)))
			fmt.Printf("  ID:    %s\n", ws.ID)
			fmt.Printf("  Dir:   %s\n", ws.RootDir)
			fmt.Printf("  Repos: %d\n", len(ws.Repos))
			for _, r := range ws.Repos {
				branch := r.Branch
				if branch == "" {
					branch = "(default)"
				}
				fmt.Printf("    - %s @ %s\n", r.URL, branch)
			}
			return nil
		},
	}
	initCmd.Flags().StringSlice("repo", nil, "Repo URL (with optional @branch)")
	initCmd.Flags().String("desc", "", "Workspace description")

	cloneCmd := &cobra.Command{
		Use:   "clone <name>",
		Short: "Clone all repos in the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWorkspaceDir(workspaceDir)
			mgr := workspace.NewManager(dir)

			ws, err := mgr.Clone(args[0])
			if err != nil {
				return fmt.Errorf("failed to clone workspace: %w", err)
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Cloning: %s", ws.Name)))
			for _, repo := range ws.Repos {
				switch repo.Status {
				case workspace.RepoCloned:
					fmt.Printf("  %s %s @ %s (%s)\n",
						pretty.Sprint(pretty.Success, "✓"),
						repo.Path, repo.Branch, repo.Commit)
				case workspace.RepoError:
					fmt.Printf("  %s %s — %s\n",
						pretty.Sprint(pretty.Warning, "✗"),
						repo.Path, repo.Error)
				default:
					fmt.Printf("  %s %s — %s\n",
						pretty.Sprint(pretty.DimF, "○"),
						repo.Path, repo.Status)
				}
			}
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWorkspaceDir(workspaceDir)
			mgr := workspace.NewManager(dir)

			workspaces, err := mgr.List()
			if err != nil {
				return err
			}

			if len(workspaces) == 0 {
				fmt.Println(pretty.InfoLine("No workspaces found"))
				fmt.Println("  Create one with: forge workspace init <name> --repo <url>")
				return nil
			}

			fmt.Println(pretty.HeaderLine("Workspaces"))
			fmt.Println()
			for _, ws := range workspaces {
				cloned := 0
				for _, r := range ws.Repos {
					if r.Status == workspace.RepoCloned {
						cloned++
					}
				}
				fmt.Printf("  ● %-20s %d/%d repos cloned  %s\n",
					pretty.Sprint(pretty.Info, ws.Name),
					cloned, len(ws.Repos),
					pretty.Sprint(pretty.DimF, ws.RootDir),
				)
			}
			fmt.Printf("\n  %d workspace(s)\n", len(workspaces))
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show workspace status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWorkspaceDir(workspaceDir)
			mgr := workspace.NewManager(dir)

			ws, err := mgr.Status(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.HeaderLine(fmt.Sprintf("Workspace: %s", ws.Name)))
			if ws.Description != "" {
				fmt.Printf("  %s\n", ws.Description)
			}
			fmt.Println()

			for _, repo := range ws.Repos {
				var statusIcon string
				switch repo.Status {
				case workspace.RepoCloned:
					statusIcon = pretty.Sprint(pretty.Success, "●")
				case workspace.RepoModified:
					statusIcon = pretty.Sprint(pretty.Warning, "●")
				case workspace.RepoMissing:
					statusIcon = pretty.Sprint(pretty.DimF, "○")
				case workspace.RepoError:
					statusIcon = pretty.Sprint(pretty.Warning, "✗")
				default:
					statusIcon = pretty.Sprint(pretty.DimF, "○")
				}

				branch := repo.Branch
				if branch == "" {
					branch = "(detached)"
				}

				fmt.Printf("  %s %-20s branch: %-12s commit: %s",
					statusIcon, repo.Path, branch, repo.Commit)
				if repo.Dirty {
					fmt.Printf(" %s", pretty.Sprint(pretty.Warning, "dirty"))
				}
				fmt.Println()
			}
			return nil
		},
	}

	diffCmd := &cobra.Command{
		Use:   "diff <name>",
		Short: "Show changes across all repos",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWorkspaceDir(workspaceDir)
			mgr := workspace.NewManager(dir)

			results, err := mgr.Diff(args[0])
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Println(pretty.InfoLine("No changes across workspace"))
				return nil
			}

			fmt.Println(pretty.HeaderLine("Workspace Diff"))
			for _, r := range results {
				total := len(r.Modified) + len(r.Added) + len(r.Deleted) + len(r.Untracked)
				if total == 0 {
					continue
				}

				fmt.Printf("\n  %s (%s) — %s\n", pretty.Sprint(pretty.Info, r.Repo), r.Branch, r.Summary)
				for _, f := range r.Modified {
					fmt.Printf("    %s %s\n", pretty.Sprint(pretty.Warning, "~"), f)
				}
				for _, f := range r.Added {
					fmt.Printf("    %s %s\n", pretty.Sprint(pretty.Success, "+"), f)
				}
				for _, f := range r.Deleted {
					fmt.Printf("    %s %s\n", pretty.Sprint(pretty.Warning, "-"), f)
				}
				for _, f := range r.Untracked {
					fmt.Printf("    %s %s\n", pretty.Sprint(pretty.DimF, "?"), f)
				}
			}
			return nil
		},
	}

	planCmd := &cobra.Command{
		Use:   "plan <name>",
		Short: "Generate coordination plan for cross-repo changes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWorkspaceDir(workspaceDir)
			mgr := workspace.NewManager(dir)

			plan, err := mgr.PlanCoordination(args[0])
			if err != nil {
				return err
			}

			fmt.Println(pretty.HeaderLine("Coordination Plan"))
			fmt.Printf("  %s\n\n", plan.Notes)

			if len(plan.Steps) == 0 {
				return nil
			}

			for _, step := range plan.Steps {
				depStr := ""
				if step.DependsOn != "" {
					depStr = fmt.Sprintf(" (after %s)", step.DependsOn)
				}
				fmt.Printf("  %d. [%s] %s: %s%s\n",
					step.Priority, step.Repo, step.Action, step.Message, depStr)
			}
			return nil
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := getWorkspaceDir(workspaceDir)
			mgr := workspace.NewManager(dir)

			if err := mgr.Delete(args[0]); err != nil {
				return err
			}

			fmt.Println(pretty.SuccessLine(fmt.Sprintf("Deleted workspace: %s", args[0])))
			return nil
		},
	}

	cmd.AddCommand(initCmd, cloneCmd, listCmd, statusCmd, diffCmd, planCmd, deleteCmd)
	cmd.PersistentFlags().StringVar(&workspaceDir, "dir", "", "Workspace store directory (default: ~/.forge/workspaces)")

	return cmd
}

func getWorkspaceDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge", "workspaces")
}

// parseRepoRef splits "url@branch" into url and branch.
func parseRepoRef(ref string) (url, branch string) {
	atIdx := strings.LastIndex(ref, "@")
	if atIdx > 0 && !strings.Contains(ref[atIdx:], "/") {
		return ref[:atIdx], ref[atIdx+1:]
	}
	return ref, ""
}
