package secret

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type BitwardenProvider struct{}

func NewBitwardenProvider() *BitwardenProvider {
	return &BitwardenProvider{}
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
			return errors.New("Bitwarden vault is locked. Please run: export BW_SESSION=$(bw unlock --raw)")
		}
	}

	return nil
}

func (b *BitwardenProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	bwPath, err := exec.LookPath("bw")
	if err != nil {
		return "", errors.New("'bw' command not found. Install it via 'brew install bitwarden-cli'")
	}

	field := ref.Field
	if field == "" {
		field = "password"
	}

	sessionKey := os.Getenv("BW_SESSION")
	if sessionKey == "" {
		return "", errors.New("Bitwarden vault is locked (BW_SESSION is not set in this terminal).\nPlease run: bwu (or: export BW_SESSION=$(bw unlock --raw))")
	}

	// If requesting standard password, 'bw get password' is the cleanest and fastest
	if strings.ToLower(field) == "password" {
		args := []string{"get", "password", ref.Item, "--session", sessionKey}

		cmd := exec.CommandContext(ctx, bwPath, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		stderrStr := strings.TrimSpace(stderr.String())
		stdoutStr := strings.TrimSpace(stdout.String())

		if strings.Contains(stderrStr, "Vault is locked") || strings.Contains(stderrStr, "Session key is invalid") || strings.Contains(stderrStr, "Master password") {
			return "", errors.New("Bitwarden vault is locked or session expired.\nPlease run: bwu (or: export BW_SESSION=$(bw unlock --raw))")
		}

		if err != nil {
			if stderrStr != "" {
				return "", fmt.Errorf("%s", stderrStr)
			}
			return "", err
		}

		if stdoutStr == "" {
			return "", fmt.Errorf("item %q found in Bitwarden, but its Password field is empty", ref.Item)
		}

		return stdoutStr, nil
	}

	// For custom field, username, or notes, query full item JSON
	args := []string{"get", "item", ref.Item, "--session", sessionKey}

	cmd := exec.CommandContext(ctx, bwPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	stderrStr := strings.TrimSpace(stderr.String())

	if strings.Contains(stderrStr, "Vault is locked") || strings.Contains(stderrStr, "Session key is invalid") || strings.Contains(stderrStr, "Master password") {
		return "", errors.New("Bitwarden vault is locked or session expired.\nPlease run: bwu (or: export BW_SESSION=$(bw unlock --raw))")
	}

	if err != nil {
		if stderrStr != "" {
			return "", fmt.Errorf("%s", stderrStr)
		}
		return "", err
	}

	var itemData struct {
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

	if err := json.Unmarshal(stdout.Bytes(), &itemData); err != nil {
		return "", fmt.Errorf("failed to parse Bitwarden item JSON: %w", err)
	}

	switch strings.ToLower(field) {
	case "username":
		return itemData.Login.Username, nil
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
