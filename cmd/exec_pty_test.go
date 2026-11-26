package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

var testWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestBuildConsoleWSURL(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		serviceID string
		want      string
	}{
		{
			name:      "https to wss",
			baseURL:   "https://api.clank.host/",
			serviceID: "svc-123",
			want:      "wss://api.clank.host/api/services/svc-123/console/ws",
		},
		{
			name:      "http to ws",
			baseURL:   "http://127.0.0.1:8000",
			serviceID: "11111111-1111-1111-1111-111111111111",
			want:      "ws://127.0.0.1:8000/api/services/11111111-1111-1111-1111-111111111111/console/ws",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildConsoleWSURL(tc.baseURL, tc.serviceID)
			if err != nil {
				t.Fatalf("buildConsoleWSURL error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExecPTYModeAuthInputOutputAndExitCode(t *testing.T) {
	const serviceID = "55555555-5555-5555-5555-555555555555"

	var gotAuthHeader string
	var gotQuery string
	var gotInput string

	setupExecTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/services/"+serviceID+"/console/ws" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotAuthHeader = r.Header.Get("Authorization")
		gotQuery = r.URL.RawQuery

		conn, err := testWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Send ready frame in pty mode.
		if err := conn.WriteJSON(consoleServerFrame{
			Type:        "ready",
			Mode:        "pty",
			ServiceName: "web-api",
			Cwd:         "/app",
		}); err != nil {
			t.Errorf("write ready: %v", err)
			return
		}

		// Read raw input frame from client.
		var inFrame consoleClientFrame
		if err := conn.ReadJSON(&inFrame); err != nil {
			t.Errorf("read input frame: %v", err)
			return
		}
		if inFrame.Type != "input" {
			t.Errorf("frame type = %q, want %q", inFrame.Type, "input")
		}
		gotInput = inFrame.Data

		// Stream output and exit 0.
		_ = conn.WriteJSON(consoleServerFrame{
			Type:   "output",
			Stream: "stdout",
			Data:   "hello from pty\r\n",
		})
		_ = conn.WriteJSON(consoleServerFrame{
			Type:     "exit",
			ExitCode: 0,
		})
	}))

	resetFlags(execCmd.Flags())
	if err := execCmd.Flags().Set("stdin", "true"); err != nil {
		t.Fatal(err)
	}
	if err := execCmd.Flags().Set("tty", "true"); err != nil {
		t.Fatal(err)
	}
	execInput = strings.NewReader("echo hi\r")

	stdout, _, err := captureStdIO(t, func() error {
		return runExec(execCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runExec -it error: %v", err)
	}

	if gotAuthHeader != "Bearer clank_testtoken" {
		t.Errorf("Authorization header = %q, want %q", gotAuthHeader, "Bearer clank_testtoken")
	}
	if gotQuery != "" {
		t.Errorf("query string = %q, want empty (token must never be in query string)", gotQuery)
	}
	if gotInput != "echo hi\r" {
		t.Errorf("input data = %q, want %q", gotInput, "echo hi\r")
	}
	if stdout != "hello from pty\r\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hello from pty\r\n")
	}
}

func TestExecPTYModeNonZeroExitCodeReturnsError(t *testing.T) {
	const serviceID = "66666666-6666-6666-6666-666666666666"

	setupExecTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		_ = conn.WriteJSON(consoleServerFrame{
			Type:        "ready",
			Mode:        "pty",
			ServiceName: "worker",
			Cwd:         "/",
		})
		_ = conn.WriteJSON(consoleServerFrame{
			Type:   "output",
			Stream: "stdout",
			Data:   "failing...\n",
		})
		_ = conn.WriteJSON(consoleServerFrame{
			Type:     "exit",
			ExitCode: 42,
		})
	}))

	resetFlags(execCmd.Flags())
	if err := execCmd.Flags().Set("tty", "true"); err != nil {
		t.Fatal(err)
	}
	execInput = strings.NewReader("")

	stdout, _, err := captureStdIO(t, func() error {
		return runExec(execCmd, []string{serviceID})
	})
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "42") {
		t.Errorf("error = %q, want exit code 42", err.Error())
	}
	if stdout != "failing...\n" {
		t.Errorf("stdout = %q, want %q", stdout, "failing...\n")
	}
}

func TestExecPTYSessionFallbackMode(t *testing.T) {
	const serviceID = "77777777-7777-7777-7777-777777777777"

	var receivedCommands []string
	var gotAuthHeader string

	setupExecTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		conn, err := testWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		_ = conn.WriteJSON(consoleServerFrame{
			Type:        "ready",
			Mode:        "session",
			ServiceName: "legacy-svc",
			Cwd:         "/srv",
		})

		for {
			var frame consoleClientFrame
			if err := conn.ReadJSON(&frame); err != nil {
				return
			}
			if frame.Type != "exec" {
				t.Errorf("frame type = %q, want %q", frame.Type, "exec")
				continue
			}
			receivedCommands = append(receivedCommands, frame.Command)

			switch frame.Command {
			case "pwd":
				_ = conn.WriteJSON(consoleServerFrame{
					Type:   "output",
					Stream: "stdout",
					Data:   "/srv\n",
				})
				_ = conn.WriteJSON(consoleServerFrame{
					Type:     "exit",
					ExitCode: 0,
					Cwd:      "/srv",
				})
			case "badcmd":
				_ = conn.WriteJSON(consoleServerFrame{
					Type:   "output",
					Stream: "stderr",
					Data:   "command not found\n",
				})
				_ = conn.WriteJSON(consoleServerFrame{
					Type:     "exit",
					ExitCode: 127,
				})
			}
		}
	}))

	resetFlags(execCmd.Flags())
	if err := execCmd.Flags().Set("stdin", "true"); err != nil {
		t.Fatal(err)
	}
	if err := execCmd.Flags().Set("tty", "true"); err != nil {
		t.Fatal(err)
	}
	execInput = strings.NewReader("pwd\nbadcmd\nexit\n")

	stdout, stderr, err := captureStdIO(t, func() error {
		return runExec(execCmd, []string{serviceID})
	})
	if err != nil {
		t.Fatalf("runExec -it session fallback error: %v", err)
	}

	if gotAuthHeader != "Bearer clank_testtoken" {
		t.Errorf("Authorization header = %q, want %q", gotAuthHeader, "Bearer clank_testtoken")
	}
	if len(receivedCommands) != 2 || receivedCommands[0] != "pwd" || receivedCommands[1] != "badcmd" {
		t.Errorf("receivedCommands = %#v, want [pwd badcmd]", receivedCommands)
	}
	if !strings.Contains(stdout, "/srv\n") {
		t.Errorf("stdout = %q, want /srv", stdout)
	}
	if !strings.Contains(stderr, "command not found\n") || !strings.Contains(stderr, "[exit 127]") {
		t.Errorf("stderr = %q, want error output and [exit 127]", stderr)
	}
}
