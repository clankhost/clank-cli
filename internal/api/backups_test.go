package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBackupsAPI(t *testing.T) {
	const serviceID = "11111111-1111-1111-1111-111111111111"
	const backupID = "22222222-2222-2222-2222-222222222222"

	size := int64(4096)
	offsiteStatus := "uploaded"
	offsiteKey := "backups/svc/bk.tar.gz"
	restoreStatus := "pending"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+serviceID+"/backups":
			_ = json.NewEncoder(w).Encode([]Backup{
				{
					ID:            backupID,
					ServiceID:     serviceID,
					BackupType:    "database",
					Status:        "completed",
					SizeBytes:     &size,
					OffsiteKey:    &offsiteKey,
					OffsiteStatus: &offsiteStatus,
					TriggeredBy:   "scheduled",
					CreatedAt:     "2026-01-01T00:00:00Z",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/"+serviceID+"/backups":
			_ = json.NewEncoder(w).Encode(Backup{
				ID:          backupID,
				ServiceID:   serviceID,
				BackupType:  "database",
				Status:      "pending",
				TriggeredBy: "manual",
				CreatedAt:   "2026-01-01T00:00:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/"+serviceID+"/backups/"+backupID+"/restore":
			_ = json.NewEncoder(w).Encode(Backup{
				ID:            backupID,
				ServiceID:     serviceID,
				BackupType:    "database",
				Status:        "completed",
				RestoreStatus: &restoreStatus,
				TriggeredBy:   "manual",
				CreatedAt:     "2026-01-01T00:00:00Z",
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/services/"+serviceID+"/backups/"+backupID:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New(server.URL, "token")

	list, err := ListBackups(client, serviceID)
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(list))
	}
	if list[0].OffsiteStatus == nil || *list[0].OffsiteStatus != "uploaded" {
		t.Errorf("expected offsite_status uploaded, got %v", list[0].OffsiteStatus)
	}
	if list[0].OffsiteKey == nil || *list[0].OffsiteKey != offsiteKey {
		t.Errorf("expected offsite_key %q, got %v", offsiteKey, list[0].OffsiteKey)
	}

	created, err := CreateBackup(client, serviceID)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	if created.ID != backupID || created.Status != "pending" {
		t.Errorf("unexpected created backup: %+v", created)
	}

	restored, err := RestoreBackup(client, serviceID, backupID)
	if err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}
	if restored.RestoreStatus == nil || *restored.RestoreStatus != "pending" {
		t.Errorf("expected restore_status pending, got %v", restored.RestoreStatus)
	}

	if err := DeleteBackup(client, serviceID, backupID); err != nil {
		t.Fatalf("DeleteBackup failed: %v", err)
	}
}
