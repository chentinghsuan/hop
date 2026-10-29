package secret

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
)

// GCPKMSProvider handles decrypting credentials using Google Cloud KMS.
type GCPKMSProvider struct{}

func NewGCPKMSProvider() *GCPKMSProvider {
	return &GCPKMSProvider{}
}

func (k *GCPKMSProvider) Name() string {
	return "gcp_kms"
}

func (k *GCPKMSProvider) Validate(ctx context.Context) error {
	// First check if gcloud CLI is available
	gcloudPath, err := exec.LookPath("gcloud")
	if err == nil {
		cmd := exec.CommandContext(ctx, gcloudPath, "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err == nil && strings.TrimSpace(stdout.String()) != "" {
			return nil
		}
	}

	// Next check SDK client
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return errors.New("GCP authentication missing for KMS. Run 'gcloud auth application-default login' (for SDK) or 'gcloud auth login' (for CLI)")
	}
	_ = client.Close()

	return nil
}

func (k *GCPKMSProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	key := ref.Key
	if key == "" {
		key = ref.Item // fallback if key was passed in item
	}
	if key == "" {
		return "", errors.New("KMS key path is required (e.g., 'projects/my-prj/locations/global/keyRings/my-ring/cryptoKeys/my-key')")
	}

	ciphertext := ref.Ciphertext
	if ciphertext == "" {
		return "", errors.New("KMS ciphertext is required")
	}

	mode := strings.ToLower(ref.Mode)
	if mode == "" {
		mode = "sdk"
	}

	var plaintext string
	var err error

	if mode == "gcloud" {
		plaintext, err = DecryptWithGcloud(ctx, key, ciphertext)
	} else {
		plaintext, err = DecryptWithSDK(ctx, key, ciphertext)
		if err != nil && (mode == "" || mode == "sdk") {
			// Auto fallback to gcloud if ADC missing
			if strings.Contains(err.Error(), "credentials") || strings.Contains(err.Error(), "authentication") {
				if gcloudPath, lookErr := exec.LookPath("gcloud"); lookErr == nil && gcloudPath != "" {
					if fbVal, fbErr := DecryptWithGcloud(ctx, key, ciphertext); fbErr == nil {
						plaintext = fbVal
						err = nil
					}
				}
			}
		}
	}

	if err != nil {
		return "", err
	}

	return extractJSONField(plaintext, ref.Field)
}

// DecryptWithSDK uses the native GCP Cloud KMS Go SDK for low-latency decryption.
func DecryptWithSDK(ctx context.Context, key, ciphertextBase64 string) (string, error) {
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create GCP KMS client: %w", err)
	}
	defer client.Close()

	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertextBase64))
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 ciphertext: %w", err)
	}

	req := &kmspb.DecryptRequest{
		Name:       key,
		Ciphertext: ciphertext,
	}

	resp, err := client.Decrypt(ctx, req)
	if err != nil {
		return "", fmt.Errorf("KMS decryption failed: %w", err)
	}

	return string(resp.Plaintext), nil
}

// DecryptWithGcloud uses the local gcloud CLI to decrypt the ciphertext.
func DecryptWithGcloud(ctx context.Context, key, ciphertextBase64 string) (string, error) {
	gcloudPath, err := exec.LookPath("gcloud")
	if err != nil {
		return "", errors.New("'gcloud' CLI not found")
	}

	ciphertextBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertextBase64))
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 ciphertext: %w", err)
	}

	cmd := exec.CommandContext(ctx, gcloudPath, "kms", "decrypt",
		"--key="+key,
		"--ciphertext-file=-",
		"--plaintext-file=-",
	)

	cmd.Stdin = bytes.NewReader(ciphertextBytes)
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

// EncryptKMS encrypts a plaintext string using KMS and returns a base64 string.
func EncryptKMS(ctx context.Context, key, plaintext, mode string) (string, error) {
	mode = strings.ToLower(mode)
	if mode == "" {
		mode = "sdk"
	}

	if mode == "gcloud" {
		return encryptWithGcloud(ctx, key, plaintext)
	}

	// SDK mode
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		// Fallback to gcloud if gcloud exists
		if gcloudPath, lookErr := exec.LookPath("gcloud"); lookErr == nil && gcloudPath != "" {
			return encryptWithGcloud(ctx, key, plaintext)
		}
		return "", fmt.Errorf("failed to initialize GCP KMS client: %w", err)
	}
	defer client.Close()

	req := &kmspb.EncryptRequest{
		Name:      key,
		Plaintext: []byte(plaintext),
	}

	resp, err := client.Encrypt(ctx, req)
	if err != nil {
		return "", fmt.Errorf("KMS encryption failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(resp.Ciphertext), nil
}

func encryptWithGcloud(ctx context.Context, key, plaintext string) (string, error) {
	gcloudPath, err := exec.LookPath("gcloud")
	if err != nil {
		return "", errors.New("'gcloud' CLI not found")
	}

	cmd := exec.CommandContext(ctx, gcloudPath, "kms", "encrypt",
		"--key="+key,
		"--plaintext-file=-",
		"--ciphertext-file=-",
	)

	cmd.Stdin = strings.NewReader(plaintext)
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

	return base64.StdEncoding.EncodeToString(stdout.Bytes()), nil
}
