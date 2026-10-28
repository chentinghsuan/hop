package secret

import (
	"context"
	"os"
	"testing"
)

func TestEnvProvider(t *testing.T) {
	p := NewEnvProvider()
	if p.Name() != "env" {
		t.Fatalf("expected provider name 'env', got %q", p.Name())
	}

	ctx := context.Background()
	if err := p.Validate(ctx); err != nil {
		t.Fatalf("expected Validate to succeed, got %v", err)
	}

	// 1. Missing env
	_, err := p.GetSecret(ctx, SecretRef{Item: "NON_EXISTENT_VAR_12345"})
	if err == nil {
		t.Fatalf("expected error for non-existent env var, got nil")
	}

	// 2. Existing env
	os.Setenv("HOP_TEST_SECRET", "superSecretValue")
	defer os.Unsetenv("HOP_TEST_SECRET")

	val, err := p.GetSecret(ctx, SecretRef{Item: "HOP_TEST_SECRET"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "superSecretValue" {
		t.Fatalf("expected 'superSecretValue', got %q", val)
	}
}

