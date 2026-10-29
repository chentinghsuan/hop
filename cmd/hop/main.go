package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/connector"
	"github.com/ting-hsuanchen/hop/internal/matcher"
	"github.com/ting-hsuanchen/hop/internal/secret"
	"github.com/ting-hsuanchen/hop/internal/ui"
)

const Version = "1.0.0"

var (
	configFlag string
	flatFlag   bool
	listFlag   bool
)

func main() {
	// Initialize core components
	secResolver := secret.NewResolver(
		secret.NewBitwardenProvider(),
		secret.NewOnePasswordProvider(),
		secret.NewGCPSecretProvider(),
		secret.NewGCPKMSProvider(),
		secret.NewEnvProvider(),
	)

	connRegistry := connector.NewRegistry(
		connector.NewGCPVMConnector(),
		connector.NewMySQLConnector(),
		connector.NewMongoConnector(),
	)

	rootCmd := &cobra.Command{
		Use:   "hop [tokens...]",
		Short: "hop: Fast & Secure multi-environment connection tool",
		Long: `hop connects you to company environments (MongoDB, MySQL, GCP VMs, etc.)
quickly and securely. Supports Bitwarden, 1Password, and GCP Secret Manager.

Examples:
  # Interactive mode:
  hop

  # Fast connect with tokens (order does not matter):
  hop mongo prod user
  hop prod mysql user
  hop gcp prod api

  # Partial filter:
  hop prod mongo     (opens menu with prod mongo clusters)
  hop user           (opens menu filtered to 'user')`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listFlag {
				return runList(cmd.Context(), connRegistry, secResolver)
			}
			return runConnect(cmd.Context(), args, connRegistry, secResolver)
		},
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVarP(&configFlag, "config", "c", "", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&flatFlag, "flat", "F", false, "Use flat fuzzy-search menu instead of 3-stage hierarchical")
	rootCmd.Flags().BoolVarP(&listFlag, "list", "l", false, "List all configured connection targets")

	// Subcommands
	connectCmd := &cobra.Command{
		Use:          "connect [tokens...]",
		Short:        "Connect to target using tokens or interactive selection",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnect(cmd.Context(), args, connRegistry, secResolver)
		},
	}
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(newListCmd(connRegistry, secResolver))
	rootCmd.AddCommand(newCheckCmd(connRegistry, secResolver))
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newEncryptCmd())

	os.Args = routeArgs(os.Args)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func routeArgs(args []string) []string {
	if len(args) <= 1 {
		return append([]string{args[0], "connect"}, args[1:]...)
	}

	known := map[string]bool{
		"list": true, "check": true, "init": true, "version": true, "help": true, "completion": true, "connect": true, "encrypt": true,
	}

	// If help or list flag is present anywhere, handle accordingly
	for i, a := range args[1:] {
		if a == "-h" || a == "--help" || a == "help" {
			return args
		}
		if a == "-l" || a == "--list" {
			newArgs := []string{args[0], "list"}
			for j, arg := range args[1:] {
				if j != i {
					newArgs = append(newArgs, arg)
				}
			}
			return newArgs
		}
	}

	// Check first non-flag argument
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if known[a] {
			return args
		}
		// First non-flag argument is a token -> route to connect
		return append([]string{args[0], "connect"}, args[1:]...)
	}

	// Only flags provided (e.g. hop -F)
	return append([]string{args[0], "connect"}, args[1:]...)
}

