# Contributing to hop

Thank you for your interest in contributing to **hop**! We welcome contributions, bug fixes, feature requests, and new service connectors.

---

## 🧭 Design Principles

Before making any changes, please keep in mind our core design philosophy:
- **KISS (Keep It Simple, Stupid)**: Keep `hop` lean and lightning-fast. Avoid bloated heuristic fallbacks or heavy magic.
- **Zero-Plaintext Security**: Never store plaintext passwords in local configuration files or print credentials in error logs/terminal traces.
- **Deterministic & Backward Compatible**: Ensure that configuration order and existing user workflows remain unbroken.

---

## 🛠 Local Development & Testing

### Prerequisites
- **Go**: Version 1.21 or higher
- **Make**: (Optional, for running Makefile tasks)

### Common Commands
```bash
# Build binary to project root
make build
# or: go build -ldflags="-s -w" -o hop ./cmd/hop

# Run all unit tests
make test
# or: go test -v ./...

# Run code vet / static check
go vet ./...

# Install to ~/.local/bin
make install
```

> 💡 **About `*_test.go` files**: In Go, all test files are named `*_test.go`. They are automatically detected by `go test` and **completely excluded** by the Go compiler when building production binaries (`go build`), keeping binary size minimal.

---

## 🔌 How to Add a New Connector

`hop` is built with a decoupled driver interface. Adding support for a new service (such as **GKE / Kubernetes**, **Redis**, or **PostgreSQL**) requires just 3 simple steps:

### Step 1: Implement the `Connector` Interface
Create a new driver file in `internal/connector/` (e.g., `internal/connector/gke.go`):

```go
package connector

import (
	"context"
	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/secret"
)

type GKEConnector struct{}

func NewGKEConnector() *GKEConnector { return &GKEConnector{} }

// Type returns the canonical service type in config.yaml (e.g. environments.prod.gke)
func (c *GKEConnector) Type() string { return "gke" }

// Aliases returns alternative CLI routing tokens
func (c *GKEConnector) Aliases() []string { return []string{"k8s", "kube"} }

// ValidateDependencies verifies local CLI tools (e.g., kubectl, gcloud)
func (c *GKEConnector) ValidateDependencies() error {
	// Check if kubectl or gcloud is in PATH
	return nil
}

// Connect executes the connection process
func (c *GKEConnector) Connect(ctx context.Context, target config.ResolvedTarget, sec secret.Resolver) error {
	// e.g., execute gcloud container clusters get-credentials or kubectl exec
	return nil
}
```

### Step 2: Register in `cmd/hop/main.go`
Add your new connector to `connRegistry`:

```go
connRegistry := connector.NewRegistry(
    connector.NewGCPVMConnector(),
    connector.NewMySQLConnector(),
    connector.NewMongoConnector(),
    connector.NewGKEConnector(), // 👈 Register new connector here
)
```

### Step 3: Add Target in `config.yaml` and Connect!
Configure your target under the registered service type:

```yaml
environments:
  prod:
    gke:
      cluster-01:
        description: "Main production Kubernetes cluster"
        project: "company-prod-gcp"
        region: "asia-east1"
        cluster: "prod-k8s-cluster"
```

You can now instantly connect via:
```bash
hop gke prod cluster-01
```

---

## 📬 Pull Request Guidelines

1. **Keep PRs focused**: Each PR should address a single feature or bug fix.
2. **Add unit tests**: If you add new logic in `internal/config`, `internal/secret`, or `internal/matcher`, please include corresponding `*_test.go` tests.
3. **Run tests before submitting**: Ensure `go test ./...` and `go vet ./...` pass cleanly.

