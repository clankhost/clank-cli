package api

import "fmt"

// Project represents a Clank project.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ListProjects returns all projects.
func ListProjects(c *Client) ([]Project, error) {
	var projects []Project
	if err := c.get("/api/projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// GetProject returns a project by ID.
func GetProject(c *Client, id string) (*Project, error) {
	var project Project
	if err := c.get(fmt.Sprintf("/api/projects/%s", id), &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// CreateProjectRequest is the body for creating a project.
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateProject creates a new project.
func CreateProject(c *Client, name, description string) (*Project, error) {
	var project Project
	body := CreateProjectRequest{Name: name, Description: description}
	if err := c.post("/api/projects", body, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// DeleteProject deletes a project by ID.
func DeleteProject(c *Client, id string) error {
	return c.delete(fmt.Sprintf("/api/projects/%s", id))
}
