# hop

> ⚡ **Fast, Secure & Extensible Multi-Environment Connection CLI for Engineering Teams**  
> Connect to MongoDB, MySQL, GCP VMs, and more in seconds with zero-plaintext security and smart token routing.

English | [繁體中文](README.zh-TW.md)

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 🌟 Key Features

- 🔒 **Zero-Plaintext Security (AI Agent Safe)**: No credentials or secrets are ever stored in local config files. Secrets are retrieved dynamically at connection time from **Bitwarden**, **1Password**, **GCP Secret Manager**, or **GCP Cloud KMS** and injected in-memory/environment, safe from local file-reading scripts or AI agents.
- ⚡ **Dual Engine (Go SDK & CLI Mode)**: Supports high-performance in-process **Go Official SDKs** (sub-millisecond latency, zero-process spawning) and local **gcloud CLI** execution, seamlessly switchable via `mode: "sdk"` or `mode: "gcloud"`.
- 🎯 **Smart Token Routing**: Order-independent instant connection. Running `hop mongo prod user` or `hop prod user mongo` automatically resolves the target environment, service, and instance.
- ⚡ **Service Defaults Inheritance**: Define shared defaults (such as `port`, `user`, or `auth_source`) at the service level in YAML so individual cluster configs remain lightweight.
- 🎨 **Modern Interactive TUI**:
  - **Hierarchical Drilldown (Default)**: Step-by-step selection (Environment ➔ Service ➔ Instance).
  - **Flat Fuzzy Search**: Pass `-F` or `--flat` for instantaneous keyword filtering across all targets.
- 🔌 **Pluggable Architecture**: Built on clean Go interfaces. Adding new connectors such as **GKE, Redis, or PostgreSQL** requires implementing just a single driver file.

---

## 🏛 Architecture

```text
┌────────────────────────────────────────────────────────────────────────┐
│                     CLI Entrypoint (hop)                         │
│  - Smart Token Matcher (e.g., `hop mongo prod user`)              │
│  - Interactive TUI Selection (Hierarchical Drilldown / Flat Fuzzy)     │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                       ┌────────────┴────────────┐
                       │   Config & Target Model │
                       │    (Defaults Merge)     │
                       └────────────┬────────────┘
                                    │
             ┌──────────────────────┴──────────────────────┐
             ▼                                             ▼
┌──────────────────────────┐                 ┌──────────────────────────┐
│   Secret Providers       │                 │   Connectors (Drivers)   │
├──────────────────────────┤                 ├──────────────────────────┤
│ - Bitwarden (`bw`)       │ ──(In-Memory)──▶│ - GCP VM (`gcloud ssh`)  │
│ - 1Password (`op`)       │                 │ - MySQL CLI (`mysql`)    │
│ - GCP Secret Manager     │                 │ - MongoDB Atlas (`mongosh)
│ - [Extensible Provider]  │                 │ - [Extensible: GKE, ...] │
└──────────────────────────┘                 └──────────────────────────┘
```

---

## 🚀 Quick Start

### 1. Installation

#### 🍏 macOS / 🐧 Linux:
```bash
# Clone repository
git clone https://github.com/chentinghsuan/hop.git
cd hop

# Build and install to ~/.local/bin (ensure ~/.local/bin is in your PATH)
make install
```
> 💡 To install to a different directory, override `INSTALL_DIR`: `make install INSTALL_DIR=/usr/local/bin`

#### 🪟 Windows (PowerShell / CMD):
```powershell
# Clone repository
git clone https://github.com/chentinghsuan/hop.git
cd hop

# Build binary
go build -ldflags="-s -w" -o hop.exe ./cmd/hop
```
> 💡 If `make` is installed, you can also run `make build-windows`. Place `hop.exe` in any folder included in your system `PATH`.

### 2. Initialize Configuration

```bash
hop init
```
- **macOS / Linux**: Created at `~/.config/hop/config.yaml`.
- **Windows**: Created at `%APPDATA%\hop\config.yaml` (also backward compatible with `%USERPROFILE%\.config\hop\config.yaml`).
- To overwrite an existing configuration, pass `--force` / `-f`.

### 3. Dependency & Auth Health Check

```bash
hop check
```
Verifies local CLI dependencies (`gcloud`, `mysql`, `mongosh`, `bw`, etc.) and vault authentication status.

---

## 💡 Usage

### 1. Smart Token Connect
Token order does not matter:
```bash
# 3 tokens: exact match [prod] mongo > user
hop mongo prod user
hop prod user mongo

