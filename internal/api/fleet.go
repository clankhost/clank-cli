package api

import "fmt"

// FleetDeployRequest is the request payload for POST /api/projects/{id}/fleet-deploy.
type FleetDeployRequest struct {
	ImageTag      string   `json:"image_tag"`
	ServiceIDs    []string `json:"service_ids,omitempty"`
	CanaryFirst   bool     `json:"canary_first"`
	BatchSize     int      `json:"batch_size"`
	HaltOnFailure bool     `json:"halt_on_failure"`
	DryRun        bool     `json:"dry_run"`
}

// FleetDeployItem represents a single service's status inside a rolling fleet deploy.
type FleetDeployItem struct {
	ServiceID    string  `json:"service_id"`
	ServiceName  string  `json:"service_name"`
	BatchIndex   int     `json:"batch_index"`
	IsCanary     bool    `json:"is_canary"`
	Status       string  `json:"status"`
	DeploymentID *string `json:"deployment_id"`
	ErrorMessage *string `json:"error_message"`
}

// FleetDeployBatch summarizes one rollout batch.
type FleetDeployBatch struct {
	BatchIndex   int      `json:"batch_index"`
	IsCanary     bool     `json:"is_canary"`
	ServiceIDs   []string `json:"service_ids"`
	ServiceNames []string `json:"service_names"`
}

// FleetDeployResponse is the response returned by fleet deploy endpoints.
type FleetDeployResponse struct {
	FleetDeployID     *string            `json:"fleet_deploy_id"`
	ProjectID         string             `json:"project_id"`
	ImageTag          string             `json:"image_tag"`
	Status            string             `json:"status"`
	CanaryFirst       bool               `json:"canary_first"`
	BatchSize         int                `json:"batch_size"`
	HaltOnFailure     bool               `json:"halt_on_failure"`
	CurrentBatchIndex int                `json:"current_batch_index"`
	TotalBatches      int                `json:"total_batches"`
	TotalServices     int                `json:"total_services"`
	CompletedCount    int                `json:"completed_count"`
	FailedCount       int                `json:"failed_count"`
	Batches           []FleetDeployBatch `json:"batches"`
	Items             []FleetDeployItem  `json:"items"`
}

// IsFleetDeployTerminal returns true if a fleet deploy has reached a terminal state.
func IsFleetDeployTerminal(status string) bool {
	switch status {
	case "completed", "halted", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// StartFleetDeploy triggers (or dry-runs) a rolling fleet deploy for a project.
func StartFleetDeploy(c *Client, projectID string, req FleetDeployRequest) (*FleetDeployResponse, error) {
	var resp FleetDeployResponse
	if err := c.post(fmt.Sprintf("/api/projects/%s/fleet-deploy", projectID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFleetDeploy fetches the current status of a fleet deploy (advancing batches if ready).
func GetFleetDeploy(c *Client, projectID, fleetDeployID string) (*FleetDeployResponse, error) {
	var resp FleetDeployResponse
	if err := c.get(fmt.Sprintf("/api/projects/%s/fleet-deploy/%s", projectID, fleetDeployID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelFleetDeploy cancels an in-progress fleet deploy.
func CancelFleetDeploy(c *Client, projectID, fleetDeployID string) (*FleetDeployResponse, error) {
	var resp FleetDeployResponse
	if err := c.post(fmt.Sprintf("/api/projects/%s/fleet-deploy/%s/cancel", projectID, fleetDeployID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
