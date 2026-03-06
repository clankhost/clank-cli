package api

import "fmt"

// Team represents a Clank team.
type Team struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TeamMember represents a member of a team.
type TeamMember struct {
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

// ListTeams returns teams the current user belongs to.
func ListTeams(c *Client) ([]Team, error) {
	var teams []Team
	if err := c.get("/api/teams", &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// CreateTeam creates a new team.
func CreateTeam(c *Client, name string) (*Team, error) {
	var team Team
	body := map[string]string{"name": name}
	if err := c.post("/api/teams", body, &team); err != nil {
		return nil, err
	}
	return &team, nil
}

// ListTeamMembers returns members of a team.
func ListTeamMembers(c *Client, teamID string) ([]TeamMember, error) {
	var members []TeamMember
	if err := c.get(fmt.Sprintf("/api/teams/%s/members", teamID), &members); err != nil {
		return nil, err
	}
	return members, nil
}

// InviteTeamMember invites a user to a team.
func InviteTeamMember(c *Client, teamID, email, role string) (*InviteResponse, error) {
	var invite InviteResponse
	body := map[string]string{"email": email, "role": role}
	if err := c.post(fmt.Sprintf("/api/teams/%s/members/invite", teamID), body, &invite); err != nil {
		return nil, err
	}
	return &invite, nil
}

// ChangeTeamRole changes a member's role in a team.
func ChangeTeamRole(c *Client, teamID, memberID, role string) (*TeamMember, error) {
	var member TeamMember
	body := map[string]string{"role": role}
	if err := c.patch(fmt.Sprintf("/api/teams/%s/members/%s", teamID, memberID), body, &member); err != nil {
		return nil, err
	}
	return &member, nil
}

// RemoveTeamMember removes a member from a team.
func RemoveTeamMember(c *Client, teamID, memberID string) error {
	return c.delete(fmt.Sprintf("/api/teams/%s/members/%s", teamID, memberID))
}
