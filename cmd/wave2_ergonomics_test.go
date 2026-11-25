package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/config"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/pflag"
)

// captureStdout runs fn while capturing os.Stdout and returns the captured string.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), runErr
}

// resetCommandFlags resets all flag values and Changed states on a cobra command.
func resetFlags(fs *pflag.FlagSet) {
	fs.VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

func setupTestServer(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	prevCfg := cfg
	prevFormat := output.Format
	prevInterval := waitPollInterval

	cfg = &config.Config{BaseURL: srv.URL, Token: "test-token"}
	output.Format = ""
	waitPollInterval = 5 * time.Millisecond

	t.Cleanup(func() {
		cfg = prevCfg
		output.Format = prevFormat
		waitPollInterval = prevInterval
	})
}

func TestDeployWithImageAndJSONOutput(t *testing.T) {
	const serviceID = "11111111-1111-1111-1111-111111111111"
	const wantImage = "registry.clank.host/acme/web:sha-abc123"

	var received map[string]any
	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/services/"+serviceID+"/deploy" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		img := wantImage
		_ = json.NewEncoder(w).Encode(api.Deployment{
			ID:          "dep-1111-2222",
			ServiceID:   serviceID,
			Status:      "pending",
			ImageTag:    &img,
			TriggeredBy: "manual",
		})
	}))

	resetFlags(deployCmd.Flags())
	output.Format = "json"
	if err := deployCmd.Flags().Set("image", wantImage); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return runDeploy(deployCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runDeploy error: %v", err)
	}

	if received["image"] != wantImage {
		t.Errorf("request body image = %v, want %q", received["image"], wantImage)
	}
	if received["image_tag"] != wantImage {
		t.Errorf("request body image_tag = %v, want %q", received["image_tag"], wantImage)
	}

	var got api.Deployment
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout=%q", err, out)
	}
	if got.ID != "dep-1111-2222" || got.ImageTag == nil || *got.ImageTag != wantImage {
		t.Errorf("unexpected deployment JSON: %+v", got)
	}
}

