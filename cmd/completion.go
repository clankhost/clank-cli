package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for your shell.

To load completions:

Bash:
  $ source <(clank completion bash)

  # To persist across sessions (Linux):
  $ clank completion bash > /etc/bash_completion.d/clank

  # To persist across sessions (macOS):
  $ clank completion bash > $(brew --prefix)/etc/bash_completion.d/clank

Zsh:
  $ source <(clank completion zsh)

  # To persist across sessions:
  $ clank completion zsh > "${fpath[1]}/_clank"

Fish:
  $ clank completion fish | source

  # To persist across sessions:
  $ clank completion fish > ~/.config/fish/completions/clank.fish

PowerShell:
  PS> clank completion powershell | Out-String | Invoke-Expression

  # To persist across sessions:
  PS> clank completion powershell >> $PROFILE
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			cmd.Help()
			return fmt.Errorf("specify a shell: bash, zsh, fish, or powershell")
		}
		return cobra.OnlyValidArgs(cmd, args)
	},
	Example: `  clank completion bash
  clank completion zsh
  clank completion fish
  clank completion powershell`,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
