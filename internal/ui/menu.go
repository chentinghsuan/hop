package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/ting-hsuanchen/hop/internal/config"
)

// SelectTarget presents an interactive search/select menu for a list of targets.
func SelectTarget(targets []config.ResolvedTarget) (*config.ResolvedTarget, error) {
	if len(targets) == 0 {
		return nil, errors.New("no matching targets available")
	}

	if len(targets) == 1 {
		return &targets[0], nil
	}

	options := make([]string, len(targets))
	targetMap := make(map[string]*config.ResolvedTarget)

	for i, t := range targets {
		label := formatTargetLabel(t)
		options[i] = label
		targetMap[label] = &targets[i]
	}

	var selected string
	prompt := &survey.Select{
		Message:  "Select connection target:",
		Options:  options,
		PageSize: 15,
	}

	err := survey.AskOne(prompt, &selected)
	if err != nil {
		return nil, err
	}

	target, ok := targetMap[selected]
	if !ok {
		return nil, errors.New("invalid selection")
	}

	return target, nil
}

// SelectHierarchical allows the user to drill down: Environment -> Service -> Instance
func SelectHierarchical(allTargets []config.ResolvedTarget) (*config.ResolvedTarget, error) {
	if len(allTargets) == 0 {
		return nil, errors.New("no targets configured")
	}

	// -------------------------------------------------------------
	// Step 1: Select Environment (if multiple exist)
	// -------------------------------------------------------------
	seenEnvs := make(map[string]bool)
	var envKeys []string
	var envOptions []string
	envMap := make(map[string]string) // option label -> envKey

	for _, t := range allTargets {
		if !seenEnvs[t.EnvKey] {
			seenEnvs[t.EnvKey] = true
			envKeys = append(envKeys, t.EnvKey)
			label := fmt.Sprintf("[%s] %s", t.EnvKey, t.EnvName)
			envOptions = append(envOptions, label)
			envMap[label] = t.EnvKey
		}
	}

	selectedEnvKey := envKeys[0]
	if len(envKeys) > 1 {
		var selectedEnvLabel string
		promptEnv := &survey.Select{
			Message:  "請選擇環境 (Environment):",
			Options:  envOptions,
			PageSize: 10,
		}
		if err := survey.AskOne(promptEnv, &selectedEnvLabel); err != nil {
			return nil, err
		}
		selectedEnvKey = envMap[selectedEnvLabel]
	}

	// Filter targets for this environment
	var envTargets []config.ResolvedTarget
	for _, t := range allTargets {
		if t.EnvKey == selectedEnvKey {
			envTargets = append(envTargets, t)
		}
	}

	if len(envTargets) == 0 {
		return nil, fmt.Errorf("no targets found for environment %q", selectedEnvKey)
	}

	// -------------------------------------------------------------
	// Step 2: Select Service (mongo, mysql, gcp, etc.)
	// -------------------------------------------------------------
	seenServices := make(map[string]bool)
	var serviceKeys []string
	var serviceOptions []string
	serviceCounts := make(map[string]int)
	serviceMap := make(map[string]string) // option label -> serviceType

	for _, t := range envTargets {
		serviceCounts[t.ServiceType]++
		if !seenServices[t.ServiceType] {
			seenServices[t.ServiceType] = true
			serviceKeys = append(serviceKeys, t.ServiceType)
		}
	}

	for _, s := range serviceKeys {
		var desc string
		switch s {
		case "mongo", "mongodb", "atlas":
			desc = "MongoDB / Atlas 叢集"
		case "mysql", "mariadb", "proxysql":
			desc = "MySQL / ProxySQL 資料庫"
		case "gcp", "vm", "gcp_vm":
			desc = "GCP Compute Engine VM"
		default:
			desc = strings.ToUpper(s)
		}
		label := fmt.Sprintf("%-10s - %s (%d 個實例)", s, desc, serviceCounts[s])
		serviceOptions = append(serviceOptions, label)
		serviceMap[label] = s
	}

	selectedService := serviceKeys[0]
	if len(serviceKeys) > 1 {
		var selectedServiceLabel string
		promptService := &survey.Select{
			Message:  fmt.Sprintf("[%s] 請選擇服務 (Service):", selectedEnvKey),
			Options:  serviceOptions,
			PageSize: 10,
		}
		if err := survey.AskOne(promptService, &selectedServiceLabel); err != nil {
			return nil, err
		}
		selectedService = serviceMap[selectedServiceLabel]
	}

	// Filter targets for this environment and service
	var svcTargets []config.ResolvedTarget
	for _, t := range envTargets {
		if t.ServiceType == selectedService {
			svcTargets = append(svcTargets, t)
		}
	}

	if len(svcTargets) == 0 {
		return nil, fmt.Errorf("no targets found for %s in %s", selectedService, selectedEnvKey)
	}
	if len(svcTargets) == 1 {
		return &svcTargets[0], nil
	}

	// -------------------------------------------------------------
	// Step 3: Select Instance / Cluster
	// -------------------------------------------------------------
	instanceOptions := make([]string, len(svcTargets))
	instanceMap := make(map[string]*config.ResolvedTarget)

	for i, t := range svcTargets {
		desc := ""
		if d, ok := t.Properties["description"].(string); ok && d != "" {
			desc = fmt.Sprintf(" (%s)", d)
		}
		label := fmt.Sprintf("%-12s%s", t.InstanceKey, desc)
		instanceOptions[i] = label
		instanceMap[label] = &svcTargets[i]
	}

	var selectedInstanceLabel string
	promptInstance := &survey.Select{
		Message:  fmt.Sprintf("[%s > %s] 請選擇實例/叢集 (Instance):", selectedEnvKey, selectedService),
		Options:  instanceOptions,
		PageSize: 15,
	}
	if err := survey.AskOne(promptInstance, &selectedInstanceLabel); err != nil {
		return nil, err
	}

	target, ok := instanceMap[selectedInstanceLabel]
	if !ok {
		return nil, errors.New("invalid instance selection")
	}

	return target, nil
}

func formatTargetLabel(t config.ResolvedTarget) string {
	svcColor := color.New(color.FgCyan).SprintfFunc()
	envColor := color.New(color.FgGreen).SprintfFunc()

	var desc string
	if d, ok := t.Properties["description"].(string); ok && d != "" {
		desc = d
	}

	if desc != "" {
		return fmt.Sprintf("%-18s %-16s > %-12s (%s)",
			envColor("[%s]", t.EnvKey),
			svcColor("%s", t.ServiceType),
			t.InstanceKey,
			desc,
		)
	}

	return fmt.Sprintf("%-18s %-16s > %s",
		envColor("[%s]", t.EnvKey),
		svcColor("%s", t.ServiceType),
		t.InstanceKey,
	)
}

// PrintBanner prints a clean CLI banner
func PrintBanner() {
	color.HiGreen(`
╔═══════════════════════════════════════════════════════╗
║                      hop CLI                          ║
║       Fast & Secure Multi-Environment Access          ║
╚═══════════════════════════════════════════════════════╝`)
	fmt.Println()
}
