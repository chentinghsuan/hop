package ui

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/ting-hsuanchen/hop/internal/config"
)

// PrefetchFunc is a callback invoked in the background when the user's cursor highlights a target.
type PrefetchFunc func(target config.ResolvedTarget)

func init() {
	// Evaluate Description ONLY for the currently focused option (.SelectedIndex == .CurrentIndex)
	// so we can use survey.Select.Description as an exact cursor-hover hook for secret prefetching.
	survey.SelectQuestionTemplate = strings.Replace(
		survey.SelectQuestionTemplate,
		`{{ if ne ($.GetDescription .CurrentOpt) "" }} - {{color "cyan"}}{{ $.GetDescription .CurrentOpt }}{{end}}`,
		`{{ if eq .SelectedIndex .CurrentIndex }}{{ if ne ($.GetDescription .CurrentOpt) "" }} - {{color "cyan"}}{{ $.GetDescription .CurrentOpt }}{{end}}{{end}}`,
		1,
	)
}

func firstPrefetchFn(fns []PrefetchFunc) PrefetchFunc {
	if len(fns) > 0 && fns[0] != nil {
		return fns[0]
	}
	return nil
}

// newHoverHandler creates a debounced Description callback for survey.Select
// that triggers onHover(index) when the cursor rests on an option.
func newHoverHandler(onHover func(index int)) (func(value string, index int) string, func()) {
	if onHover == nil {
		return nil, func() {}
	}
	var (
		mu        sync.Mutex
		lastIndex = -1
		timer     *time.Timer
	)
	descFn := func(value string, index int) string {
		mu.Lock()
		defer mu.Unlock()

		if index == lastIndex {
			return ""
		}
		lastIndex = index

		if timer != nil {
			timer.Stop()
		}
		idx := index
		timer = time.AfterFunc(100*time.Millisecond, func() {
			onHover(idx)
		})
		return ""
	}
	stopFn := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
	}
	return descFn, stopFn
}

