package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SecretConfig defines how credentials are dynamically retrieved from a secret provider.
type SecretConfig struct {
	Provider   string `yaml:"provider" json:"provider"`
	Item       string `yaml:"item" json:"item"`
	Field      string `yaml:"field" json:"field"`           // e.g., "password" (default)
	Version    string `yaml:"version" json:"version"`       // e.g., "latest" for GCP
	Vault      string `yaml:"vault" json:"vault"`           // Optional for 1Password
	Project    string `yaml:"project" json:"project"`       // GCP Project ID for Secret Manager / KMS
	Mode       string `yaml:"mode" json:"mode"`             // "sdk" (default) or "gcloud"
	Ciphertext string `yaml:"ciphertext" json:"ciphertext"` // Encrypted base64 ciphertext for KMS
	Key        string `yaml:"key" json:"key"`               // KMS CryptoKey resource path
}

// HasSecret returns true if a secret item or KMS ciphertext is configured.
func (s SecretConfig) HasSecret() bool {
	return s.Item != "" || s.Ciphertext != ""
}

// Summary returns a concise human-readable description for CLI tables.
func (s SecretConfig) Summary() string {
	if !s.HasSecret() {
		return "-"
	}
	if s.Item != "" {
		return fmt.Sprintf("%s (%s)", s.Provider, s.Item)
	}
	if s.Key != "" {
		shortKey := s.Key
		if idx := strings.LastIndex(s.Key, "/"); idx != -1 && idx+1 < len(s.Key) {
			shortKey = s.Key[idx+1:]
		}
		return fmt.Sprintf("%s (key:%s)", s.Provider, shortKey)
	}
	return fmt.Sprintf("%s (ciphertext)", s.Provider)
}

// Config is the root configuration structure.
type Config struct {
	Version               string       `yaml:"version" json:"version"`
	DefaultSecretProvider string       `yaml:"default_secret_provider" json:"default_secret_provider"`
	Environments          yaml.Node    `yaml:"environments" json:"environments"`
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

// ResolveTargets flattens and parses all environment targets in the exact order defined in config.yaml.
func (c *Config) ResolveTargets() ([]ResolvedTarget, error) {
	var targets []ResolvedTarget

	defaultProvider := c.DefaultSecretProvider
	if defaultProvider == "" {
		defaultProvider = "bitwarden"
	}

	envNode := &c.Environments
	if envNode.Kind == yaml.DocumentNode && len(envNode.Content) > 0 {
		envNode = envNode.Content[0]
	}
	if envNode.Kind != yaml.MappingNode {
		return nil, nil // empty or not configured
	}

	for i := 0; i < len(envNode.Content); i += 2 {
		envKey := envNode.Content[i].Value
		envValNode := envNode.Content[i+1]

		var rawEnv map[string]any
		if err := envValNode.Decode(&rawEnv); err != nil {
			return nil, fmt.Errorf("failed to parse environment %q: %w", envKey, err)
		}

		envName := envKey
		if n, ok := rawEnv["name"].(string); ok && n != "" {
			envName = n
		}

		servicesValNode := envValNode
		if servicesValNode.Kind == yaml.DocumentNode && len(servicesValNode.Content) > 0 {
			servicesValNode = servicesValNode.Content[0]
		}
		if servicesValNode.Kind != yaml.MappingNode {
			continue
		}

		// Check if user used legacy "services" wrapper
		for j := 0; j < len(servicesValNode.Content); j += 2 {
			if servicesValNode.Content[j].Value == "services" {
				servicesValNode = servicesValNode.Content[j+1]
				if servicesValNode.Kind == yaml.DocumentNode && len(servicesValNode.Content) > 0 {
					servicesValNode = servicesValNode.Content[0]
				}
				break
			}
		}

		for j := 0; j < len(servicesValNode.Content); j += 2 {
			serviceType := servicesValNode.Content[j].Value
			if serviceType == "name" {
				continue
			}
			svcValNode := servicesValNode.Content[j+1]
			if svcValNode.Kind == yaml.DocumentNode && len(svcValNode.Content) > 0 {
				svcValNode = svcValNode.Content[0]
			}
			if svcValNode.Kind != yaml.MappingNode {
				continue
			}

			// Check defaults and instances
			var defaults map[string]any
			instancesValNode := svcValNode

			for k := 0; k < len(svcValNode.Content); k += 2 {
				kName := svcValNode.Content[k].Value
				if kName == "defaults" {
					_ = svcValNode.Content[k+1].Decode(&defaults)
				}
				if kName == "instances" {
					instancesValNode = svcValNode.Content[k+1]
					if instancesValNode.Kind == yaml.DocumentNode && len(instancesValNode.Content) > 0 {
						instancesValNode = instancesValNode.Content[0]
					}
				}
			}

			if instancesValNode.Kind != yaml.MappingNode {
				continue
			}

			for k := 0; k < len(instancesValNode.Content); k += 2 {
				instanceKey := instancesValNode.Content[k].Value
				if instanceKey == "defaults" || instanceKey == "instances" {
					continue
				}

				instNode := instancesValNode.Content[k+1]
				var instMap map[string]any
				_ = instNode.Decode(&instMap)
				if instMap == nil {
					instMap = make(map[string]any)
				}

				// Merge properties (defaults + instance properties, with sub-map merge for nested blocks like 'secret')
				resolvedProps := mergeProperties(defaults, instMap)

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

func toStringMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	if m, ok := v.(map[any]any); ok {
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	}
	return nil, false
}

func mergeProperties(defaults, overrides map[string]any) map[string]any {
	resolved := make(map[string]any, len(defaults)+len(overrides))
	for k, v := range defaults {
		if subMap, ok := toStringMap(v); ok {
			copied := make(map[string]any, len(subMap))
			for sk, sv := range subMap {
				copied[sk] = sv
			}
			resolved[k] = copied
		} else {
			resolved[k] = v
		}
	}
	for k, v := range overrides {
		if overSub, ok := toStringMap(v); ok {
			if baseSub, exists := resolved[k].(map[string]any); exists {
				for sk, sv := range overSub {
					baseSub[sk] = sv
				}
				continue
			}
			resolved[k] = overSub
			continue
		}
		resolved[k] = v
	}
	return resolved
}

func extractSecretConfig(resolvedProps map[string]any, defaultProvider string) SecretConfig {
	sec := SecretConfig{
		Provider: defaultProvider,
	}

	rawSec, ok := resolvedProps["secret"]
	if !ok {
		return sec
	}

	secMap, ok := toStringMap(rawSec)
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
	if project, ok := secMap["project"].(string); ok && project != "" {
		sec.Project = project
	}
	if mode, ok := secMap["mode"].(string); ok && mode != "" {
		sec.Mode = mode
	}
	if ct, ok := secMap["ciphertext"].(string); ok && ct != "" {
		sec.Ciphertext = ct
	}
	if key, ok := secMap["key"].(string); ok && key != "" {
		sec.Key = key
	}

	return sec
}
