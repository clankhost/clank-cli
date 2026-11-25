package cmd

import (
	"fmt"
	"strings"

	"github.com/clankhost/clank-cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return fmt.Sprintf("%s...%s", token[:4], token[len(token)-4:])
}

func maskAPIKeyPrefix(value string) string {
	if len(value) > 14 {
		return value[:14] + "..."
	}
	return maskToken(value)
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a config value (or all values if no key given)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Print all config values.
			fmt.Printf("base_url: %s\n", cfg.BaseURL)
			if cfg.Token != "" {
				fmt.Printf("token:    %s\n", maskToken(cfg.Token))
			} else {
				fmt.Println("token:    (not set)")
			}
			return nil
		}

		key := strings.ToLower(args[0])
		switch key {
		case "base_url":
			fmt.Println(cfg.BaseURL)
		case "token":
			if cfg.Token == "" {
				fmt.Println("(not set)")
			} else {
				fmt.Println(maskToken(cfg.Token))
			}
		default:
			return fmt.Errorf("unknown config key: %s (valid keys: base_url, token)", key)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(args[0])
		value := args[1]

		switch key {
		case "base_url":
			if err := config.SaveBaseURL(value); err != nil {
				return err
			}
			fmt.Printf("base_url set to %s\n", value)
		case "token":
			if strings.HasPrefix(value, "clank_") {
				// Allow setting API keys directly
				if err := config.SaveToken(value); err != nil {
					return err
				}
				fmt.Printf("API key set (%s)\n", maskAPIKeyPrefix(value))
			} else {
				return fmt.Errorf("use 'clank login' to set a JWT token, or provide an API key (clank_...)")
			}
		default:
			return fmt.Errorf("unknown config key: %s (valid keys: base_url, token)", key)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}