// SelectTarget presents an interactive search/select menu for a list of targets.
func SelectTarget(targets []config.ResolvedTarget, prefetch ...PrefetchFunc) (*config.ResolvedTarget, error) {
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

	prefetchFn := firstPrefetchFn(prefetch)
	descFn, stopHover := newHoverHandler(func(index int) {
		if prefetchFn != nil && index >= 0 && index < len(targets) {
			prefetchFn(targets[index])
		}
	})
	defer stopHover()

	var selected string
	prompt := &survey.Select{
		Message:     "Select connection target:",
		Options:     options,
		PageSize:    15,
		Description: descFn,
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

// SelectHierarchical allows the user to drill down: Environment -> Service -> Instance,
// with full support for navigating back to previous steps.
func SelectHierarchical(allTargets []config.ResolvedTarget, prefetch ...PrefetchFunc) (*config.ResolvedTarget, error) {
	if len(allTargets) == 0 {
		return nil, errors.New("no targets configured")
	}

	prefetchFn := firstPrefetchFn(prefetch)
	const backOption = "← 返回上一步 (Back)"

	// Build Environment list
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
	selectedEnvLabel := envOptions[0]
	var selectedService string
	var selectedServiceLabel string
	var serviceKeys []string

	currentStep := 1
	if len(envKeys) <= 1 {
		currentStep = 2 // only 1 environment, skip step 1
	}

	for {
		switch currentStep {
		case 1:
			// -------------------------------------------------------------
			// Step 1: Select Environment
			// -------------------------------------------------------------
			descEnv, stopEnvHover := newHoverHandler(func(index int) {
				if prefetchFn == nil || index < 0 || index >= len(envKeys) {
					return
				}
				hoveredEnv := envKeys[index]
				for _, t := range allTargets {
					if t.EnvKey == hoveredEnv && t.Secret.HasSecret() {
						prefetchFn(t)
						break
					}
				}
			})

			promptEnv := &survey.Select{
				Message:     "請選擇環境 (Environment):",
				Options:     envOptions,
				Default:     selectedEnvLabel,
				PageSize:    10,
				Description: descEnv,
			}
			err := survey.AskOne(promptEnv, &selectedEnvLabel)
			stopEnvHover()
			if err != nil {
				return nil, err
			}
			selectedEnvKey = envMap[selectedEnvLabel]
			currentStep = 2

		case 2:
			// -------------------------------------------------------------
			// Step 2: Select Service (mongo, mysql, gcp, etc.)
			// -------------------------------------------------------------
			var envTargets []config.ResolvedTarget
			for _, t := range allTargets {
				if t.EnvKey == selectedEnvKey {
					envTargets = append(envTargets, t)
				}
			}

			if len(envTargets) == 0 {
				return nil, fmt.Errorf("no targets found for environment %q", selectedEnvKey)
			}

			seenServices := make(map[string]bool)
			serviceKeys = nil
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

			hasBack := len(envKeys) > 1
			opts := append([]string{}, serviceOptions...)
			if hasBack {
				opts = append(opts, backOption)
			}

			if len(serviceKeys) == 1 && !hasBack {
				selectedService = serviceKeys[0]
				currentStep = 3
				continue
			}

			descSvc, stopSvcHover := newHoverHandler(func(index int) {
				if prefetchFn == nil || index < 0 || index >= len(serviceKeys) {
					return
				}
				hoveredSvc := serviceKeys[index]
				for _, t := range envTargets {
					if t.ServiceType == hoveredSvc && t.Secret.HasSecret() {
						prefetchFn(t)
						break
					}
				}
			})

			defaultOption := selectedServiceLabel
			if defaultOption == "" && len(serviceOptions) > 0 {
				defaultOption = serviceOptions[0]
			}

			promptService := &survey.Select{
				Message:     fmt.Sprintf("[%s] 請選擇服務 (Service):", selectedEnvKey),
				Options:     opts,
				Default:     defaultOption,
				PageSize:    10,
				Description: descSvc,
			}
			var chosenServiceLabel string
			err := survey.AskOne(promptService, &chosenServiceLabel)
			stopSvcHover()
			if err != nil {
				return nil, err
			}

			if chosenServiceLabel == backOption {
				currentStep = 1
				continue
			}

			selectedServiceLabel = chosenServiceLabel
			selectedService = serviceMap[chosenServiceLabel]
			currentStep = 3

		case 3:
			// -------------------------------------------------------------
			// Step 3: Select Instance / Cluster
			// -------------------------------------------------------------
			var svcTargets []config.ResolvedTarget
			for _, t := range allTargets {
				if t.EnvKey == selectedEnvKey && t.ServiceType == selectedService {
					svcTargets = append(svcTargets, t)
				}
			}

			if len(svcTargets) == 0 {
				return nil, fmt.Errorf("no targets found for %s in %s", selectedService, selectedEnvKey)
			}

			if len(svcTargets) == 1 && len(envKeys) <= 1 && len(serviceKeys) <= 1 {
				return &svcTargets[0], nil
			}

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

			opts := append([]string{}, instanceOptions...)
			opts = append(opts, backOption)

			descInst, stopInstHover := newHoverHandler(func(index int) {
				if prefetchFn != nil && index >= 0 && index < len(svcTargets) {
					prefetchFn(svcTargets[index])
				}
			})

			var selectedInstanceLabel string
			promptInstance := &survey.Select{
				Message:     fmt.Sprintf("[%s > %s] 請選擇實例/叢集 (Instance):", selectedEnvKey, selectedService),
				Options:     opts,
				PageSize:    15,
				Description: descInst,
			}
			err := survey.AskOne(promptInstance, &selectedInstanceLabel)
			stopInstHover()
			if err != nil {
				return nil, err
			}

			if selectedInstanceLabel == backOption {
				if len(serviceKeys) > 1 || len(envKeys) > 1 {
					currentStep = 2
				} else {
					currentStep = 1
				}
				continue
			}

			target, ok := instanceMap[selectedInstanceLabel]
			if !ok {
				return nil, errors.New("invalid instance selection")
			}

			return target, nil
		}
	}
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
