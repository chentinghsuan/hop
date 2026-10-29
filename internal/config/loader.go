package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDirName       = "hop"
	LegacyConfigDirName = "env-login"
	ConfigFileName      = "config.yaml"
	EnvConfigVar        = "HOP_CONFIG"
	LegacyEnvConfigVar  = "ENV_LOGIN_CONFIG"
)

// GetDefaultConfigPath returns standard user config path for the current OS.
// On Windows: %APPDATA%\hop\config.yaml (or ~/.config/hop/config.yaml if APPDATA is unset)
// On Linux/macOS: ~/.config/hop/config.yaml
func GetDefaultConfigPath() (string, error) {
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, ConfigDirName, ConfigFileName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine user home directory: %w", err)
	}
	return filepath.Join(home, ".config", ConfigDirName, ConfigFileName), nil
}

// FindConfigFile checks multiple standard paths for a valid config file.
func FindConfigFile(customPath string) (string, error) {
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath, nil
		}
		return "", fmt.Errorf("specified config file does not exist: %s", customPath)
	}

	// 1. Check environment variable (HOP_CONFIG or legacy ENV_LOGIN_CONFIG)
	if envPath := os.Getenv(EnvConfigVar); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}
	if envPath := os.Getenv(LegacyEnvConfigVar); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	// 2. Check standard OS config directories (%APPDATA% on Windows, ~/.config on Linux/macOS)
	var userConfigs []string
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		userConfigs = append(userConfigs,
			filepath.Join(configDir, ConfigDirName, "config.yaml"),
			filepath.Join(configDir, ConfigDirName, "config.yml"),
			filepath.Join(configDir, LegacyConfigDirName, "config.yaml"),
			filepath.Join(configDir, LegacyConfigDirName, "config.yml"),
		)
	}

	// Also check ~/.config on all platforms (common for developers on Windows with Git Bash / WSL)
	home, err := os.UserHomeDir()
	if err == nil {
		userConfigs = append(userConfigs,
			filepath.Join(home, ".config", ConfigDirName, "config.yaml"),
			filepath.Join(home, ".config", ConfigDirName, "config.yml"),
			filepath.Join(home, ".config", LegacyConfigDirName, "config.yaml"),
			filepath.Join(home, ".config", LegacyConfigDirName, "config.yml"),
		)
	}

	for _, p := range userConfigs {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// 3. Check current directory ./config.yaml and ./config.yml
	localConfigs := []string{
		"config.yaml",
		"config.yml",
	}
	for _, p := range localConfigs {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	defaultPath, _ := GetDefaultConfigPath()
	return "", fmt.Errorf("no config file found. Expected at %s (run 'hop init' to create one)", defaultPath)
}

// LoadConfig loads and unmarshals the configuration.
func LoadConfig(customPath string) (*Config, string, error) {
	path, err := FindConfigFile(customPath)
	if err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read config file at %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, "", fmt.Errorf("failed to parse yaml config at %s: %w", path, err)
	}

	return &cfg, path, nil
}
