package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/config"
	"github.com/clankhost/clank-cli/internal/output"
)

func setupBackupsTestServer(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	prevCfg := cfg
	prevFormat := output.Format
	prevInput := backupsInput

	cfg = &config.Config{BaseURL: srv.URL, Token: "clank_testtoken"}
	output.Format = ""
	backupsInput = nil

	t.Cleanup(func() {
		cfg = prevCfg
		output.Format = prevFormat
		backupsInput = prevInput
	})
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{5368709120, "5.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatBytes(tt.input)
			if got != tt.expected {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBackupsListTableAndResolveByName(t *testing.T) {
	const projectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const serviceID = "11111111-1111-1111-1111-111111111111"
	const backupID = "22222222-2222-2222-2222-222222222222"

	size := int64(2048)
	offsite := "uploaded"

	setupBackupsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_ = json.NewEncoder(w).Encode([]api.Project{
				{ID: projectID, Name: "Core", Slug: "core"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+projectID+"/services":
			_ = json.NewEncoder(w).Encode([]api.Service{
				{ID: serviceID, ProjectID: projectID, Name: "postgres-db", Slug: "postgres-db"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+serviceID+"/backups":
			_ = json.NewEncoder(w).Encode([]api.Backup{
				{
					ID:            backupID,
					ServiceID:     serviceID,
					BackupType:    "database",
					Status:        "completed",
					SizeBytes:     &size,
					OffsiteStatus: &offsite,
					TriggeredBy:   "scheduled",
					CreatedAt:     "2026-01-01T00:00:00Z",
				},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	stdout, _, err := captureStdIO(t, func() error {
		return runBackupsList(backupsListCmd, []string{"postgres-db"})
	})
	if err != nil {
		t.Fatalf("runBackupsList error: %v", err)
	}

	for _, want := range []string{"ID", "STATUS", "TYPE", "SIZE", "OFFSITE", "CREATED", "22222222", "completed", "database", "2.0 KB", "uploaded"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected stdout to contain %q, got:\n%s", want, stdout)
		}
	}
}

func TestBackupsListJSON(t *testing.T) {
	const serviceID = "11111111-1111-1111-1111-111111111111"
	const backupID = "22222222-2222-2222-2222-222222222222"

	setupBackupsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/services/"+serviceID+"/backups" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode([]api.Backup{
			{ID: backupID, ServiceID: serviceID, BackupType: "database", Status: "completed"},
		})
	}))

	output.Format = "json"
	stdout, _, err := captureStdIO(t, func() error {
		return runBackupsList(backupsListCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runBackupsList JSON error: %v", err)
	}

	var got []api.Backup
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%q", err, stdout)
	}
	if len(got) != 1 || got[0].ID != backupID {
		t.Errorf("unexpected backups JSON: %+v", got)
	}
}

func TestBackupsCreateJSON(t *testing.T) {
	const serviceID = "11111111-1111-1111-1111-111111111111"
	const backupID = "33333333-3333-3333-3333-333333333333"

	setupBackupsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/services/"+serviceID+"/backups" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(api.Backup{
			ID:         backupID,
			ServiceID:  serviceID,
			BackupType: "database",
			Status:     "pending",
		})
	}))

	output.Format = "json"
	stdout, _, err := captureStdIO(t, func() error {
		return runBackupsCreate(backupsCreateCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runBackupsCreate error: %v", err)
	}

	var got api.Backup
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%q", err, stdout)
	}
	if got.ID != backupID || got.Status != "pending" {
		t.Errorf("unexpected created backup: %+v", got)
	}
}

func TestBackupsRestoreWithYesFlagAndJSON(t *testing.T) {
	const serviceID = "11111111-1111-1111-1111-111111111111"
	const backupID = "44444444-4444-4444-4444-444444444444"

	restoreStatus := "pending"
	called := false

	setupBackupsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/services/"+serviceID+"/backups/"+backupID+"/restore" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		called = true
		_ = json.NewEncoder(w).Encode(api.Backup{
			ID:            backupID,
			ServiceID:     serviceID,
			BackupType:    "database",
			Status:        "completed",
			RestoreStatus: &restoreStatus,
		})
	}))

	resetFlags(backupsRestoreCmd.Flags())
	if err := backupsRestoreCmd.Flags().Set("yes", "true"); err != nil {
		t.Fatal(err)
	}
	output.Format = "json"

	stdout, _, err := captureStdIO(t, func() error {
		return runBackupsRestore(backupsRestoreCmd, []string{serviceID, backupID})
	})
	if err != nil {
		t.Fatalf("runBackupsRestore error: %v", err)
	}
	if !called {
		t.Fatal("expected restore endpoint to be called")
	}

	var got api.Backup
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%q", err, stdout)
	}
	if got.RestoreStatus == nil || *got.RestoreStatus != "pending" {
		t.Errorf("expected restore_status pending, got %+v", got)
	}
}

func TestBackupsRestoreInteractiveConfirmAndCancel(t *testing.T) {
	const serviceID = "11111111-1111-1111-1111-111111111111"
	const backupID = "55555555-5555-5555-5555-555555555555"

	restoreStatus := "pending"
	calls := 0

	setupBackupsTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(api.Backup{
			ID:            backupID,
			ServiceID:     serviceID,
			Status:        "completed",
			RestoreStatus: &restoreStatus,
		})
	}))

	// Cancel when user enters "n"
	resetFlags(backupsRestoreCmd.Flags())
	backupsInput = strings.NewReader("n\n")
	stdout, stderr, err := captureStdIO(t, func() error {
		return runBackupsRestore(backupsRestoreCmd, []string{serviceID, backupID})
	})
	if err != nil {
		t.Fatalf("runBackupsRestore cancel error: %v", err)
	}
	if calls != 0 {
		t.Errorf("expected 0 API calls when cancelled, got %d", calls)
	}
	if !strings.Contains(stderr, "overwrite current data") {
		t.Errorf("expected confirmation prompt on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Restore cancelled.") {
		t.Errorf("expected cancellation message on stdout, got %q", stdout)
	}

	// Proceed when user enters "y"
	resetFlags(backupsRestoreCmd.Flags())
	backupsInput = strings.NewReader("y\n")
	stdout, _, err = captureStdIO(t, func() error {
		return runBackupsRestore(backupsRestoreCmd, []string{serviceID, backupID})
	})
	if err != nil {
		t.Fatalf("runBackupsRestore confirm error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 API call when confirmed, got %d", calls)
	}
	if !strings.Contains(stdout, "Restore of backup 55555555 started (status: pending)") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}
