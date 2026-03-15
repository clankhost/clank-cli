package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var deploymentsCmd = &cobra.Command{
	Use:     "deployments",
	Aliases: []string{"deps"},
	Short:   "View deployment history",
}

var deploymentsListCmd = &cobra.Command{
	Use:   "list <service-id>",
	Short: "List deployments for a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		deployments, err := api.ListDeployments(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(deployments)
		}

		if len(deployments) == 0 {
			fmt.Println("No deployments.")
			return nil
		}

		headers := []string{"ID", "STATUS", "TRIGGER", "GIT SHA", "CREATED"}
		rows := make([][]string, len(deployments))
		for i, d := range deployments {
			sha := "-"
			if d.GitSHA != nil {
				s := *d.GitSHA
				if len(s) > 7 {
					s = s[:7]
				}
				sha = s
			}
			rows[i] = []string{
				output.ShortID(d.ID),
				output.StatusColor(d.Status),
				d.TriggeredBy,
				sha,
				output.TimeSince(d.CreatedAt),
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var deploymentsInfoCmd = &cobra.Command{
	Use:   "info <deployment-id>",
	Short: "Show deployment details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		d, err := api.GetDeployment(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(d)
		}

		fmt.Printf("Deployment %s\n", output.ShortID(d.ID))
		fmt.Printf("  Status:      %s\n", output.StatusColor(d.Status))
		fmt.Printf("  Triggered:   %s\n", d.TriggeredBy)
		if d.GitSHA != nil {
			fmt.Printf("  Git SHA:     %s\n", *d.GitSHA)
		}
		if d.GitBranch != nil {
			fmt.Printf("  Branch:      %s\n", *d.GitBranch)
		}
		if d.ImageTag != nil {
			fmt.Printf("  Image:       %s\n", *d.ImageTag)
		}
		if d.ErrorMessage != nil {
			fmt.Printf("  Error:       %s\n", *d.ErrorMessage)
		}
		fmt.Printf("  Created:     %s\n", output.TimeSince(d.CreatedAt))
		if d.StartedAt != nil {
			fmt.Printf("  Started:     %s\n", output.TimeSince(*d.StartedAt))
		}
		if d.FinishedAt != nil {
			fmt.Printf("  Finished:    %s\n", output.TimeSince(*d.FinishedAt))
		}
		return nil
	},
}

var deploymentsEventsCmd = &cobra.Command{
	Use:   "events <deployment-id>",
	Short: "Show deployment lifecycle events",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		events, err := api.GetDeploymentEvents(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(events)
		}

		if len(events) == 0 {
			fmt.Println("No events.")
			return nil
		}

		headers := []string{"TIME", "TYPE", "MESSAGE"}
		rows := make([][]string, len(events))
		for i, e := range events {
			rows[i] = []string{
				output.TimeSince(e.CreatedAt),
				e.EventType,
				e.Message,
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

func init() {
	deploymentsCmd.AddCommand(deploymentsListCmd)
	deploymentsCmd.AddCommand(deploymentsInfoCmd)
	deploymentsCmd.AddCommand(deploymentsEventsCmd)
	rootCmd.AddCommand(deploymentsCmd)
}
