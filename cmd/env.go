package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment variables for a service",
}

var envListCmd = &cobra.Command{
	Use:   "list <service-id>",
	Short: "List environment variables",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		vars, err := api.ListEnvVars(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(vars)
		}

		if len(vars) == 0 {
			fmt.Println("No environment variables configured.")
			return nil
		}

		headers := []string{"KEY", "VALUE", "SECRET"}
		rows := make([][]string, len(vars))
		for i, v := range vars {
			secret := ""
			if v.IsSecret {
				secret = "yes"
			}
			rows[i] = []string{v.Key, v.Value, secret}
		}
		output.Table(headers, rows)
		return nil
	},
}

var envSetCmd = &cobra.Command{
	Use:   "set <service-id> KEY=VALUE [KEY=VALUE...]",
	Short: "Set environment variables",
	Long: `Set one or more environment variables on a service.

Pass KEY=VALUE pairs as arguments, or use -f to load from a .env file.
Use --secret to mark all variables as secrets (values will be masked).`,
	Example: `  clank env set abc123 DATABASE_URL=postgres://...
  clank env set abc123 API_KEY=sk-123 --secret
  clank env set abc123 FOO=bar BAZ=qux
  clank env set abc123 -f .env`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]
		isSecret, _ := cmd.Flags().GetBool("secret")
		envFile, _ := cmd.Flags().GetString("file")

		var vars []api.EnvVarCreateRequest

		// Load from file if specified.
		if envFile != "" {
			fileVars, err := parseEnvFile(envFile)
			if err != nil {
				return err
			}
			for _, v := range fileVars {
				v.IsSecret = isSecret
				vars = append(vars, v)
			}
		}

		// Parse KEY=VALUE arguments (skip the service ID).
		for _, arg := range args[1:] {
			key, value, ok := strings.Cut(arg, "=")
			if !ok {
				return fmt.Errorf("invalid format %q — use KEY=VALUE", arg)
			}
			vars = append(vars, api.EnvVarCreateRequest{
				Key:      key,
				Value:    value,
				IsSecret: isSecret,
			})
		}

		if len(vars) == 0 {
			return fmt.Errorf("no variables to set — pass KEY=VALUE pairs or use -f")
		}

		client := newClient()

		if len(vars) == 1 {
			v, err := api.CreateEnvVar(client, serviceID, vars[0])
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(v)
			}
			fmt.Printf("Set %s\n", v.Key)
			return nil
		}

		// Bulk create.
		resp, err := api.BulkCreateEnvVars(client, serviceID, api.EnvVarBulkCreateRequest{Vars: vars})
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(resp)
		}

		if len(resp.Created) > 0 {
			fmt.Printf("Set %d variable(s)\n", len(resp.Created))
		}
		if len(resp.Skipped) > 0 {
			fmt.Printf("Skipped %d (already exist): %s\n", len(resp.Skipped), strings.Join(resp.Skipped, ", "))
		}
		if len(resp.Errors) > 0 {
			for _, e := range resp.Errors {
				fmt.Fprintf(os.Stderr, "Error: %s\n", e)
			}
		}
		return nil
	},
}

var envDeleteCmd = &cobra.Command{
	Use:   "delete <service-id> <KEY>",
	Short: "Delete an environment variable",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]
		key := args[1]

		client := newClient()

		// Find the var by key to get its ID.
		vars, err := api.ListEnvVars(client, serviceID)
		if err != nil {
			return err
		}

		var varID string
		for _, v := range vars {
			if v.Key == key {
				varID = v.ID
				break
			}
		}
		if varID == "" {
			return fmt.Errorf("variable %q not found", key)
		}

		if err := api.DeleteEnvVar(client, varID); err != nil {
			return err
		}

		fmt.Printf("Deleted %s\n", key)
		return nil
	},
}

var envRevealCmd = &cobra.Command{
	Use:   "reveal <service-id> <KEY>",
	Short: "Reveal the value of a secret environment variable",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]
		key := args[1]

		client := newClient()

		// Find the var by key to get its ID.
		vars, err := api.ListEnvVars(client, serviceID)
		if err != nil {
			return err
		}

		var varID string
		for _, v := range vars {
			if v.Key == key {
				varID = v.ID
				break
			}
		}
		if varID == "" {
			return fmt.Errorf("variable %q not found", key)
		}

		value, err := api.RevealEnvVar(client, varID)
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(map[string]string{"key": key, "value": value})
		}

		fmt.Printf("%s=%s\n", key, value)
		return nil
	},
}

// parseEnvFile reads a .env file and returns env var create requests.
func parseEnvFile(path string) ([]api.EnvVarCreateRequest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	var vars []api.EnvVarCreateRequest
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		// Strip optional quotes from value.
		value = strings.Trim(value, `"'`)
		vars = append(vars, api.EnvVarCreateRequest{
			Key:   strings.TrimSpace(key),
			Value: value,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return vars, nil
}

func init() {
	envSetCmd.Flags().Bool("secret", false, "mark variables as secrets")
	envSetCmd.Flags().StringP("file", "f", "", "load variables from a .env file")

	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envSetCmd)
	envCmd.AddCommand(envDeleteCmd)
	envCmd.AddCommand(envRevealCmd)
	rootCmd.AddCommand(envCmd)
}
