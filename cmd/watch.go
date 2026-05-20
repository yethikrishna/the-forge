package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/forge/sword/internal/pretty"
	"github.com/forge/sword/internal/watcher"
	"github.com/spf13/cobra"
)

func watchCmd() *cobra.Command {
	var exts []string
	var ignore []string
	var interval string
	var command string
	var recursive bool

	cmd := &cobra.Command{
		Use:   "watch [paths...]",
		Short: "Watch files for changes and trigger actions",
		Long: `Monitor files and directories for changes.
Emits events for created, modified, and deleted files.

Examples:
  forge watch .
  forge watch ./src --ext .go,.rs
  forge watch . --run "go test ./..."
  forge watch . --interval 1s
  forge watch ./src ./pkg --ignore vendor,node_modules`,
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to current directory
			paths := args
			if len(paths) == 0 {
				paths = []string{"."}
			}

			// Resolve paths
			for i, p := range paths {
				abs, err := filepath.Abs(p)
				if err != nil {
					return fmt.Errorf("invalid path %q: %w", p, err)
				}
				info, err := os.Stat(abs)
				if err != nil {
					return fmt.Errorf("path %q does not exist: %w", p, err)
				}
				if !info.IsDir() {
					return fmt.Errorf("path %q is not a directory", p)
				}
				paths[i] = abs
			}

			// Parse extensions
			var extensions []string
			if len(exts) > 0 {
				for _, e := range strings.Split(strings.Join(exts, ","), ",") {
					e = strings.TrimSpace(e)
					if e != "" {
						if !strings.HasPrefix(e, ".") {
							e = "." + e
						}
						extensions = append(extensions, e)
					}
				}
			}

			// Parse ignore patterns
			ignorePatterns := []string{".git", "node_modules", ".forge", "vendor", "*.tmp", "*.swp", ".DS_Store"}
			if len(ignore) > 0 {
				for _, ig := range strings.Split(strings.Join(ignore, ","), ",") {
					ig = strings.TrimSpace(ig)
					if ig != "" {
						ignorePatterns = append(ignorePatterns, ig)
					}
				}
			}

			// Parse interval
			pollInterval := 500 * time.Millisecond
			if interval != "" {
				d, err := time.ParseDuration(interval)
				if err != nil {
					return fmt.Errorf("invalid interval %q: %w", interval, err)
				}
				pollInterval = d
			}

			// Print config
			fmt.Println(pretty.HeaderLine("Forge File Watcher"))
			fmt.Println()
			fmt.Printf("  Watching: %s\n", strings.Join(paths, ", "))
			if len(extensions) > 0 {
				fmt.Printf("  Extensions: %s\n", strings.Join(extensions, ", "))
			} else {
				fmt.Printf("  Extensions: all\n")
			}
			fmt.Printf("  Ignore: %s\n", strings.Join(ignorePatterns, ", "))
			fmt.Printf("  Interval: %s\n", pollInterval)
			if command != "" {
				fmt.Printf("  On change: %s\n", command)
			}
			fmt.Println()
			fmt.Println("  Press Ctrl+C to stop")
			fmt.Println()

			// Create handler
			handler := func(evt watcher.Event) {
				relPath := evt.Path
				for _, p := range paths {
					if strings.HasPrefix(evt.Path, p) {
						rel, err := filepath.Rel(p, evt.Path)
						if err == nil {
							relPath = rel
						}
						break
					}
				}

				var style pretty.ColorFunc
				switch evt.Type {
				case watcher.EventCreate:
					style = pretty.GreenF
				case watcher.EventModify:
					style = pretty.YellowF
				case watcher.EventDelete:
					style = pretty.RedF
				default:
					style = pretty.DimF
				}

				ts := evt.Timestamp.Format("15:04:05")
				fmt.Printf("  %s %s %s\n",
					pretty.Sprint(pretty.DimF, ts),
					pretty.Sprint(style, evt.Type.String()),
					relPath,
				)

				// Run command if specified
				if command != "" {
					runOnChange(command, evt)
				}
			}

			// Create and start watcher
			w := watcher.New(watcher.Config{
				Paths:        paths,
				Extensions:   extensions,
				Ignore:       ignorePatterns,
				Debounce:     300 * time.Millisecond,
				PollInterval: pollInterval,
			}, handler)

			// Handle signals
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigCh
				fmt.Println()
				fmt.Println(pretty.Sprint(pretty.DimF, "  Stopping watcher..."))
				w.Stop()
			}()

			return w.Start()
		},
	}

	cmd.Flags().StringArrayVar(&exts, "ext", nil, "File extensions to watch (comma-separated, e.g. .go,.rs)")
	cmd.Flags().StringArrayVar(&ignore, "ignore", nil, "Additional ignore patterns (comma-separated)")
	cmd.Flags().StringVar(&interval, "interval", "500ms", "Polling interval (e.g. 500ms, 1s, 2s)")
	cmd.Flags().StringVarP(&command, "run", "r", "", "Command to run on change")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", true, "Watch subdirectories recursively")

	return cmd
}

func runOnChange(cmd string, evt watcher.Event) {
	// Expand environment variables in the command
	cmd = os.ExpandEnv(cmd)

	fmt.Printf("  %s Running: %s\n", pretty.Sprint(pretty.DimF, "→"), cmd)
	start := time.Now()

	// #nosec G204 — user-provided command, intentional
	result, err := execShell(cmd)
	elapsed := time.Since(start).Truncate(time.Millisecond)

	if err != nil {
		fmt.Printf("  %s Failed (%s): %v\n",
			pretty.Sprint(pretty.Error, pretty.Cross),
			elapsed, err)
		return
	}

	if result != "" {
		// Print last few lines of output
		lines := strings.Split(strings.TrimSpace(result), "\n")
		maxLines := 5
		if len(lines) > maxLines {
			fmt.Printf("  %s (%s, %d lines):\n",
				pretty.Sprint(pretty.Success, pretty.Checkmark),
				elapsed, len(lines))
			for _, l := range lines[len(lines)-maxLines:] {
				fmt.Printf("    %s\n", pretty.Sprint(pretty.DimF, l))
			}
		} else {
			fmt.Printf("  %s (%s):\n",
				pretty.Sprint(pretty.Success, pretty.Checkmark),
				elapsed)
			for _, l := range lines {
				fmt.Printf("    %s\n", l)
			}
		}
	} else {
		fmt.Printf("  %s Done (%s)\n",
			pretty.Sprint(pretty.Success, pretty.Checkmark),
			elapsed)
	}
}

func execShell(cmdStr string) (string, error) {
	cmd := exec.Command("sh", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
