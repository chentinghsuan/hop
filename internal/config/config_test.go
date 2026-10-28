package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestResolveTargetsSimplified(t *testing.T) {
	yamlData := `
version: "1"
default_secret_provider: bitwarden

environments:
  prod:
    name: "Production"
    mongo:
      user:
        description: "User Cluster"
        host: "user-mongo.company.internal"
        port: 27017
        user: "admin"
        secret:
          item: "prod-mongo-user"
      analytics:
        host: "analytics.mongodb.net"
        srv: true
        user: "dw"
        secret:
          provider: 1password
          item: "op://analytics/pwd"
    mysql:
      order:
        host: "order-db.internal"
        port: 3306
        user: "order_app"
        database: "order_prod"
        secret:
          item: "prod-mysql-order"
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("failed to unmarshal yaml: %v", err)
	}

	targets, err := cfg.ResolveTargets()
	if err != nil {
		t.Fatalf("unexpected error resolving targets: %v", err)
	}

	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}

	var userMongo, analyticsMongo, orderMySQL *ResolvedTarget
	for i := range targets {
		tgt := &targets[i]
		if tgt.ServiceType == "mongo" && tgt.InstanceKey == "user" {
			userMongo = tgt
		}
		if tgt.ServiceType == "mongo" && tgt.InstanceKey == "analytics" {
			analyticsMongo = tgt
		}
		if tgt.ServiceType == "mysql" && tgt.InstanceKey == "order" {
			orderMySQL = tgt
		}
	}

	if userMongo == nil || analyticsMongo == nil || orderMySQL == nil {
		t.Fatalf("missing target in resolved list")
	}

	// Verify userMongo
	if host, _ := userMongo.Properties["host"].(string); host != "user-mongo.company.internal" {
		t.Errorf("expected host 'user-mongo.company.internal', got %q", host)
	}
	if userMongo.Secret.Item != "prod-mongo-user" {
		t.Errorf("expected secret item 'prod-mongo-user', got %q", userMongo.Secret.Item)
	}
	if userMongo.Secret.Provider != "bitwarden" {
		t.Errorf("expected secret provider 'bitwarden', got %q", userMongo.Secret.Provider)
	}

	// Verify analyticsMongo
	if srv, _ := analyticsMongo.Properties["srv"].(bool); !srv {
		t.Errorf("expected srv to be true")
	}
	if analyticsMongo.Secret.Provider != "1password" {
		t.Errorf("expected secret provider '1password', got %q", analyticsMongo.Secret.Provider)
	}
	if analyticsMongo.Secret.Item != "op://analytics/pwd" {
		t.Errorf("expected secret item 'op://analytics/pwd', got %q", analyticsMongo.Secret.Item)
	}

	// Verify orderMySQL
	if db, _ := orderMySQL.Properties["database"].(string); db != "order_prod" {
		t.Errorf("expected db 'order_prod', got %q", db)
	}
}