func runConnect(ctx context.Context, args []string, reg *connector.Registry, sec secret.Resolver) error {
	cfg, _, err := config.LoadConfig(configFlag)
	if err != nil {
		color.Red("Config Error: %v", err)
		color.Yellow("\nTip: Run 'hop init' to create a starter configuration.")
		return err
	}

	targets, err := cfg.ResolveTargets()
	if err != nil {
		return fmt.Errorf("failed to parse targets: %w", err)
	}

	if len(targets) == 0 {
		return fmt.Errorf("no connection targets found in configuration")
	}

	var targetToConnect *config.ResolvedTarget

	if len(args) > 0 {
		// Smart Token Matching
		matchResult := matcher.SmartMatch(args, targets, reg)

		if matchResult.ExactSingle != nil {
			targetToConnect = matchResult.ExactSingle
		} else if len(matchResult.Candidates) > 0 {
			color.Cyan("Multiple targets matched your tokens %v. Please choose one:", args)
			var selected *config.ResolvedTarget
			var err error
			if flatFlag {
				selected, err = ui.SelectTarget(matchResult.Candidates)
			} else {
				selected, err = ui.SelectHierarchical(matchResult.Candidates)
			}
			if err != nil {
				return err
			}
			targetToConnect = selected
		} else {
			color.Yellow("No targets matched your input tokens: %v", args)
			color.White("Falling back to target selection:")
			var selected *config.ResolvedTarget
			var err error
			if flatFlag {
				selected, err = ui.SelectTarget(targets)
			} else {
				selected, err = ui.SelectHierarchical(targets)
			}
			if err != nil {
				return err
			}
			targetToConnect = selected
		}
	} else {
		// No arguments: default to 3-stage hierarchical drill down!
		var selected *config.ResolvedTarget
		var err error
		if flatFlag {
			selected, err = ui.SelectTarget(targets)
		} else {
			selected, err = ui.SelectHierarchical(targets)
		}
		if err != nil {
			return err
		}
		targetToConnect = selected
	}

	if targetToConnect == nil {
		return nil
	}

	// Dispatch to appropriate connector
	conn, ok := reg.Get(targetToConnect.ServiceType)
	if !ok {
		return fmt.Errorf("no connector registered for service type %q (supported: %s)",
			targetToConnect.ServiceType, reg.FormatSupportedTypes())
	}

	return conn.Connect(ctx, *targetToConnect, sec)
}

func newListCmd(reg *connector.Registry, sec secret.Resolver) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), reg, sec)
		},
	}
}

func runList(ctx context.Context, reg *connector.Registry, sec secret.Resolver) error {
	cfg, cfgPath, err := config.LoadConfig(configFlag)
	if err != nil {
		return err
	}

	targets, err := cfg.ResolveTargets()
	if err != nil {
		return err
	}

	color.Green("Configuration: %s (%d targets found)\n", cfgPath, len(targets))

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Env", "Service", "Instance", "Description", "Secret Provider"})
	table.SetBorder(true)
	table.SetAutoWrapText(false)

	for _, t := range targets {
		secInfo := "-"
		if t.Secret.Item != "" {
			secInfo = fmt.Sprintf("%s (%s)", t.Secret.Provider, t.Secret.Item)
		}

		desc := connector.GetString(t.Properties, "description", "-")

		table.Append([]string{
			t.EnvKey,
			t.ServiceType,
			t.InstanceKey,
			desc,
			secInfo,
		})
	}

	table.Render()
	return nil
}

