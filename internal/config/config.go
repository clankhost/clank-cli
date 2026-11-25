package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	DefaultBaseURL = "https://clank.host"
	appName        = "clank"
)

var (
	overridePathMu sync.RWMutex
	overridePath   string
)

// SetConfigPath sets an override config file path (e.g. from --config)
// used by Save and the Save* convenience helpers.
func SetConfigPath(path string) {
	overridePathMu.Lock()
	overridePath = path
	overridePathMu.Unlock()
}

func getConfigPathOverride() string {
	overridePathMu.RLock()
	defer overridePathMu.RUnlock()
	return overridePath
}

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

// Save writes the config to disk atomically with restrictive permissions (0600),
// honoring any path configured via SetConfigPath (e.g. --config).
func Save(cfg *Config) error {
	return SaveTo(getConfigPathOverride(), cfg)
}

// SaveTo writes the config atomically to overridePath (or the default config path
// if overridePath is empty) via a 0600 temp file + rename.
func SaveTo(targetPath string, cfg *Config) error {
	cfgPath := targetPath
	if cfgPath == "" {
		var err error
		cfgPath, err = ConfigPath()
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// os.CreateTemp creates the file with mode 0600 (owner read/write only),
	// avoiding any TOCTOU window where secrets are world-readable.
	tmpFile, err := os.CreateTemp(dir, ".config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("creating temp config file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if runtime.GOOS != "windows" {
		if err := tmpFile.Chmod(0600); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("setting config permissions: %w", err)
		}
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing config: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("syncing config: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp config: %w", err)
	}

	if err := os.Rename(tmpPath, cfgPath); err != nil {
		return fmt.Errorf("replacing config file: %w", err)
	}
	cleanupTmp = false
	return nil
}

// SaveToken is a convenience method: load config, update token, save.
func SaveToken(token string) error {
	path := getConfigPathOverride()
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	cfg.Token = token
	return SaveTo(path, cfg)
}

// SaveBaseURL is a convenience method: load config, update base_url, save.
func SaveBaseURL(baseURL string) error {
	path := getConfigPathOverride()
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	cfg.BaseURL = baseURL
	return SaveTo(path, cfg)
}

// SaveTeamID is a convenience method: load config, update team_id, save.
func SaveTeamID(teamID string) error {
	path := getConfigPathOverride()
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	cfg.TeamID = teamID
	return SaveTo(path, cfg)
}
