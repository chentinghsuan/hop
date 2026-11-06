package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDirName       = "hop"
	LegacyConfigDirName = "env-login"
	ConfigFileName      = "config.yaml"
	EnvConfigVar        = "HOP_CONFIG"
	LegacyEnvConfigVar  = "ENV_LOGIN_CONFIG"
)

// GetDefaultConfigDir returns standard user config directory for the current OS.
// On Windows: %APPDATA%\hop (or ~/.config/hop if APPDATA is unset)
// On Linux/macOS: ~/.config/hop (respects $XDG_CONFIG_HOME if set)
func GetDefaultConfigDir() (string, error) {
	if runtime.GOOS == "windows" {
		if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
			return filepath.Join(configDir, ConfigDirName), nil
		}
	} else {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, ConfigDirName), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine user home directory: %w", err)
	}
	return filepath.Join(home, ".config", ConfigDirName), nil
}

// GetDefaultConfigPath returns standard user config path for the current OS.
// On Windows: %APPDATA%\hop\config.yaml (or ~/.config/hop/config.yaml if APPDATA is unset)
// On Linux/macOS: ~/.config/hop/config.yaml (respects $XDG_CONFIG_HOME if set)
func GetDefaultConfigPath() (string, error) {
	dir, err := GetDefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
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

	var userConfigs []string

	// 2. Primary default config directory (~/.config/hop on Linux/macOS, %APPDATA%\hop on Windows)
	if defDir, err := GetDefaultConfigDir(); err == nil && defDir != "" {
		userConfigs = append(userConfigs,
			filepath.Join(defDir, "config.yaml"),
			filepath.Join(defDir, "config.yml"),
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

	// 3. Fallback: OS standard config directory (%APPDATA% on Windows, ~/Library/Application Support on macOS for backward compatibility)
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		userConfigs = append(userConfigs,
			filepath.Join(configDir, ConfigDirName, "config.yaml"),
			filepath.Join(configDir, ConfigDirName, "config.yml"),
			filepath.Join(configDir, LegacyConfigDirName, "config.yaml"),
			filepath.Join(configDir, LegacyConfigDirName, "config.yml"),
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