func newCheckCmd(reg *connector.Registry, sec secret.Resolver) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check local tools and authentication status",
		Run: func(cmd *cobra.Command, args []string) {
			color.Cyan("Checking dependencies and authentication status:\n")

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Category", "Tool / Service", "Status", "Details"})
			table.SetBorder(true)

			// 1. Secret Providers
			for name, p := range sec.AllProviders() {
				err := p.Validate(cmd.Context())
				status := color.GreenString("READY")
				detail := "CLI installed and ready"
				if err != nil {
					status = color.YellowString("ACTION NEEDED")
					detail = err.Error()
				}
				table.Append([]string{"Secret Provider", name, status, detail})
			}

			// 2. Connectors
			for _, c := range reg.AllUnique() {
				err := c.ValidateDependencies()
				status := color.GreenString("READY")
				detail := "Dependencies available"
				if err != nil {
					status = color.RedString("MISSING")
					detail = err.Error()
				}
				table.Append([]string{"Connector", c.Type(), status, detail})
			}

			table.Render()

			// Bitwarden Session Note
			if bwSession := os.Getenv("BW_SESSION"); bwSession == "" {
				color.Yellow("\nTip for Bitwarden users:")
				fmt.Println("  To unlock Bitwarden vault, run:")
				if runtime.GOOS == "windows" {
					color.Cyan("  $env:BW_SESSION = (bw unlock --raw)\n")
				} else {
					color.Cyan("  export BW_SESSION=$(bw unlock --raw)\n")
				}
			}
		},
	}
}

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize starter config file (e.g., ~/.config/hop/config.yaml or %APPDATA%\\hop\\config.yaml)",
		RunE: func(cmd *cobra.Command, args []string) error {
			defaultPath, err := config.GetDefaultConfigPath()
			if err != nil {
				return err
			}

			if _, err := os.Stat(defaultPath); err == nil && !force {
				color.Yellow("Config file already exists at: %s", defaultPath)
				color.HiBlack("Use 'hop init --force' to overwrite it with the latest template.")
				return nil
			}

			dir := filepath.Dir(defaultPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory %s: %w", dir, err)
			}

			if err := os.WriteFile(defaultPath, []byte(config.DefaultExampleConfig), 0600); err != nil {
				return fmt.Errorf("failed to write starter config: %w", err)
			}

			color.Green("✔ Starter configuration initialized at: %s", defaultPath)
			color.Cyan("Edit this file to customize your company environments and connection targets.")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing configuration file")
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("hop version %s\n", Version)
		},
	}
}

func newEncryptCmd() *cobra.Command {
	var (
		kmsKey    string
		mode      string
		secretVal string
	)

	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a secret using GCP Cloud KMS for use in config.yaml",
		Long: `Encrypts a plaintext secret with GCP Cloud KMS and outputs
the ciphertext ready to be pasted into your ~/.config/hop/config.yaml.`,
		Example: `  # Interactive prompt (hides secret from terminal history):
  hop encrypt --key projects/my-prj/locations/global/keyRings/my-ring/cryptoKeys/my-key

  # Specify plaintext directly via flag:
  hop encrypt --key projects/my-prj/locations/global/keyRings/my-ring/cryptoKeys/my-key --secret "my-password"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if kmsKey == "" {
				return errors.New("missing required flag --key (KMS CryptoKey resource name, e.g. projects/P/locations/L/keyRings/R/cryptoKeys/K)")
			}

			if secretVal == "" {
				prompt := &survey.Password{
					Message: "Enter secret to encrypt:",
				}
				if err := survey.AskOne(prompt, &secretVal); err != nil {
					return err
				}
			}

			if strings.TrimSpace(secretVal) == "" {
				return errors.New("cannot encrypt empty secret")
			}

			color.Cyan("Encrypting with GCP KMS (%s mode)...", mode)
			ciphertext, err := secret.EncryptKMS(cmd.Context(), kmsKey, secretVal, mode)
			if err != nil {
				return err
			}

			color.Green("\n✔ Encryption successful!\n")
			fmt.Println("Ciphertext (Base64):")
			color.Yellow("  %s\n", ciphertext)
			fmt.Println("Paste this into your config.yaml:")
			fmt.Printf(`  secret:
    provider: gcp_kms
    key: %q
    ciphertext: %q
`, kmsKey, ciphertext)
			if mode != "" && mode != "sdk" {
				fmt.Printf("    mode: %q\n", mode)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&kmsKey, "key", "k", "", "GCP Cloud KMS CryptoKey resource path")
	cmd.Flags().StringVarP(&mode, "mode", "m", "sdk", "KMS execution mode: 'sdk' (default) or 'gcloud'")
	cmd.Flags().StringVarP(&secretVal, "secret", "s", "", "Plaintext secret to encrypt (if omitted, will prompt securely)")

	return cmd
}
