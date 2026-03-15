package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

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
			return genPowerShellNativeCompletion(rootCmd, os.Stdout)
		}
		return nil
	},
}

// genPowerShellNativeCompletion generates PowerShell completion with the -Native
// flag on Register-ArgumentCompleter. Cobra omits -Native, which means completions
// only work for PowerShell cmdlets/functions, not external executables like clank.exe.
func genPowerShellNativeCompletion(cmd *cobra.Command, w io.Writer) error {
	var buf bytes.Buffer
	if err := cmd.GenPowerShellCompletionWithDesc(&buf); err != nil {
		return err
	}

	script := buf.String()

	// Patch 1: add -Native flag so it works for external executables (.exe).
	// Cobra omits -Native, which only matches PowerShell cmdlets/functions.
	script = strings.Replace(script,
		"Register-ArgumentCompleter -CommandName",
		"Register-ArgumentCompleter -Native -CommandName",
		1,
	)

	// Patch 2: fix empty argument passing for PowerShell 5.1.
	// PS 5.1 strips empty strings ("") when passed to native commands via
	// Invoke-Expression. Cobra uses `"`" (backtick-escaped quotes) which
	// also gets stripped. Wrapping in single quotes ('`"`"') makes
	// Invoke-Expression pass the literal "" to the native command.
	//
	// Old:  + ' `"`"'       → Invoke-Expression sees "" → stripped in PS 5.1
	// New:  + " '`"`"'"     → Invoke-Expression sees '""' → passed as literal
	//
	// Go backtick = PS backtick = 0x60, must use regular strings here.
	script = strings.Replace(script,
		"$RequestComp=\"$RequestComp\" + ' \x60\"\x60\"'",
		"$RequestComp=\"$RequestComp\" + \" '\x60\"\x60\"'\"",
		1,
	)

	_, err := io.WriteString(w, script)
	return err
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
