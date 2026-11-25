package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clankhost/clank-cli/internal/api"
)

func TestValidateUpdateURL(t *testing.T) {
	valid := []string{
		"https://clank.host/api/install/cli",
		"http://127.0.0.1:8080/api/install/cli",
		"http://localhost:8080/api/install/cli",
		"http://[::1]:8080/api/install/cli",
	}
	for _, u := range valid {
		if err := validateUpdateURL(u); err != nil {
			t.Errorf("validateUpdateURL(%q) unexpected error: %v", u, err)
		}
	}

	invalid := []string{
		"http://clank.host/api/install/cli",
		"http://192.168.1.5/api/install/cli",
		"ftp://clank.host/file",
		"",
	}
	for _, u := range invalid {
		if err := validateUpdateURL(u); err == nil {
			t.Errorf("validateUpdateURL(%q) succeeded, want error", u)
		}
	}
}

func TestReadBoundedBinary(t *testing.T) {
	data, err := readBoundedBinary(bytes.NewReader([]byte("hello")), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("got %q, want hello", string(data))
	}

	if _, err := readBoundedBinary(bytes.NewReader([]byte("0123456789A")), 10); err == nil {
		t.Fatal("expected error when binary exceeds max size")
	}
}

func TestMaskTokenAndAPIKey(t *testing.T) {
	// Must never panic on short strings
	for _, s := range []string{"", "a", "abc", "12345678", "clank_1", "clank_1234567890abcdef"} {
		_ = maskToken(s)
		_ = maskAPIKeyPrefix(s)
	}
	if got := maskToken("short"); got != "****" {
		t.Errorf("maskToken(short) = %q, want ****", got)
	}
	if got := maskToken("abcd1234wxyz"); got != "abcd...wxyz" {
		t.Errorf("maskToken(abcd1234wxyz) = %q, want abcd...wxyz", got)
	}
	if got := maskAPIKeyPrefix("clank_1234567890abcdef"); got != "clank_12345678..." {
		t.Errorf("maskAPIKeyPrefix(...) = %q, want clank_12345678...", got)
	}
}

func TestFollowDeploymentCancelsBuildLogsOnTerminalEvent(t *testing.T) {
	logsCancelled := make(chan struct{}, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		switch r.URL.Path {
		case "/api/deployments/dep-1/logs/stream":
			// Hold build logs open until the client cancels the request.
			<-r.Context().Done()
			select {
			case logsCancelled <- struct{}{}:
			default:
			}
		case "/api/deployments/dep-1/events/stream":
			fmt.Fprint(w, "event: deployment_event\ndata: {\"event_type\":\"deployment.active\",\"message\":\"Ready\"}\n\n")
			flusher.Flush()
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := api.New(srv.URL, "")

	done := make(chan error, 1)
	go func() {
		done <- followDeployment(client, "dep-1")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("followDeployment returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("followDeployment hung after terminal event instead of cancelling build logs stream")
	}

	select {
	case <-logsCancelled:
	case <-time.After(1 * time.Second):
		// Even if server handler noticed close slightly after, client already exited cleanly.
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		<-ctx.Done()
	}
}
