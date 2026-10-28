package secret

import (
	"context"
	"fmt"
	"os"
)

// EnvProvider retrieves secrets from local environment variables.
type EnvProvider struct{}

// NewEnvProvider creates a new EnvProvider instance.
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{}
}

func (e *EnvProvider) Name() string {
	return "env"
}

func (e *EnvProvider) Validate(ctx context.Context) error {
	return nil
}

func (e *EnvProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	val := os.Getenv(ref.Item)
	if val == "" {
		return "", fmt.Errorf("environment variable %q is empty or not set", ref.Item)
	}
	return val, nil
}

