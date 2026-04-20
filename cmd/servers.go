package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
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
		client := newClient()
		servers, err := api.ListServers(client)
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(servers)
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
		client := newClient()
		token, err := api.CreateServer(client, args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Server created (id: %s)\n\n", output.ShortID(token.ServerID))
		fmt.Println("Run this command on the target server to install the agent:")
		fmt.Println()
		fmt.Printf("  %s\n", token.InstallCommand)
		fmt.Println()
		if token.ManualEnrollCommand != "" {
			fmt.Println("Or if the agent binary is already installed:")
			fmt.Println()
			fmt.Printf("  %s\n", token.ManualEnrollCommand)
			fmt.Println()
		}
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
		client := newClient()
		if err := api.DeleteServer(client, args[0]); err != nil {
			return err
		}
		fmt.Println("Server decommissioned.")
		return nil
	},
}

func formatCleanupBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

var serversCleanupCmd = &cobra.Command{
	Use:   "cleanup <server-id>",
	Short: "Preview or apply safe Docker cleanup on a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverID := args[0]
		apply, _ := cmd.Flags().GetBool("apply")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		keep, _ := cmd.Flags().GetInt("keep")

		if apply && dryRun {
			return fmt.Errorf("choose either --dry-run or --apply")
		}
		if !apply {
			dryRun = true
		}

		client := newClient()
		req := api.DockerCleanupRequest{KeepSuccessfulImages: keep}

		var (
			resp *api.DockerCleanupResponse
			err  error
		)
		if apply {
			resp, err = api.ApplyDockerCleanup(client, serverID, req)
		} else {
			resp, err = api.PreviewDockerCleanup(client, serverID, req)
		}
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(resp)
		}

		mode := "Preview"
		if apply {
			mode = "Applied"
		}
		fmt.Printf("%s docker cleanup on server %s\n", mode, output.ShortID(resp.ServerID))
		fmt.Printf("Keep successful images: %d\n", resp.KeepSuccessfulImages)
		fmt.Printf("Protected refs: %d\n", len(resp.ProtectedImageRefs))
		fmt.Printf("Stopped containers: %d (%s)\n", resp.StoppedContainers.Count, formatCleanupBytes(resp.StoppedContainers.ReclaimableBytes))
		fmt.Printf("Unused images: %d (%s)\n", resp.UnusedImages.Count, formatCleanupBytes(resp.UnusedImages.ReclaimableBytes))
		fmt.Printf("Build cache: %d (%s)\n", resp.BuildCache.Count, formatCleanupBytes(resp.BuildCache.ReclaimableBytes))
		fmt.Printf("Total reclaimable: %s\n", formatCleanupBytes(resp.ReclaimableBytes))
		if resp.ReclaimedBytesActual != nil {
			fmt.Printf("Actual reclaimed: %s\n", formatCleanupBytes(*resp.ReclaimedBytesActual))
		}
		return nil
	},
}

func init() {
	serversCmd.AddCommand(serversListCmd)
	serversCmd.AddCommand(serversAddCmd)
	serversCmd.AddCommand(serversRemoveCmd)
	serversCleanupCmd.Flags().Bool("dry-run", false, "preview reclaimable Docker artifacts without removing anything")
	serversCleanupCmd.Flags().Bool("apply", false, "remove reclaimable Docker artifacts using Clank's safe retention rules")
	serversCleanupCmd.Flags().Int("keep", 3, "number of successful deploy images per service to preserve")
	serversCmd.AddCommand(serversCleanupCmd)
	rootCmd.AddCommand(serversCmd)
}
