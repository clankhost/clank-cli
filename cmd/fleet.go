package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

// fleetPollInterval is the polling cadence for `clank fleet deploy --wait` (overridable in tests).
var fleetPollInterval = 3 * time.Second

var fleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "Manage rolling fleet deployments across a project",
}

var fleetDeployCmd = &cobra.Command{
	Use:   "deploy <project>",
	Short: "Roll out a pre-built image across services in a project",
	Long: `Start a rolling, canary-first, batched image deployment across services in a project.

Use --dry-run to preview the rollout plan without creating deployments.
Use --wait to poll every 3s until the fleet rollout completes, halts, fails, or is cancelled.`,
	Example: `  clank fleet deploy my-project --image registry.clank.host/team/app:v2
  clank fleet deploy my-project --image ghcr.io/org/app:v2 --canary=true --batch-size=3 --wait
  clank fleet deploy my-project --image ghcr.io/org/app:v2 --services svc1,svc2 --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runFleetDeploy,
}

var fleetStatusCmd = &cobra.Command{
	Use:   "status <project> <fleet-deploy-id>",
	Short: "Check status of a rolling fleet deploy",
	Args:  cobra.ExactArgs(2),
	RunE:  runFleetStatus,
}

var fleetCancelCmd = &cobra.Command{
	Use:   "cancel <project> <fleet-deploy-id>",
	Short: "Cancel an in-progress rolling fleet deploy",
	Args:  cobra.ExactArgs(2),
	RunE:  runFleetCancel,
}

func runFleetDeploy(cmd *cobra.Command, args []string) error {
	imageTag, _ := cmd.Flags().GetString("image")
	if strings.TrimSpace(imageTag) == "" {
		return fmt.Errorf("--image is required")
	}

	servicesFlag, _ := cmd.Flags().GetString("services")
	canaryFirst, _ := cmd.Flags().GetBool("canary")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	haltOnFailure, _ := cmd.Flags().GetBool("halt-on-failure")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	wait, _ := cmd.Flags().GetBool("wait")

	var serviceIDs []string
	if strings.TrimSpace(servicesFlag) != "" {
		for _, part := range strings.Split(servicesFlag, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				serviceIDs = append(serviceIDs, trimmed)
			}
		}
	}

	client := newClient()
	projectID, err := resolveProjectID(client, args[0])
	if err != nil {
		return err
	}

	resp, err := api.StartFleetDeploy(client, projectID, api.FleetDeployRequest{
		ImageTag:      imageTag,
		ServiceIDs:    serviceIDs,
		CanaryFirst:   canaryFirst,
		BatchSize:     batchSize,
		HaltOnFailure: haltOnFailure,
		DryRun:        dryRun,
	})
	if err != nil {
		return err
	}

	if dryRun || !wait || resp.FleetDeployID == nil || *resp.FleetDeployID == "" {
		if output.IsJSON() {
			return output.JSON(resp)
		}
		printFleetDeploySummary(resp)
		return nil
	}

	finalResp, err := waitForFleetDeploy(client, projectID, *resp.FleetDeployID, output.IsJSON())
	if finalResp == nil {
		finalResp = resp
	}

	if output.IsJSON() {
		_ = output.JSON(finalResp)
	} else {
		printFleetDeploySummary(finalResp)
	}

	if err != nil {
		return err
	}
	if finalResp.Status == "halted" || finalResp.Status == "failed" {
		return fmt.Errorf("fleet deploy %s ended with status %s (%d/%d active, %d failed)",
			*resp.FleetDeployID, finalResp.Status, finalResp.CompletedCount, finalResp.TotalServices, finalResp.FailedCount)
	}
	return nil
}

func runFleetStatus(cmd *cobra.Command, args []string) error {
	client := newClient()
	projectID, err := resolveProjectID(client, args[0])
	if err != nil {
		return err
	}

	resp, err := api.GetFleetDeploy(client, projectID, args[1])
	if err != nil {
		return err
	}

	if output.IsJSON() {
		return output.JSON(resp)
	}
	printFleetDeploySummary(resp)
	return nil
}

func runFleetCancel(cmd *cobra.Command, args []string) error {
	client := newClient()
	projectID, err := resolveProjectID(client, args[0])
	if err != nil {
		return err
	}

	resp, err := api.CancelFleetDeploy(client, projectID, args[1])
	if err != nil {
		return err
	}

	if output.IsJSON() {
		return output.JSON(resp)
	}
	printFleetDeploySummary(resp)
	return nil
}

func waitForFleetDeploy(client *api.Client, projectID, fleetDeployID string, quiet bool) (*api.FleetDeployResponse, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(fleetPollInterval)
	defer ticker.Stop()

	var latest *api.FleetDeployResponse
	lastLine := ""

	poll := func() (*api.FleetDeployResponse, bool, error) {
		state, err := api.GetFleetDeploy(client, projectID, fleetDeployID)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "Error checking fleet deploy status: %v\n", err)
			}
			return latest, false, nil
		}
		latest = state

		if !quiet {
			line := fmt.Sprintf("Batch %d/%d | status=%s | active=%d/%d | failed=%d",
				state.CurrentBatchIndex+1, state.TotalBatches,
				state.Status, state.CompletedCount, state.TotalServices, state.FailedCount)
			if line != lastLine {
				fmt.Println(line)
				lastLine = line
			}
		}

		if api.IsFleetDeployTerminal(state.Status) {
			return state, true, nil
		}
		return latest, false, nil
	}

	if state, done, err := poll(); done || err != nil {
		return state, err
	}

	for {
		select {
		case <-sigCh:
			if !quiet {
				fmt.Println("\nInterrupted. Fleet deploy continues in background.")
			}
			return latest, nil
		case <-ticker.C:
			if state, done, err := poll(); done || err != nil {
				return state, err
			}
		}
	}
}

func printFleetDeploySummary(resp *api.FleetDeployResponse) {
	idLabel := "(dry-run)"
	if resp.FleetDeployID != nil && *resp.FleetDeployID != "" {
		idLabel = *resp.FleetDeployID
	}
	fmt.Printf("Fleet Deploy: %s\n", idLabel)
	fmt.Printf("Status:       %s\n", output.StatusColor(resp.Status))
	fmt.Printf("Image:        %s\n", resp.ImageTag)
	fmt.Printf("Progress:     batch %d/%d (%d/%d active, %d failed)\n\n",
		resp.CurrentBatchIndex+1, resp.TotalBatches,
		resp.CompletedCount, resp.TotalServices, resp.FailedCount)

	headers := []string{"BATCH", "CANARY", "SERVICE", "STATUS", "DEPLOYMENT", "ERROR"}
	rows := make([][]string, len(resp.Items))
	for i, item := range resp.Items {
		canary := ""
		if item.IsCanary {
			canary = "yes"
		}
		depID := "-"
		if item.DeploymentID != nil && *item.DeploymentID != "" {
			depID = output.ShortID(*item.DeploymentID)
		}
		errMsg := ""
		if item.ErrorMessage != nil {
			errMsg = *item.ErrorMessage
		}
		rows[i] = []string{
			fmt.Sprintf("%d", item.BatchIndex),
			canary,
			item.ServiceName,
			output.StatusColor(item.Status),
			depID,
			errMsg,
		}
	}
	output.Table(headers, rows)
}

func init() {
	fleetDeployCmd.Flags().String("image", "", "pre-built image reference to deploy (required)")
	fleetDeployCmd.Flags().String("services", "", "comma-separated list of service IDs to target")
	fleetDeployCmd.Flags().Bool("canary", true, "deploy the first service alone as a canary batch")
	fleetDeployCmd.Flags().Int("batch-size", 3, "number of services per rollout batch")
	fleetDeployCmd.Flags().Bool("halt-on-failure", true, "halt rollout if any deployment fails")
	fleetDeployCmd.Flags().Bool("dry-run", false, "preview the rollout plan without creating deployments")
	fleetDeployCmd.Flags().Bool("wait", false, "poll until the fleet deploy reaches a terminal state")

	fleetCmd.AddCommand(fleetDeployCmd)
	fleetCmd.AddCommand(fleetStatusCmd)
	fleetCmd.AddCommand(fleetCancelCmd)
	rootCmd.AddCommand(fleetCmd)
}
