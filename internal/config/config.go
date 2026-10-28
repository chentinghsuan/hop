package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// SecretConfig defines how credentials are dynamically retrieved from a secret provider.
type SecretConfig struct {
	Provider string `yaml:"provider" json:"provider"`
	Item     string `yaml:"item" json:"item"`
	Field    string `yaml:"field" json:"field"`   // e.g., "password" (default)
	Version  string `yaml:"version" json:"version"` // e.g., "latest" for GCP
	Vault    string `yaml:"vault" json:"vault"`     // Optional for 1Password
}

// Config is the root configuration structure.
type Config struct {
	Version               string                 `yaml:"version" json:"version"`
	DefaultSecretProvider string                 `yaml:"default_secret_provider" json:"default_secret_provider"`
	Environments          map[string]yaml.Node   `yaml:"environments" json:"environments"`
}

// ResolvedTarget represents a fully-resolved connection target ready for execution.
type ResolvedTarget struct {
	EnvKey      string         `json:"env_key"`      // e.g., "prod"
	EnvName     string         `json:"env_name"`     // e.g., "Production"
	ServiceType string         `json:"service_type"` // e.g., "mongo", "mysql", "gcp"
	InstanceKey string         `json:"instance_key"` // e.g., "user"
	DisplayName string         `json:"display_name"` // e.g., "[prod] mongo > user"
	Properties  map[string]any `json:"properties"`
	Secret      SecretConfig   `json:"secret"`
}

// ResolveTargets flattens and parses all environment targets.
func (c *Config) ResolveTargets() ([]ResolvedTarget, error) {
	var targets []ResolvedTarget

	defaultProvider := c.DefaultSecretProvider
	if defaultProvider == "" {
		defaultProvider = "bitwarden"
	}

	for envKey, envNode := range c.Environments {
		var rawEnv map[string]any
		if err := envNode.Decode(&rawEnv); err != nil {
			return nil, fmt.Errorf("failed to parse environment %q: %w", envKey, err)
		}

		envName := envKey
		if n, ok := rawEnv["name"].(string); ok && n != "" {
			envName = n
		}

		// Check if user used legacy "services" wrapper
		servicesMap := rawEnv
		if s, ok := rawEnv["services"].(map[string]any); ok {
			servicesMap = s
		}

		for serviceType, serviceVal := range servicesMap {
			if serviceType == "name" {
				continue
			}

			svcMap, ok := serviceVal.(map[string]any)
			if !ok {
				continue
			}

			// Check if user nested under "instances"
			instancesMap := svcMap
			var defaults map[string]any
			if d, ok := svcMap["defaults"].(map[string]any); ok {
				defaults = d
			}
			if inst, ok := svcMap["instances"].(map[string]any); ok {
				instancesMap = inst
			}

			for instanceKey, instVal := range instancesMap {
				if instanceKey == "defaults" {
					continue
				}

				instMap, ok := instVal.(map[string]any)
				if !ok {
					instMap = make(map[string]any)
				}

				// Merge properties (defaults + instance properties)
				resolvedProps := make(map[string]any)
				for k, v := range defaults {
					resolvedProps[k] = v
				}
				for k, v := range instMap {
					resolvedProps[k] = v
				}

				// Extract secret
				secretCfg := extractSecretConfig(resolvedProps, defaultProvider)

				displayName := fmt.Sprintf("[%s] %s > %s", envKey, serviceType, instanceKey)
				if desc, ok := resolvedProps["description"].(string); ok && desc != "" {
					displayName = fmt.Sprintf("[%s] %s > %s (%s)", envKey, serviceType, instanceKey, desc)
				}

				targets = append(targets, ResolvedTarget{
					EnvKey:      envKey,
					EnvName:     envName,
					ServiceType: serviceType,
					InstanceKey: instanceKey,
					DisplayName: displayName,
					Properties:  resolvedProps,
					Secret:      secretCfg,
				})
			}
		}
	}

	return targets, nil
}

func extractSecretConfig(resolvedProps map[string]any, defaultProvider string) SecretConfig {
	sec := SecretConfig{
		Provider: defaultProvider,
		Field:    "password",
		Version:  "latest",
	}

	rawSec, ok := resolvedProps["secret"]
	if !ok {
		return sec
	}

	secMap, ok := rawSec.(map[string]any)
	if !ok {
		return sec
	}

	if p, ok := secMap["provider"].(string); ok && p != "" {
		sec.Provider = p
	}
	if item, ok := secMap["item"].(string); ok && item != "" {
		sec.Item = item
	}
	if f, ok := secMap["field"].(string); ok && f != "" {
		sec.Field = f
	}
	if v, ok := secMap["version"].(string); ok && v != "" {
		sec.Version = v
	}
	if vault, ok := secMap["vault"].(string); ok && vault != "" {
		sec.Vault = vault
	}

	return sec
}
