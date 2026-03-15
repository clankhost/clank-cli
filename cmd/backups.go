package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var backupsCmd = &cobra.Command{
	Use:   "backups",
	Short: "Manage backups for a service",
}

var backupsListCmd = &cobra.Command{
	Use:   "list <service-id>",
	Short: "List backups for a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		backups, err := api.ListBackups(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(backups)
		}

		if len(backups) == 0 {
			fmt.Println("No backups.")
			return nil
		}

		headers := []string{"ID", "TYPE", "STATUS", "SIZE", "TRIGGER", "CREATED"}
		rows := make([][]string, len(backups))
		for i, b := range backups {
			size := "-"
			if b.SizeBytes != nil {
				size = formatBytes(*b.SizeBytes)
			}
			rows[i] = []string{
				output.ShortID(b.ID),
				b.BackupType,
				output.StatusColor(b.Status),
				size,
				b.TriggeredBy,
				output.TimeSince(b.CreatedAt),
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var backupsCreateCmd = &cobra.Command{
	Use:   "create <service-id>",
	Short: "Create a backup of a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		backup, err := api.CreateBackup(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(backup)
		}

		fmt.Printf("Backup %s created (status: %s)\n", output.ShortID(backup.ID), backup.Status)
		return nil
	},
}

var backupsDeleteCmd = &cobra.Command{
	Use:   "delete <service-id> <backup-id>",
	Short: "Delete a backup",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		if err := api.DeleteBackup(client, args[0], args[1]); err != nil {
			return err
		}
		fmt.Println("Backup deleted.")
		return nil
	},
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func init() {
	backupsCmd.AddCommand(backupsListCmd)
	backupsCmd.AddCommand(backupsCreateCmd)
	backupsCmd.AddCommand(backupsDeleteCmd)
	rootCmd.AddCommand(backupsCmd)
}
