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

func TestResolveTargets_DeterministicAndDeepMerge(t *testing.T) {
	rawYAML := `
version: "1.0"
default_secret_provider: bitwarden
environments:
  stage:
    name: Staging
    mysql:
      defaults:
        host: 127.0.0.1
        port: 3306
        secret:
          provider: gcp_kms
          key: "projects/my-prj/locations/global/keyRings/ring/cryptoKeys/db-key"
      b_order:
        database: order_db
        secret:
          ciphertext: "CIPHER_ORDER"
      a_user:
        database: user_db
        secret:
          ciphertext: "CIPHER_USER"
  prod:
    name: Production
    mongo:
      user:
        uri: "mongodb://localhost:27017"
        secret:
          provider: gcp
          item: "prod-mongo-user"
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(rawYAML), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	targets, err := cfg.ResolveTargets()
	if err != nil {
		t.Fatalf("ResolveTargets failed: %v", err)
	}

	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}

	// 1. Verify document preservation order: stage/b_order -> stage/a_user -> prod/user
	if targets[0].EnvKey != "stage" || targets[0].InstanceKey != "b_order" {
		t.Errorf("expected first target to be stage/b_order (file order), got %s/%s", targets[0].EnvKey, targets[0].InstanceKey)
	}
	if targets[1].EnvKey != "stage" || targets[1].InstanceKey != "a_user" {
		t.Errorf("expected second target to be stage/a_user (file order), got %s/%s", targets[1].EnvKey, targets[1].InstanceKey)
	}
	if targets[2].EnvKey != "prod" || targets[2].InstanceKey != "user" {
		t.Errorf("expected third target to be prod/user (file order), got %s/%s", targets[2].EnvKey, targets[2].InstanceKey)
	}

	// 2. Verify deep merge of defaults.secret with instance secret
	kmsTarget := targets[1]
	if kmsTarget.Secret.Provider != "gcp_kms" {
		t.Errorf("expected inherited provider 'gcp_kms', got %q", kmsTarget.Secret.Provider)
	}
	if kmsTarget.Secret.Key != "projects/my-prj/locations/global/keyRings/ring/cryptoKeys/db-key" {
		t.Errorf("expected inherited KMS key, got %q", kmsTarget.Secret.Key)
	}
	if kmsTarget.Secret.Ciphertext != "CIPHER_USER" {
		t.Errorf("expected instance ciphertext 'CIPHER_USER', got %q", kmsTarget.Secret.Ciphertext)
	}
	if !kmsTarget.Secret.HasSecret() {
		t.Errorf("expected HasSecret() to be true for KMS target")
	}
	if got := kmsTarget.Secret.Summary(); got != "gcp_kms (key:db-key)" {
		t.Errorf("expected Summary() 'gcp_kms (key:db-key)', got %q", got)
	}

	// 3. Verify empty default Field so plain secrets in GCP/KMS work without JSON parsing errors
	if kmsTarget.Secret.Field != "" {
		t.Errorf("expected empty default Field, got %q", kmsTarget.Secret.Field)
	}
}
