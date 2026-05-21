package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/forge/sword/internal/gitnfs"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
)

func gitnfsCmd() *cobra.Command {
	var maxCommits int

	cmd := &cobra.Command{
		Use:   "git nfs",
		Short: "Mount git history as a browsable filesystem",
		Long: `Browse git commits as directories, diffs as files.
Each commit becomes a directory with metadata, diff files,
and changed file contents.

Examples:
  forge git nfs .
  forge git nfs /path/to/repo --max-commits 100`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := "."
			if len(args) > 0 {
				repoPath = args[0]
			}

			gfs := gitnfs.NewGitFS(gitnfs.FSConfig{
				RepoPath:   repoPath,
				MaxCommits: maxCommits,
			})

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			fmt.Println(pretty.HeaderLine("Forge GitNFS — Git History as Filesystem"))

			if err := gfs.Load(ctx); err != nil {
				return fmt.Errorf("load git history: %w", err)
			}

			commits := gfs.Commits()
			fmt.Printf("  Loaded %d commits from %s\n", len(commits), repoPath)

			if len(commits) > 0 {
				latest, _ := gfs.Latest()
				fmt.Printf("  Latest: %s — %s\n", latest.Short, latest.Subject)
			}

			// Start HTTP API for browsing
			mux := http.NewServeMux()
			mux.HandleFunc("/api/commits", func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(commits)
			})
			mux.HandleFunc("/api/commit/", func(w http.ResponseWriter, r *http.Request) {
				hash := r.URL.Path[len("/api/commit/"):]
				diff, err := gfs.Diff(r.Context(), hash)
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				json.NewEncoder(w).Encode(diff)
			})
			mux.HandleFunc("/api/browse/", func(w http.ResponseWriter, r *http.Request) {
				parts := splitPath(r.URL.Path[len("/api/browse/"):])
				if len(parts) < 2 {
					http.Error(w, "usage: /api/browse/<hash>/<path>", http.StatusBadRequest)
					return
				}
				content, err := gfs.ReadFileAtCommit(r.Context(), parts[0], parts[1])
				if err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				w.Write([]byte(content))
			})

			server := &http.Server{Addr: ":8095", Handler: mux}
			go server.ListenAndServe()
			fmt.Printf("\n  API: http://localhost:8095/api/commits\n")

			<-ctx.Done()
			server.Close()
			return nil
		},
	}

	cmd.Flags().IntVar(&maxCommits, "max-commits", 1000, "Maximum commits to load")

	return cmd
}

func splitPath(p string) []string {
	var parts []string
	for _, s := range splitString(p, "/") {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if string(c) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	result = append(result, current)
	return result
}

