package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// LoginRequest is the body for POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is the response from POST /api/auth/login.
type LoginResponse struct {
	Message string `json:"message"`
	User    struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

// UserResponse is the response from GET /api/auth/me.
type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Login authenticates and returns the login response plus the JWT token
// extracted from the Set-Cookie header. The token is NOT stored — the caller
// is responsible for persisting it via config.SaveToken().
func Login(c *Client, email, password string) (*LoginResponse, string, error) {
	body, err := json.Marshal(LoginRequest{Email: email, Password: password})
	if err != nil {
		return nil, "", fmt.Errorf("marshaling login request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("creating login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		detail := string(respBody)
		var errBody apiErrorBody
		if json.Unmarshal(respBody, &errBody) == nil && errBody.Detail != "" {
			detail = errBody.Detail
		}
		return nil, "", &APIError{StatusCode: resp.StatusCode, Detail: detail}
	}

	// Parse response body.
	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, "", fmt.Errorf("decoding login response: %w", err)
	}

	// Extract JWT from Set-Cookie header.
	token := ""
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "access_token" {
			token = cookie.Value
			break
		}
	}
	if token == "" {
		return nil, "", fmt.Errorf("login succeeded but no access_token cookie in response")
	}

	return &loginResp, token, nil
}

// Me returns the currently authenticated user.
func Me(c *Client) (*UserResponse, error) {
	var user UserResponse
	if err := c.get("/api/auth/me", &user); err != nil {
		return nil, err
	}
	return &user, nil
}
