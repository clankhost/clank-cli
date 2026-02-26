package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage services",
}

var servicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List services in a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID, _ := cmd.Flags().GetString("project")
		if projectID == "" {
			return fmt.Errorf("--project is required")
		}

		client := api.New(cfg.BaseURL, cfg.Token)
		services, err := api.ListServices(client, projectID)
		if err != nil {
			return err
		}

		if len(services) == 0 {
			fmt.Println("No services found.")
			return nil
		}

		headers := []string{"ID", "NAME", "REPO", "BRANCH", "PORT", "STATUS", "CREATED"}
		rows := make([][]string, len(services))
		for i, s := range services {
			status := "no deployment"
			if s.CurrentDeploymentID != nil {
				status = "active"
			}
			rows[i] = []string{
				output.ShortID(s.ID),
				s.Name,
				s.RepoURL,
				s.Branch,
				fmt.Sprintf("%d", s.Port),
				output.StatusColor(status),
				output.TimeSince(s.CreatedAt),
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var servicesInfoCmd = &cobra.Command{
	Use:   "info <service-id>",
	Short: "Show service details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.BaseURL, cfg.Token)
		s, err := api.GetService(client, args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Name:        %s\n", s.Name)
		fmt.Printf("ID:          %s\n", s.ID)
		fmt.Printf("Project:     %s\n", s.ProjectID)
		fmt.Printf("Repo:        %s\n", s.RepoURL)
		fmt.Printf("Branch:      %s\n", s.Branch)
		fmt.Printf("Port:        %d\n", s.Port)
		fmt.Printf("Health:      %s\n", s.HealthCheckPath)
		fmt.Printf("Auto-deploy: %v\n", s.AutoDeploy)

		if s.CurrentDeploymentID != nil {
			fmt.Printf("Deployment:  %s\n", output.ShortID(*s.CurrentDeploymentID))
		} else {
			fmt.Printf("Deployment:  none\n")
		}

		return nil
	},
}

var servicesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new service",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID, _ := cmd.Flags().GetString("project")
		name, _ := cmd.Flags().GetString("name")
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		port, _ := cmd.Flags().GetInt("port")

		if projectID == "" || name == "" || repo == "" {
			return fmt.Errorf("--project, --name, and --repo are required")
		}

		client := api.New(cfg.BaseURL, cfg.Token)
		svc, err := api.CreateService(client, projectID, api.CreateServiceRequest{
			Name:    name,
			RepoURL: repo,
			Branch:  branch,
			Port:    port,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Created service %s (id: %s)\n", svc.Name, output.ShortID(svc.ID))
		return nil
	},
}

var servicesDeleteCmd = &cobra.Command{
	Use:   "delete <service-id>",
	Short: "Delete a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.BaseURL, cfg.Token)
		if err := api.DeleteService(client, args[0]); err != nil {
			return err
		}
		fmt.Println("Service deleted.")
		return nil
	},
}

func init() {
	servicesListCmd.Flags().String("project", "", "project ID (required)")
	servicesCreateCmd.Flags().String("project", "", "project ID (required)")
	servicesCreateCmd.Flags().String("name", "", "service name (required)")
	servicesCreateCmd.Flags().String("repo", "", "git repository URL (required)")
	servicesCreateCmd.Flags().String("branch", "main", "git branch")
	servicesCreateCmd.Flags().Int("port", 8080, "container port")

	servicesCmd.AddCommand(servicesListCmd)
	servicesCmd.AddCommand(servicesInfoCmd)
	servicesCmd.AddCommand(servicesCreateCmd)
	servicesCmd.AddCommand(servicesDeleteCmd)
	rootCmd.AddCommand(servicesCmd)
}
