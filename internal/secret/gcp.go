package secret

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type GCPSecretProvider struct{}

func NewGCPSecretProvider() *GCPSecretProvider {
	return &GCPSecretProvider{}
}

func (g *GCPSecretProvider) Name() string {
	return "gcp"
}

func (g *GCPSecretProvider) Validate(ctx context.Context) error {
	gcloudPath, err := exec.LookPath("gcloud")
	if err != nil {
		return errors.New("gcloud CLI is not installed or not in PATH")
	}

	cmd := exec.CommandContext(ctx, gcloudPath, "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil || strings.TrimSpace(stdout.String()) == "" {
		return errors.New("gcloud is not authenticated. Please run 'gcloud auth login' first")
	}
	return nil
}

func (g *GCPSecretProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	gcloudPath, err := exec.LookPath("gcloud")
	if err != nil {
		return "", errors.New("'gcloud' command not found. Install Google Cloud SDK first")
	}

	version := ref.Version
	if version == "" {
		version = "latest"
	}

	args := []string{"secrets", "versions", "access", version, "--secret", ref.Item}
	if ref.Vault != "" { // We can repurpose ref.Vault as gcp project override if specified
		args = append(args, "--project", ref.Vault)
	}

	cmd := exec.CommandContext(ctx, gcloudPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr != "" {
			return "", fmt.Errorf("%s", errStr)
		}
		return "", err
	}

	return strings.TrimRight(stdout.String(), "\r\n"), nil
}
