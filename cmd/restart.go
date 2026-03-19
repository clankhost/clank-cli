package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [service-id]",
	Short: "Restart a service",
	Long: `Restart a service's container. Use --all --project to restart every service
in a project at once.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			return runBulkAction(cmd, "restart", api.RestartService)
		}
		if len(args) == 0 {
			return fmt.Errorf("service-id is required (or use --all --project)")
		}
		client := newClient()
		resp, err := api.RestartService(client, args[0])
		if err != nil {
			return err
		}
		if output.IsJSON() {
			return output.JSON(resp)
		}
		fmt.Printf("Service %s\n", resp.Status)
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [service-id]",
	Short: "Stop a service",
	Long: `Stop a service's container. Use --all --project to stop every service
in a project at once.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			return runBulkAction(cmd, "stop", api.StopService)
		}
		if len(args) == 0 {
			return fmt.Errorf("service-id is required (or use --all --project)")
		}
		client := newClient()
		resp, err := api.StopService(client, args[0])
		if err != nil {
			return err
		}
		if output.IsJSON() {
			return output.JSON(resp)
		}
		fmt.Printf("Service %s\n", resp.Status)
		return nil
	},
}

var startCmd = &cobra.Command{
	Use:   "start [service-id]",
	Short: "Start a service",
	Long: `Start a service's container. Use --all --project to start every service
in a project at once.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			return runBulkAction(cmd, "start", api.StartService)
		}
		if len(args) == 0 {
			return fmt.Errorf("service-id is required (or use --all --project)")
		}
		client := newClient()
		resp, err := api.StartService(client, args[0])
		if err != nil {
			return err
		}
		if output.IsJSON() {
			return output.JSON(resp)
		}
		fmt.Printf("Service %s\n", resp.Status)
		return nil
	},
}

type serviceActionFn func(c *api.Client, serviceID string) (*api.ContainerControlResponse, error)

func runBulkAction(cmd *cobra.Command, action string, fn serviceActionFn) error {
	projectID, _ := cmd.Flags().GetString("project")
	if projectID == "" {
		return fmt.Errorf("--project is required with --all")
	}

	client := newClient()
	services, err := api.ListServices(client, projectID)
	if err != nil {
		return err
	}

	// Only act on services that have an active deployment.
	var targets []api.Service
	for _, s := range services {
		if s.CurrentDeploymentID != nil {
			targets = append(targets, s)
		}
	}

	if len(targets) == 0 {
		fmt.Println("No deployed services found in this project.")
		return nil
	}

	succeeded := 0
	for _, s := range targets {
		fmt.Printf("%sing %s... ", action, s.Name)
		_, err := fn(client, s.ID)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Println("done")
			succeeded++
		}
	}

	fmt.Printf("\n%sed %d/%d services.\n", action, succeeded, len(targets))
	if succeeded < len(targets) {
		return fmt.Errorf("some services failed")
	}
	return nil
}

func init() {
	for _, cmd := range []*cobra.Command{restartCmd, stopCmd, startCmd} {
		cmd.Flags().Bool("all", false, "apply to all services in a project")
		cmd.Flags().String("project", "", "project ID (required with --all)")
		rootCmd.AddCommand(cmd)
	}
}
