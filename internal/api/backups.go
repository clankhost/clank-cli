package api

import "fmt"

// Backup represents a service backup.
type Backup struct {
	ID           string  `json:"id"`
	ServiceID    string  `json:"service_id"`
	BackupType   string  `json:"backup_type"`
	Status       string  `json:"status"`
	DatabaseType *string `json:"database_type"`
	SizeBytes    *int64  `json:"size_bytes"`
	ErrorMessage *string `json:"error_message"`
	TriggeredBy  string  `json:"triggered_by"`
	CreatedAt    string  `json:"created_at"`
	CompletedAt  *string `json:"completed_at"`
}

// ListBackups returns all backups for a service.
func ListBackups(c *Client, serviceID string) ([]Backup, error) {
	var backups []Backup
	if err := c.get(fmt.Sprintf("/api/services/%s/backups", serviceID), &backups); err != nil {
		return nil, err
	}
	return backups, nil
}

// CreateBackup triggers a new backup for a service.
func CreateBackup(c *Client, serviceID string) (*Backup, error) {
	var backup Backup
	if err := c.post(fmt.Sprintf("/api/services/%s/backups", serviceID), nil, &backup); err != nil {
		return nil, err
	}
	return &backup, nil
}

// DeleteBackup deletes a backup.
func DeleteBackup(c *Client, serviceID, backupID string) error {
	return c.delete(fmt.Sprintf("/api/services/%s/backups/%s", serviceID, backupID))
}
