package cmd

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

// msysMangledPath detects values that look like MSYS/Git Bash path-mangled
// (e.g., /data → C:/Program Files/Git/data).
var msysMangledPath = regexp.MustCompile(`^[A-Z]:/Program Files/Git/`)

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
			if vars == nil {
				vars = []api.EnvVar{}
			}
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

type serviceEnvSetResult struct {
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
	Created     int    `json:"created"`
	Updated     int    `json:"updated"`
	Error       string `json:"error,omitempty"`
}

func upsertServiceEnvVars(client *api.Client, serviceID string, vars []api.EnvVarCreateRequest) (int, int, *api.EnvVar, error) {
	existing, err := api.ListEnvVars(client, serviceID)
	if err != nil {
		return 0, 0, nil, err
	}
	existingByKey := make(map[string]string, len(existing))
	for _, v := range existing {
		existingByKey[v.Key] = v.ID
	}

	var created, updated int
	var lastResult *api.EnvVar
	for _, v := range vars {
		if varID, exists := existingByKey[v.Key]; exists {
			result, err := api.UpdateEnvVar(client, varID, api.EnvVarUpdateRequest{
				Value:    &v.Value,
				IsSecret: &v.IsSecret,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", v.Key, err)
				continue
			}
			updated++
			lastResult = result
		} else {
			result, err := api.CreateEnvVar(client, serviceID, v)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", v.Key, err)
				continue
			}
			created++
			lastResult = result
		}
	}
	return created, updated, lastResult, nil
}

var envSetCmd = &cobra.Command{
	Use:   "set [service-id] KEY=VALUE [KEY=VALUE...]",
	Short: "Set environment variables",
	Long: `Set one or more environment variables on a service or across all services in a project.

Pass KEY=VALUE pairs as arguments, or use -f to load from a .env file.
Use --secret to mark all variables as secrets (values will be masked).
Use --project <project-id-or-slug> to apply variables to every service in the project.`,
	Example: `  clank env set abc123 DATABASE_URL=postgres://...
  clank env set abc123 API_KEY=sk-123 --secret
  clank env set abc123 FOO=bar BAZ=qux
  clank env set abc123 -f .env
  clank env set --project my-proj LOG_LEVEL=debug`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRef, _ := cmd.Flags().GetString("project")
		isSecret, _ := cmd.Flags().GetBool("secret")
		envFile, _ := cmd.Flags().GetString("file")

		var serviceID string
		var kvArgs []string
		if projectRef != "" {
			kvArgs = args
		} else {
			if len(args) < 1 {
				return fmt.Errorf("service-id is required (or use --project <project-id-or-slug>)")
			}
			serviceID = args[0]
			kvArgs = args[1:]
		}

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

		// Parse KEY=VALUE arguments.
		for _, arg := range kvArgs {
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

		// Warn about MSYS path mangling on Git Bash.
		if os.Getenv("MSYSTEM") != "" {
			for _, v := range vars {
				if msysMangledPath.MatchString(v.Value) {
					fmt.Fprintf(os.Stderr, "Warning: %q looks like an MSYS-mangled path.\n", v.Value)
					fmt.Fprintf(os.Stderr, "  Git Bash rewrites Unix paths (e.g., /data → C:/Program Files/Git/data).\n")
					fmt.Fprintf(os.Stderr, "  Fix: MSYS_NO_PATHCONV=1 clank env set ...\n")
				}
			}
		}

		if projectRef != "" {
			projectID, err := resolveProjectID(client, projectRef)
			if err != nil {
				return err
			}
			services, err := api.ListServices(client, projectID)
			if err != nil {
				return err
			}
			results := make([]serviceEnvSetResult, 0, len(services))
			var servicesUpdated, totalCreated, totalUpdated, failed int
			for _, s := range services {
				c, u, _, err := upsertServiceEnvVars(client, s.ID, vars)
				res := serviceEnvSetResult{
					ServiceID:   s.ID,
					ServiceName: s.Name,
					Created:     c,
					Updated:     u,
				}
				if err != nil {
					res.Error = err.Error()
					failed++
				} else {
					servicesUpdated++
					totalCreated += c
					totalUpdated += u
				}
				results = append(results, res)
			}

			if output.IsJSON() {
				if err := output.JSON(map[string]any{
					"project_id":       projectID,
					"services_updated": servicesUpdated,
					"services_total":   len(services),
					"created":          totalCreated,
					"updated":          totalUpdated,
					"results":          results,
				}); err != nil {
					return err
				}
				if failed > 0 {
					return fmt.Errorf("failed to update %d/%d service(s)", failed, len(services))
				}
				return nil
			}

			if len(services) == 0 {
				fmt.Println("No services found in project.")
				return nil
			}
			fmt.Printf("Updated %d/%d service(s) (%d created, %d updated)\n", servicesUpdated, len(services), totalCreated, totalUpdated)
			if failed > 0 {
				return fmt.Errorf("failed to update %d/%d service(s)", failed, len(services))
			}
			return nil
		}

		created, updated, lastResult, err := upsertServiceEnvVars(client, serviceID, vars)
		if err != nil {
			return err
		}

		if output.IsJSON() {
			if len(vars) == 1 && lastResult != nil {
				return output.JSON(lastResult)
			}
			return output.JSON(map[string]int{"created": created, "updated": updated})
		}

		var parts []string
		if created > 0 {
			parts = append(parts, fmt.Sprintf("set %d", created))
		}
		if updated > 0 {
			parts = append(parts, fmt.Sprintf("updated %d", updated))
		}
		if len(parts) > 0 {
			fmt.Printf("%s variable(s)\n", strings.Join(parts, ", "))
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
	envSetCmd.Flags().String("project", "", "apply variables to all services in a project (ID or slug)")

	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envSetCmd)
	envCmd.AddCommand(envDeleteCmd)
	envCmd.AddCommand(envRevealCmd)
	rootCmd.AddCommand(envCmd)
}
