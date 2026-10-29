package secret

import (
	"context"
	"testing"
)

func TestGCPKMSProviderValidation(t *testing.T) {
	p := NewGCPKMSProvider()
	if p.Name() != "gcp_kms" {
		t.Fatalf("expected name 'gcp_kms', got %q", p.Name())
	}

	ctx := context.Background()

	// Missing key
	_, err := p.GetSecret(ctx, SecretRef{
		Ciphertext: "dGVzdA==", // base64 "test"
	})
	if err == nil {
		t.Fatalf("expected error for missing key, got nil")
	}

	// Missing ciphertext
	_, err = p.GetSecret(ctx, SecretRef{
		Key: "projects/p/locations/l/keyRings/r/cryptoKeys/k",
	})
	if err == nil {
		t.Fatalf("expected error for missing ciphertext, got nil")
	}

	// Invalid base64 ciphertext
	_, err = p.GetSecret(ctx, SecretRef{
		Key:        "projects/p/locations/l/keyRings/r/cryptoKeys/k",
		Ciphertext: "invalid-not-base64!@#$%",
	})
	if err == nil {
		t.Fatalf("expected error for invalid base64, got nil")
	}
}
