package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func completionCmd() *cobra.Command {
	var noDesc bool

	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for The Forge.

To load completions:

Bash:
  $ source <(forge completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ forge completion bash > /etc/bash_completion.d/forge
  # macOS:
  $ forge completion bash > $(brew --prefix)/etc/bash_completion.d/forge

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ forge completion zsh > "${fpath[1]}/_forge"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ forge completion fish | source

  # To load completions for each session, execute once:
  $ forge completion fish > ~/.config/fish/completions/forge.fish

PowerShell:
  PS> forge completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> forge completion powershell > forge.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			root := cmd.Root()
			if noDesc {
				root.DisableSuggestions = true
				root.CompletionOptions.DisableDescriptions = true
			}

			switch shell {
			case "bash":
				return root.GenBashCompletionV2(os.Stdout, !noDesc)
			case "zsh":
				if noDesc {
					return root.GenZshCompletionNoDesc(os.Stdout)
				}
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, !noDesc)
			case "powershell":
				if noDesc {
					return root.GenPowerShellCompletion(os.Stdout)
				}
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell type %q (supported: bash, zsh, fish, powershell)", shell)
			}
		},
	}

	cmd.Flags().BoolVar(&noDesc, "no-descriptions", false, "Disable completion descriptions")

	return cmd
}
