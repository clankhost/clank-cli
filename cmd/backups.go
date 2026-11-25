package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

// backupsInput overrides stdin for interactive confirmation prompts in tests.
var backupsInput io.Reader

var backupsCmd = &cobra.Command{
	Use:   "backups",
	Short: "Manage backups for a service",
}

var backupsListCmd = &cobra.Command{
	Use:   "list <service>",
	Short: "List backups for a service",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupsList,
}

func runBackupsList(cmd *cobra.Command, args []string) error {
	client := newClient()
	serviceID, err := resolveServiceID(client, args[0])
	if err != nil {
		return err
	}

	backups, err := api.ListBackups(client, serviceID)
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

	headers := []string{"ID", "STATUS", "TYPE", "SIZE", "OFFSITE", "CREATED"}
	rows := make([][]string, len(backups))
	for i, b := range backups {
		size := "-"
		if b.SizeBytes != nil {
			size = formatBytes(*b.SizeBytes)
		}
		offsite := "-"
		if b.OffsiteStatus != nil && *b.OffsiteStatus != "" {
			offsite = output.StatusColor(*b.OffsiteStatus)
		}
		rows[i] = []string{
			output.ShortID(b.ID),
			output.StatusColor(b.Status),
			b.BackupType,
			size,
			offsite,
			output.TimeSince(b.CreatedAt),
		}
	}
	output.Table(headers, rows)
	return nil
}

var backupsCreateCmd = &cobra.Command{
	Use:   "create <service>",
	Short: "Create a backup of a service",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupsCreate,
}

func runBackupsCreate(cmd *cobra.Command, args []string) error {
	client := newClient()
	serviceID, err := resolveServiceID(client, args[0])
	if err != nil {
		return err
	}

	backup, err := api.CreateBackup(client, serviceID)
	if err != nil {
		return err
	}

	if output.IsJSON() {
		return output.JSON(backup)
	}

	fmt.Printf("Backup %s created (status: %s)\n", output.ShortID(backup.ID), backup.Status)
	return nil
}

var backupsRestoreCmd = &cobra.Command{
	Use:   "restore <service> <backup-id>",
	Short: "Restore a service from a backup",
	Args:  cobra.ExactArgs(2),
	RunE:  runBackupsRestore,
}

func runBackupsRestore(cmd *cobra.Command, args []string) error {
	client := newClient()
	serviceID, err := resolveServiceID(client, args[0])
	if err != nil {
		return err
	}
	backupID := args[1]

	yes, _ := cmd.Flags().GetBool("yes")
	if !yes {
		fmt.Fprintf(
			os.Stderr,
			"Restore backup %s to service %s? This will overwrite current data. [y/N]: ",
			output.ShortID(backupID),
			args[0],
		)
		in := backupsInput
		if in == nil {
			in = os.Stdin
		}
		reader := bufio.NewReader(in)
		line, _ := reader.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			fmt.Println("Restore cancelled.")
			return nil
		}
	}

	backup, err := api.RestoreBackup(client, serviceID, backupID)
	if err != nil {
		return err
	}

	if output.IsJSON() {
		return output.JSON(backup)
	}

	status := backup.Status
	if backup.RestoreStatus != nil && *backup.RestoreStatus != "" {
		status = *backup.RestoreStatus
	}
	fmt.Printf("Restore of backup %s started (status: %s)\n", output.ShortID(backup.ID), status)
	return nil
}

var backupsDeleteCmd = &cobra.Command{
	Use:   "delete <service> <backup-id>",
	Short: "Delete a backup",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		serviceID, err := resolveServiceID(client, args[0])
		if err != nil {
			return err
		}
		if err := api.DeleteBackup(client, serviceID, args[1]); err != nil {
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
	backupsRestoreCmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
	backupsCmd.AddCommand(backupsListCmd)
	backupsCmd.AddCommand(backupsCreateCmd)
	backupsCmd.AddCommand(backupsRestoreCmd)
	backupsCmd.AddCommand(backupsDeleteCmd)
	rootCmd.AddCommand(backupsCmd)
}
