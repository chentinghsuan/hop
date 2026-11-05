package secret

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ting-hsuanchen/hop/internal/config"
)

// SecretRef wraps the reference info required to fetch a secret.
type SecretRef struct {
	Provider   string
	Item       string
	Field      string
	Version    string
	Vault      string
	Project    string
	Mode       string
	Ciphertext string
	Key        string
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
	Prefetch(ctx context.Context, sec config.SecretConfig)
	GetProvider(name string) (SecretProvider, bool)
	AllProviders() map[string]SecretProvider
}

type cacheEntry struct {
	done chan struct{}
	val  string
	err  error
}

type defaultResolver struct {
	providers   map[string]SecretProvider
	mu          sync.Mutex
	cache       map[SecretRef]*cacheEntry
	prefetchSem chan struct{}
}

// NewResolver creates a resolver with registered providers, in-memory caching, and background prefetch support.
func NewResolver(providers ...SecretProvider) Resolver {
	m := make(map[string]SecretProvider)
	for _, p := range providers {
		m[strings.ToLower(p.Name())] = p
	}
	return &defaultResolver{
		providers:   m,
		cache:       make(map[SecretRef]*cacheEntry),
		prefetchSem: make(chan struct{}, 2), // limit concurrent background CLI processes
	}
}

func (r *defaultResolver) GetProvider(name string) (SecretProvider, bool) {
	p, ok := r.providers[strings.ToLower(name)]
	return p, ok
}

func (r *defaultResolver) AllProviders() map[string]SecretProvider {
	return r.providers
}

func (r *defaultResolver) prepareRef(sec config.SecretConfig) (SecretRef, SecretProvider, error) {
	providerName := strings.ToLower(sec.Provider)
	if providerName == "" {
		providerName = "bitwarden"
	}

	// Normalize provider aliases
	switch providerName {
	case "secret_manager", "gcp_secret_manager", "gcp-secret-manager":
		providerName = "gcp"
	case "kms", "gcp-kms":
		providerName = "gcp_kms"
	}

	p, ok := r.providers[providerName]
	if !ok {
		return SecretRef{}, nil, fmt.Errorf("unsupported secret provider: %q (available: bitwarden, 1password, gcp, gcp_kms, env)", providerName)
	}

	ref := SecretRef{
		Provider:   providerName,
		Item:       sec.Item,
		Field:      sec.Field,
		Version:    sec.Version,
		Vault:      sec.Vault,
		Project:    sec.Project,
		Mode:       sec.Mode,
		Ciphertext: sec.Ciphertext,
		Key:        sec.Key,
	}
	return ref, p, nil
}

// Prefetch asynchronously fetches and caches a secret in the background.
// If the secret is already cached or currently being fetched, Prefetch is a no-op.
func (r *defaultResolver) Prefetch(ctx context.Context, sec config.SecretConfig) {
	if !sec.HasSecret() {
		return
	}

	ref, p, err := r.prepareRef(sec)
	if err != nil {
		return
	}

	r.mu.Lock()
	if _, exists := r.cache[ref]; exists {
		r.mu.Unlock()
		return
	}
	entry := &cacheEntry{done: make(chan struct{})}
	r.cache[ref] = entry
	r.mu.Unlock()

	go func() {
		// Acquire concurrency slot so rapid cursor movements don't spawn too many CLI processes at once
		r.prefetchSem <- struct{}{}
		defer func() { <-r.prefetchSem }()

		fetchCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		val, fetchErr := p.GetSecret(fetchCtx, ref)
		entry.val = val
		entry.err = fetchErr

		if fetchErr != nil || val == "" {
			r.mu.Lock()
			if r.cache[ref] == entry {
				delete(r.cache, ref)
			}
			r.mu.Unlock()
		}
		close(entry.done)
	}()
}

func (r *defaultResolver) Resolve(ctx context.Context, sec config.SecretConfig) (string, error) {
	if !sec.HasSecret() {
		return "", nil // No secret item or ciphertext configured
	}

	ref, p, err := r.prepareRef(sec)
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	if existing, ok := r.cache[ref]; ok {
		r.mu.Unlock()
		select {
		case <-existing.done:
			if existing.err == nil && existing.val != "" {
				return existing.val, nil
			}
			// If background prefetch failed, fall through to retry synchronously
		case <-ctx.Done():
			return "", ctx.Err()
		}
		r.mu.Lock()
	}

	entry := &cacheEntry{done: make(chan struct{})}
	r.cache[ref] = entry
	r.mu.Unlock()

	secretVal, err := p.GetSecret(ctx, ref)
	entry.val = secretVal
	entry.err = err

	if err != nil || secretVal == "" {
		r.mu.Lock()
		if r.cache[ref] == entry {
			delete(r.cache, ref)
		}
		r.mu.Unlock()
	}
	close(entry.done)

	if err != nil {
		targetName := sec.Item
		if targetName == "" {
			targetName = sec.Key
		}
		return "", fmt.Errorf("[%s] failed to retrieve secret for %q: %w", p.Name(), targetName, err)
	}

	return secretVal, nil
}
