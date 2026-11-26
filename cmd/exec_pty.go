package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

type consoleServerFrame struct {
	Type        string `json:"type"`
	Mode        string `json:"mode,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	Cwd         string `json:"cwd,omitempty"`
	Data        string `json:"data,omitempty"`
	Stream      string `json:"stream,omitempty"`
	ExitCode    int    `json:"exit_code,omitempty"`
	Message     string `json:"message,omitempty"`
}

type consoleClientFrame struct {
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
	Command string `json:"command,omitempty"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
}

type wsJSONWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *wsJSONWriter) writeJSON(v any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(v)
}

func buildConsoleWSURL(baseURL, serviceID string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return "", fmt.Errorf("API base URL is empty")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid API base URL %q: %w", baseURL, err)
	}

	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "wss", "ws":
		// already a WebSocket scheme
	default:
		return "", fmt.Errorf("unsupported URL scheme %q in base URL %q", u.Scheme, baseURL)
	}

	u.Path = strings.TrimRight(u.Path, "/") + "/api/services/" + url.PathEscape(serviceID) + "/console/ws"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func runExecPTY(client *api.Client, serviceID, serviceRef string) error {
	wsURL, err := buildConsoleWSURL(client.BaseURL, serviceID)
	if err != nil {
		return err
	}

	headers := http.Header{}
	if client.Token != "" {
		headers.Set("Authorization", "Bearer "+client.Token)
	}
	if client.TeamID != "" {
		headers.Set("X-Team-Id", client.TeamID)
	}

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("connecting to console WebSocket (HTTP %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("connecting to console WebSocket: %w", err)
	}
	defer conn.Close()

	writer := &wsJSONWriter{conn: conn}

	// Read initial ready frame from the server.
	var ready consoleServerFrame
	if err := conn.ReadJSON(&ready); err != nil {
		return fmt.Errorf("reading console handshake: %w", err)
	}
	if ready.Type == "error" {
		return fmt.Errorf("console error: %s", ready.Message)
	}
	if ready.Type != "ready" {
		return fmt.Errorf("unexpected initial console frame %q", ready.Type)
	}

	if ready.Mode == "pty" {
		return runPTYLoop(conn, writer)
	}
	return runSessionLoop(conn, writer, serviceRef, ready.Cwd)
}

func runPTYLoop(conn *websocket.Conn, writer *wsJSONWriter) error {
	in := execInput
	useOSStdin := false
	if in == nil {
		in = os.Stdin
		useOSStdin = true
	}

	if useOSStdin {
		fd := int(os.Stdin.Fd())
		if term.IsTerminal(fd) {
			oldState, err := term.MakeRaw(fd)
			if err == nil {
				defer func() {
					_ = term.Restore(fd, oldState)
				}()
			}
			if cols, rows, err := term.GetSize(fd); err == nil && cols > 0 && rows > 0 {
				_ = writer.writeJSON(consoleClientFrame{
					Type: "resize",
					Cols: cols,
					Rows: rows,
				})
			}
		}
	}

	done := make(chan struct{})
	defer close(done)

	// Stream raw input bytes from stdin to the remote PTY.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := in.Read(buf)
			if n > 0 {
				select {
				case <-done:
					return
				default:
				}
				if writeErr := writer.writeJSON(consoleClientFrame{
					Type: "input",
					Data: string(buf[:n]),
				}); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Periodic keepalive ping.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := writer.writeJSON(consoleClientFrame{Type: "ping"}); err != nil {
					return
				}
			}
		}
	}()

	for {
		var frame consoleServerFrame
		if err := conn.ReadJSON(&frame); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return fmt.Errorf("reading console stream: %w", err)
		}

		switch frame.Type {
		case "output":
			if frame.Data != "" {
				_, _ = io.WriteString(os.Stdout, frame.Data)
			}
		case "exit":
			if frame.ExitCode != 0 {
				return fmt.Errorf("command exited with status %d", frame.ExitCode)
			}
			return nil
		case "error":
			return fmt.Errorf("console error: %s", frame.Message)
		case "pong":
			// keepalive ack
		}
	}
}

func runSessionLoop(conn *websocket.Conn, writer *wsJSONWriter, serviceRef, _ string) error {
	in := execInput
	if in == nil {
		in = os.Stdin
	}

	fmt.Fprintf(os.Stdout, "Connected to %s (session mode). Type 'exit' to quit.\n", serviceRef)
	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprintf(os.Stdout, "clank:%s$ ", serviceRef)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return nil
		}

		if err := writer.writeJSON(consoleClientFrame{
			Type:    "exec",
			Command: line,
		}); err != nil {
			return fmt.Errorf("sending command: %w", err)
		}

		for {
			var frame consoleServerFrame
			if err := conn.ReadJSON(&frame); err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return nil
				}
				return fmt.Errorf("reading session output: %w", err)
			}

			switch frame.Type {
			case "output":
				if frame.Data != "" {
					if frame.Stream == "stderr" {
						_, _ = io.WriteString(os.Stderr, frame.Data)
					} else {
						_, _ = io.WriteString(os.Stdout, frame.Data)
					}
				}
			case "exit":
				if frame.ExitCode != 0 {
					fmt.Fprintf(os.Stderr, "[exit %d]\n", frame.ExitCode)
				}
				goto nextPrompt
			case "error":
				fmt.Fprintf(os.Stderr, "error: %s\n", frame.Message)
				goto nextPrompt
			case "pong":
				// ignore keepalive ack
			}
		}
	nextPrompt:
	}
}
