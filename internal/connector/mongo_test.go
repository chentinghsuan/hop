package connector

import (
	"context"
	"testing"

	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/secret"
)

func TestMaskMongoURI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URI with username and password",
			input:    "mongodb://admin:superSecret123@localhost:27017/my_db?authSource=admin",
			expected: "mongodb://admin:******@localhost:27017/my_db?authSource=admin",
		},
		{
			name:     "URI with username only",
			input:    "mongodb://admin@localhost:27017/my_db",
			expected: "mongodb://admin@localhost:27017/my_db",
		},
		{
			name:     "URI without credentials",
			input:    "mongodb://localhost:27017/my_db",
			expected: "mongodb://localhost:27017/my_db",
		},
		{
			name:     "Invalid URI format returns original",
			input:    "://invalid-uri",
			expected: "://invalid-uri",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := maskMongoURI(tt.input)
			if masked != tt.expected {
				t.Errorf("maskMongoURI(%q) = %q, expected %q", tt.input, masked, tt.expected)
			}
		})
	}
}

func TestMongoConnectorMetadata(t *testing.T) {
	c := NewMongoConnector()
	if c.Type() != "mongo" {
		t.Errorf("expected type 'mongo', got %q", c.Type())
	}
	aliases := c.Aliases()
	foundAtlas := false
	for _, a := range aliases {
		if a == "atlas" {
			foundAtlas = true
			break
		}
	}
	if !foundAtlas {
		t.Errorf("expected alias 'atlas' in MongoConnector aliases")
	}
}

type mockKMSProvider struct{}

func (m *mockKMSProvider) Name() string                      { return "gcp_kms" }
func (m *mockKMSProvider) Validate(ctx context.Context) error { return nil }
func (m *mockKMSProvider) GetSecret(ctx context.Context, ref secret.SecretRef) (string, error) {
	if ref.Field == "username" {
		return "kms_admin", nil
	}
	return "kms_decrypted_pwd", nil
}

func TestResolveCredentials_KMSAndGetStringSlice(t *testing.T) {
	resolver := secret.NewResolver(&mockKMSProvider{})
	target := config.ResolvedTarget{
		Properties: map[string]any{
			"extra_args": []any{"--tls", "--quiet"},
		},
		Secret: config.SecretConfig{
			Provider:   "gcp_kms",
			Key:        "projects/p/locations/l/keyRings/r/cryptoKeys/k",
			Ciphertext: "BASE64_CIPHER",
			Field:      "password",
		},
	}

	user, pwd, err := ResolveCredentials(context.Background(), "Mongo", target, resolver, "default_user")
	if err != nil {
		t.Fatalf("ResolveCredentials failed: %v", err)
	}
	if pwd != "kms_decrypted_pwd" {
		t.Errorf("expected password 'kms_decrypted_pwd', got %q", pwd)
	}
	if user != "kms_admin" {
		t.Errorf("expected auto-resolved username 'kms_admin', got %q", user)
	}

	extra := GetStringSlice(target.Properties, "extra_args")
	if len(extra) != 2 || extra[0] != "--tls" || extra[1] != "--quiet" {
		t.Errorf("unexpected extra_args: %v", extra)
	}
}


