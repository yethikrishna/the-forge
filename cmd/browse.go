package cmd

import (
	"fmt"
	"github.com/forge/sword/internal/pretty"
	"github.com/spf13/cobra"
	"os/exec"
	"runtime"
)

func browseCmd() *cobra.Command {
	var browser string

	cmd := &cobra.Command{
		Use:   "browse [url]",
		Short: "Open a URL in the browser for manual intervention",
		Long: `Open a URL in the system browser. Used when agents need
manual intervention for tasks like CAPTCHAs, complex logins,
or visual verification.

Examples:
  forge browse https://example.com
  forge browse --browser firefox https://example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]

			fmt.Println(pretty.InfoLine(fmt.Sprintf("Opening %s in browser...", url)))

			var cmdExec *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				cmdExec = exec.Command("open", url)
			case "linux":
				if browser != "" {
					cmdExec = exec.Command(browser, url)
				} else {
					cmdExec = exec.Command("xdg-open", url)
				}
			case "windows":
				cmdExec = exec.Command("cmd", "/c", "start", url)
			default:
				return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
			}

			if err := cmdExec.Start(); err != nil {
				return fmt.Errorf("failed to open browser: %w", err)
			}

			fmt.Println(pretty.SuccessLine("Browser opened"))
			return nil
		},
	}

	cmd.Flags().StringVarP(&browser, "browser", "b", "", "Browser executable (default: system default)")

	return cmd
}
