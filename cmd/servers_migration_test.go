package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
)

func TestServersAddJSONOutput(t *testing.T) {
	const fullServerID = "11111111-2222-3333-4444-555555555555"

	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/servers/" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req api.CreateServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if req.Name != "my-host" {
			t.Errorf("name = %q, want %q", req.Name, "my-host")
		}
		_ = json.NewEncoder(w).Encode(api.EnrollmentToken{
			ServerID:            fullServerID,
			EnrollmentToken:     "tok_secret_123",
			ExpiresAt:           "2026-12-01T00:00:00Z",
			InstallCommand:      "curl -fsSL https://clank.host/install.sh | sh -s -- tok_secret_123",
			ManualEnrollCommand: "clank-agent enroll --token tok_secret_123",
		})
	}))

	output.Format = "json"
	out, err := captureStdout(t, func() error {
		return serversAddCmd.RunE(serversAddCmd, []string{"my-host"})
	})
	if err != nil {
		t.Fatalf("serversAddCmd error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout=%q", err, out)
	}

	checks := map[string]string{
		"id":                    fullServerID,
		"server_id":             fullServerID,
		"name":                  "my-host",
		"enrollment_token":      "tok_secret_123",
		"install_command":       "curl -fsSL https://clank.host/install.sh | sh -s -- tok_secret_123",
		"manual_enroll_command": "clank-agent enroll --token tok_secret_123",
		"expires_at":            "2026-12-01T00:00:00Z",
	}
	for k, want := range checks {
		if got[k] != want {
			t.Errorf("json[%q] = %v, want %q", k, got[k], want)
		}
	}
}

func TestServersRemoveResolvesShortPrefix(t *testing.T) {
	const fullServerID = "abcdef12-3456-7890-abcd-ef1234567890"
	var deletedPath string

	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/servers/":
			_ = json.NewEncoder(w).Encode([]api.Server{
				{ID: fullServerID, Name: "prod-1", Slug: "prod-1", Status: "online"},
				{ID: "99999999-1111-2222-3333-444444444444", Name: "prod-2", Slug: "prod-2", Status: "online"},
			})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/servers/"):
			deletedPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	_, err := captureStdout(t, func() error {
		return serversRemoveCmd.RunE(serversRemoveCmd, []string{"abcdef12"})
	})
	if err != nil {
		t.Fatalf("serversRemoveCmd error: %v", err)
	}
	wantPath := "/api/servers/" + fullServerID
	if deletedPath != wantPath {
		t.Errorf("DELETE path = %q, want %q", deletedPath, wantPath)
	}
}

