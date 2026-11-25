package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/config"
	"github.com/clankhost/clank-cli/internal/output"
)

func setupFleetTestServer(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	prevCfg := cfg
	prevFormat := output.Format
	prevInterval := fleetPollInterval

	cfg = &config.Config{BaseURL: srv.URL, Token: "clank_testtoken"}
	output.Format = ""
	fleetPollInterval = 10 * time.Millisecond

	t.Cleanup(func() {
		cfg = prevCfg
		output.Format = prevFormat
		fleetPollInterval = prevInterval
		_ = fleetDeployCmd.Flags().Set("image", "")
		_ = fleetDeployCmd.Flags().Set("services", "")
		_ = fleetDeployCmd.Flags().Set("canary", "true")
		_ = fleetDeployCmd.Flags().Set("batch-size", "3")
		_ = fleetDeployCmd.Flags().Set("halt-on-failure", "true")
		_ = fleetDeployCmd.Flags().Set("dry-run", "false")
		_ = fleetDeployCmd.Flags().Set("wait", "false")
	})
}

func TestFleetDeployDryRunJSON(t *testing.T) {
	const projectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	var received api.FleetDeployRequest
	setupFleetTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/projects/"+projectID+"/fleet-deploy" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(api.FleetDeployResponse{
			ProjectID:     projectID,
			ImageTag:      received.ImageTag,
			Status:        "dry_run",
			CanaryFirst:   received.CanaryFirst,
			BatchSize:     received.BatchSize,
			HaltOnFailure: received.HaltOnFailure,
			TotalBatches:  2,
			TotalServices: 2,
			Items: []api.FleetDeployItem{
				{ServiceID: "svc-1", ServiceName: "alpha", BatchIndex: 0, IsCanary: true, Status: "pending"},
				{ServiceID: "svc-2", ServiceName: "bravo", BatchIndex: 1, IsCanary: false, Status: "pending"},
			},
		})
	}))

	output.Format = "json"
	_ = fleetDeployCmd.Flags().Set("image", "ghcr.io/org/app:v2")
	_ = fleetDeployCmd.Flags().Set("services", "svc-1, svc-2")
	_ = fleetDeployCmd.Flags().Set("dry-run", "true")

	stdout, _, err := captureStdIO(t, func() error {
		return runFleetDeploy(fleetDeployCmd, []string{projectID})
	})
	if err != nil {
		t.Fatalf("runFleetDeploy error: %v", err)
	}
	if received.ImageTag != "ghcr.io/org/app:v2" || !received.DryRun || len(received.ServiceIDs) != 2 {
		t.Fatalf("unexpected request payload: %+v", received)
	}
	if !strings.Contains(stdout, `"status": "dry_run"`) {
		t.Errorf("expected dry_run JSON output, got:\n%s", stdout)
	}
}

func TestFleetDeployWaitCompletes(t *testing.T) {
	const projectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	fleetID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	depID := "dddddddd-dddd-dddd-dddd-dddddddddddd"

	var polls atomic.Int32
	setupFleetTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+projectID+"/fleet-deploy":
			_ = json.NewEncoder(w).Encode(api.FleetDeployResponse{
				FleetDeployID: &fleetID,
				ProjectID:     projectID,
				ImageTag:      "ghcr.io/org/app:v2",
				Status:        "running",
				TotalBatches:  2,
				TotalServices: 2,
				Items: []api.FleetDeployItem{
					{ServiceID: "s1", ServiceName: "alpha", BatchIndex: 0, IsCanary: true, Status: "deploying", DeploymentID: &depID},
					{ServiceID: "s2", ServiceName: "bravo", BatchIndex: 1, IsCanary: false, Status: "pending"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+projectID+"/fleet-deploy/"+fleetID:
			n := polls.Add(1)
			status := "running"
			completed := 1
			batchIdx := 1
			if n >= 2 {
				status = "completed"
				completed = 2
			}
			_ = json.NewEncoder(w).Encode(api.FleetDeployResponse{
				FleetDeployID:     &fleetID,
				ProjectID:         projectID,
				ImageTag:          "ghcr.io/org/app:v2",
				Status:            status,
				CurrentBatchIndex: batchIdx,
				TotalBatches:      2,
				TotalServices:     2,
				CompletedCount:    completed,
				Items: []api.FleetDeployItem{
					{ServiceID: "s1", ServiceName: "alpha", BatchIndex: 0, IsCanary: true, Status: "active", DeploymentID: &depID},
					{ServiceID: "s2", ServiceName: "bravo", BatchIndex: 1, IsCanary: false, Status: "active", DeploymentID: &depID},
				},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	_ = fleetDeployCmd.Flags().Set("image", "ghcr.io/org/app:v2")
	_ = fleetDeployCmd.Flags().Set("wait", "true")

	stdout, _, err := captureStdIO(t, func() error {
		return runFleetDeploy(fleetDeployCmd, []string{projectID})
	})
	if err != nil {
		t.Fatalf("expected nil error on completed fleet deploy, got: %v", err)
	}
	if !strings.Contains(stdout, "completed") {
		t.Errorf("expected completed status in output, got:\n%s", stdout)
	}
}

func TestFleetDeployWaitHaltedReturnsError(t *testing.T) {
	const projectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	fleetID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	errMsg := "healthcheck failed"

	setupFleetTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+projectID+"/fleet-deploy":
			_ = json.NewEncoder(w).Encode(api.FleetDeployResponse{
				FleetDeployID: &fleetID,
				ProjectID:     projectID,
				ImageTag:      "ghcr.io/org/app:bad",
				Status:        "running",
				TotalBatches:  2,
				TotalServices: 2,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+projectID+"/fleet-deploy/"+fleetID:
			_ = json.NewEncoder(w).Encode(api.FleetDeployResponse{
				FleetDeployID: &fleetID,
				ProjectID:     projectID,
				ImageTag:      "ghcr.io/org/app:bad",
				Status:        "halted",
				TotalBatches:  2,
				TotalServices: 2,
				FailedCount:   1,
				Items: []api.FleetDeployItem{
					{ServiceID: "s1", ServiceName: "alpha", BatchIndex: 0, IsCanary: true, Status: "failed", ErrorMessage: &errMsg},
					{ServiceID: "s2", ServiceName: "bravo", BatchIndex: 1, IsCanary: false, Status: "skipped"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	_ = fleetDeployCmd.Flags().Set("image", "ghcr.io/org/app:bad")
	_ = fleetDeployCmd.Flags().Set("wait", "true")

	_, _, err := captureStdIO(t, func() error {
		return runFleetDeploy(fleetDeployCmd, []string{projectID})
	})
	if err == nil {
		t.Fatalf("expected non-nil error when fleet deploy halts")
	}
	if !strings.Contains(err.Error(), "halted") {
		t.Errorf("expected error to mention halted, got: %v", err)
	}
}
