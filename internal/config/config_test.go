package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Loading from a nonexistent path should return defaults.
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.BaseURL != DefaultBaseURL {
		t.Errorf("expected default BaseURL %q, got %q", DefaultBaseURL, cfg.BaseURL)
	}
	if cfg.Token != "" {
		t.Errorf("expected empty token, got %q", cfg.Token)
	}
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("CLANK_URL", "https://custom.example.com")
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.BaseURL != "https://custom.example.com" {
		t.Errorf("expected CLANK_URL override, got %q", cfg.BaseURL)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	original := &Config{
		BaseURL: "https://test.example.com",
		Token:   "test-jwt-token-value",
		TeamID:  "team-123",
	}
	if err := SaveTo(cfgPath, original); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	// Ensure no temp files were left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.yaml" {
		t.Fatalf("unexpected files in config dir: %v", entries)
	}

	// Load it back.
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.BaseURL != original.BaseURL {
		t.Errorf("BaseURL: expected %q, got %q", original.BaseURL, loaded.BaseURL)
	}
	if loaded.Token != original.Token {
		t.Errorf("Token: expected %q, got %q", original.Token, loaded.Token)
	}
	if loaded.TeamID != original.TeamID {
		t.Errorf("TeamID: expected %q, got %q", original.TeamID, loaded.TeamID)
	}
}

func TestSaveHonorsOverridePath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "custom.yaml")

	SetConfigPath(cfgPath)
	t.Cleanup(func() { SetConfigPath("") })

	if err := SaveBaseURL("https://override.example.com"); err != nil {
		t.Fatalf("SaveBaseURL failed: %v", err)
	}
	if err := SaveToken("clank_override_token_value"); err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}
	if err := SaveTeamID("team-override"); err != nil {
		t.Fatalf("SaveTeamID failed: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.BaseURL != "https://override.example.com" || loaded.Token != "clank_override_token_value" || loaded.TeamID != "team-override" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission test not applicable on Windows")
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	if err := SaveTo(cfgPath, &Config{
		BaseURL: "https://test.example.com",
		Token:   "secret",
	}); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}