# 2 tokens: connects instantly if unique; opens candidates menu if ambiguous
hop gcp prod api
hop prod mysql

# 1 token: opens menu filtered by keyword
hop user
```

### 2. Interactive Selection Mode

- **Hierarchical Drilldown (Default)**: Run without arguments to drill down (Environment ➔ Service ➔ Instance):
  ```bash
  hop
  ```
- **Flat Fuzzy Search**: Use `-F` or `--flat` to list all targets with real-time fuzzy search:
  ```bash
  hop -F
  # or
  hop --flat
  ```

### 3. List All Targets
```bash
hop list
# or
hop -l
```
Prints an ASCII table displaying all configured environments, services, instances, descriptions, and secret providers.

---

## ⚙️ Configuration Example (`~/.config/hop/config.yaml`)

```yaml
version: "1"

# Global default secret provider (options: bitwarden, 1password, gcp)
default_secret_provider: bitwarden

environments:
  prod:
    name: "Production Environment"

    # MongoDB / Atlas
    # Command: hop mongo prod user
    mongo:
      defaults:
        port: 27017
        auth_source: "admin"
      user:
        description: "User profile cluster"
        host: "user-mongo.company.internal"
        user: "admin"
        secret:
          item: "prod-mongo-user" # Vault item name

      order:
        description: "Order & transaction cluster"
        host: "order-mongo.company.internal"
        user: "admin"
        secret:
          item: "prod-mongo-order"

      analytics:
        description: "Atlas Analytics Cluster"
        host: "analytics-prod.mongodb.net"
        srv: true # Enables mongodb+srv://
        user: "dw_reader"
        database: "dw_analytics"
        secret:
          item: "prod-mongo-analytics"

    # MySQL
    # Command: hop mysql prod user
    mysql:
      user:
        description: "User microservice primary database"
        host: "user-db.company.internal"
        port: 3306
        user: "app_user"
        database: "user_service"
        secret:
          item: "prod-mysql-user"

    # GCP Compute Engine VMs
    # Command: hop gcp prod api
    gcp:
      api:
        description: "Production API gateway"
        project: "company-prod-gcp"
        zone: "asia-east1-b"
        instance: "prod-api-server"
        tunnel_through_iap: true
```

---

## 🔐 Secret Providers Setup

### 1. Bitwarden (`bw`)
1. Install CLI: `brew install bitwarden-cli`
2. Log in: `bw login`
3. Unlock vault session:
   ```bash
   export BW_SESSION=$(bw unlock --raw)
   ```
   > 💡 With `BW_SESSION` set, credentials are provided directly in memory. No plaintext files are written to disk.

### 2. 1Password (`op`)
1. Install CLI: `brew install --cask 1password-cli`
2. Supports native `op://<vault>/<item>/<field>` URI syntax or simple item names.

### 3. GCP Secret Manager (`gcloud`)
1. Authenticate via `gcloud auth login`.
2. The config's `item` maps to the GCP Secret ID (defaults to the `latest` version).

---

## 🛠 Adding a New Connector

Thanks to the decoupled driver interface, adding a new connector (e.g. **GKE**) requires just 3 steps:

1. Create `internal/connector/gke.go`:
   ```go
   package connector

   import (
       "context"
       "github.com/ting-hsuanchen/hop/internal/config"
       "github.com/ting-hsuanchen/hop/internal/secret"
   )

   type GKEConnector struct{}

   func NewGKEConnector() *GKEConnector { return &GKEConnector{} }
   func (c *GKEConnector) Type() string { return "gke" }
   func (c *GKEConnector) Aliases() []string { return []string{"k8s", "kube"} }
   func (c *GKEConnector) ValidateDependencies() error { /* Check kubectl / gcloud */ return nil }
   func (c *GKEConnector) Connect(ctx context.Context, target config.ResolvedTarget, sec secret.Resolver) error {
       // Execute gcloud container clusters get-credentials or kubectl exec
       return nil
   }
   ```
2. Register the connector in `cmd/hop/main.go`:
   ```go
   connRegistry := connector.NewRegistry(
       connector.NewGCPVMConnector(),
       connector.NewMySQLConnector(),
       connector.NewMongoConnector(),
       connector.NewGKEConnector(), // 👈 Registered here
   )
   ```
3. Add targets under `gke` in `config.yaml`, and connect with `hop gke prod cluster-01`!

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
