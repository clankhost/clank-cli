package api

import "fmt"

// RuntimeState represents the live container state of a service.
type RuntimeState struct {
	ContainerStatus string  `json:"container_status"`
	IsHealthy       *bool   `json:"is_healthy"`
	LastCheckedAt   *string `json:"last_checked_at"`
}

// Service represents a Clank service.
type Service struct {
	ID                     string        `json:"id"`
	ProjectID              string        `json:"project_id"`
	Name                   string        `json:"name"`
	Slug                   string        `json:"slug"`
	ResourceType           string        `json:"resource_type"`
	RepoURL                string        `json:"repo_url"`
	Image                  *string       `json:"image"`
	Branch                 string        `json:"branch"`
	DockerfilePath         string        `json:"dockerfile_path"`
	Port                   int           `json:"port"`
	HealthCheckPath        string        `json:"health_check_path"`
	HealthCheckTimeoutS    int           `json:"health_check_timeout_s"`
	HealthCheckRetries     int           `json:"health_check_retries"`
	HealthCheckIntervalS   int           `json:"health_check_interval_s"`
	BuildTimeoutS          int           `json:"build_timeout_s"`
	DeployTimeoutS         int           `json:"deploy_timeout_s"`
	CPULimit               float64       `json:"cpu_limit"`
	MemoryLimitMB          int           `json:"memory_limit_mb"`
	CurrentDeploymentID    *string       `json:"current_deployment_id"`
	LatestDeploymentStatus *string       `json:"latest_deployment_status"`
	AutoDeploy             bool          `json:"auto_deploy"`
	ServerID               *string       `json:"server_id"`
	RuntimeState           *RuntimeState `json:"runtime_state,omitempty"`
	CreatedAt              string        `json:"created_at"`
	UpdatedAt              string        `json:"updated_at"`
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

// DeployRequest is the optional body for POST /api/services/{id}/deploy.
type DeployRequest struct {
	ImageTag string `json:"image_tag,omitempty"`
}

// TriggerDeploy triggers a manual deployment for a service.
func TriggerDeploy(c *Client, serviceID string) (*Deployment, error) {
	var deployment Deployment
	if err := c.post(fmt.Sprintf("/api/services/%s/deploy", serviceID), nil, &deployment); err != nil {
		return nil, err
	}
	return &deployment, nil
}

// TriggerDeployWithImage triggers a deployment using a pre-built image.
func TriggerDeployWithImage(c *Client, serviceID, imageTag string) (*Deployment, error) {
	var deployment Deployment
	body := DeployRequest{ImageTag: imageTag}
	if err := c.post(fmt.Sprintf("/api/services/%s/deploy", serviceID), body, &deployment); err != nil {
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
	ID                string  `json:"id"`
	ServiceID         string  `json:"service_id"`
	Domain            string  `json:"domain"`
	IsPrimary         bool    `json:"is_primary"`
	IsGenerated       bool    `json:"is_generated"`
	Status            string  `json:"status"`
	VerificationToken *string `json:"verification_token"`
	TxtRecord         *string `json:"txt_record"`
	ErrorMessage      *string `json:"error_message"`
	CreatedAt         string  `json:"created_at"`
}

// ListDomains returns all domains for a service.
func ListDomains(c *Client, serviceID string) ([]Domain, error) {
	var domains []Domain
	if err := c.get(fmt.Sprintf("/api/services/%s/domains", serviceID), &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// AddDomainRequest is the body for adding a custom domain.
type AddDomainRequest struct {
	Domain    string `json:"domain"`
	IsPrimary bool   `json:"is_primary,omitempty"`
}

// AddDomain adds a custom domain to a service.
func AddDomain(c *Client, serviceID string, req AddDomainRequest) (*Domain, error) {
	var domain Domain
	if err := c.post(fmt.Sprintf("/api/services/%s/domains", serviceID), req, &domain); err != nil {
		return nil, err
	}
	return &domain, nil
}

// RemoveDomain deletes a domain.
func RemoveDomain(c *Client, domainID string) error {
	return c.delete(fmt.Sprintf("/api/services/domains/%s", domainID))
}

// RecheckDomain triggers DNS re-verification for a domain.
func RecheckDomain(c *Client, domainID string) (*Domain, error) {
	var domain Domain
	if err := c.post(fmt.Sprintf("/api/services/domains/%s/recheck", domainID), nil, &domain); err != nil {
		return nil, err
	}
	return &domain, nil
}

// ContainerControlResponse is the response from restart/stop/start.
type ContainerControlResponse struct {
	Status string `json:"status"`
}

// RestartService restarts a service's container.
func RestartService(c *Client, serviceID string) (*ContainerControlResponse, error) {
	var resp ContainerControlResponse
	if err := c.post(fmt.Sprintf("/api/services/%s/restart", serviceID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StopService stops a service's container.
func StopService(c *Client, serviceID string) (*ContainerControlResponse, error) {
	var resp ContainerControlResponse
	if err := c.post(fmt.Sprintf("/api/services/%s/stop", serviceID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StartService starts a service's container.
func StartService(c *Client, serviceID string) (*ContainerControlResponse, error) {
	var resp ContainerControlResponse
	if err := c.post(fmt.Sprintf("/api/services/%s/start", serviceID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateResourcesRequest is the body for updating service resource limits.
type UpdateResourcesRequest struct {
	CPULimit      *float64 `json:"cpu_limit,omitempty"`
	MemoryLimitMB *int     `json:"memory_limit_mb,omitempty"`
}

// UpdateServiceResources updates CPU/memory limits for a service.
func UpdateServiceResources(c *Client, serviceID string, req UpdateResourcesRequest) (*Service, error) {
	var service Service
	if err := c.patch(fmt.Sprintf("/api/services/%s", serviceID), req, &service); err != nil {
		return nil, err
	}
	return &service, nil
}
