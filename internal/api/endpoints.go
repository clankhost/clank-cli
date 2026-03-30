package api

import "fmt"

// Endpoint represents an endpoint for a service.
type Endpoint struct {
	ID              string  `json:"id"`
	ServiceID       string  `json:"service_id"`
	Provider        string  `json:"provider"`
	Status          string  `json:"status"`
	Hostname        *string `json:"hostname"`
	PathPrefix      *string `json:"path_prefix"`
	TLSMode         string  `json:"tls_mode"`
	IsPrimary       bool    `json:"is_primary"`
	URL             *string `json:"url"`
	ProviderDisplay *string `json:"provider_display"`
	LastError       *string `json:"last_error"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// ListEndpoints returns all endpoints for a service.
func ListEndpoints(c *Client, serviceID string) ([]Endpoint, error) {
	var endpoints []Endpoint
	if err := c.get(fmt.Sprintf("/api/services/%s/endpoints", serviceID), &endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}

// UpdateEndpointRequest is the body for updating an endpoint.
type UpdateEndpointRequest struct {
	Hostname  *string `json:"hostname,omitempty"`
	IsPrimary *bool   `json:"is_primary,omitempty"`
}

// UpdateEndpoint updates an endpoint's hostname or primary status.
func UpdateEndpoint(c *Client, endpointID string, req UpdateEndpointRequest) (*Endpoint, error) {
	var ep Endpoint
	if err := c.patch(fmt.Sprintf("/api/services/endpoints/%s", endpointID), req, &ep); err != nil {
		return nil, err
	}
	return &ep, nil
}

// DeleteEndpoint removes an endpoint.
func DeleteEndpoint(c *Client, endpointID string) error {
	return c.delete(fmt.Sprintf("/api/services/endpoints/%s", endpointID))
}

// CheckEndpoint triggers a health/status check on an endpoint.
func CheckEndpoint(c *Client, endpointID string) (*Endpoint, error) {
	var ep Endpoint
	if err := c.post(fmt.Sprintf("/api/services/endpoints/%s/check", endpointID), nil, &ep); err != nil {
		return nil, err
	}
	return &ep, nil
}
