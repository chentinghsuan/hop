package connector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/secret"
)

type GCPVMConnector struct{}

func NewGCPVMConnector() *GCPVMConnector {
	return &GCPVMConnector{}
}

func (c *GCPVMConnector) Type() string {
	return "gcp"
}

func (c *GCPVMConnector) Aliases() []string {
	return []string{"gcp_vm", "vm", "compute"}
}

func (c *GCPVMConnector) ValidateDependencies() error {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return errors.New("'gcloud' CLI is not found. Install Google Cloud SDK first")
	}
	return nil
}

func (c *GCPVMConnector) Connect(ctx context.Context, target config.ResolvedTarget, resolver secret.Resolver) error {
	if err := c.ValidateDependencies(); err != nil {
		return err
	}

	project := GetString(target.Properties, "project", "")
	zone := GetString(target.Properties, "zone", "")
	instance := GetString(target.Properties, "instance", target.InstanceKey)
	user := GetString(target.Properties, "user", "")
	tunnelIAP := GetBool(target.Properties, "tunnel_through_iap", false)

	if project == "" {
		return fmt.Errorf("missing required property 'project' for GCP target %s", target.DisplayName)
	}
	if zone == "" {
		return fmt.Errorf("missing required property 'zone' for GCP target %s", target.DisplayName)
	}
	if instance == "" {
		return fmt.Errorf("missing required property 'instance' for GCP target %s", target.DisplayName)
	}

	targetHost := instance
	if user != "" {
		targetHost = fmt.Sprintf("%s@%s", user, instance)
	}

	args := []string{
		"compute", "ssh", targetHost,
		"--zone", zone,
		"--project", project,
	}

	if tunnelIAP {
		args = append(args, "--tunnel-through-iap")
	}

	// Extra args if any
	if extra, ok := target.Properties["extra_args"].([]any); ok {
		for _, a := range extra {
			if s, ok := a.(string); ok {
				args = append(args, s)
			}
		}
	}

	color.Cyan("Connecting to GCP VM: %s (zone: %s, project: %s)...", instance, zone, project)
	color.HiBlackString("Executing: gcloud %v", args)

	cmd := exec.CommandContext(ctx, "gcloud", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
