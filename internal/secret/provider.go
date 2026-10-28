package secret

import (
	"context"
	"fmt"
	"strings"

	"github.com/ting-hsuanchen/hop/internal/config"
)

// SecretRef wraps the reference info required to fetch a secret.
type SecretRef struct {
	Provider string
	Item     string
	Field    string
	Version  string
	Vault    string
}

// SecretProvider abstracts different secret vaults (Bitwarden, 1Password, GCP, etc.).
type SecretProvider interface {
	Name() string
	GetSecret(ctx context.Context, ref SecretRef) (string, error)
	Validate(ctx context.Context) error
}

// Resolver resolves secrets for a target using registered providers.
type Resolver interface {
	Resolve(ctx context.Context, sec config.SecretConfig) (string, error)
	GetProvider(name string) (SecretProvider, bool)
	AllProviders() map[string]SecretProvider
}

type defaultResolver struct {
	providers map[string]SecretProvider
}

// NewResolver creates a resolver with registered providers.
func NewResolver(providers ...SecretProvider) Resolver {
	m := make(map[string]SecretProvider)
	for _, p := range providers {
		m[strings.ToLower(p.Name())] = p
	}
	return &defaultResolver{providers: m}
}

func (r *defaultResolver) GetProvider(name string) (SecretProvider, bool) {
	p, ok := r.providers[strings.ToLower(name)]
	return p, ok
}

func (r *defaultResolver) AllProviders() map[string]SecretProvider {
	return r.providers
}

func (r *defaultResolver) Resolve(ctx context.Context, sec config.SecretConfig) (string, error) {
	if sec.Item == "" {
		return "", nil // No secret item configured
	}

	providerName := strings.ToLower(sec.Provider)
	if providerName == "" {
		providerName = "bitwarden"
	}

	p, ok := r.providers[providerName]
	if !ok {
		return "", fmt.Errorf("unsupported secret provider: %q (available: bitwarden, 1password, gcp)", providerName)
	}

	ref := SecretRef{
		Provider: providerName,
		Item:     sec.Item,
		Field:    sec.Field,
		Version:  sec.Version,
		Vault:    sec.Vault,
	}

	secretVal, err := p.GetSecret(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("[%s] failed to retrieve secret for %q: %w", p.Name(), sec.Item, err)
	}

	return secretVal, nil
}
