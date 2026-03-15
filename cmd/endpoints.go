package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var endpointsCmd = &cobra.Command{
	Use:     "endpoints <service-id>",
	Aliases: []string{"ep"},
	Short:   "List endpoints for a service",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		endpoints, err := api.ListEndpoints(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(endpoints)
		}

		if len(endpoints) == 0 {
			fmt.Println("No endpoints configured.")
			return nil
		}

		headers := []string{"PROVIDER", "STATUS", "URL", "PRIMARY"}
		rows := make([][]string, len(endpoints))
		for i, ep := range endpoints {
			url := "-"
			if ep.URL != nil {
				url = *ep.URL
			}
			provider := ep.Provider
			if ep.ProviderDisplay != nil {
				provider = *ep.ProviderDisplay
			}
			primary := ""
			if ep.IsPrimary {
				primary = "*"
			}
			rows[i] = []string{
				provider,
				output.StatusColor(ep.Status),
				url,
				primary,
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(endpointsCmd)
}
