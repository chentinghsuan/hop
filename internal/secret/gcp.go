package secret

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

type GCPSecretProvider struct{}

func NewGCPSecretProvider() *GCPSecretProvider {
	return &GCPSecretProvider{}
}

func (g *GCPSecretProvider) Name() string {
	return "gcp"
}

func (g *GCPSecretProvider) Validate(ctx context.Context) error {
	// First verify if gcloud CLI is available (useful for gcloud mode and auth status)
	gcloudPath, err := exec.LookPath("gcloud")
	if err == nil {
		cmd := exec.CommandContext(ctx, gcloudPath, "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err == nil && strings.TrimSpace(stdout.String()) != "" {
			return nil
		}
	}

	// For SDK mode, verify client creation (ADC verification)
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return errors.New("GCP authentication missing. Run 'gcloud auth application-default login' (for SDK) or 'gcloud auth login' (for CLI)")
	}
	_ = client.Close()

	return nil
}

func (g *GCPSecretProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	mode := strings.ToLower(ref.Mode)
	if mode == "" {
		mode = "sdk" // default to high-performance Go SDK
	}

	version := ref.Version
	if version == "" {
		version = "latest"
	}

	project := ref.Project
	if project == "" {
		project = ref.Vault // backward compatibility with vault field
	}

	var rawPayload string
	var err error

	if mode == "gcloud" {
		rawPayload, err = g.getViaGcloud(ctx, ref.Item, version, project)
	} else {
		rawPayload, err = g.getViaSDK(ctx, ref.Item, version, project)
		if err != nil && (mode == "" || mode == "sdk") {
			// If SDK fails due to missing ADC, provide clean fallback suggestion or attempt gcloud
			if strings.Contains(err.Error(), "credentials") || strings.Contains(err.Error(), "authentication") {
				if gcloudPath, lookErr := exec.LookPath("gcloud"); lookErr == nil && gcloudPath != "" {
					if fallbackVal, fbErr := g.getViaGcloud(ctx, ref.Item, version, project); fbErr == nil {
						rawPayload = fallbackVal
						err = nil
					}
				}
			}
		}
	}

	if err != nil {
		return "", err
	}

	return extractJSONField(rawPayload, ref.Field)
}

func (g *GCPSecretProvider) getViaSDK(ctx context.Context, item, version, project string) (string, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create GCP Secret Manager client (make sure 'gcloud auth application-default login' is run): %w", err)
	}
	defer client.Close()

	resourceName := buildSecretResourceName(item, version, project)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: resourceName,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %q: %w", resourceName, err)
	}

	return string(result.Payload.Data), nil
}

func (g *GCPSecretProvider) getViaGcloud(ctx context.Context, item, version, project string) (string, error) {
	gcloudPath, err := exec.LookPath("gcloud")
	if err != nil {
		return "", errors.New("'gcloud' command not found. Install Google Cloud SDK first")
	}

	args := []string{"secrets", "versions", "access", version, "--secret", item}
	if project != "" {
		args = append(args, "--project", project)
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

func buildSecretResourceName(item, version, project string) string {
	if strings.HasPrefix(item, "projects/") {
		if strings.Contains(item, "/versions/") {
			return item
		}
		return fmt.Sprintf("%s/versions/%s", item, version)
	}

	if project == "" {
		// If project not specified, try default project
		return fmt.Sprintf("projects/default/secrets/%s/versions/%s", item, version)
	}

	return fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, item, version)
}

func extractJSONField(data string, field string) (string, error) {
	if field == "" {
		return data, nil
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		// Not JSON, return raw or error if field specifically requested
		return "", fmt.Errorf("secret payload is not JSON, cannot extract field %q: %w", field, err)
	}

	val, ok := m[field]
	if !ok {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", fmt.Errorf("field %q not found in secret JSON (available keys: %s)", field, strings.Join(keys, ", "))
	}

	return fmt.Sprintf("%v", val), nil
}
