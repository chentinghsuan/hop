package matcher

import (
	"testing"

	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/connector"
)

func TestSmartMatch(t *testing.T) {
	reg := connector.NewRegistry(
		connector.NewGCPVMConnector(),
		connector.NewMySQLConnector(),
		connector.NewMongoConnector(),
	)

	allTargets := []config.ResolvedTarget{
		{
			EnvKey:      "prod",
			ServiceType: "mongo",
			InstanceKey: "user",
			DisplayName: "[prod] mongo > user",
		},
		{
			EnvKey:      "prod",
			ServiceType: "mongo",
			InstanceKey: "order",
			DisplayName: "[prod] mongo > order",
		},
		{
			EnvKey:      "prod",
			ServiceType: "mysql",
			InstanceKey: "user",
			DisplayName: "[prod] mysql > user",
		},
		{
			EnvKey:      "uat",
			ServiceType: "mongo",
			InstanceKey: "user",
			DisplayName: "[uat] mongo > user",
		},
		{
			EnvKey:      "prod",
			ServiceType: "gcp",
			InstanceKey: "api",
			DisplayName: "[prod] gcp > api",
		},
	}

	// Case 1: Exact match - tokens in standard order: mongo prod user
	t.Run("StandardOrder", func(t *testing.T) {
		res := SmartMatch([]string{"mongo", "prod", "user"}, allTargets, reg)
		if res.ExactSingle == nil {
			t.Fatalf("expected exact single match, got nil (candidates: %d)", len(res.Candidates))
		}
		if res.ExactSingle.EnvKey != "prod" || res.ExactSingle.ServiceType != "mongo" || res.ExactSingle.InstanceKey != "user" {
			t.Errorf("matched wrong target: %+v", res.ExactSingle)
		}
	})

	// Case 2: Exact match - tokens in reversed/arbitrary order: user prod mongo
	t.Run("ArbitraryOrder", func(t *testing.T) {
		res := SmartMatch([]string{"user", "prod", "mongo"}, allTargets, reg)
		if res.ExactSingle == nil {
			t.Fatalf("expected exact single match, got nil")
		}
		if res.ExactSingle.EnvKey != "prod" || res.ExactSingle.InstanceKey != "user" {
			t.Errorf("matched wrong target: %+v", res.ExactSingle)
		}
	})

	// Case 3: Exact match with alias: mongodb prod user
	t.Run("ServiceAlias", func(t *testing.T) {
		res := SmartMatch([]string{"mongodb", "prod", "user"}, allTargets, reg)
		if res.ExactSingle == nil {
			t.Fatalf("expected exact single match using service alias 'mongodb'")
		}
	})

	// Case 4: Partial 2 tokens with unique result: prod api
	t.Run("PartialUnique", func(t *testing.T) {
		res := SmartMatch([]string{"prod", "api"}, allTargets, reg)
		if res.ExactSingle == nil {
			t.Fatalf("expected exact single match for prod api")
		}
		if res.ExactSingle.InstanceKey != "api" {
			t.Errorf("expected instance api, got %s", res.ExactSingle.InstanceKey)
		}
	})

	// Case 5: Partial 2 tokens with multiple candidates: prod mongo
	t.Run("PartialMultipleCandidates", func(t *testing.T) {
		res := SmartMatch([]string{"prod", "mongo"}, allTargets, reg)
		if res.ExactSingle != nil {
			t.Errorf("expected multiple candidates, got single match")
		}
		if len(res.Candidates) != 2 { // prod mongo user and prod mongo order
			t.Fatalf("expected 2 candidates, got %d", len(res.Candidates))
		}
	})

	// Case 6: Single token matching multiple targets: user
	t.Run("SingleTokenUser", func(t *testing.T) {
		res := SmartMatch([]string{"user"}, allTargets, reg)
		if res.ExactSingle != nil {
			t.Errorf("expected multiple candidates, got single match")
		}
		if len(res.Candidates) != 3 { // prod mongo user, prod mysql user, uat mongo user
			t.Fatalf("expected 3 candidates, got %d", len(res.Candidates))
		}
	})
}
