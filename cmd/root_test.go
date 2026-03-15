package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestSkipConfigLoad_BuiltinCommands(t *testing.T) {
	// Test commands that should skip config loading.
	skipNames := []string{"version", "completion"}
	for _, name := range skipNames {
		for _, c := range rootCmd.Commands() {
			if c.Name() == name {
				if !skipConfigLoad(c) {
					t.Errorf("skipConfigLoad(%q) = false, want true", name)
				}
			}
		}
	}

	// Test internal cobra completion commands.
	for _, name := range []string{"__complete", "__completeNoDesc"} {
		c := &cobra.Command{Use: name}
		if !skipConfigLoad(c) {
			t.Errorf("skipConfigLoad(%q) = false, want true", name)
		}
	}
}

func TestSkipConfigLoad_RegularCommands(t *testing.T) {
	// Commands that should NOT skip config loading.
	noSkipNames := []string{"deploy", "projects", "services", "env", "restart"}
	for _, name := range noSkipNames {
		for _, c := range rootCmd.Commands() {
			if c.Name() == name {
				if skipConfigLoad(c) {
					t.Errorf("skipConfigLoad(%q) = true, want false", name)
				}
			}
		}
	}
}

func TestSkipConfigLoad_SkillSubcommands(t *testing.T) {
	// Find the skill command.
	var skill *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "skill" {
			skill = c
			break
		}
	}
	if skill == nil {
		t.Fatal("skill command not found")
	}

	// All skill subcommands should skip config.
	for _, sub := range skill.Commands() {
		if !skipConfigLoad(sub) {
			t.Errorf("skipConfigLoad(skill %s) = false, want true", sub.Name())
		}
	}
}

func TestNeedsBaseURL(t *testing.T) {
	// Config subcommands should not need base URL.
	for _, c := range rootCmd.Commands() {
		if c.Name() == "config" {
			for _, sub := range c.Commands() {
				if needsBaseURL(sub) {
					t.Errorf("needsBaseURL(config %s) = true, want false", sub.Name())
				}
			}
		}
	}

	// Deploy should need base URL.
	for _, c := range rootCmd.Commands() {
		if c.Name() == "deploy" {
			if !needsBaseURL(c) {
				t.Error("needsBaseURL(deploy) = false, want true")
			}
		}
	}
}
