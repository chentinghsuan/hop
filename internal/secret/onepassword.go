package secret

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

type opCacheEntry struct {
	done chan struct{}
	val  string
	err  error
}

type OnePasswordProvider struct {
	mu    sync.Mutex
	cache map[string]*opCacheEntry
}

func NewOnePasswordProvider() *OnePasswordProvider {
	return &OnePasswordProvider{
		cache: make(map[string]*opCacheEntry),
	}
}

func (o *OnePasswordProvider) Name() string {
	return "1password"
}

func (o *OnePasswordProvider) Validate(ctx context.Context) error {
	opPath, err := exec.LookPath("op")
	if err != nil {
		return errors.New("1Password CLI ('op') is not installed or not in PATH")
	}

	cmd := exec.CommandContext(ctx, opPath, "whoami")
	if err := cmd.Run(); err != nil {
		return errors.New("1Password CLI is not signed in. Run 'eval $(op signin)' first")
	}
	return nil
}

func (o *OnePasswordProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	opPath, err := exec.LookPath("op")
	if err != nil {
		return "", errors.New("'op' command not found. Install it via 'brew install --cask 1password-cli'")
	}

	field := ref.Field
	if field == "" {
		field = "password"
	}

	var args []string
	if strings.HasPrefix(ref.Item, "op://") {
		// Native secret reference syntax (e.g. op://vault/item/password)
		itemURI := ref.Item
		if field == "username" {
			// If user configured op://vault/item/password, change to op://vault/item/username
			if idx := strings.LastIndex(itemURI, "/"); idx != -1 {
				itemURI = itemURI[:idx] + "/username"
			}
		}
		args = []string{"read", itemURI}
	} else {
		// Item name or ID
		args = []string{"item", "get", ref.Item, "--fields", field, "--reveal"}
		if ref.Vault != "" {
			args = append(args, "--vault", ref.Vault)
		}
	}

	cacheKey := strings.Join(args, " ")

	o.mu.Lock()
	if entry, ok := o.cache[cacheKey]; ok {
		o.mu.Unlock()
		select {
		case <-entry.done:
			return entry.val, entry.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	entry := &opCacheEntry{done: make(chan struct{})}
	o.cache[cacheKey] = entry
	o.mu.Unlock()

	cmd := exec.CommandContext(ctx, opPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr != "" {
			entry.err = fmt.Errorf("%s", errStr)
		} else {
			entry.err = runErr
		}
		o.mu.Lock()
		delete(o.cache, cacheKey)
		o.mu.Unlock()
		close(entry.done)
		return "", entry.err
	}

	entry.val = strings.TrimSpace(stdout.String())
	close(entry.done)

	return entry.val, nil
}