func TestServicesRestartByNameWithWaitAndJSON(t *testing.T) {
	const projectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const serviceID = "22222222-2222-2222-2222-222222222222"
	const depID = "33333333-3333-3333-3333-333333333333"

	var polls atomic.Int32
	var restarted bool

	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_ = json.NewEncoder(w).Encode([]api.Project{
				{ID: projectID, Name: "Core", Slug: "core"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+projectID+"/services":
			_ = json.NewEncoder(w).Encode([]api.Service{
				{ID: serviceID, ProjectID: projectID, Name: "api-worker", Slug: "api-worker"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/"+serviceID+"/restart":
			restarted = true
			img := "clank-api-worker:abc123"
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(api.Deployment{
				ID:          depID,
				ServiceID:   serviceID,
				Status:      "pending",
				ImageTag:    &img,
				TriggeredBy: "redeploy",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/"+depID:
			n := polls.Add(1)
			status := "deploying"
			if n >= 2 {
				status = "active"
			}
			img := "clank-api-worker:abc123"
			_ = json.NewEncoder(w).Encode(api.Deployment{
				ID:          depID,
				ServiceID:   serviceID,
				Status:      status,
				ImageTag:    &img,
				TriggeredBy: "redeploy",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	resetFlags(servicesRestartCmd.Flags())
	output.Format = "json"
	if err := servicesRestartCmd.Flags().Set("wait", "true"); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return servicesRestartCmd.RunE(servicesRestartCmd, []string{"api-worker"})
	})
	if err != nil {
		t.Fatalf("servicesRestartCmd error: %v", err)
	}
	if !restarted {
		t.Fatal("expected POST /api/services/{id}/restart to be called")
	}

	var got api.Deployment
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout=%q", err, out)
	}
	if got.ID != depID || got.Status != "active" {
		t.Errorf("got deployment %+v, want id=%s status=active", got, depID)
	}
}

func TestServicesUpdateAutoDeploy(t *testing.T) {
	const serviceID = "44444444-4444-4444-4444-444444444444"

	for _, want := range []bool{false, true} {
		var body map[string]any
		setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch || r.URL.Path != "/api/services/"+serviceID {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(api.Service{
				ID:         serviceID,
				Name:       "web",
				AutoDeploy: want,
			})
		}))

		resetFlags(servicesUpdateCmd.Flags())
		output.Format = "json"
		valStr := "false"
		if want {
			valStr = "true"
		}
		if err := servicesUpdateCmd.Flags().Set("auto-deploy", valStr); err != nil {
			t.Fatal(err)
		}

		out, err := captureStdout(t, func() error {
			return servicesUpdateCmd.RunE(servicesUpdateCmd, []string{serviceID})
		})
		if err != nil {
			t.Fatalf("servicesUpdateCmd(auto-deploy=%v) error: %v", want, err)
		}
		gotVal, exists := body["auto_deploy"]
		if !exists || gotVal != want {
			t.Errorf("PATCH body auto_deploy = %v (exists=%v), want %v", gotVal, exists, want)
		}
		var svc api.Service
		if err := json.Unmarshal([]byte(out), &svc); err != nil {
			t.Fatalf("invalid JSON output: %v", err)
		}
		if svc.AutoDeploy != want {
			t.Errorf("output AutoDeploy = %v, want %v", svc.AutoDeploy, want)
		}
	}
}

func TestEnvSetProjectBatch(t *testing.T) {
	const projectID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	const svcA = "55555555-5555-5555-5555-555555555555"
	const svcB = "66666666-6666-6666-6666-666666666666"

	created := map[string]map[string]string{}
	updated := map[string]string{}

	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_ = json.NewEncoder(w).Encode([]api.Project{
				{ID: projectID, Name: "Markets", Slug: "markets"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+projectID+"/services":
			_ = json.NewEncoder(w).Encode([]api.Service{
				{ID: svcA, ProjectID: projectID, Name: "svc-a"},
				{ID: svcB, ProjectID: projectID, Name: "svc-b"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+svcA+"/env-vars":
			_ = json.NewEncoder(w).Encode([]api.EnvVar{
				{ID: "var-a-1", Key: "LOG_LEVEL", Value: "info"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+svcB+"/env-vars":
			_ = json.NewEncoder(w).Encode([]api.EnvVar{})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/services/env-vars/var-a-1":
			var req api.EnvVarUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Value != nil {
				updated["var-a-1"] = *req.Value
			}
			_ = json.NewEncoder(w).Encode(api.EnvVar{ID: "var-a-1", Key: "LOG_LEVEL", Value: "debug"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/env-vars"):
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
			sid := parts[0]
			var req api.EnvVarCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if created[sid] == nil {
				created[sid] = map[string]string{}
			}
			created[sid][req.Key] = req.Value
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(api.EnvVar{ID: "new-" + req.Key, Key: req.Key, Value: req.Value})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	resetFlags(envSetCmd.Flags())
	output.Format = "json"
	if err := envSetCmd.Flags().Set("project", "markets"); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return envSetCmd.RunE(envSetCmd, []string{"LOG_LEVEL=debug", "REGION=us"})
	})
	if err != nil {
		t.Fatalf("envSetCmd --project error: %v", err)
	}

	if updated["var-a-1"] != "debug" {
		t.Errorf("expected svcA LOG_LEVEL to be updated to debug, got %q", updated["var-a-1"])
	}
	if created[svcA]["REGION"] != "us" {
		t.Errorf("expected svcA REGION=us created, got %+v", created[svcA])
	}
	if created[svcB]["LOG_LEVEL"] != "debug" || created[svcB]["REGION"] != "us" {
		t.Errorf("expected svcB LOG_LEVEL=debug and REGION=us created, got %+v", created[svcB])
	}

	var summary struct {
		ProjectID       string                `json:"project_id"`
		ServicesUpdated int                   `json:"services_updated"`
		ServicesTotal   int                   `json:"services_total"`
		Created         int                   `json:"created"`
		Updated         int                   `json:"updated"`
		Results         []serviceEnvSetResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout=%q", err, out)
	}
	if summary.ProjectID != projectID || summary.ServicesUpdated != 2 || summary.ServicesTotal != 2 {
		t.Errorf("unexpected summary: %+v", summary)
	}
	if summary.Created != 3 || summary.Updated != 1 {
		t.Errorf("expected created=3 updated=1, got %+v", summary)
	}
}

func TestConsistentJSONOutput_DeploymentsInfoAndEnvList(t *testing.T) {
	const depID = "77777777-7777-7777-7777-777777777777"
	const svcID = "88888888-8888-8888-8888-888888888888"

	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/deployments/"+depID:
			img := "registry.clank.host/acme/app:v2"
			_ = json.NewEncoder(w).Encode(api.Deployment{
				ID:          depID,
				ServiceID:   svcID,
				Status:      "active",
				ImageTag:    &img,
				TriggeredBy: "manual",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+svcID+"/env-vars":
			_ = json.NewEncoder(w).Encode([]api.EnvVar{
				{ID: "ev-1", Key: "PORT", Value: "8080", IsSecret: false},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	output.Format = "json"
	depOut, err := captureStdout(t, func() error {
		return deploymentsInfoCmd.RunE(deploymentsInfoCmd, []string{depID})
	})
	if err != nil {
		t.Fatalf("deploymentsInfoCmd error: %v", err)
	}
	var dep api.Deployment
	if err := json.Unmarshal([]byte(depOut), &dep); err != nil {
		t.Fatalf("deployments info invalid JSON: %v\n%q", err, depOut)
	}
	if dep.ID != depID || dep.Status != "active" {
		t.Errorf("unexpected deployment: %+v", dep)
	}

	envOut, err := captureStdout(t, func() error {
		return envListCmd.RunE(envListCmd, []string{svcID})
	})
	if err != nil {
		t.Fatalf("envListCmd error: %v", err)
	}
	var vars []api.EnvVar
	if err := json.Unmarshal([]byte(envOut), &vars); err != nil {
		t.Fatalf("env list invalid JSON: %v\n%q", err, envOut)
	}
	if len(vars) != 1 || vars[0].Key != "PORT" || vars[0].Value != "8080" {
		t.Errorf("unexpected env vars: %+v", vars)
	}
}
