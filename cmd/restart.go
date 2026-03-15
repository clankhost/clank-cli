package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <service-id>",
	Short: "Restart a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
	Use:   "stop <service-id>",
	Short: "Stop a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
	Use:   "start <service-id>",
	Short: "Start a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

func init() {
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(startCmd)
}
