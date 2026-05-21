package cmd

import (
	"fmt"

	"github.com/forge/sword/internal/autoconfig"
	"github.com/spf13/cobra"
)

func autodetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autodetect",
		Short: "Zero-config auto-detection for project setup",
		Long: `Auto-detect project configuration:
  • Project type and language from files
  • Package manager, build tool, test framework
  • Git remote and branch
  • Available API keys from environment
  • Docker, CI, linter, formatter presence`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			d := autoconfig.NewDetector(dir)
			cfg := d.Detect()
			fmt.Print(autoconfig.FormatConfig(cfg))
			return nil
		},
	}
	return cmd
}
