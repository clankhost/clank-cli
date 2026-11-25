package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/config"
	"github.com/clankhost/clank-cli/internal/output"
)

func setupUpTestServer(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	prevCfg := cfg
	prevFormat := output.Format

	cfg = &config.Config{BaseURL: srv.URL, Token: "clank_testtoken"}
	output.Format = ""

	t.Cleanup(func() {
		cfg = prevCfg
		output.Format = prevFormat
		_ = upCmd.Flags().Set("file", "clank.toml")
		_ = upCmd.Flags().Set("server", "")
		_ = upCmd.Flags().Set("dry-run", "false")
		_ = upCmd.Flags().Set("deploy", "false")
	})
}

func writeTempClankToml(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "clank.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write clank.toml: %v", err)
	}
	return path
}

func TestUpDryRunTableOutput(t *testing.T) {
	const projectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	svcID := "11111111-1111-1111-1111-111111111111"

	tomlBody := `
[project]
name = "my-app"

[[databases]]
name = "db"
engine = "postgres"
version = "16"

[[services]]
name = "web"
image = "nginx:alpine"
port = 8000
`
	tomlPath := writeTempClankToml(t, tomlBody)

	var received api.StackSyncRequest
	setupUpTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/projects/"+projectID+"/stack-sync" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(api.StackSyncResponse{
			ProjectID: projectID,
			DryRun:    true,
			Actions: []api.StackSyncAction{
				{Kind: "database", Name: "db", Action: "create"},
				{Kind: "service", Name: "web", Action: "update", ChangedFields: []string{"port"}, ServiceID: &svcID},
				{Kind: "service", Name: "worker", Action: "unchanged", ServiceID: &svcID},
			},
			Summary: api.StackSyncSummary{Create: 1, Update: 1, Unchanged: 1, Deployed: 0},
		})
	}))

	_ = upCmd.Flags().Set("file", tomlPath)
	_ = upCmd.Flags().Set("dry-run", "true")

	stdout, _, err := captureStdIO(t, func() error {
		return runUp(upCmd, []string{projectID})
	})
	if err != nil {
		t.Fatalf("runUp error: %v", err)
	}
	if !received.DryRun || received.Deploy || !strings.Contains(received.TomlContent, "nginx:alpine") {
		t.Fatalf("unexpected request payload: %+v", received)
	}
	for _, want := range []string{"Plan (dry-run)", "+ create", "~ update", "= unchanged", "Summary: 1 create, 1 update, 1 unchanged, 0 deployed"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in output, got:\n%s", want, stdout)
		}
	}
}

func TestUpApplyDeployJSONWithProjectFromToml(t *testing.T) {
	const projectID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	const serverID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	svcID := "11111111-1111-1111-1111-111111111111"
	depID := "dddddddd-dddd-dddd-dddd-dddddddddddd"

	tomlBody := `
[project]
name = "my-app"

[[services]]
name = "web"
image = "nginx:alpine"
`
	tomlPath := writeTempClankToml(t, tomlBody)

	var received api.StackSyncRequest
	setupUpTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_ = json.NewEncoder(w).Encode([]api.Project{
				{ID: projectID, Name: "my-app", Slug: "my-app"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+projectID+"/stack-sync":
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(api.StackSyncResponse{
				ProjectID: projectID,
				DryRun:    false,
				Actions: []api.StackSyncAction{
					{Kind: "service", Name: "web", Action: "create", ServiceID: &svcID, DeploymentID: &depID},
				},
				Summary: api.StackSyncSummary{Create: 1, Deployed: 1},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	output.Format = "json"
	_ = upCmd.Flags().Set("file", tomlPath)
	_ = upCmd.Flags().Set("server", serverID)
	_ = upCmd.Flags().Set("deploy", "true")

	stdout, _, err := captureStdIO(t, func() error {
		return runUp(upCmd, nil)
	})
	if err != nil {
		t.Fatalf("runUp error: %v", err)
	}
	if received.DryRun || !received.Deploy || received.ServerID == nil || *received.ServerID != serverID {
		t.Fatalf("unexpected request payload: %+v", received)
	}
	if !strings.Contains(stdout, `"deployed": 1`) {
		t.Errorf("expected deployed JSON output, got:\n%s", stdout)
	}
}

func TestUpMissingFileReturnsError(t *testing.T) {
	setupUpTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	_ = upCmd.Flags().Set("file", filepath.Join(t.TempDir(), "does-not-exist.toml"))

	err := runUp(upCmd, []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"})
	if err == nil {
		t.Fatalf("expected error for missing clank.toml")
	}
}

func TestUpRejectsDryRunWithDeploy(t *testing.T) {
	called := false
	setupUpTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	tomlPath := writeTempClankToml(t, "[project]\nname = \"my-app\"\n[[services]]\nname = \"web\"\nimage = \"nginx:alpine\"\n")
	_ = upCmd.Flags().Set("file", tomlPath)
	_ = upCmd.Flags().Set("dry-run", "true")
	_ = upCmd.Flags().Set("deploy", "true")

	err := runUp(upCmd, []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"})
	if err == nil {
		t.Fatalf("expected error when both --dry-run and --deploy are set")
	}
	if !strings.Contains(err.Error(), "--dry-run") || !strings.Contains(err.Error(), "--deploy") {
		t.Errorf("unexpected error message: %v", err)
	}
	if called {
		t.Error("API should not be called when --dry-run and --deploy are both set")
	}
}
