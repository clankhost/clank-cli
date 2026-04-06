package api

import "fmt"

// Deployment represents a Clank deployment.
type Deployment struct {
	ID                 string         `json:"id"`
	ServiceID          string         `json:"service_id"`
	BuildID            *string        `json:"build_id"`
	Status             string         `json:"status"`
	ImageTag           *string        `json:"image_tag"`
	ContainerID        *string        `json:"container_id"`
	Port               int            `json:"port"`
	HealthCheckConfig  map[string]any `json:"health_check_config"`
	DomainsSnapshot    []string       `json:"domains_snapshot"`
	GitSHA             *string        `json:"git_sha"`
	GitBranch          *string        `json:"git_branch"`
	TriggeredBy        string         `json:"triggered_by"`
	SourceDeploymentID *string        `json:"source_deployment_id"`
	ErrorMessage       *string        `json:"error_message"`
	ArtifactScope      *string        `json:"artifact_scope"`
	RollbackCapable    bool           `json:"rollback_capable"`
	ImageDigest        *string        `json:"image_digest"`
	ContainerName      *string        `json:"container_name"`
	StartedAt          *string        `json:"started_at"`
	FinishedAt         *string        `json:"finished_at"`
	CreatedAt          string         `json:"created_at"`
	UpdatedAt          string         `json:"updated_at"`
}

// DeploymentEvent represents a lifecycle event for a deployment.
type DeploymentEvent struct {
	ID           string         `json:"id"`
	DeploymentID string         `json:"deployment_id"`
	EventType    string         `json:"event_type"`
	Message      string         `json:"message"`
	MetadataJSON map[string]any `json:"metadata_json"`
	CreatedAt    string         `json:"created_at"`
}

// GetDeployment returns a deployment by ID.
func GetDeployment(c *Client, id string) (*Deployment, error) {
	var deployment Deployment
	if err := c.get(fmt.Sprintf("/api/deployments/%s", id), &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// GetDeploymentEvents returns all events for a deployment.
func GetDeploymentEvents(c *Client, id string) ([]DeploymentEvent, error) {
	var events []DeploymentEvent
	if err := c.get(fmt.Sprintf("/api/deployments/%s/events", id), &events); err != nil {
		return nil, err
	}
	return events, nil
}

// ListDeployments returns all deployments for a service.
func ListDeployments(c *Client, serviceID string) ([]Deployment, error) {
	var deployments []Deployment
	if err := c.get(fmt.Sprintf("/api/services/%s/deployments", serviceID), &deployments); err != nil {
		return nil, err
	}
	return deployments, nil
}

// PushToRegistryResponse is the response from POST /deployments/{id}/push-to-registry.
type PushToRegistryResponse struct {
	Status      string `json:"status"`
	TargetImage string `json:"target_image"`
}

// PushToRegistry initiates pushing a deployment's image to the registry.
func PushToRegistry(c *Client, deploymentID string) (*PushToRegistryResponse, error) {
	var resp PushToRegistryResponse
	if err := c.post(fmt.Sprintf("/api/deployments/%s/push-to-registry", deploymentID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelDeployment cancels a pending or in-progress deployment.
func CancelDeployment(c *Client, deploymentID string) (*Deployment, error) {
	var dep Deployment
	if err := c.post(fmt.Sprintf("/api/deployments/%s/cancel", deploymentID), nil, &dep); err != nil {
		return nil, err
	}
	return &dep, nil
}

// IsTerminalStatus returns true if the deployment status is a terminal state.
func IsTerminalStatus(status string) bool {
	switch status {
	case "active", "failed", "superseded", "rolled_back", "cancelled":
		return true
	default:
		return false
	}
}
