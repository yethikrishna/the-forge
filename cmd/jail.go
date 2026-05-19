package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

func jailCmd() *cobra.Command {
	var rule string
	var jsRule string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "jail [command...]",
		Short: "Run a command inside the httpjail network sandbox",
		Long: `Sandboxes any command with httpjail network isolation.
All HTTP/HTTPS traffic is intercepted and filtered.
Default: deny all. Only explicitly allowed requests pass through.

Requires httpjail installed: cargo install httpjail`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpjailPath, err := exec.LookPath("httpjail")
			if err != nil {
				return fmt.Errorf("httpjail not found. Install with: cargo install httpjail")
			}

			jailArgs := []string{}
			if rule != "" {
				jailArgs = append(jailArgs, "--allow", rule)
			}
			if jsRule != "" {
				jailArgs = append(jailArgs, "--js", jsRule)
			}
			if verbose {
				jailArgs = append(jailArgs, "--verbose")
			}
			jailArgs = append(jailArgs, "--")
			jailArgs = append(jailArgs, args...)

			fmt.Printf("Forge: Running in jail: %v\n", args)
			fmt.Printf("   Rule:    %s\n", rule)
			fmt.Printf("   JS Rule: %s\n", jsRule)

			return syscall.Exec(httpjailPath, append([]string{"httpjail"}, jailArgs...), os.Environ())
		},
	}

	cmd.Flags().StringVarP(&rule, "rule", "r", "github.com", "Allow rule (host pattern)")
	cmd.Flags().StringVarP(&jsRule, "js", "", "", "JavaScript rule")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose logging")
	return cmd
}
