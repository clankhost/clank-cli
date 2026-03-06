package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

const (
	DefaultBaseURL = ""
	appName        = "clank"
)

// Config holds the CLI configuration.
type Config struct {
	BaseURL string `mapstructure:"base_url" yaml:"base_url"`
	Token   string `mapstructure:"token" yaml:"token"`
	TeamID string `mapstructure:"team_id" yaml:"team_id"`
}

// configDir returns the platform-appropriate config directory.
func configDir() (string, error) {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("cannot determine home directory: %w", err)
			}
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, appName), nil
	}

	// Unix: respect XDG_CONFIG_HOME, default to ~/.config
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, appName), nil
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads config from disk (or defaults). If overridePath is non-empty,
// it is used instead of the default location.
func Load(overridePath string) (*Config, error) {
	v := viper.New()
	v.SetDefault("base_url", DefaultBaseURL)
	_ = v.BindEnv("base_url", "CLANK_URL")
	v.SetDefault("token", "")
	v.SetDefault("team_id", "")

	if overridePath != "" {
		v.SetConfigFile(overridePath)
	} else {
		dir, err := configDir()
		if err != nil {
			return nil, err
		}
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(dir)
	}

	// Read config; missing file is not an error (we use defaults).
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// If the file simply doesn't exist yet, that's fine.
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading config: %w", err)
			}
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Migration: if team_id is empty but old org_id key exists, adopt it.
	if cfg.TeamID == "" {
		if orgID := v.GetString("org_id"); orgID != "" {
			cfg.TeamID = orgID
		}
	}

	return cfg, nil
}

// Save writes the config to disk with restrictive permissions.
func Save(cfg *Config) error {
	cfgPath, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	v := viper.New()
	v.Set("base_url", cfg.BaseURL)
	v.Set("token", cfg.Token)
	v.Set("team_id", cfg.TeamID)
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")

	if err := v.WriteConfig(); err != nil {
		// WriteConfig fails if file doesn't exist; use SafeWriteConfig first time.
		if err := v.SafeWriteConfig(); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
	}

	// Restrict permissions on Unix (no-op on Windows).
	if runtime.GOOS != "windows" {
		if err := os.Chmod(cfgPath, 0600); err != nil {
			return fmt.Errorf("setting config permissions: %w", err)
		}
	}

	return nil
}

// SaveToken is a convenience method: load config, update token, save.
func SaveToken(token string) error {
	cfg, err := Load("")
	if err != nil {
		return err
	}
	cfg.Token = token
	return Save(cfg)
}

// SaveBaseURL is a convenience method: load config, update base_url, save.
func SaveBaseURL(baseURL string) error {
	cfg, err := Load("")
	if err != nil {
		return err
	}
	cfg.BaseURL = baseURL
	return Save(cfg)
}

// SaveTeamID is a convenience method: load config, update team_id, save.
func SaveTeamID(teamID string) error {
	cfg, err := Load("")
	if err != nil {
		return err
	}
	cfg.TeamID = teamID
	return Save(cfg)
}
