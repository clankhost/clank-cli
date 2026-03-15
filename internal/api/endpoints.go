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
