package cmd

import (
	"fmt"
	"io"
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
			return genPowerShellCompletion(os.Stdout)
		}
		return nil
	},
}

// genPowerShellCompletion writes a custom PowerShell completion script.
//
// Cobra's built-in PowerShell completion has multiple bugs on PS 5.1:
//  1. Missing -Native on Register-ArgumentCompleter (only matches cmdlets)
//  2. Empty string args get stripped via Invoke-Expression (breaks __complete)
//  3. Invoke-Expression output gets intercepted by the completer pipeline
//
// This custom script uses System.Diagnostics.Process to call the binary,
// completely bypassing PS's output pipeline interference.
func genPowerShellCompletion(w io.Writer) error {
	// Note: Go raw strings (backtick-delimited) cannot contain backticks.
	// PS backtick (`) is U+0060. We avoid using PS backticks entirely
	// by using [char] codes and .NET APIs instead.
	const script = `# PowerShell completion for clank
Register-ArgumentCompleter -Native -CommandName 'clank' -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    # Parse command elements (force array to avoid single-item unwrapping)
    $elements = @($commandAst.CommandElements | ForEach-Object { $_.ToString() })
    $prog = $elements[0]

    # Resolve binary path from PATH
    $exePath = $null
    foreach ($dir in $env:Path -split ';') {
        if (-not $dir) { continue }
        $candidate = Join-Path $dir "$prog.exe"
        if (Test-Path $candidate) { $exePath = $candidate; break }
    }
    if (-not $exePath) { return }

    # Build __complete arguments
    $rest = @()
    if ($elements.Length -gt 1) { $rest = @($elements[1..($elements.Length - 1)]) }
    $argParts = @('__complete') + $rest
    if ($wordToComplete -eq '') { $argParts += '""' }
    $argStr = $argParts -join ' '

    # Use System.Diagnostics.Process to capture output without PS pipeline
    # interference. Invoke-Expression and & operator output gets intercepted
    # by the completer scriptblock context in PS 5.1.
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $exePath
    $psi.Arguments = $argStr
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $psi.EnvironmentVariables['CLANK_ACTIVE_HELP'] = '0'

    try {
        $proc = [System.Diagnostics.Process]::Start($psi)
        $stdout = $proc.StandardOutput.ReadToEnd()
        $proc.WaitForExit()
    } catch { return }

    $lines = @($stdout -split [char]10 | ForEach-Object { $_.TrimEnd([char]13) } | Where-Object { $_ -ne '' })
    if ($lines.Count -eq 0) { return }

    # Last line is the directive (e.g., ":4")
    $last = $lines[-1]
    if ($last -match '^:\d+$') {
        if ($lines.Count -gt 1) { $lines = @($lines[0..($lines.Count - 2)]) }
        else { return }
    }

    foreach ($line in $lines) {
        $tab = $line.IndexOf([char]9)
        if ($tab -ge 0) {
            $text = $line.Substring(0, $tab)
            $desc = $line.Substring($tab + 1)
        } else {
            $text = $line; $desc = $line
        }
        [System.Management.Automation.CompletionResult]::new($text, $text, 'ParameterValue', $desc)
    }
}
`
	_, err := io.WriteString(w, script)
	return err
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
