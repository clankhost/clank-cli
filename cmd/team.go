package cmd

import (
	"fmt"
	"strings"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage team members in the current organization",
}

func requireOrgID() (string, error) {
	if cfg.OrgID == "" {
		return "", fmt.Errorf("no active organization set\n\nSet it with: clank org switch <name>")
	}
	return cfg.OrgID, nil
}


var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List members of the current organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID, err := requireOrgID()
		if err != nil {
			return err
		}

		client := newClient()
		members, err := api.ListMembers(client, orgID)
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

var inviteRole string

var teamInviteCmd = &cobra.Command{
	Use:   "invite <email>",
	Short: "Invite a member to the current organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID, err := requireOrgID()
		if err != nil {
			return err
		}

		client := newClient()
		invite, err := api.InviteMember(client, orgID, args[0], inviteRole)
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

var teamRemoveCmd = &cobra.Command{
	Use:   "remove <email>",
	Short: "Remove a member from the current organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID, err := requireOrgID()
		if err != nil {
			return err
		}

		client := newClient()

		// Find member by email.
		members, err := api.ListMembers(client, orgID)
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

		if err := api.RemoveMember(client, orgID, memberID); err != nil {
			return err
		}

		fmt.Printf("Removed %s from the organization.\n", args[0])
		return nil
	},
}

var teamRoleCmd = &cobra.Command{
	Use:   "role <email> <role>",
	Short: "Change a member's role (admin, developer, viewer)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID, err := requireOrgID()
		if err != nil {
			return err
		}

		client := newClient()

		// Find member by email.
		members, err := api.ListMembers(client, orgID)
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

		member, err := api.ChangeRole(client, orgID, memberID, args[1])
		if err != nil {
			return err
		}

		fmt.Printf("Changed %s role to %s.\n", member.Email, member.Role)
		return nil
	},
}

func init() {
	teamInviteCmd.Flags().StringVar(&inviteRole, "role", "developer", "Role for invited member (admin, developer, viewer)")
	teamCmd.AddCommand(teamListCmd)
	teamCmd.AddCommand(teamInviteCmd)
	teamCmd.AddCommand(teamRemoveCmd)
	teamCmd.AddCommand(teamRoleCmd)
	rootCmd.AddCommand(teamCmd)
}
