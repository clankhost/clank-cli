package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback <service-id>",
	Short: "Rollback a service to a previous deployment",
	Long: `Rollback a service to a previous deployment. If --to is not specified,
rolls back to the most recent deployment before the current one.`,
	Args: cobra.ExactArgs(1),
	RunE: runRollback,
}

func runRollback(cmd *cobra.Command, args []string) error {
	serviceID := args[0]
	targetID, _ := cmd.Flags().GetString("to")
	noFollow, _ := cmd.Flags().GetBool("no-follow")

	client := newClient()

	// If no target specified, find the previous deployment.
	if targetID == "" {
		var err error
		targetID, err = findPreviousDeployment(client, serviceID)
		if err != nil {
			return err
		}
		fmt.Printf("Rolling back to deployment %s\n", output.ShortID(targetID))
	}

	deployment, err := api.TriggerRollback(client, serviceID, targetID)
	if err != nil {
		if api.IsConflict(err) {
			return fmt.Errorf("a deployment is already in progress for this service")
		}
		return err
	}

	fmt.Printf("Rollback %s triggered (%s)\n", output.ShortID(deployment.ID), deployment.Status)

	if noFollow {
		fmt.Printf("Deployment ID: %s\n", deployment.ID)
		return nil
	}

	return followDeployment(client, deployment.ID)
}

// findPreviousDeployment finds the most recent deployment that is not the current one
// and has an image_tag (so it can be rolled back to).
func findPreviousDeployment(client *api.Client, serviceID string) (string, error) {
	svc, err := api.GetService(client, serviceID)
	if err != nil {
		return "", fmt.Errorf("getting service: %w", err)
	}

	deployments, err := api.ListDeployments(client, serviceID)
	if err != nil {
		return "", fmt.Errorf("listing deployments: %w", err)
	}

	for _, d := range deployments {
		// Skip the current active deployment.
		if svc.CurrentDeploymentID != nil && d.ID == *svc.CurrentDeploymentID {
			continue
		}
		// Must have an image tag to roll back to.
		if d.ImageTag == nil {
			continue
		}
		// Must have been active or superseded (not failed).
		if d.Status == "active" || d.Status == "superseded" {
			return d.ID, nil
		}
	}

	return "", fmt.Errorf("no previous deployment found to rollback to")
}

func init() {
	rollbackCmd.Flags().String("to", "", "target deployment ID to rollback to (auto-detects if omitted)")
	rollbackCmd.Flags().Bool("no-follow", false, "don't stream logs; just print deployment ID and exit")
	rootCmd.AddCommand(rollbackCmd)
}
