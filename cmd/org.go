package cmd

import (
	"fmt"
	"strings"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/config"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Manage organizations",
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations you belong to",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		orgs, err := api.ListOrgs(client)
		if err != nil {
			return err
		}

		if len(orgs) == 0 {
			fmt.Println("No organizations found.")
			return nil
		}

		headers := []string{"ID", "NAME", "SLUG", "ROLE", "CREATED"}
		rows := make([][]string, len(orgs))
		for i, o := range orgs {
			name := o.Name
			if o.ID == cfg.OrgID {
				name += " *"
			}
			rows[i] = []string{
				output.ShortID(o.ID),
				name,
				o.Slug,
				o.Role,
				output.TimeSince(o.CreatedAt),
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var orgCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		org, err := api.CreateOrg(client, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Organization created: %s (id: %s)\n", org.Name, output.ShortID(org.ID))
		return nil
	},
}

var orgSwitchCmd = &cobra.Command{
	Use:   "switch <name-or-id>",
	Short: "Set the active organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		orgs, err := api.ListOrgs(client)
		if err != nil {
			return err
		}

		search := strings.ToLower(args[0])
		var match *api.Organization
		for _, o := range orgs {
			if strings.HasPrefix(o.ID, search) || strings.ToLower(o.Slug) == search || strings.ToLower(o.Name) == search {
				match = &o
				break
			}
		}

		if match == nil {
			return fmt.Errorf("no organization matching %q found", args[0])
		}

		if err := config.SaveOrgID(match.ID); err != nil {
			return err
		}

		fmt.Printf("Switched to organization: %s\n", match.Name)
		return nil
	},
}

var orgCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.OrgID == "" {
			fmt.Println("No active organization set. Run 'clank org switch <name>'.")
			return nil
		}

		client := newClient()
		orgs, err := api.ListOrgs(client)
		if err != nil {
			return err
		}

		for _, o := range orgs {
			if o.ID == cfg.OrgID {
				fmt.Printf("%s (%s) — role: %s\n", o.Name, o.Slug, o.Role)
				return nil
			}
		}

		fmt.Printf("Active org ID: %s (not found in your orgs)\n", output.ShortID(cfg.OrgID))
		return nil
	},
}

func init() {
	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgCreateCmd)
	orgCmd.AddCommand(orgSwitchCmd)
	orgCmd.AddCommand(orgCurrentCmd)
	rootCmd.AddCommand(orgCmd)
}
