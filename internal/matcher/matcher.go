package matcher

import (
	"strings"

	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/connector"
)

// MatchResult contains the matching targets and classification.
type MatchResult struct {
	ExactSingle *config.ResolvedTarget
	Candidates  []config.ResolvedTarget
}

// SmartMatch takes user-provided command-line arguments (in any order)
// and finds matching targets against EnvKey, ServiceType, and InstanceKey.
func SmartMatch(tokens []string, allTargets []config.ResolvedTarget, reg *connector.Registry) MatchResult {
	if len(tokens) == 0 {
		return MatchResult{
			Candidates: allTargets,
		}
	}

	cleanTokens := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t != "" {
			cleanTokens = append(cleanTokens, strings.ToLower(t))
		}
	}

	if len(cleanTokens) == 0 {
		return MatchResult{Candidates: allTargets}
	}

	// 1. Try Strict Match (every token must match a distinct field: env, service, or instance)
	var exactCandidates []config.ResolvedTarget
	for _, target := range allTargets {
		if matchesTargetStrict(cleanTokens, target, reg) {
			targetCopy := target
			exactCandidates = append(exactCandidates, targetCopy)
		}
	}

	if len(exactCandidates) == 1 {
		return MatchResult{
			ExactSingle: &exactCandidates[0],
			Candidates:  exactCandidates,
		}
	}

	if len(exactCandidates) > 1 {
		return MatchResult{
			Candidates: exactCandidates,
		}
	}

	// 2. Fallback: Fuzzy / Substring match across all fields and DisplayName
	var fuzzyCandidates []config.ResolvedTarget
	for _, target := range allTargets {
		if matchesTargetFuzzy(cleanTokens, target) {
			targetCopy := target
			fuzzyCandidates = append(fuzzyCandidates, targetCopy)
		}
	}

	if len(fuzzyCandidates) == 1 {
		return MatchResult{
			ExactSingle: &fuzzyCandidates[0],
			Candidates:  fuzzyCandidates,
		}
	}

	return MatchResult{
		Candidates: fuzzyCandidates,
	}
}

// matchesTargetStrict checks if all tokens can be mapped to distinct target attributes:
// EnvKey, ServiceType (or alias), and InstanceKey.
func matchesTargetStrict(tokens []string, target config.ResolvedTarget, reg *connector.Registry) bool {
	envKey := strings.ToLower(target.EnvKey)
	serviceType := strings.ToLower(target.ServiceType)
	instanceKey := strings.ToLower(target.InstanceKey)

	// Build alias set for the target's service
	serviceAliases := map[string]bool{serviceType: true}
	if reg != nil {
		if conn, ok := reg.Get(serviceType); ok {
			serviceAliases[strings.ToLower(conn.Type())] = true
			for _, a := range conn.Aliases() {
				serviceAliases[strings.ToLower(a)] = true
			}
		}
	}

	usedEnv := false
	usedService := false
	usedInstance := false

	// Try matching each token
	for _, tok := range tokens {
		matchedThis := false

		// Check Env
		if !usedEnv && tok == envKey {
			usedEnv = true
			matchedThis = true
			continue
		}

		// Check Service
		if !usedService && serviceAliases[tok] {
			usedService = true
			matchedThis = true
			continue
		}

		// Check Instance
		if !usedInstance && tok == instanceKey {
			usedInstance = true
			matchedThis = true
			continue
		}

		if !matchedThis {
			return false
		}
	}

	return true
}

// matchesTargetFuzzy checks if each token appears as a substring in the target's metadata
func matchesTargetFuzzy(tokens []string, target config.ResolvedTarget) bool {
	searchHaystack := strings.ToLower(strings.Join([]string{
		target.EnvKey,
		target.EnvName,
		target.ServiceType,
		target.InstanceKey,
		target.DisplayName,
	}, " "))

	for _, tok := range tokens {
		if !strings.Contains(searchHaystack, tok) {
			return false
		}
	}
	return true
}
