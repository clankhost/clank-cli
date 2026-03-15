package api

import "fmt"

// EnvVar represents an environment variable for a service.
type EnvVar struct {
	ID        string  `json:"id"`
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	IsSecret  bool    `json:"is_secret"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

// EnvVarCreateRequest is the body for creating a single env var.
type EnvVarCreateRequest struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret,omitempty"`
}

// EnvVarBulkCreateRequest is the body for creating multiple env vars.
type EnvVarBulkCreateRequest struct {
	Vars []EnvVarCreateRequest `json:"vars"`
}

// EnvVarBulkResponse is the response from a bulk create.
type EnvVarBulkResponse struct {
	Created []EnvVar `json:"created"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors"`
}

// EnvVarRevealResponse is the response from revealing a secret.
type EnvVarRevealResponse struct {
	Value string `json:"value"`
}

// ListEnvVars returns all env vars for a service.
func ListEnvVars(c *Client, serviceID string) ([]EnvVar, error) {
	var vars []EnvVar
	if err := c.get(fmt.Sprintf("/api/services/%s/env-vars", serviceID), &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

// CreateEnvVar creates a single env var on a service.
func CreateEnvVar(c *Client, serviceID string, req EnvVarCreateRequest) (*EnvVar, error) {
	var v EnvVar
	if err := c.post(fmt.Sprintf("/api/services/%s/env-vars", serviceID), req, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// BulkCreateEnvVars creates multiple env vars on a service.
func BulkCreateEnvVars(c *Client, serviceID string, req EnvVarBulkCreateRequest) (*EnvVarBulkResponse, error) {
	var resp EnvVarBulkResponse
	if err := c.post(fmt.Sprintf("/api/services/%s/env-vars/bulk", serviceID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RevealEnvVar returns the plaintext value of a secret env var.
func RevealEnvVar(c *Client, varID string) (string, error) {
	var resp EnvVarRevealResponse
	if err := c.get(fmt.Sprintf("/api/env-vars/%s/reveal", varID), &resp); err != nil {
		return "", err
	}
	return resp.Value, nil
}

// DeleteEnvVar deletes an env var by ID.
func DeleteEnvVar(c *Client, varID string) error {
	return c.delete(fmt.Sprintf("/api/env-vars/%s", varID))
}
