package secret

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var loadEnvOnce sync.Once

func loadDotEnvFiles() {
	loadEnvOnce.Do(func() {
		// Priority 1: ~/.config/hop/.env (or %APPDATA%\hop\.env)
		if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
			parseAndSetEnv(filepath.Join(configDir, "hop", ".env"))
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			parseAndSetEnv(filepath.Join(home, ".config", "hop", ".env"))
		}
		// Priority 2: ./.env (current working directory)
		parseAndSetEnv(".env")
	})
}

func parseAndSetEnv(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
				v = v[1 : len(v)-1]
			}
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

// EnvProvider retrieves secrets from local environment variables or .env files.
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
	loadDotEnvFiles()

	val := os.Getenv(ref.Item)
	if val == "" {
		return "", fmt.Errorf("environment variable %q is empty or not set", ref.Item)
	}
	return extractJSONField(val, ref.Field)
}

