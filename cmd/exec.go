package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

// execInput allows tests to supply a custom reader for REPL mode.
var execInput io.Reader

var execCmd = &cobra.Command{
	Use:   "exec <service-id-or-name> [-- <command...>]",
	Short: "Execute a command inside a running service container",
	Long: `Run a one-shot command inside a service's active container, or omit the
command to open an interactive REPL session.

Examples:
  clank exec my-api -- ls -la
  clank exec my-api --timeout 60 -o json -- env
  clank exec my-api`,
	Args: cobra.MinimumNArgs(1),
	RunE: runExec,
}

func runExec(cmd *cobra.Command, args []string) error {
	timeout, _ := cmd.Flags().GetInt("timeout")
	if timeout < 1 || timeout > 120 {
		return fmt.Errorf("--timeout must be between 1 and 120 seconds")
	}

	serviceRef := args[0]
	client := newClient()
	serviceID, err := resolveServiceID(client, serviceRef)
	if err != nil {
		return err
	}

	// One-shot mode when command arguments are provided.
	if len(args) > 1 {
		argv := args[1:]
		resp, err := api.ExecService(client, serviceID, api.ExecRequest{
			Command:        argv,
			TimeoutSeconds: timeout,
		})
		if err != nil {
			return err
		}

		if output.IsJSON() {
			if err := output.JSON(resp); err != nil {
				return err
			}
		} else {
			if resp.Stdout != "" {
				fmt.Fprint(os.Stdout, resp.Stdout)
			}
			if resp.Stderr != "" {
				fmt.Fprint(os.Stderr, resp.Stderr)
			}
			if resp.Truncated {
				fmt.Fprintln(os.Stderr, "\n[output truncated]")
			}
		}

		if resp.ExitCode != 0 {
			return fmt.Errorf("command exited with status %d", resp.ExitCode)
		}
		return nil
	}

	// Interactive REPL mode when no command arguments are provided.
	in := execInput
	if in == nil {
		in = os.Stdin
	}

	fmt.Fprintf(os.Stdout, "Connected to %s. Type 'exit' to quit.\n", serviceRef)
	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprintf(os.Stdout, "clank:%s$ ", serviceRef)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return nil
		}

		resp, err := api.ExecService(client, serviceID, api.ExecRequest{
			Command:        line,
			TimeoutSeconds: timeout,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}

		if output.IsJSON() {
			if err := output.JSON(resp); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			continue
		}

		if resp.Stdout != "" {
			fmt.Fprint(os.Stdout, resp.Stdout)
			if !strings.HasSuffix(resp.Stdout, "\n") {
				fmt.Fprintln(os.Stdout)
			}
		}
		if resp.Stderr != "" {
			fmt.Fprint(os.Stderr, resp.Stderr)
			if !strings.HasSuffix(resp.Stderr, "\n") {
				fmt.Fprintln(os.Stderr)
			}
		}
		if resp.Truncated {
			fmt.Fprintln(os.Stderr, "[output truncated]")
		}
		if resp.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "[exit %d]\n", resp.ExitCode)
		}
	}
}

func init() {
	execCmd.Flags().Int("timeout", 30, "command timeout in seconds (1-120)")
}
