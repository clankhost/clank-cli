package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the Clank API HTTP client.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// New creates a new API client.
func New(baseURL, token string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// apiErrorBody is the error response shape from FastAPI.
type apiErrorBody struct {
	Detail string `json:"detail"`
}

// do performs an HTTP request and decodes the JSON response.
func (c *Client) do(method, path string, body any, result any) error {
	url := c.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Auth: send JWT as cookie (matches FastAPI Cookie(default=None) dependency).
	if c.Token != "" {
		req.Header.Set("Cookie", "access_token="+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-2xx responses.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Special case: 204 No Content is success with no body.
		if resp.StatusCode == 204 {
			return nil
		}

		respBody, _ := io.ReadAll(resp.Body)
		detail := string(respBody)

		// Try to parse structured error.
		var errBody apiErrorBody
		if json.Unmarshal(respBody, &errBody) == nil && errBody.Detail != "" {
			detail = errBody.Detail
		}

		if resp.StatusCode == 401 {
			return &APIError{StatusCode: 401, Detail: "session expired or invalid — run 'clank login'"}
		}

		return &APIError{StatusCode: resp.StatusCode, Detail: detail}
	}

	// 204 No Content — nothing to decode.
	if resp.StatusCode == 204 {
		return nil
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

func (c *Client) get(path string, result any) error {
	return c.do(http.MethodGet, path, nil, result)
}

func (c *Client) post(path string, body any, result any) error {
	return c.do(http.MethodPost, path, body, result)
}

func (c *Client) patch(path string, body any, result any) error {
	return c.do(http.MethodPatch, path, body, result)
}

func (c *Client) delete(path string) error {
	return c.do(http.MethodDelete, path, nil, nil)
}

// SSEStream opens a GET request for Server-Sent Events and returns the
// raw response body. The caller must close it when done.
func (c *Client) SSEStream(path string) (io.ReadCloser, error) {
	url := c.BaseURL + path

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	if c.Token != "" {
		req.Header.Set("Cookie", "access_token="+c.Token)
	}

	// No timeout for SSE — streams are long-lived.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SSE request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		detail := string(body)

		var errBody apiErrorBody
		if json.Unmarshal(body, &errBody) == nil && errBody.Detail != "" {
			detail = errBody.Detail
		}

		return nil, &APIError{StatusCode: resp.StatusCode, Detail: detail}
	}

	return resp.Body, nil
}
