package connector

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/secret"
)

// Connector defines how a specific service type (gcp_vm, mysql, mongo, etc.) is connected.
type Connector interface {
	// Type returns the canonical type name, e.g., "gcp", "mysql", "mongo"
	Type() string
	// Aliases returns alternative type names, e.g., ["gcp_vm", "vm"]
	Aliases() []string
	// ValidateDependencies checks if required local tools (e.g., mysql, gcloud, mongosh) exist
	ValidateDependencies() error
	// Connect establishes the interactive connection
	Connect(ctx context.Context, target config.ResolvedTarget, resolver secret.Resolver) error
}

// Registry maintains all available connectors.
type Registry struct {
	connectors map[string]Connector
}

// NewRegistry initializes a registry with connectors.
func NewRegistry(conns ...Connector) *Registry {
	r := &Registry{
		connectors: make(map[string]Connector),
	}
	for _, c := range conns {
		r.Register(c)
	}
	return r
}

// Register registers a connector and its aliases.
func (r *Registry) Register(c Connector) {
	primary := strings.ToLower(c.Type())
	r.connectors[primary] = c
	for _, alias := range c.Aliases() {
		r.connectors[strings.ToLower(alias)] = c
	}
}

// Get finds a connector by type or alias.
func (r *Registry) Get(serviceType string) (Connector, bool) {
	c, ok := r.connectors[strings.ToLower(serviceType)]
	return c, ok
}

// AllUnique returns a deterministic list of unique connectors sorted by Type.
func (r *Registry) AllUnique() []Connector {
	seen := make(map[string]bool)
	var list []Connector
	for _, c := range r.connectors {
		if !seen[c.Type()] {
			seen[c.Type()] = true
			list = append(list, c)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Type() < list[j].Type()
	})
	return list
}

// FormatSupportedTypes returns a user-friendly string of supported service types.
func (r *Registry) FormatSupportedTypes() string {
	unique := r.AllUnique()
	var names []string
	for _, c := range unique {
		names = append(names, c.Type())
	}
	return strings.Join(names, ", ")
}

// ResolveCredentials retrieves the username and password for a target from its SecretConfig or fallback properties.
func ResolveCredentials(ctx context.Context, serviceLabel string, target config.ResolvedTarget, resolver secret.Resolver, defaultUser string) (string, string, error) {
	user := GetString(target.Properties, "user", "")
	var password string

	if target.Secret.HasSecret() {
		color.HiBlack("Fetching %s credentials from [%s]...", serviceLabel, target.Secret.Provider)
		var err error
		password, err = resolver.Resolve(ctx, target.Secret)
		if err != nil {
			return "", "", err
		}
		if password == "" {
			return "", "", fmt.Errorf("retrieved password from [%s] is empty", target.Secret.Provider)
		}

		// If user is not explicitly configured, attempt to fetch the username field
		if user == "" {
			userSec := target.Secret
			userSec.Field = "username"
			if u, err := resolver.Resolve(ctx, userSec); err == nil && u != "" {
				user = u
			}
		}
	} else if pwd := GetString(target.Properties, "password", ""); pwd != "" {
		password = pwd
	}

	if user == "" {
		user = defaultUser
	}

	return user, password, nil
}

// PrefetchCredentials asynchronously warms up the secret cache for a target
// while the user is browsing the interactive menu.
func PrefetchCredentials(ctx context.Context, target config.ResolvedTarget, resolver secret.Resolver) {
	if resolver == nil || !target.Secret.HasSecret() {
		return
	}
	resolver.Prefetch(ctx, target.Secret)

	user := GetString(target.Properties, "user", "")
	if user == "" {
		userSec := target.Secret
		userSec.Field = "username"
		resolver.Prefetch(ctx, userSec)
	}
}

// Helper to convert any to string
func GetString(props map[string]any, key, fallback string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

// Helper to convert any to []string
func GetStringSlice(props map[string]any, key string) []string {
	raw, ok := props[key]
	if !ok {
		return nil
	}
	switch slice := raw.(type) {
	case []string:
		return slice
	case []any:
		var out []string
		for _, item := range slice {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// Helper to convert any to int
func GetInt(props map[string]any, key string, fallback int) int {
	if v, ok := props[key]; ok {
		switch num := v.(type) {
		case int:
			return num
		case int64:
			return int(num)
		case float64:
			return int(num)
		}
	}
	return fallback
}

// Helper to convert any to bool
func GetBool(props map[string]any, key string, fallback bool) bool {
	if v, ok := props[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return fallback
}
