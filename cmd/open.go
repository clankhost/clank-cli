package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open <service-id>",
	Short: "Open a service URL in the default browser",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpen,
}

func runOpen(cmd *cobra.Command, args []string) error {
	serviceID := args[0]
	client := newClient()

	// Get domains for the service.
	domains, err := api.ListDomains(client, serviceID)
	if err != nil {
		return fmt.Errorf("listing domains: %w", err)
	}

	if len(domains) == 0 {
		return fmt.Errorf("no domains configured for this service")
	}

	// Prefer primary domain, otherwise use the first one.
	domain := domains[0].Domain
	for _, d := range domains {
		if d.IsPrimary {
			domain = d.Domain
			break
		}
	}

	url := "https://" + domain
	fmt.Printf("Opening %s\n", url)

	if err := browser.OpenURL(url); err != nil {
		// If browser open fails, just print the URL.
		fmt.Printf("Could not open browser. Visit: %s\n", url)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(openCmd)
}
