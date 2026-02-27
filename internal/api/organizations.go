package api

import "fmt"

// Organization represents a Clank organization.
type Organization struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// OrgMember represents a member of an organization.
type OrgMember struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	Email       string  `json:"email"`
	DisplayName string  `json:"display_name"`
	Role        string  `json:"role"`
	JoinedAt    *string `json:"joined_at"`
	CreatedAt   string  `json:"created_at"`
}

// InviteResponse is the response from inviting a member.
type InviteResponse struct {
	InvitationToken string `json:"invitation_token"`
	ExpiresAt       string `json:"expires_at"`
	InviteURL       string `json:"invite_url"`
}

// ListOrgs returns organizations the current user belongs to.
func ListOrgs(c *Client) ([]Organization, error) {
	var orgs []Organization
	if err := c.get("/api/orgs", &orgs); err != nil {
		return nil, err
	}
	return orgs, nil
}

// CreateOrg creates a new organization.
func CreateOrg(c *Client, name string) (*Organization, error) {
	var org Organization
	body := map[string]string{"name": name}
	if err := c.post("/api/orgs", body, &org); err != nil {
		return nil, err
	}
	return &org, nil
}

// ListMembers returns members of an organization.
func ListMembers(c *Client, orgID string) ([]OrgMember, error) {
	var members []OrgMember
	if err := c.get(fmt.Sprintf("/api/orgs/%s/members", orgID), &members); err != nil {
		return nil, err
	}
	return members, nil
}

// InviteMember invites a user to an organization.
func InviteMember(c *Client, orgID, email, role string) (*InviteResponse, error) {
	var invite InviteResponse
	body := map[string]string{"email": email, "role": role}
	if err := c.post(fmt.Sprintf("/api/orgs/%s/members/invite", orgID), body, &invite); err != nil {
		return nil, err
	}
	return &invite, nil
}

// ChangeRole changes a member's role in an organization.
func ChangeRole(c *Client, orgID, memberID, role string) (*OrgMember, error) {
	var member OrgMember
	body := map[string]string{"role": role}
	if err := c.patch(fmt.Sprintf("/api/orgs/%s/members/%s", orgID, memberID), body, &member); err != nil {
		return nil, err
	}
	return &member, nil
}

// RemoveMember removes a member from an organization.
func RemoveMember(c *Client, orgID, memberID string) error {
	return c.delete(fmt.Sprintf("/api/orgs/%s/members/%s", orgID, memberID))
}
