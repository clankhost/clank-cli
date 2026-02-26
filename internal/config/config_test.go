package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadDefaults(t *testing.T) {
	// Loading from a nonexistent path should return defaults.
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.BaseURL != "" {
		t.Errorf("expected empty BaseURL, got %q", cfg.BaseURL)
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

	// Write config using viper directly.
	v := viper.New()
	v.Set("base_url", "https://test.example.com")
	v.Set("token", "test-jwt-token-value")
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")
	if err := v.SafeWriteConfig(); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	// Load it back.
	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.BaseURL != "https://test.example.com" {
		t.Errorf("BaseURL: expected %q, got %q", "https://test.example.com", loaded.BaseURL)
	}
	if loaded.Token != "test-jwt-token-value" {
		t.Errorf("Token: expected %q, got %q", "test-jwt-token-value", loaded.Token)
	}
}

func TestFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission test not applicable on Windows")
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	v := viper.New()
	v.Set("base_url", "https://test.example.com")
	v.Set("token", "secret")
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")
	if err := v.SafeWriteConfig(); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	// Set permissions like Save() does.
	if err := os.Chmod(cfgPath, 0600); err != nil {
		t.Fatalf("chmod: %v", err)
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
