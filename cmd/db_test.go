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

func setupDBTestServer(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	prevCfg := cfg
	prevFormat := output.Format
	prevInput := dbShellInput

	cfg = &config.Config{BaseURL: srv.URL, Token: "clank_testtoken"}
	output.Format = ""
	dbShellInput = nil

	t.Cleanup(func() {
		cfg = prevCfg
		output.Format = prevFormat
		dbShellInput = prevInput
	})
}

func strPtr(s string) *string { return &s }

func TestDetectDBEngine(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		want    dbEngine
		wantErr bool
	}{
		{name: "postgres standard", image: "postgres:16-alpine", want: dbEnginePostgres},
		{name: "postgres with registry", image: "docker.io/library/postgres:15", want: dbEnginePostgres},
		{name: "postgres with registry port", image: "localhost:5000/postgres:16", want: dbEnginePostgres},
		{name: "postgres with digest", image: "postgres@sha256:deadbeef", want: dbEnginePostgres},
		{name: "pgvector", image: "pgvector/pgvector:pg16", want: dbEnginePostgres},
		{name: "postgis", image: "postgis/postgis:16-3.4", want: dbEnginePostgres},
		{name: "mysql standard", image: "mysql:8.0", want: dbEngineMySQL},
		{name: "mariadb standard", image: "mariadb:11.4", want: dbEngineMySQL},
		{name: "bitnami mysql", image: "docker.io/bitnami/mysql:8", want: dbEngineMySQL},
		{name: "mongo standard", image: "mongo:7", want: dbEngineMongo},
		{name: "mongodb community", image: "mongodb/mongodb-community-server:7.0-ubuntu2204", want: dbEngineMongo},
		{name: "redis standard", image: "redis:7-alpine", want: dbEngineRedis},
		{name: "redis stack", image: "redis/redis-stack:latest", want: dbEngineRedis},
		{name: "valkey", image: "valkey/valkey:8-alpine", want: dbEngineRedis},
		{name: "unsupported nginx", image: "nginx:latest", wantErr: true},
		{name: "unsupported node", image: "node:20-alpine", wantErr: true},
		{name: "empty image", image: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := detectDBEngine(tc.image)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("detectDBEngine(%q) = %q, nil; want error", tc.image, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("detectDBEngine(%q) unexpected error: %v", tc.image, err)
			}
			if got != tc.want {
				t.Errorf("detectDBEngine(%q) = %q, want %q", tc.image, got, tc.want)
			}
		})
	}
}

func TestNormalizeDBEngine(t *testing.T) {
	cases := map[string]dbEngine{
		"postgres":   dbEnginePostgres,
		"PostgreSQL": dbEnginePostgres,
		"psql":       dbEnginePostgres,
		"pg":         dbEnginePostgres,
		"mysql":      dbEngineMySQL,
		"mariadb":    dbEngineMySQL,
		"mongo":      dbEngineMongo,
		"mongodb":    dbEngineMongo,
		"mongosh":    dbEngineMongo,
		"redis":      dbEngineRedis,
		"redis-cli":  dbEngineRedis,
		"valkey":     dbEngineRedis,
	}
	for input, want := range cases {
		got, err := normalizeDBEngine(input)
		if err != nil {
			t.Errorf("normalizeDBEngine(%q) unexpected error: %v", input, err)
		}
		if got != want {
			t.Errorf("normalizeDBEngine(%q) = %q, want %q", input, got, want)
		}
	}

	if _, err := normalizeDBEngine("sqlite"); err == nil {
		t.Fatal("expected error for unsupported engine 'sqlite'")
	}
}

