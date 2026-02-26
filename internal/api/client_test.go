package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendsCookieHeader(t *testing.T) {
	var receivedCookie string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := New(server.URL, "test-jwt-token")
	var result map[string]string
	err := client.get("/api/health", &result)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}

	expected := "access_token=test-jwt-token"
	if receivedCookie != expected {
		t.Errorf("expected Cookie header %q, got %q", expected, receivedCookie)
	}
}

func TestClientPostJSON(t *testing.T) {
	var receivedBody map[string]string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer server.Close()

	client := New(server.URL, "token")
	body := map[string]string{"name": "test"}
	var result map[string]string
	err := client.post("/api/projects", body, &result)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}

	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", receivedContentType)
	}
	if receivedBody["name"] != "test" {
		t.Errorf("expected body name 'test', got %q", receivedBody["name"])
	}
	if result["id"] != "123" {
		t.Errorf("expected result id '123', got %q", result["id"])
	}
}

func TestClientHandles401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"detail": "Not authenticated"})
	}))
	defer server.Close()

	client := New(server.URL, "expired-token")
	err := client.get("/api/auth/me", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !IsUnauthorized(err) {
		t.Errorf("expected unauthorized error, got: %v", err)
	}
}

func TestClientHandles404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"detail": "Not found"})
	}))
	defer server.Close()

	client := New(server.URL, "token")
	err := client.get("/api/projects/nonexistent", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !IsNotFound(err) {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestClientHandles409(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{"detail": "Deployment in progress"})
	}))
	defer server.Close()

	client := New(server.URL, "token")
	err := client.post("/api/services/123/deploy", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !IsConflict(err) {
		t.Errorf("expected conflict error, got: %v", err)
	}
}

func TestLoginExtractsCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/auth/login" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}

		var body LoginRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Email != "admin@test.com" || body.Password != "secret" {
			t.Error("unexpected credentials")
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "access_token",
			Value:    "fake-jwt-token-12345",
			HttpOnly: true,
			Path:     "/",
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{
			Message: "Login successful",
			User: struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			}{ID: "user-1", Email: "admin@test.com"},
		})
	}))
	defer server.Close()

	client := New(server.URL, "")
	resp, token, err := Login(client, "admin@test.com", "secret")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if token != "fake-jwt-token-12345" {
		t.Errorf("expected token 'fake-jwt-token-12345', got %q", token)
	}
	if resp.User.Email != "admin@test.com" {
		t.Errorf("expected email 'admin@test.com', got %q", resp.User.Email)
	}
}
