package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

const maxStackBlueprintBytes = 128 * 1024 // 128 KiB

var upCmd = &cobra.Command{
	Use:   "up [project]",
	Short: "Sync a multi-service clank.toml stack blueprint to a project",
	Long: `Parse a local multi-service clank.toml file ([[databases]] and [[services]])
and synchronize it with a Clank project.

Use --dry-run to preview the Terraform-like plan without mutating resources.
Use --deploy to enqueue deployments (databases first, then app services).
If [project] is omitted, clank up falls back to [project].name in clank.toml.`,
	Example: `  clank up my-app --dry-run
  clank up my-app -f ./clank.toml --deploy
  clank up my-app --server <server-id> --deploy -o json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUp,
}

func runUp(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	serverIDFlag, _ := cmd.Flags().GetString("server")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	deploy, _ := cmd.Flags().GetBool("deploy")

	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}
	if len(rawBytes) == 0 {
		return fmt.Errorf("%s is empty", filePath)
	}
	if len(rawBytes) > maxStackBlueprintBytes {
		return fmt.Errorf("%s exceeds maximum size of %d bytes", filePath, maxStackBlueprintBytes)
	}
	tomlContent := string(rawBytes)

	projectRef := ""
	if len(args) > 0 {
		projectRef = strings.TrimSpace(args[0])
	} else {
		projectRef = extractProjectNameFromTOML(tomlContent)
	}
	if projectRef == "" {
		return fmt.Errorf("project is required (pass `clank up <project>` or set [project].name in %s)", filePath)
	}

	client := newClient()
	projectID, err := resolveProjectID(client, projectRef)
	if err != nil {
		return err
	}

	var serverID *string
	if trimmed := strings.TrimSpace(serverIDFlag); trimmed != "" {
		serverID = &trimmed
	}

	resp, err := api.SyncProjectStack(client, projectID, api.StackSyncRequest{
		TomlContent: tomlContent,
		ServerID:    serverID,
		DryRun:      dryRun,
		Deploy:      deploy,
	})
	if err != nil {
		return err
	}

	if output.IsJSON() {
		return output.JSON(resp)
	}

	printStackSyncResult(resp)
	return nil
}

// extractProjectNameFromTOML extracts [project].name from a clank.toml file
// without requiring an external TOML parser dependency.
func extractProjectNameFromTOML(content string) string {
	inProject := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inProject = line == "[project]"
			continue
		}
		if !inProject {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) != "name" {
			continue
		}
		val = strings.TrimSpace(val)
		if idx := strings.Index(val, " #"); idx >= 0 {
			val = strings.TrimSpace(val[:idx])
		}
		if unquoted, err := strconv.Unquote(val); err == nil {
			return strings.TrimSpace(unquoted)
		}
		return strings.Trim(val, "'\" ")
	}
	return ""
}

func formatStackAction(action string) string {
	switch action {
	case "create":
		return "\033[32m+ create\033[0m"
	case "update":
		return "\033[33m~ update\033[0m"
	case "unchanged":
		return "\033[90m= unchanged\033[0m"
	default:
		return action
	}
}

func printStackSyncResult(resp *api.StackSyncResponse) {
	mode := "Apply"
	if resp.DryRun {
		mode = "Plan (dry-run)"
	}
	fmt.Printf("Stack Sync: %s\n", mode)
	fmt.Printf("Project:    %s\n\n", resp.ProjectID)

	headers := []string{"ACTION", "KIND", "NAME", "CHANGES", "SERVICE", "DEPLOYMENT"}
	rows := make([][]string, len(resp.Actions))
	for i, a := range resp.Actions {
		changes := "-"
		if len(a.ChangedFields) > 0 {
			changes = strings.Join(a.ChangedFields, ", ")
		}
		svcID := "-"
		if a.ServiceID != nil && *a.ServiceID != "" {
			svcID = output.ShortID(*a.ServiceID)
		}
		depID := "-"
		if a.DeploymentID != nil && *a.DeploymentID != "" {
			depID = output.ShortID(*a.DeploymentID)
		}
		rows[i] = []string{
			formatStackAction(a.Action),
			a.Kind,
			a.Name,
			changes,
			svcID,
			depID,
		}
	}
	output.Table(headers, rows)
	fmt.Printf("\nSummary: %d create, %d update, %d unchanged, %d deployed\n",
		resp.Summary.Create,
		resp.Summary.Update,
		resp.Summary.Unchanged,
		resp.Summary.Deployed,
	)
}

func init() {
	upCmd.Flags().StringP("file", "f", "clank.toml", "path to clank.toml stack blueprint")
	upCmd.Flags().String("server", "", "target server ID for synced services")
	upCmd.Flags().Bool("dry-run", false, "preview changes without modifying the project")
	upCmd.Flags().Bool("deploy", false, "enqueue deployments after syncing (databases first)")

	rootCmd.AddCommand(upCmd)
}
