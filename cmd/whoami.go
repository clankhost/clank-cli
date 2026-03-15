package cmd

import (
	"fmt"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		user, err := api.Me(client)
		if err != nil {
			if api.IsUnauthorized(err) {
				return fmt.Errorf("not logged in — run: clank login")
			}
			return fmt.Errorf("checking auth: %w", err)
		}

		if output.IsJSON() {
			return output.JSON(user)
		}

		fmt.Printf("%s\n", user.Email)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
