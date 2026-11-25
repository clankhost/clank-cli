package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/config"
	"github.com/clankhost/clank-cli/internal/output"
)

// captureStdIO runs fn while capturing both os.Stdout and os.Stderr.
func captureStdIO(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()

	oldOut := os.Stdout
	oldErr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stderr: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	runErr := fn()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr

	var outBuf, errBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, rOut)
	_, _ = io.Copy(&errBuf, rErr)
	_ = rOut.Close()
	_ = rErr.Close()

	return outBuf.String(), errBuf.String(), runErr
}

func setupExecTestServer(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	prevCfg := cfg
	prevFormat := output.Format
	prevInput := execInput

	cfg = &config.Config{BaseURL: srv.URL, Token: "clank_testtoken"}
	output.Format = ""
	execInput = nil

	t.Cleanup(func() {
		cfg = prevCfg
		output.Format = prevFormat
		execInput = prevInput
	})
}

func TestExecOneShotByServiceName(t *testing.T) {
	const projectID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const serviceID = "11111111-1111-1111-1111-111111111111"

	var received map[string]any
	setupExecTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
			_ = json.NewEncoder(w).Encode([]api.Project{
				{ID: projectID, Name: "Core", Slug: "core"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/services/"+projectID+"/services":
			_ = json.NewEncoder(w).Encode([]api.Service{
				{ID: serviceID, ProjectID: projectID, Name: "api-svc", Slug: "api-svc"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/"+serviceID+"/exec":
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Errorf("decode body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(api.ExecResponse{
				ExitCode:   0,
				Stdout:     "hello stdout\n",
				Stderr:     "hello stderr\n",
				Truncated:  false,
				DurationMS: 42,
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	resetFlags(execCmd.Flags())
	if err := execCmd.Flags().Set("timeout", "45"); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := captureStdIO(t, func() error {
		return runExec(execCmd, []string{"api-svc", "echo", "hi"})
	})
	if err != nil {
		t.Fatalf("runExec error: %v", err)
	}

	if stdout != "hello stdout\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hello stdout\n")
	}
	if stderr != "hello stderr\n" {
		t.Errorf("stderr = %q, want %q", stderr, "hello stderr\n")
	}
	if received["timeout_seconds"] != float64(45) {
		t.Errorf("timeout_seconds = %v, want 45", received["timeout_seconds"])
	}
	cmdSlice, ok := received["command"].([]any)
	if !ok || len(cmdSlice) != 2 || cmdSlice[0] != "echo" || cmdSlice[1] != "hi" {
		t.Errorf("command = %#v, want [echo hi]", received["command"])
	}
}

func TestExecOneShotJSONOutput(t *testing.T) {
	const serviceID = "22222222-2222-2222-2222-222222222222"

	setupExecTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/services/"+serviceID+"/exec" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(api.ExecResponse{
			ExitCode:   0,
			Stdout:     "ok\n",
			Stderr:     "",
			Truncated:  false,
			DurationMS: 12,
		})
	}))

	resetFlags(execCmd.Flags())
	output.Format = "json"

	stdout, _, err := captureStdIO(t, func() error {
		return runExec(execCmd, []string{serviceID, "true"})
	})
	if err != nil {
		t.Fatalf("runExec error: %v", err)
	}

	var got api.ExecResponse
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout=%q", err, stdout)
	}
	if got.ExitCode != 0 || got.Stdout != "ok\n" || got.DurationMS != 12 {
		t.Errorf("unexpected ExecResponse: %+v", got)
	}
}

func TestExecOneShotNonZeroExitCodeReturnsError(t *testing.T) {
	const serviceID = "33333333-3333-3333-3333-333333333333"

	setupExecTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(api.ExecResponse{
			ExitCode:   7,
			Stdout:     "",
			Stderr:     "boom\n",
			Truncated:  false,
			DurationMS: 5,
		})
	}))

	resetFlags(execCmd.Flags())

	_, stderr, err := captureStdIO(t, func() error {
		return runExec(execCmd, []string{serviceID, "false"})
	})
	if err == nil {
		t.Fatal("expected non-nil error for non-zero exit_code")
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("error = %q, want exit code 7 mentioned", err.Error())
	}
	if stderr != "boom\n" {
		t.Errorf("stderr = %q, want %q", stderr, "boom\n")
	}
}

func TestExecInteractiveREPLMode(t *testing.T) {
	const serviceID = "44444444-4444-4444-4444-444444444444"

	var commands []any
	setupExecTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/services/"+serviceID+"/exec" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		commands = append(commands, body["command"])

		switch body["command"] {
		case "pwd":
			_ = json.NewEncoder(w).Encode(api.ExecResponse{
				ExitCode:   0,
				Stdout:     "/app\n",
				DurationMS: 3,
			})
		case "failcmd":
			_ = json.NewEncoder(w).Encode(api.ExecResponse{
				ExitCode:   1,
				Stderr:     "not found\n",
				DurationMS: 2,
			})
		default:
			_ = json.NewEncoder(w).Encode(api.ExecResponse{ExitCode: 0})
		}
	}))

	resetFlags(execCmd.Flags())
	execInput = strings.NewReader("pwd\n\nfailcmd\nexit\n")

	stdout, stderr, err := captureStdIO(t, func() error {
		return runExec(execCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runExec REPL error: %v", err)
	}

	if !strings.Contains(stdout, "Connected to "+serviceID+". Type 'exit' to quit.") {
		t.Errorf("missing REPL banner in stdout: %q", stdout)
	}
	if !strings.Contains(stdout, "clank:"+serviceID+"$ ") {
		t.Errorf("missing REPL prompt in stdout: %q", stdout)
	}
	if !strings.Contains(stdout, "/app\n") {
		t.Errorf("missing command output in stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "not found\n") {
		t.Errorf("missing command stderr: %q", stderr)
	}
	if len(commands) != 2 || commands[0] != "pwd" || commands[1] != "failcmd" {
		t.Errorf("commands sent = %#v, want [pwd failcmd]", commands)
	}
}
