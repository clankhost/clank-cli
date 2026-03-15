package cmd

import (
	"fmt"
	"os"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/config"
	"github.com/anaremore/clank/apps/cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "clank",
	Short: "CLI for the Clank PaaS platform",
	Long:  "Deploy and manage containerized apps on Clank from your terminal.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it.
		if skipConfigLoad(cmd) {
			return nil
		}

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Guard: commands that need API access require a configured base URL.
		if cfg.BaseURL == "" && needsBaseURL(cmd) {
			return fmt.Errorf(
				"no platform URL configured\n\n" +
					"Set it with:  clank config set base_url https://your-clank-instance.com\n" +
					"Or export:    CLANK_URL=https://your-clank-instance.com")
		}

		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

// skipConfigLoad returns true if the command doesn't need any config.
func skipConfigLoad(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "version", "completion", "__complete", "__completeNoDesc":
		return true
	}
	for p := cmd; p != nil; p = p.Parent() {
		if p.Name() == "skill" {
			return true
		}
	}
	return false
}

// needsBaseURL returns true if the command requires a configured platform URL.
func needsBaseURL(cmd *cobra.Command) bool {
	// Commands that work without a base URL.
	switch cmd.Name() {
	case "init":
		return false
	}
	// All config and skill subcommands work without a base URL.
	for p := cmd; p != nil; p = p.Parent() {
		if p.Name() == "config" || p.Name() == "skill" {
			return false
		}
	}
	return true
}

// newClient returns an API client configured with team context.
func newClient() *api.Client {
	client := api.New(cfg.BaseURL, cfg.Token)
	client.TeamID = cfg.TeamID
	return client
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/clank/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&output.Format, "output", "o", "", "output format: json or table (default table)")
}
