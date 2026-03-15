package cmd

import (
	"fmt"

	"github.com/clankhost/clank-cli/internal/api"
	"github.com/clankhost/clank-cli/internal/output"
	"github.com/spf13/cobra"
)

var domainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "Manage custom domains for a service",
}

var domainsListCmd = &cobra.Command{
	Use:   "list <service-id>",
	Short: "List domains for a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		domains, err := api.ListDomains(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(domains)
		}

		if len(domains) == 0 {
			fmt.Println("No domains configured.")
			return nil
		}

		headers := []string{"ID", "DOMAIN", "STATUS", "PRIMARY", "TYPE"}
		rows := make([][]string, len(domains))
		for i, d := range domains {
			primary := ""
			if d.IsPrimary {
				primary = "*"
			}
			dtype := "custom"
			if d.IsGenerated {
				dtype = "auto"
			}
			rows[i] = []string{
				output.ShortID(d.ID),
				d.Domain,
				output.StatusColor(d.Status),
				primary,
				dtype,
			}
		}
		output.Table(headers, rows)
		return nil
	},
}

var domainsAddCmd = &cobra.Command{
	Use:     "add <service-id> <domain>",
	Short:   "Add a custom domain to a service",
	Example: `  clank domains add abc123 app.example.com --primary`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		isPrimary, _ := cmd.Flags().GetBool("primary")

		client := newClient()
		domain, err := api.AddDomain(client, args[0], api.AddDomainRequest{
			Domain:    args[1],
			IsPrimary: isPrimary,
		})
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(domain)
		}

		fmt.Printf("Added domain %s (status: %s)\n", domain.Domain, domain.Status)
		if domain.TxtRecord != nil && domain.VerificationToken != nil {
			fmt.Println()
			fmt.Println("To verify ownership, add this DNS TXT record:")
			fmt.Printf("  Name:  %s\n", *domain.TxtRecord)
			fmt.Printf("  Value: %s\n", *domain.VerificationToken)
			fmt.Println()
			fmt.Printf("Then run: clank domains recheck %s\n", domain.ID)
		}
		return nil
	},
}

var domainsRemoveCmd = &cobra.Command{
	Use:   "remove <domain-id>",
	Short: "Remove a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		if err := api.RemoveDomain(client, args[0]); err != nil {
			return err
		}
		fmt.Println("Domain removed.")
		return nil
	},
}

var domainsRecheckCmd = &cobra.Command{
	Use:   "recheck <domain-id>",
	Short: "Re-check DNS verification for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()
		domain, err := api.RecheckDomain(client, args[0])
		if err != nil {
			return err
		}

		if output.IsJSON() {
			return output.JSON(domain)
		}

		fmt.Printf("Domain %s — status: %s\n", domain.Domain, output.StatusColor(domain.Status))
		if domain.ErrorMessage != nil {
			fmt.Printf("Error: %s\n", *domain.ErrorMessage)
		}
		return nil
	},
}

func init() {
	domainsAddCmd.Flags().Bool("primary", false, "set as the primary domain")

	domainsCmd.AddCommand(domainsListCmd)
	domainsCmd.AddCommand(domainsAddCmd)
	domainsCmd.AddCommand(domainsRemoveCmd)
	domainsCmd.AddCommand(domainsRecheckCmd)
	rootCmd.AddCommand(domainsCmd)
}
