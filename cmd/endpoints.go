package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

var endpointsCmd = &cobra.Command{
	Use:     "endpoints",
	Aliases: []string{"ep"},
	Short:   "Manage endpoints for a service",
}

var endpointsListCmd = &cobra.Command{
	Use:   "list <service-id>",
	Short: "List endpoints for a service",
	Args:  cobra.ExactArgs(1),
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

		headers := []string{"ID", "PROVIDER", "STATUS", "URL", "PRIMARY"}
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
				output.ShortID(ep.ID),
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

var endpointsUpdateCmd = &cobra.Command{
	Use:   "update <endpoint-id>",
	Short: "Update endpoint hostname or primary status",
	Args:  cobra.ExactArgs(1),
	Example: `  clank endpoints update <endpoint-id> --hostname app.example.com
  clank endpoints update <endpoint-id> --primary`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hostnameFlag := cmd.Flags().Lookup("hostname")
		primaryFlag := cmd.Flags().Lookup("primary")

		if !hostnameFlag.Changed && !primaryFlag.Changed {
			return fmt.Errorf("at least one of --hostname or --primary is required")
		}

		var req api.UpdateEndpointRequest

		if hostnameFlag.Changed {
			h, _ := cmd.Flags().GetString("hostname")
			req.Hostname = &h
		}

		if primaryFlag.Changed {
			p, _ := cmd.Flags().GetBool("primary")
			req.IsPrimary = &p
		}

		client := newClient()
		ep, err := api.UpdateEndpoint(client, args[0], req)
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(ep)
		}

		url := "-"
		if ep.URL != nil {
			url = *ep.URL
		}
		fmt.Printf("Updated endpoint %s\n", output.ShortID(ep.ID))
		fmt.Printf("  URL:    %s\n", url)
		fmt.Printf("  Status: %s\n", output.StatusColor(ep.Status))
		return nil
	},
}

var endpointsRemoveCmd = &cobra.Command{
	Use:   "remove <endpoint-id>",
	Short: "Remove an endpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		if err := api.DeleteEndpoint(client, args[0]); err != nil {
			return err
		}
		fmt.Println("Endpoint removed.")
		return nil
	},
}

var endpointsCheckCmd = &cobra.Command{
	Use:   "check <endpoint-id>",
	Short: "Verify endpoint health and status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		ep, err := api.CheckEndpoint(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(ep)
		}

		url := "-"
		if ep.URL != nil {
			url = *ep.URL
		}
		fmt.Printf("Endpoint %s — %s\n", output.ShortID(ep.ID), output.StatusColor(ep.Status))
		fmt.Printf("  URL: %s\n", url)
		if ep.LastError != nil {
			fmt.Printf("  Error: %s\n", *ep.LastError)
		}
		return nil
	},
}

func init() {
	endpointsUpdateCmd.Flags().String("hostname", "", "endpoint hostname/domain")
	endpointsUpdateCmd.Flags().Bool("primary", false, "set as primary endpoint")

	endpointsCmd.AddCommand(endpointsListCmd)
	endpointsCmd.AddCommand(endpointsUpdateCmd)
	endpointsCmd.AddCommand(endpointsRemoveCmd)
	endpointsCmd.AddCommand(endpointsCheckCmd)
	rootCmd.AddCommand(endpointsCmd)
}
