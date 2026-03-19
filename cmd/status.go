package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [service-id]",
	Short: "Show operational status overview",
	Long: `Show a dashboard-style overview of services and their status.

Use --project to see all services in a project at a glance, or pass a
service ID for a detailed single-service view including endpoints and
recent deployments.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	projectID, _ := cmd.Flags().GetString("project")

	if len(args) == 1 {
		return runServiceStatus(args[0])
	}

	if projectID == "" {
		return fmt.Errorf("provide a service-id or use --project <id>")
	}

	return runProjectStatus(projectID)
}

func runProjectStatus(projectID string) error {
	client := newClient()
	services, err := api.ListServices(client, projectID)
	if err != nil {
		return err
	}

	if output.IsJSON() {
		return output.JSON(services)
	}

	if len(services) == 0 {
		fmt.Println("No services found.")
		return nil
	}

	headers := []string{"SERVICE", "STATUS", "DEPLOYMENT", "UPDATED"}
	rows := make([][]string, len(services))
	for i, s := range services {
		status := "no deployment"
		if s.LatestDeploymentStatus != nil {
			status = *s.LatestDeploymentStatus
		}

		deployment := "-"
		if s.CurrentDeploymentID != nil {
			deployment = output.ShortID(*s.CurrentDeploymentID)
		}

		rows[i] = []string{
			s.Name,
			output.StatusColor(status),
			deployment,
			output.TimeSince(s.UpdatedAt),
		}
	}
	output.Table(headers, rows)
	return nil
}

func runServiceStatus(serviceID string) error {
	client := newClient()

	s, err := api.GetService(client, serviceID)
	if err != nil {
		return err
	}

	if output.IsJSON() {
		return output.JSON(s)
	}

	// Header
	status := "no deployment"
	if s.LatestDeploymentStatus != nil {
		status = *s.LatestDeploymentStatus
	}

	runtimeInfo := ""
	if s.RuntimeState != nil {
		runtimeInfo = s.RuntimeState.ContainerStatus
		if s.RuntimeState.IsHealthy != nil {
			if *s.RuntimeState.IsHealthy {
				runtimeInfo += ", healthy"
			} else {
				runtimeInfo += ", unhealthy"
			}
		}
	}

	fmt.Printf("Service:    %s\n", s.Name)
	if runtimeInfo != "" {
		fmt.Printf("Status:     %s (%s)\n", output.StatusColor(status), runtimeInfo)
	} else {
		fmt.Printf("Status:     %s\n", output.StatusColor(status))
	}

	if s.CurrentDeploymentID != nil {
		fmt.Printf("Deployment: %s (%s)\n", output.ShortID(*s.CurrentDeploymentID), output.TimeSince(s.UpdatedAt))
	} else {
		fmt.Printf("Deployment: none\n")
	}

	fmt.Printf("Branch:     %s\n", s.Branch)
	fmt.Printf("Port:       %d\n", s.Port)

	// Endpoints
	endpoints, err := api.ListEndpoints(client, serviceID)
	if err == nil && len(endpoints) > 0 {
		fmt.Println("Endpoints:")
		for _, ep := range endpoints {
			url := "-"
			if ep.URL != nil {
				url = *ep.URL
			}
			provider := ep.Provider
			if ep.ProviderDisplay != nil {
				provider = *ep.ProviderDisplay
			}
			marker := "  "
			if ep.IsPrimary {
				marker = "* "
			}
			fmt.Printf("  %s%s (%s)\n", marker, url, provider)
		}
	}

	// Recent deployments
	deployments, err := api.ListDeployments(client, serviceID)
	if err == nil && len(deployments) > 0 {
		fmt.Println("Recent:")
		limit := 5
		if len(deployments) < limit {
			limit = len(deployments)
		}
		headers := []string{"ID", "STATUS", "TRIGGERED BY", "WHEN"}
		rows := make([][]string, limit)
		for i := 0; i < limit; i++ {
			d := deployments[i]
			rows[i] = []string{
				output.ShortID(d.ID),
				output.StatusColor(d.Status),
				d.TriggeredBy,
				output.TimeSince(d.CreatedAt),
			}
		}
		output.Table(headers, rows)
	}

	return nil
}

func init() {
	statusCmd.Flags().String("project", "", "project ID for project-level overview")
	rootCmd.AddCommand(statusCmd)
}
