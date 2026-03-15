package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage projects",
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		projects, err := api.ListProjects(client)
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(projects)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return nil
		}

		headers := []string{"ID", "NAME", "SLUG", "CREATED"}
		rows := make([][]string, len(projects))
		for i, p := range projects {
			rows[i] = []string{
				p.ID,
				p.Name,
				p.Slug,
				output.TimeSince(p.CreatedAt),
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var projectsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		desc, _ := cmd.Flags().GetString("description")

		client := newClient()
		project, err := api.CreateProject(client, name, desc)
		if err != nil {
			return err
		}

		fmt.Printf("Created project %s (id: %s)\n", project.Name, output.ShortID(project.ID))
		return nil
	},
}

var projectsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		if err := api.DeleteProject(client, args[0]); err != nil {
			return err
		}
		fmt.Println("Project deleted.")
		return nil
	},
}

func init() {
	projectsCreateCmd.Flags().String("description", "", "project description")
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsCreateCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	rootCmd.AddCommand(projectsCmd)
}
