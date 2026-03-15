package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anaremore/clank/apps/cli/internal/skill"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the Clank Claude Code skill",
	Long:  "Install or update the /clank skill for Claude Code, enabling AI-assisted platform management.",
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the /clank skill for Claude Code",
	Long: `Install the Clank management skill to your Claude Code skills directory.

This writes the /clank skill to ~/.claude/skills/clank/SKILL.md so it's
available across all your projects. After installing, you can use /clank
in Claude Code to manage your Clank platform.

Run this again after updating the CLI to get the latest skill version.`,
	Example: `  clank skill install
  clank skill install --project    # Install to current project only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectScope, _ := cmd.Flags().GetBool("project")

		var dir string
		if projectScope {
			// Install to .claude/skills/clank/ in current directory.
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			dir = filepath.Join(cwd, ".claude", "skills", "clank")
		} else {
			// Install to ~/.claude/skills/clank/ (global).
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting home directory: %w", err)
			}
			dir = filepath.Join(home, ".claude", "skills", "clank")
		}

		dest := filepath.Join(dir, "SKILL.md")

		// Create directory if needed.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}

		if err := os.WriteFile(dest, []byte(skill.Content), 0o644); err != nil {
			return fmt.Errorf("writing skill file: %w", err)
		}

		scope := "globally"
		if projectScope {
			scope = "for this project"
		}
		fmt.Printf("Installed /clank skill %s\n", scope)
		fmt.Printf("  → %s\n", dest)
		fmt.Println()
		fmt.Println("Use /clank in Claude Code to manage your Clank platform.")
		return nil
	},
}

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the /clank skill from Claude Code",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectScope, _ := cmd.Flags().GetBool("project")

		var dir string
		if projectScope {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			dir = filepath.Join(cwd, ".claude", "skills", "clank")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting home directory: %w", err)
			}
			dir = filepath.Join(home, ".claude", "skills", "clank")
		}

		dest := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			fmt.Println("Skill not installed — nothing to remove.")
			return nil
		}

		if err := os.Remove(dest); err != nil {
			return fmt.Errorf("removing skill file: %w", err)
		}

		// Try to clean up empty directory.
		_ = os.Remove(dir)

		fmt.Println("Removed /clank skill.")
		return nil
	},
}

func init() {
	skillInstallCmd.Flags().Bool("project", false, "install to current project instead of globally")
	skillUninstallCmd.Flags().Bool("project", false, "remove from current project instead of globally")

	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillUninstallCmd)
	rootCmd.AddCommand(skillCmd)
}
