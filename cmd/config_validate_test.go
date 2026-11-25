package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clankhost/clank-cli/internal/output"
)

func captureValidateStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := fn()

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), runErr
}

func TestConfigValidate_ValidFullConfig_HumanAndJSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "clank.toml")
	content := `
[service]
port = 3000
health_check_path = "/healthz"
dockerfile_path = "deploy/Dockerfile"
pre_deploy_command = "alembic upgrade head"

[resources]
memory_mb = 1024
cpu = 1.5

[[cron]]
name = "cleanup"
schedule = "0 3 * * *"
command = "python cleanup.py"

[[cron]]
name = "hourly-sync"
schedule = "@hourly"
command = "python sync.py"
enabled = false

[env]
required = ["DATABASE_URL", "REDIS_URL"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write clank.toml: %v", err)
	}

	// Human output
	output.Format = ""
	out, err := captureValidateStdout(t, func() error {
		return runConfigValidate(cfgPath)
	})
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
	for _, want := range []string{"Valid clank.toml", "3000", "/healthz", "1024", "cleanup", "DATABASE_URL"} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q:\n%s", want, out)
		}
	}

	// JSON output
	output.Format = "json"
	t.Cleanup(func() { output.Format = "" })
	jsonOut, err := captureValidateStdout(t, func() error {
		return runConfigValidate(cfgPath)
	})
	if err != nil {
		t.Fatalf("expected valid config in json mode, got error: %v", err)
	}
	var parsed configValidateOutput
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, jsonOut)
	}
	if !parsed.Valid || parsed.Config == nil {
		t.Fatalf("expected valid=true with config, got %+v", parsed)
	}
	if parsed.Config.Service.Port == nil || *parsed.Config.Service.Port != 3000 {
		t.Errorf("unexpected port: %+v", parsed.Config.Service.Port)
	}
	if parsed.Config.Resources.MemoryMB == nil || *parsed.Config.Resources.MemoryMB != 1024 {
		t.Errorf("unexpected memory_mb: %+v", parsed.Config.Resources.MemoryMB)
	}
	if len(parsed.Config.Cron) != 2 {
		t.Errorf("expected 2 cron jobs, got %d", len(parsed.Config.Cron))
	}
	if len(parsed.Config.Env.Required) != 2 {
		t.Errorf("expected 2 required env vars, got %d", len(parsed.Config.Env.Required))
	}
}

func TestConfigValidate_InvalidConfigs(t *testing.T) {
	cases := []struct {
		name      string
		toml      string
		wantMatch string
	}{
		{"malformed toml", "[service\nport = 8080", "malformed clank.toml"},
		{"invalid port", "[service]\nport = 70000", "port"},
		{"invalid health path", `[service]` + "\n" + `health_check_path = "health"`, "health_check_path"},
		{"traversal dockerfile", `[service]` + "\n" + `dockerfile_path = "../Dockerfile"`, "dockerfile_path"},
		{"memory too low", "[resources]\nmemory_mb = 32", "memory_mb"},
		{"cpu too high", "[resources]\ncpu = 128.0", "cpu"},
		{"bad cron schedule", "[[cron]]\nname = \"j\"\nschedule = \"bad\"\ncommand = \"echo 1\"", "schedule"},
		{"duplicate cron name", "[[cron]]\nname = \"dup\"\nschedule = \"@daily\"\ncommand = \"a\"\n[[cron]]\nname = \"dup\"\nschedule = \"@hourly\"\ncommand = \"b\"", "duplicate"},
		{"invalid env var", `[env]` + "\n" + `required = ["lower_case"]`, "required[0]"},
	}

	output.Format = ""
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "clank.toml")
			if err := os.WriteFile(cfgPath, []byte(tc.toml), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			err := runConfigValidate(cfgPath)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantMatch) {
				t.Errorf("error %q did not contain %q", err.Error(), tc.wantMatch)
			}
		})
	}
}

func TestConfigValidate_InvalidJSONOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "clank.toml")
	if err := os.WriteFile(cfgPath, []byte("[resources]\nmemory_mb = 16\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	output.Format = "json"
	t.Cleanup(func() { output.Format = "" })
	out, err := captureValidateStdout(t, func() error {
		return runConfigValidate(cfgPath)
	})
	if err == nil {
		t.Fatal("expected error for invalid config in json mode")
	}
	var parsed configValidateOutput
	if jerr := json.Unmarshal([]byte(out), &parsed); jerr != nil {
		t.Fatalf("invalid json output: %v\n%s", jerr, out)
	}
	if parsed.Valid || len(parsed.Errors) == 0 {
		t.Errorf("expected valid=false with errors, got %+v", parsed)
	}
}

func TestConfigValidate_RegisteredUnderConfig(t *testing.T) {
	found := false
	for _, sub := range configCmd.Commands() {
		if sub.Name() == "validate" {
			found = true
			if needsBaseURL(sub) {
				t.Error("config validate should not require base URL")
			}
		}
	}
	if !found {
		t.Fatal("validate subcommand not registered under configCmd")
	}
}