func TestBuildDBQueryCommandSafePositionalArgs(t *testing.T) {
	// Malicious query containing quotes, command substitution, backticks, and shell metacharacters.
	trickyQuery := `SELECT 'it''s'; $(rm -rf /); ` + "`whoami`" + `; "quoted" | cat`

	engines := []struct {
		engine     dbEngine
		expectTool string
	}{
		{dbEnginePostgres, "psql"},
		{dbEngineMySQL, "mysql"},
		{dbEngineMongo, "mongosh"},
		{dbEngineRedis, "redis-cli"},
	}

	for _, tc := range engines {
		t.Run(string(tc.engine), func(t *testing.T) {
			argv, err := buildDBQueryCommand(tc.engine, trickyQuery)
			if err != nil {
				t.Fatalf("buildDBQueryCommand(%q) error: %v", tc.engine, err)
			}
			if len(argv) != 5 {
				t.Fatalf("len(argv) = %d, want 5 (%#v)", len(argv), argv)
			}
			if argv[0] != "sh" || argv[1] != "-c" || argv[3] != "--" {
				t.Errorf("unexpected argv wrapper: %#v", argv[:4])
			}
			if !strings.Contains(argv[2], tc.expectTool) {
				t.Errorf("script %q does not invoke %q", argv[2], tc.expectTool)
			}
			if !strings.Contains(argv[2], `"$1"`) {
				t.Errorf("script %q must reference positional arg \"$1\"", argv[2])
			}
			if strings.Contains(argv[2], trickyQuery) || strings.Contains(argv[2], "rm -rf") {
				t.Errorf("query was interpolated into shell script: %q", argv[2])
			}
			if argv[4] != trickyQuery {
				t.Errorf("argv[4] = %q, want exact query %q", argv[4], trickyQuery)
			}
		})
	}
}