func TestResolveServerIDAmbiguityAndLookup(t *testing.T) {
	const srvA = "deadbeef-1111-1111-1111-111111111111"
	const srvB = "deadbeef-2222-2222-2222-222222222222"

	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/servers/" {
			_ = json.NewEncoder(w).Encode([]api.Server{
				{ID: srvA, Name: "eu-west", Slug: "eu-west"},
				{ID: srvB, Name: "us-east", Slug: "us-east"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	client := newClient()

	// Ambiguous prefix should error clearly.
	_, err := resolveServerID(client, "deadbeef")
	if err == nil {
		t.Fatal("expected ambiguity error for shared prefix 'deadbeef', got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Errorf("expected error mentioning ambiguous, got: %v", err)
	}

	// Case-insensitive name/slug match.
	id, err := resolveServerID(client, "EU-WEST")
	if err != nil {
		t.Fatalf("resolveServerID by name failed: %v", err)
	}
	if id != srvA {
		t.Errorf("got %q, want %q", id, srvA)
	}

	// Not found.
	_, err = resolveServerID(client, "nonexistent")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestServicesListAllAndServerDirectEndpoint(t *testing.T) {
	const srvA = "aaaa1111-1111-1111-1111-111111111111"
	const srvB = "bbbb2222-2222-2222-2222-222222222222"
	const svc1 = "10000000-0000-0000-0000-000000000001"
	const svc2 = "20000000-0000-0000-0000-000000000002"

	var seenQueries []string
	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/servers/":
			_ = json.NewEncoder(w).Encode([]api.Server{
				{ID: srvA, Name: "host-a", Slug: "host-a"},
				{ID: srvB, Name: "host-b", Slug: "host-b"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services":
			q := r.URL.Query().Get("server_id")
			seenQueries = append(seenQueries, q)
			all := []api.Service{
				{ID: svc1, Name: "web", ServerID: strPtr(srvA), Port: 3000, CreatedAt: "2026-11-28T00:00:00Z"},
				{ID: svc2, Name: "worker", ServerID: strPtr(srvB), Port: 8080, CreatedAt: "2026-11-28T00:00:00Z"},
			}
			if q == "" {
				_ = json.NewEncoder(w).Encode(all)
				return
			}
			var filtered []api.Service
			for _, s := range all {
				if s.ServerID != nil && *s.ServerID == q {
					filtered = append(filtered, s)
				}
			}
			_ = json.NewEncoder(w).Encode(filtered)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// 1. --all in table mode should include SERVER column and both services.
	resetFlags(servicesListCmd.Flags())
	output.Format = ""
	if err := servicesListCmd.Flags().Set("all", "true"); err != nil {
		t.Fatal(err)
	}
	tableOut, err := captureStdout(t, func() error {
		return servicesListCmd.RunE(servicesListCmd, nil)
	})
	if err != nil {
		t.Fatalf("services list --all error: %v", err)
	}
	if !strings.Contains(tableOut, "SERVER") || !strings.Contains(tableOut, "aaaa1111") || !strings.Contains(tableOut, "bbbb2222") {
		t.Errorf("expected SERVER column with short server IDs in table output, got:\n%s", tableOut)
	}

	// 2. --server <8-char-prefix> with -o json should resolve UUID and filter via ?server_id=...
	resetFlags(servicesListCmd.Flags())
	output.Format = "json"
	if err := servicesListCmd.Flags().Set("server", "aaaa1111"); err != nil {
		t.Fatal(err)
	}
	jsonOut, err := captureStdout(t, func() error {
		return servicesListCmd.RunE(servicesListCmd, nil)
	})
	if err != nil {
		t.Fatalf("services list --server error: %v", err)
	}

	var got []api.Service
	if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout=%q", err, jsonOut)
	}
	if len(got) != 1 || got[0].ID != svc1 {
		t.Errorf("got %+v, want only %s", got, svc1)
	}
	if len(seenQueries) != 2 || seenQueries[0] != "" || seenQueries[1] != srvA {
		t.Errorf("seen server_id queries = %v, want [\"\" %q]", seenQueries, srvA)
	}
}

func TestServicesListFallbackOn404(t *testing.T) {
	const proj1 = "p1111111-1111-1111-1111-111111111111"
	const proj2 = "p2222222-2222-2222-2222-222222222222"
	const srvA = "cccc3333-3333-3333-3333-333333333333"
	const srvB = "dddd4444-4444-4444-4444-444444444444"
	const svc1 = "30000000-0000-0000-0000-000000000001"
	const svc2 = "40000000-0000-0000-0000-000000000002"

	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/services":
			// Simulate older control plane without GET /api/services.
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"Not Found"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/servers/":
			_ = json.NewEncoder(w).Encode([]api.Server{
				{ID: srvA, Name: "old-host", Slug: "old-host"},
				{ID: srvB, Name: "new-host", Slug: "new-host"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_ = json.NewEncoder(w).Encode([]api.Project{
				{ID: proj1, Name: "Alpha", Slug: "alpha"},
				{ID: proj2, Name: "Beta", Slug: "beta"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+proj1+"/services":
			_ = json.NewEncoder(w).Encode([]api.Service{
				{ID: svc1, ProjectID: proj1, Name: "api", ServerID: strPtr(srvA)},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+proj2+"/services":
			_ = json.NewEncoder(w).Encode([]api.Service{
				{ID: svc2, ProjectID: proj2, Name: "web", ServerID: strPtr(srvB)},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// --all via fallback should return both services.
	resetFlags(servicesListCmd.Flags())
	output.Format = "json"
	if err := servicesListCmd.Flags().Set("all", "true"); err != nil {
		t.Fatal(err)
	}
	allOut, err := captureStdout(t, func() error {
		return servicesListCmd.RunE(servicesListCmd, nil)
	})
	if err != nil {
		t.Fatalf("fallback --all error: %v", err)
	}
	var allSvcs []api.Service
	if err := json.Unmarshal([]byte(allOut), &allSvcs); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(allSvcs) != 2 {
		t.Errorf("fallback --all returned %d services, want 2", len(allSvcs))
	}

	// --server <8-char-prefix> via fallback should filter client-side.
	resetFlags(servicesListCmd.Flags())
	output.Format = "json"
	if err := servicesListCmd.Flags().Set("server", "dddd4444"); err != nil {
		t.Fatal(err)
	}
	srvOut, err := captureStdout(t, func() error {
		return servicesListCmd.RunE(servicesListCmd, nil)
	})
	if err != nil {
		t.Fatalf("fallback --server error: %v", err)
	}
	var srvSvcs []api.Service
	if err := json.Unmarshal([]byte(srvOut), &srvSvcs); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(srvSvcs) != 1 || srvSvcs[0].ID != svc2 {
		t.Errorf("fallback --server returned %+v, want only %s", srvSvcs, svc2)
	}
}

func TestEndpointsListResolvesServiceAndHintsLegacyDomains(t *testing.T) {
	const projectID = "eeeeeeee-1111-1111-1111-111111111111"
	const serviceID = "ffffffff-2222-2222-2222-222222222222"

	setupTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_ = json.NewEncoder(w).Encode([]api.Project{
				{ID: projectID, Name: "Core", Slug: "core"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+projectID+"/services":
			_ = json.NewEncoder(w).Encode([]api.Service{
				{ID: serviceID, ProjectID: projectID, Name: "frontend", Slug: "frontend"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+serviceID+"/endpoints":
			_ = json.NewEncoder(w).Encode([]api.Endpoint{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+serviceID+"/domains":
			_ = json.NewEncoder(w).Encode([]api.Domain{
				{ID: "dom-1", ServiceID: serviceID, Domain: "app.example.com", Status: "active"},
				{ID: "dom-2", ServiceID: serviceID, Domain: "www.example.com", Status: "active"},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Table mode: resolves service name "frontend" and prints legacy domain hint.
	output.Format = ""
	out, err := captureStdout(t, func() error {
		return endpointsListCmd.RunE(endpointsListCmd, []string{"frontend"})
	})
	if err != nil {
		t.Fatalf("endpointsListCmd error: %v", err)
	}
	if !strings.Contains(out, "found 2 legacy domain(s)") || !strings.Contains(out, "clank domains list "+serviceID) {
		t.Errorf("expected legacy domain hint, got: %q", out)
	}

	// JSON mode: returns valid empty JSON array without plain text hint.
	output.Format = "json"
	jsonOut, err := captureStdout(t, func() error {
		return endpointsListCmd.RunE(endpointsListCmd, []string{"ffffffff"})
	})
	if err != nil {
		t.Fatalf("endpointsListCmd json error: %v", err)
	}
	var eps []api.Endpoint
	if err := json.Unmarshal([]byte(jsonOut), &eps); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout=%q", err, jsonOut)
	}
	if len(eps) != 0 {
		t.Errorf("expected empty endpoints array, got %+v", eps)
	}
}
