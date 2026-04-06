package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push <service-id>",
	Short: "Push the current deployment's image to the registry",
	Long: `Push the active deployment's image to the Clank registry.
Resolves the service's current deployment and pushes it.
Equivalent to: clank deployments push <current-deployment-id>`,
	Args: cobra.ExactArgs(1),
	Example: `  clank push <service-id>
  clank push <service-id> --no-follow`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		svc, err := api.GetService(client, args[0])
		if err != nil {
			return err
		}
		if svc.CurrentDeploymentID == nil {
			return fmt.Errorf("service has no active deployment")
		}
		noFollow, _ := cmd.Flags().GetBool("no-follow")
		return runPush(*svc.CurrentDeploymentID, noFollow)
	},
}

func init() {
	pushCmd.Flags().Bool("no-follow", false, "return immediately without waiting for push to complete")
	rootCmd.AddCommand(pushCmd)
}