func TestDBShellOneShotByServiceName(t *testing.T) {
	const projectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const serviceID = "11111111-1111-1111-1111-111111111111"

	var received map[string]any
	setupDBTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_ = json.NewEncoder(w).Encode([]api.Project{
				{ID: projectID, Name: "Core", Slug: "core"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+projectID+"/services":
			_ = json.NewEncoder(w).Encode([]api.Service{
				{ID: serviceID, ProjectID: projectID, Name: "main-db", Slug: "main-db", Image: strPtr("postgres:16-alpine")},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+serviceID:
			_ = json.NewEncoder(w).Encode(api.Service{
				ID: serviceID, ProjectID: projectID, Name: "main-db", Slug: "main-db", Image: strPtr("postgres:16-alpine"),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/"+serviceID+"/exec":
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(api.ExecResponse{
				ExitCode:   0,
				Stdout:     " ?column? \n----------\n        1\n(1 row)\n",
				Stderr:     "",
				Truncated:  false,
				DurationMS: 18,
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	resetFlags(dbShellCmd.Flags())
	if err := dbShellCmd.Flags().Set("command", "SELECT 1;"); err != nil {
		t.Fatal(err)
	}
	if err := dbShellCmd.Flags().Set("timeout", "45"); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := captureStdIO(t, func() error {
		return runDBShell(dbShellCmd, []string{"main-db"})
	})
	if err != nil {
		t.Fatalf("runDBShell error: %v", err)
	}
	if !strings.Contains(stdout, "(1 row)") {
		t.Errorf("stdout = %q, want query output", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if received["timeout_seconds"] != float64(45) {
		t.Errorf("timeout_seconds = %v, want 45", received["timeout_seconds"])
	}
	cmdSlice, ok := received["command"].([]any)
	if !ok || len(cmdSlice) != 5 || cmdSlice[0] != "sh" || cmdSlice[1] != "-c" || cmdSlice[3] != "--" || cmdSlice[4] != "SELECT 1;" {
		t.Errorf("unexpected command payload: %#v", received["command"])
	}
	if script, _ := cmdSlice[2].(string); !strings.Contains(script, "psql") {
		t.Errorf("expected psql command in script, got: %q", script)
	}
}

func TestDBShellOneShotJSONOutput(t *testing.T) {
	const serviceID = "22222222-2222-2222-2222-222222222222"

	setupDBTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+serviceID:
			_ = json.NewEncoder(w).Encode(api.Service{
				ID: serviceID, Name: "cache", Slug: "cache", Image: strPtr("redis:7-alpine"),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/"+serviceID+"/exec":
			_ = json.NewEncoder(w).Encode(api.ExecResponse{
				ExitCode:   0,
				Stdout:     "PONG\n",
				Stderr:     "",
				Truncated:  false,
				DurationMS: 9,
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	resetFlags(dbShellCmd.Flags())
	if err := dbShellCmd.Flags().Set("command", "PING"); err != nil {
		t.Fatal(err)
	}
	output.Format = "json"

	stdout, _, err := captureStdIO(t, func() error {
		return runDBShell(dbShellCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runDBShell error: %v", err)
	}

	var got struct {
		Engine     string `json:"engine"`
		ServiceID  string `json:"service_id"`
		ExitCode   int    `json:"exit_code"`
		Stdout     string `json:"stdout"`
		DurationMS int    `json:"duration_ms"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout=%q", err, stdout)
	}
	if got.Engine != "redis" || got.ServiceID != serviceID || got.ExitCode != 0 || got.Stdout != "PONG\n" || got.DurationMS != 9 {
		t.Errorf("unexpected JSON output: %+v", got)
	}
}

func TestDBShellUnsupportedImageError(t *testing.T) {
	const serviceID = "33333333-3333-3333-3333-333333333333"

	execCalled := false
	setupDBTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+serviceID:
			_ = json.NewEncoder(w).Encode(api.Service{
				ID: serviceID, Name: "web", Slug: "web", Image: strPtr("nginx:1.27-alpine"),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/"+serviceID+"/exec":
			execCalled = true
			_ = json.NewEncoder(w).Encode(api.ExecResponse{ExitCode: 0, Stdout: "ok\n"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	resetFlags(dbShellCmd.Flags())
	if err := dbShellCmd.Flags().Set("command", "SELECT 1;"); err != nil {
		t.Fatal(err)
	}

	_, _, err := captureStdIO(t, func() error {
		return runDBShell(dbShellCmd, []string{serviceID})
	})
	if err == nil {
		t.Fatal("expected error for unsupported image")
	}
	if !strings.Contains(err.Error(), "unsupported database image") {
		t.Errorf("error = %q, want unsupported database image message", err.Error())
	}
	if execCalled {
		t.Error("exec endpoint should not be called when image is unsupported")
	}

	// Verify --engine override bypasses image auto-detection.
	resetFlags(dbShellCmd.Flags())
	if err := dbShellCmd.Flags().Set("command", "SHOW TABLES;"); err != nil {
		t.Fatal(err)
	}
	if err := dbShellCmd.Flags().Set("engine", "mysql"); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := captureStdIO(t, func() error {
		return runDBShell(dbShellCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runDBShell with --engine override failed: %v", err)
	}
	if !execCalled || stdout != "ok\n" {
		t.Errorf("expected exec to run with override; execCalled=%v stdout=%q", execCalled, stdout)
	}
}

func TestDBShellInteractiveREPL(t *testing.T) {
	const serviceID = "44444444-4444-4444-4444-444444444444"

	var queries []string
	setupDBTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+serviceID:
			_ = json.NewEncoder(w).Encode(api.Service{
				ID: serviceID, Name: "cache-svc", Slug: "cache-svc", Image: strPtr("redis:7"),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/"+serviceID+"/exec":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if argv, ok := body["command"].([]any); ok && len(argv) == 5 {
				if q, ok := argv[4].(string); ok {
					queries = append(queries, q)
				}
			}
			_ = json.NewEncoder(w).Encode(api.ExecResponse{
				ExitCode:   0,
				Stdout:     "OK\n",
				DurationMS: 4,
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	resetFlags(dbShellCmd.Flags())
	dbShellInput = strings.NewReader("PING\n\nSET a 1\n\\q\nIGNORED\n")

	stdout, _, err := captureStdIO(t, func() error {
		return runDBShell(dbShellCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runDBShell REPL error: %v", err)
	}
	if !strings.Contains(stdout, "redis("+serviceID+")> ") {
		t.Errorf("missing REPL prompt in stdout: %q", stdout)
	}
	if len(queries) != 2 || queries[0] != "PING" || queries[1] != "SET a 1" {
		t.Errorf("queries sent = %#v, want [PING SET a 1]", queries)
	}
}
