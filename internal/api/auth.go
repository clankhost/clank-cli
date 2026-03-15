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

// Login authenticates and returns the login response plus the session token
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
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

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

	// Extract session token from Set-Cookie header.
	token := ""
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "clank_session" {
			token = cookie.Value
			break
		}
	}
	if token == "" {
		return nil, "", fmt.Errorf("login succeeded but no clank_session cookie in response")
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

// DeviceCodeResponse is the response from POST /api/auth/device/code.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceTokenResponse is the response from POST /api/auth/device/token.
type DeviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	User        struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

// RequestDeviceCode initiates the device authorization flow.
func RequestDeviceCode(c *Client) (*DeviceCodeResponse, error) {
	var resp DeviceCodeResponse
	if err := c.post("/api/auth/device/code", map[string]string{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PollDeviceToken polls for device authorization completion.
// Returns the token response on success, nil if still pending, or an error.
func PollDeviceToken(c *Client, deviceCode string) (*DeviceTokenResponse, error) {
	body := map[string]string{"device_code": deviceCode}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/auth/device/token", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 {
		respBody, _ := io.ReadAll(resp.Body)
		var errBody apiErrorBody
		if json.Unmarshal(respBody, &errBody) == nil {
			if errBody.Detail == "authorization_pending" {
				return nil, nil // Still pending — not an error
			}
		}
		return nil, &APIError{StatusCode: 400, Detail: errBody.Detail}
	}

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &APIError{StatusCode: resp.StatusCode, Detail: string(respBody)}
	}

	var tokenResp DeviceTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding device token response: %w", err)
	}
	return &tokenResp, nil
}
