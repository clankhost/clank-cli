package cmd

import (
	"fmt"
	"strings"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/config"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var teamTopCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage teams",
}

var teamTopListCmd = &cobra.Command{
	Use:   "list",
	Short: "List teams you belong to",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		teams, err := api.ListTeams(client)
		if err != nil {
			return err
		}

		if len(teams) == 0 {
			fmt.Println("No teams found.")
			return nil
		}

		headers := []string{"ID", "NAME", "SLUG", "ROLE", "CREATED"}
		rows := make([][]string, len(teams))
		for i, t := range teams {
			name := t.Name
			if t.ID == cfg.TeamID {
				name += " *"
			}
			rows[i] = []string{
				output.ShortID(t.ID),
				name,
				t.Slug,
				t.Role,
				output.TimeSince(t.CreatedAt),
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var teamCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new team",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		team, err := api.CreateTeam(client, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Team created: %s (id: %s)\n", team.Name, output.ShortID(team.ID))
		return nil
	},
}

var teamSwitchCmd = &cobra.Command{
	Use:   "switch <name-or-id>",
	Short: "Set the active team",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		teams, err := api.ListTeams(client)
		if err != nil {
			return err
		}

		search := strings.ToLower(args[0])
		var match *api.Team
		for _, t := range teams {
			if strings.HasPrefix(t.ID, search) || strings.ToLower(t.Slug) == search || strings.ToLower(t.Name) == search {
				match = &t
				break
			}
		}

		if match == nil {
			return fmt.Errorf("no team matching %q found", args[0])
		}

		if err := config.SaveTeamID(match.ID); err != nil {
			return err
		}

		fmt.Printf("Switched to team: %s\n", match.Name)
		return nil
	},
}

var teamCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active team",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.TeamID == "" {
			fmt.Println("No active team set. Run 'clank team switch <name>'.")
			return nil
		}

		client := newClient()
		teams, err := api.ListTeams(client)
		if err != nil {
			return err
		}

		for _, t := range teams {
			if t.ID == cfg.TeamID {
				fmt.Printf("%s (%s) — role: %s\n", t.Name, t.Slug, t.Role)
				return nil
			}
		}

		fmt.Printf("Active team ID: %s (not found in your teams)\n", output.ShortID(cfg.TeamID))
		return nil
	},
}

// requireTeamID returns the active team ID or an error if not set.
func requireTeamID() (string, error) {
	if cfg.TeamID == "" {
		return "", fmt.Errorf("no active team set\n\nSet it with: clank team switch <name>")
	}
	return cfg.TeamID, nil
}

// --- Team member subcommands ---

var teamMemberListCmd = &cobra.Command{
	Use:   "members",
	Short: "List members of the current team",
	RunE: func(cmd *cobra.Command, args []string) error {
		teamID, err := requireTeamID()
		if err != nil {
			return err
		}

		client := newClient()
		members, err := api.ListTeamMembers(client, teamID)
		if err != nil {
			return err
		}

		if len(members) == 0 {
			fmt.Println("No members found.")
			return nil
		}

		headers := []string{"ID", "EMAIL", "NAME", "ROLE", "JOINED"}
		rows := make([][]string, len(members))
		for i, m := range members {
			name := m.DisplayName
			if name == "" {
				name = "-"
			}
			joined := "Pending"
			if m.JoinedAt != nil {
				joined = output.TimeSince(*m.JoinedAt)
			}
			rows[i] = []string{
				output.ShortID(m.ID),
				m.Email,
				name,
				m.Role,
				joined,
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var teamInviteRole string

var teamMemberInviteCmd = &cobra.Command{
	Use:   "invite <email>",
	Short: "Invite a member to the current team",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		teamID, err := requireTeamID()
		if err != nil {
			return err
		}

		client := newClient()
		invite, err := api.InviteTeamMember(client, teamID, args[0], teamInviteRole)
		if err != nil {
			return err
		}

		fmt.Println("Invite created. Share this link with the user:")
		fmt.Println()
		fmt.Printf("  %s\n", invite.InviteURL)
		fmt.Println()
		fmt.Printf("Expires: %s\n", output.TimeSince(invite.ExpiresAt))
		return nil
	},
}

var teamMemberRemoveCmd = &cobra.Command{
	Use:   "remove <email>",
	Short: "Remove a member from the current team",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		teamID, err := requireTeamID()
		if err != nil {
			return err
		}

		client := newClient()

		// Find member by email.
		members, err := api.ListTeamMembers(client, teamID)
		if err != nil {
			return err
		}

		email := strings.ToLower(args[0])
		var memberID string
		for _, m := range members {
			if strings.ToLower(m.Email) == email {
				memberID = m.ID
				break
			}
		}
		if memberID == "" {
			return fmt.Errorf("no member with email %q found", args[0])
		}

		if err := api.RemoveTeamMember(client, teamID, memberID); err != nil {
			return err
		}

		fmt.Printf("Removed %s from the team.\n", args[0])
		return nil
	},
}

var teamMemberRoleCmd = &cobra.Command{
	Use:   "role <email> <role>",
	Short: "Change a member's role (admin, developer, viewer)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		teamID, err := requireTeamID()
		if err != nil {
			return err
		}

		client := newClient()

		// Find member by email.
		members, err := api.ListTeamMembers(client, teamID)
		if err != nil {
			return err
		}

		email := strings.ToLower(args[0])
		var memberID string
		for _, m := range members {
			if strings.ToLower(m.Email) == email {
				memberID = m.ID
				break
			}
		}
		if memberID == "" {
			return fmt.Errorf("no member with email %q found", args[0])
		}

		member, err := api.ChangeTeamRole(client, teamID, memberID, args[1])
		if err != nil {
			return err
		}

		fmt.Printf("Changed %s role to %s.\n", member.Email, member.Role)
		return nil
	},
}

func init() {
	teamMemberInviteCmd.Flags().StringVar(&teamInviteRole, "role", "developer", "Role for invited member (admin, developer, viewer)")

	// Team-level subcommands (list, create, switch, current).
	teamTopCmd.AddCommand(teamTopListCmd)
	teamTopCmd.AddCommand(teamCreateCmd)
	teamTopCmd.AddCommand(teamSwitchCmd)
	teamTopCmd.AddCommand(teamCurrentCmd)

	// Member management subcommands.
	teamTopCmd.AddCommand(teamMemberListCmd)
	teamTopCmd.AddCommand(teamMemberInviteCmd)
	teamTopCmd.AddCommand(teamMemberRemoveCmd)
	teamTopCmd.AddCommand(teamMemberRoleCmd)

	rootCmd.AddCommand(teamTopCmd)
}
