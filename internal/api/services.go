package api

import "fmt"

// Service represents a Clank service.
type Service struct {
	ID                   string  `json:"id"`
	ProjectID            string  `json:"project_id"`
	Name                 string  `json:"name"`
	Slug                 string  `json:"slug"`
	RepoURL              string  `json:"repo_url"`
	Branch               string  `json:"branch"`
	DockerfilePath       string  `json:"dockerfile_path"`
	Port                 int     `json:"port"`
	HealthCheckPath      string  `json:"health_check_path"`
	HealthCheckTimeoutS  int     `json:"health_check_timeout_s"`
	HealthCheckRetries   int     `json:"health_check_retries"`
	HealthCheckIntervalS int     `json:"health_check_interval_s"`
	BuildTimeoutS        int     `json:"build_timeout_s"`
	DeployTimeoutS       int     `json:"deploy_timeout_s"`
	CurrentDeploymentID  *string `json:"current_deployment_id"`
	AutoDeploy           bool    `json:"auto_deploy"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

// ListServices returns all services in a project.
func ListServices(c *Client, projectID string) ([]Service, error) {
	var services []Service
	if err := c.get(fmt.Sprintf("/api/services/%s/services", projectID), &services); err != nil {
		return nil, err
	}
	return services, nil
}

// GetService returns a service by ID.
func GetService(c *Client, id string) (*Service, error) {
	var service Service
	if err := c.get(fmt.Sprintf("/api/services/%s", id), &service); err != nil {
		return nil, err
	}
	return &service, nil
}

// CreateServiceRequest is the body for creating a service.
type CreateServiceRequest struct {
	Name           string `json:"name"`
	RepoURL        string `json:"repo_url"`
	Branch         string `json:"branch,omitempty"`
	DockerfilePath string `json:"dockerfile_path,omitempty"`
	Port           int    `json:"port,omitempty"`
}

// CreateService creates a new service in a project.
func CreateService(c *Client, projectID string, req CreateServiceRequest) (*Service, error) {
	var service Service
	if err := c.post(fmt.Sprintf("/api/services/%s/services", projectID), req, &service); err != nil {
		return nil, err
	}
	return &service, nil
}

// DeleteService deletes a service by ID.
func DeleteService(c *Client, id string) error {
	return c.delete(fmt.Sprintf("/api/services/%s", id))
}

// TriggerDeploy triggers a manual deployment for a service.
func TriggerDeploy(c *Client, serviceID string) (*Deployment, error) {
	var deployment Deployment
	if err := c.post(fmt.Sprintf("/api/services/%s/deploy", serviceID), nil, &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// RollbackRequest is the body for POST /api/services/{id}/rollback.
type RollbackRequest struct {
	TargetDeploymentID string `json:"target_deployment_id"`
}

// TriggerRollback triggers a rollback to a specific deployment.
func TriggerRollback(c *Client, serviceID, targetDeploymentID string) (*Deployment, error) {
	var deployment Deployment
	body := RollbackRequest{TargetDeploymentID: targetDeploymentID}
	if err := c.post(fmt.Sprintf("/api/services/%s/rollback", serviceID), body, &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// Domain represents a service domain.
type Domain struct {
	ID          string `json:"id"`
	ServiceID   string `json:"service_id"`
	Domain      string `json:"domain"`
	IsPrimary   bool   `json:"is_primary"`
	IsGenerated bool   `json:"is_generated"`
	CreatedAt   string `json:"created_at"`
}

// ListDomains returns all domains for a service.
func ListDomains(c *Client, serviceID string) ([]Domain, error) {
	var domains []Domain
	if err := c.get(fmt.Sprintf("/api/services/%s/domains", serviceID), &domains); err != nil {
		return nil, err
	}
	return domains, nil
}
