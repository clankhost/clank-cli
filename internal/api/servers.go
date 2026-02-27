package api

import "fmt"

// Server represents a Clank server (agent host).
type Server struct {
	ID              string  `json:"id"`
	OwnerID         string  `json:"owner_id"`
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`
	Status          string  `json:"status"`
	AgentVersion    *string `json:"agent_version"`
	Hostname        *string `json:"hostname"`
	OS              *string `json:"os"`
	Arch            *string `json:"arch"`
	CPUCores        *int    `json:"cpu_cores"`
	MemoryBytes     *int64  `json:"memory_bytes"`
	DockerVersion   *string `json:"docker_version"`
	LastHeartbeatAt *string `json:"last_heartbeat_at"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// EnrollmentToken is the response from creating a server or regenerating a token.
type EnrollmentToken struct {
	ServerID        string `json:"server_id"`
	EnrollmentToken string `json:"enrollment_token"`
	ExpiresAt       string `json:"expires_at"`
	InstallCommand  string `json:"install_command"`
}

// CreateServerRequest is the body for POST /api/servers/.
type CreateServerRequest struct {
	Name string `json:"name"`
}

// ListServers returns all servers for the current user.
func ListServers(c *Client) ([]Server, error) {
	var servers []Server
	if err := c.get("/api/servers/", &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

// CreateServer creates a new server and returns the enrollment token.
func CreateServer(c *Client, name string) (*EnrollmentToken, error) {
	var token EnrollmentToken
	if err := c.post("/api/servers/", CreateServerRequest{Name: name}, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// DeleteServer decommissions a server.
func DeleteServer(c *Client, id string) error {
	return c.delete(fmt.Sprintf("/api/servers/%s", id))
}
