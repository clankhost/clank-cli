package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var serversCmd = &cobra.Command{
	Use:   "servers",
	Short: "Manage servers (agent hosts)",
}

var serversListCmd = &cobra.Command{
	Use:   "list",
	Short: "List enrolled servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.BaseURL, cfg.Token)
		servers, err := api.ListServers(client)
		if err != nil {
			return err
		}

		if len(servers) == 0 {
			fmt.Println("No servers found.")
			return nil
		}

		headers := []string{"ID", "NAME", "STATUS", "HOSTNAME", "HEARTBEAT", "CREATED"}
		rows := make([][]string, len(servers))
		for i, s := range servers {
			hostname := "-"
			if s.Hostname != nil {
				hostname = *s.Hostname
			}
			heartbeat := "-"
			if s.LastHeartbeatAt != nil {
				heartbeat = output.TimeSince(*s.LastHeartbeatAt)
			}
			rows[i] = []string{
				output.ShortID(s.ID),
				s.Name,
				output.StatusColor(s.Status),
				hostname,
				heartbeat,
				output.TimeSince(s.CreatedAt),
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var serversAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new server and get enrollment token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.BaseURL, cfg.Token)
		token, err := api.CreateServer(client, args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Server created (id: %s)\n\n", output.ShortID(token.ServerID))
		fmt.Println("Run this command on the target server to enroll the agent:")
		fmt.Println()
		fmt.Printf("  %s\n", token.InstallCommand)
		fmt.Println()
		fmt.Printf("Token expires: %s\n", output.TimeSince(token.ExpiresAt))
		fmt.Println("This token is shown once and cannot be retrieved again.")
		return nil
	},
}

var serversRemoveCmd = &cobra.Command{
	Use:   "remove <server-id>",
	Short: "Decommission a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.BaseURL, cfg.Token)
		if err := api.DeleteServer(client, args[0]); err != nil {
			return err
		}
		fmt.Println("Server decommissioned.")
		return nil
	},
}

func init() {
	serversCmd.AddCommand(serversListCmd)
	serversCmd.AddCommand(serversAddCmd)
	serversCmd.AddCommand(serversRemoveCmd)
	rootCmd.AddCommand(serversCmd)
}
