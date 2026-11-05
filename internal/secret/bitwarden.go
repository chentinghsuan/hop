package secret

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

func bwUnlockHint() string {
	if runtime.GOOS == "windows" {
		return "$env:BW_SESSION = (bw unlock --raw)"
	}
	return "export BW_SESSION=$(bw unlock --raw)"
}

type bwItemData struct {
	Login struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"login"`
	Notes  string `json:"notes"`
	Fields []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"fields"`
}

type bwItemEntry struct {
	done chan struct{}
	data bwItemData
	err  error
}

type BitwardenProvider struct {
	mu    sync.Mutex
	items map[string]*bwItemEntry
}

func NewBitwardenProvider() *BitwardenProvider {
	return &BitwardenProvider{
		items: make(map[string]*bwItemEntry),
	}
}

func (b *BitwardenProvider) Name() string {
	return "bitwarden"
}

func (b *BitwardenProvider) Validate(ctx context.Context) error {
	bwPath, err := exec.LookPath("bw")
	if err != nil {
		return errors.New("bitwarden CLI ('bw') is not installed or not in PATH")
	}

	// Check status
	cmd := exec.CommandContext(ctx, bwPath, "status")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to check Bitwarden CLI status: %w", err)
	}

	var status struct {
		Status string `json:"status"` // "unauthenticated", "locked", "unlocked"
	}
	if err := json.Unmarshal(stdout.Bytes(), &status); err == nil {
		if status.Status == "unauthenticated" {
			return errors.New("Bitwarden is not logged in. Please run 'bw login' first")
		}
		if status.Status == "locked" && os.Getenv("BW_SESSION") == "" {
			return fmt.Errorf("Bitwarden vault is locked. Please run: %s", bwUnlockHint())
		}
	}

	return nil
}

func (b *BitwardenProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	bwPath, err := exec.LookPath("bw")
	if err != nil {
		return "", errors.New("'bw' command not found. Install it via 'brew install bitwarden-cli' or on Windows 'winget install Bitwarden.CLI'")
	}

	field := ref.Field
	if field == "" {
		field = "password"
	}

	if os.Getenv("BW_SESSION") == "" {
		return "", fmt.Errorf("Bitwarden vault is locked (BW_SESSION is not set in this terminal).\nPlease run: %s", bwUnlockHint())
	}

	itemData, err := b.getItem(ctx, bwPath, ref.Item)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(field) {
	case "password":
		if itemData.Login.Password != "" {
			return itemData.Login.Password, nil
		}
		for _, f := range itemData.Fields {
			if strings.EqualFold(f.Name, "password") && f.Value != "" {
				return f.Value, nil
			}
		}
		return "", fmt.Errorf("item %q found in Bitwarden, but its Password field is empty", ref.Item)
	case "username":
		if itemData.Login.Username != "" {
			return itemData.Login.Username, nil
		}
		// Fallback to custom fields
		for _, f := range itemData.Fields {
			if strings.EqualFold(f.Name, "username") || strings.EqualFold(f.Name, "user") {
				if f.Value != "" {
					return f.Value, nil
				}
			}
		}
		return "", fmt.Errorf("username not found in Bitwarden item %q", ref.Item)
	case "notes":
		return itemData.Notes, nil
	default:
		// Check custom fields
		for _, f := range itemData.Fields {
			if strings.EqualFold(f.Name, field) {
				return f.Value, nil
			}
		}
	}

	return "", fmt.Errorf("field %q not found in Bitwarden item %q", field, ref.Item)
}

func (b *BitwardenProvider) getItem(ctx context.Context, bwPath, itemKey string) (bwItemData, error) {
	b.mu.Lock()
	if existing, ok := b.items[itemKey]; ok {
		b.mu.Unlock()
		select {
		case <-existing.done:
			if existing.err == nil {
				return existing.data, nil
			}
		case <-ctx.Done():
			return bwItemData{}, ctx.Err()
		}
		b.mu.Lock()
	}

	entry := &bwItemEntry{done: make(chan struct{})}
	b.items[itemKey] = entry
	b.mu.Unlock()

	stdoutStr, err := runBW(ctx, bwPath, "get", "item", itemKey)
	if err == nil {
		if unmarshalErr := json.Unmarshal([]byte(stdoutStr), &entry.data); unmarshalErr != nil {
			err = fmt.Errorf("failed to parse Bitwarden item JSON: %w", unmarshalErr)
		}
	}
	entry.err = err

	if err != nil {
		b.mu.Lock()
		if b.items[itemKey] == entry {
			delete(b.items, itemKey)
		}
		b.mu.Unlock()
	}
	close(entry.done)

	return entry.data, entry.err
}

func runBW(ctx context.Context, bwPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, bwPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	stderrStr := strings.TrimSpace(stderr.String())
	stdoutStr := strings.TrimSpace(stdout.String())

	if strings.Contains(stderrStr, "Vault is locked") || strings.Contains(stderrStr, "Session key is invalid") || strings.Contains(stderrStr, "Master password") {
		return "", fmt.Errorf("Bitwarden vault is locked or session expired.\nPlease run: %s", bwUnlockHint())
	}

	if err != nil {
		if stderrStr != "" {
			return "", fmt.Errorf("%s", stderrStr)
		}
		return "", err
	}

	return stdoutStr, nil
}
