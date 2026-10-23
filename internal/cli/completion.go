package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:       "completion bash|zsh|fish|powershell",
		Short:     "Generate shell completion script",
		Long:      "Generate a completion script for your shell. Example: source <(gopro9 completion zsh)",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(w, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(w)
			case "fish":
				return cmd.Root().GenFishCompletion(w, true)
			default:
				return cmd.Root().GenPowerShellCompletionWithDesc(w)
			}
		},
	}
}
