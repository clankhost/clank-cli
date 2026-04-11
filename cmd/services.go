package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
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

		headers := []string{"ID", "NAME", "REPO", "BRANCH", "PORT", "STATUS", "CREATED"}
		rows := make([][]string, len(services))
		for i, s := range services {
			status := "no deployment"
			if s.LatestDeploymentStatus != nil {
				status = *s.LatestDeploymentStatus
			} else if s.CurrentDeploymentID != nil {
				status = "active"
			}
			rows[i] = []string{
				s.ID,
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
		client := newClient()
		s, err := api.GetService(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(s)
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
	Example: `  clank services create --project <id> --name web --repo user/repo
  clank services create --project <id> --name web --repo user/repo --server <server-id>
  clank services create --project <id> --name app --image nginx:latest --server <server-id>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID, _ := cmd.Flags().GetString("project")
		name, _ := cmd.Flags().GetString("name")
		repo, _ := cmd.Flags().GetString("repo")
		image, _ := cmd.Flags().GetString("image")
		branch, _ := cmd.Flags().GetString("branch")
		port, _ := cmd.Flags().GetInt("port")
		serverID, _ := cmd.Flags().GetString("server")

		if projectID == "" || name == "" {
			return fmt.Errorf("--project and --name are required")
		}
		if repo == "" && image == "" {
			return fmt.Errorf("one of --repo or --image is required")
		}

		req := api.CreateServiceRequest{
			Name:     name,
			RepoURL:  repo,
			Branch:   branch,
			Port:     port,
			ServerID: serverID,
		}
		if image != "" {
			req.ResourceType = "docker_image"
			req.Image = image
		}

		client := newClient()
		svc, err := api.CreateService(client, projectID, req)
		if err != nil {
			return err
		}

		fmt.Printf("Created service %s (id: %s)\n", svc.Name, output.ShortID(svc.ID))
		return nil
	},
}

var servicesUpdateCmd = &cobra.Command{
	Use:   "update <service-id>",
	Short: "Update a service's configuration",
	Args:  cobra.ExactArgs(1),
	Example: `  clank services update <service-id> --server <server-id>
  clank services update <service-id> --name new-name
  clank services update <service-id> --port 3000`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var req api.UpdateServiceRequest
		changed := false

		if cmd.Flags().Changed("server") {
			v, _ := cmd.Flags().GetString("server")
			req.ServerID = &v
			changed = true
		}
		if cmd.Flags().Changed("name") {
			v, _ := cmd.Flags().GetString("name")
			req.Name = &v
			changed = true
		}
		if cmd.Flags().Changed("port") {
			v, _ := cmd.Flags().GetInt("port")
			req.Port = &v
			changed = true
		}
		if cmd.Flags().Changed("branch") {
			v, _ := cmd.Flags().GetString("branch")
			req.Branch = &v
			changed = true
		}

		if !changed {
			return fmt.Errorf("at least one flag is required (--server, --name, --port, --branch)")
		}

		client := newClient()
		svc, err := api.UpdateService(client, args[0], req)
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(svc)
		}

		fmt.Printf("Updated service %s\n", svc.Name)
		if req.ServerID != nil {
			fmt.Printf("  Server: %s\n", *req.ServerID)
			fmt.Println("\nRedeploy to move workloads to the new server.")
		}
		return nil
	},
}

var servicesDeleteCmd = &cobra.Command{
	Use:   "delete <service-id>",
	Short: "Delete a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
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
	servicesCreateCmd.Flags().String("repo", "", "git repository URL")
	servicesCreateCmd.Flags().String("image", "", "Docker image (for image-based services)")
	servicesCreateCmd.Flags().String("branch", "main", "git branch")
	servicesCreateCmd.Flags().Int("port", 8080, "container port")
	servicesCreateCmd.Flags().String("server", "", "server ID to deploy on")

	servicesUpdateCmd.Flags().String("server", "", "reassign to a different server")
	servicesUpdateCmd.Flags().String("name", "", "rename the service")
	servicesUpdateCmd.Flags().Int("port", 0, "change container port")
	servicesUpdateCmd.Flags().String("branch", "", "change git branch")

	servicesCmd.AddCommand(servicesListCmd)
	servicesCmd.AddCommand(servicesInfoCmd)
	servicesCmd.AddCommand(servicesCreateCmd)
	servicesCmd.AddCommand(servicesUpdateCmd)
	servicesCmd.AddCommand(servicesDeleteCmd)
	rootCmd.AddCommand(servicesCmd)
}
