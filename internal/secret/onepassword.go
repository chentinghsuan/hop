package secret

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type OnePasswordProvider struct{}

func NewOnePasswordProvider() *OnePasswordProvider {
	return &OnePasswordProvider{}
}

func (o *OnePasswordProvider) Name() string {
	return "1password"
}

func (o *OnePasswordProvider) Validate(ctx context.Context) error {
	opPath, err := exec.LookPath("op")
	if err != nil {
		return errors.New("1Password CLI ('op') is not installed or not in PATH")
	}

	cmd := exec.CommandContext(ctx, opPath, "whoami")
	if err := cmd.Run(); err != nil {
		return errors.New("1Password CLI is not signed in. Run 'eval $(op signin)' first")
	}
	return nil
}

func (o *OnePasswordProvider) GetSecret(ctx context.Context, ref SecretRef) (string, error) {
	opPath, err := exec.LookPath("op")
	if err != nil {
		return "", errors.New("'op' command not found. Install it via 'brew install --cask 1password-cli'")
	}

	field := ref.Field
	if field == "" {
		field = "password"
	}

	var args []string
	if strings.HasPrefix(ref.Item, "op://") {
		// Native secret reference syntax (e.g. op://vault/item/password)
		args = []string{"read", ref.Item}
	} else {
		// Item name or ID
		args = []string{"item", "get", ref.Item, "--fields", field, "--reveal"}
		if ref.Vault != "" {
			args = append(args, "--vault", ref.Vault)
		}
	}

	cmd := exec.CommandContext(ctx, opPath, args...)
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

	return strings.TrimSpace(stdout.String()), nil
}
