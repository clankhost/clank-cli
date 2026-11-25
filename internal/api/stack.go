package api

import "fmt"

// StackSyncRequest is the request payload for POST /api/projects/{id}/stack-sync.
type StackSyncRequest struct {
	TomlContent string  `json:"toml_content"`
	ServerID    *string `json:"server_id,omitempty"`
	DryRun      bool    `json:"dry_run"`
	Deploy      bool    `json:"deploy"`
}

// StackSyncAction describes a single database or service plan/apply action.
type StackSyncAction struct {
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	Action        string   `json:"action"`
	ChangedFields []string `json:"changed_fields"`
	ServiceID     *string  `json:"service_id"`
	DeploymentID  *string  `json:"deployment_id"`
}

// StackSyncSummary summarizes counts across all stack actions.
type StackSyncSummary struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Unchanged int `json:"unchanged"`
	Deployed  int `json:"deployed"`
}

// StackSyncResponse is returned by POST /api/projects/{id}/stack-sync.
type StackSyncResponse struct {
	ProjectID string            `json:"project_id"`
	DryRun    bool              `json:"dry_run"`
	Actions   []StackSyncAction `json:"actions"`
	Summary   StackSyncSummary  `json:"summary"`
}

// SyncProjectStack plans or applies a multi-service clank.toml stack blueprint.
func SyncProjectStack(c *Client, projectID string, req StackSyncRequest) (*StackSyncResponse, error) {
	var resp StackSyncResponse
	if err := c.post(fmt.Sprintf("/api/projects/%s/stack-sync", projectID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
