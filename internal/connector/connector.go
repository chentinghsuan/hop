package connector

import (
	"context"
	"strings"

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

// AllUnique returns a list of unique connectors.
func (r *Registry) AllUnique() []Connector {
	seen := make(map[string]bool)
	var list []Connector
	for _, c := range r.connectors {
		if !seen[c.Type()] {
			seen[c.Type()] = true
			list = append(list, c)
		}
	}
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

// Helper to convert any to string
func GetString(props map[string]any, key, fallback string) string {
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
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
