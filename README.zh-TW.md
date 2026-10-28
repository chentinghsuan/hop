# hop

> ⚡ **Fast, Secure & Extensible Multi-Environment Connection CLI for Engineering Teams**  
> 一鍵秒連 MongoDB、MySQL、GCP VM 等多環境與服務，具備零明文資安架構與智慧權杖路由。

[English](README.md) | 繁體中文

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 🌟 核心特色 (Key Features)

- 🔒 **AI Agent 安全防護 (Zero-Plaintext Security)**：本機設定檔完全不存放任何密碼。機敏資訊在連線瞬間直接由 **Bitwarden**、**1Password** 或 **GCP Secret Manager** 注入記憶體與進程環境，連本機 AI Agent 與讀檔腳本都無法窺探。
- 🎯 **智慧權杖路由 (Smart Token Matcher)**：支援無序快速連線，輸入 `hop mongo prod user` 或 `hop prod user mongo` 順序任意，系統自動解析環境、服務與實例；若有多筆相符則開啟選單過濾。
- ⚡ **預設屬性繼承 (Service Defaults Inheritance)**：支援在服務層級設定 `defaults`（例如共用的 `port`、`user`、`auth_source`），各 Cluster / Database 只要專注定義差異欄位即可。
- 🎨 **現代互動選單 (Interactive TUI Menu)**：
  - **預設階層選單**：依序鑽取選擇（環境 Environment ➔ 服務 Service ➔ 目標 Instance）。
  - **扁平模糊搜尋**：搭配 `-F` / `--flat` 即時模糊過濾所有目標。
- 🔌 **極度易擴充 (Pluggable Connectors)**：基於 Go 介面抽象與工廠註冊模式，未來擴充 **GKE、Redis、PostgreSQL** 只要實作單一檔案即可無痛接入。

---

## 🏛 架構設計 (Architecture)

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

## 🚀 快速開始 (Quick Start)

### 1. 安裝 (Installation)

```bash
# Clone 專案
git clone https://github.com/ting-hsuanchen/hop.git
cd hop

# 編譯並安裝至 ~/.local/bin (請確認 ~/.local/bin 已加入系統 PATH)
make install
```

> 💡 若需安裝到其他目錄，可指定 `INSTALL_DIR`，例如：`make install INSTALL_DIR=/usr/local/bin`

### 2. 初始化設定檔 (Initialize Config)

```bash
hop init
```
此指令會在 `~/.config/hop/config.yaml` 建立初學者範本設定檔。若欲覆蓋現有檔案，可加上 `--force` / `-f`。

### 3. 檢查環境相依性 (Health Check)

```bash
hop check
```
自動檢查本機 `gcloud`、`mysql`、`mongosh`、`bw` 等相依工具安裝狀態與密鑰庫認證狀態。

---

## 💡 使用方式 (Usage)

### 1. 智慧無序參數直連 (Smart Token Connect)
所有參數順序不拘，輸入即可秒連：
```bash
# 三個參數：精準命中 [prod] mongo > user
hop mongo prod user
hop prod user mongo

# 兩個參數：若唯一則秒連，若有多個則開啟候選選單
hop gcp prod api
hop prod mysql

# 單一關鍵字：自動過濾出包含此關鍵字的目標選單
hop user
```

### 2. 互動式選單 (Interactive Mode)

- **預設模式（階層式選單）**：直接執行不帶任何參數，以三階段階梯式選單選擇：
  ```bash
  hop
  ```
- **扁平模糊搜尋模式**：加上 `-F` 或 `--flat`，在單一選單中列出全部目標並支援關鍵字即時模糊搜尋：
  ```bash
  hop -F
  # 或
  hop --flat
  ```

### 3. 列出所有目標 (List Targets)
```bash
hop list
# 或
hop -l
```
以 ASCII 表格列出所有已配置的環境、服務、目標、描述與 Secret Provider 資訊。

---

## ⚙️ 設定檔範例 (`~/.config/hop/config.yaml`)

```yaml
version: "1"

# 全域預設 Secret Provider (支援: bitwarden, 1password, gcp)
default_secret_provider: bitwarden

environments:
  prod:
    name: "Production (線上正式環境)"

    # MongoDB / Atlas 設定
    # 連線指令: hop mongo prod user
    mongo:
      defaults:
        port: 27017
        auth_source: "admin"
      user:
        description: "User profile cluster"
        host: "user-mongo.company.internal"
        user: "admin"
        secret:
          item: "prod-mongo-user" # Bitwarden 項目名稱

      order:
        description: "Order & transaction cluster"
        host: "order-mongo.company.internal"
        user: "admin"
        secret:
          item: "prod-mongo-order"

      analytics:
        description: "Atlas Analytics Cluster"
        host: "analytics-prod.mongodb.net"
        srv: true # 啟用 mongodb+srv://
        user: "dw_reader"
        database: "dw_analytics"
        secret:
          item: "prod-mongo-analytics"

    # MySQL 設定
    # 連線指令: hop mysql prod user
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
    # 連線指令: hop gcp prod api
    gcp:
      api:
        description: "Production API gateway"
        project: "company-prod-gcp"
        zone: "asia-east1-b"
        instance: "prod-api-server"
        tunnel_through_iap: true
```

---

## 🔐 密鑰提供者設定 (Secret Providers)

### 1. Bitwarden (`bw`)
1. 安裝 CLI：`brew install bitwarden-cli`
2. 登入帳號：`bw login`
3. 工作階段解鎖：
   ```bash
   export BW_SESSION=$(bw unlock --raw)
   ```
   > 💡 只要設定 `BW_SESSION`，連線時密碼會在記憶體向 `bw` 取得，不會在硬碟產生任何明文檔案。

### 2. 1Password (`op`)
1. 安裝 CLI：`brew install --cask 1password-cli`
2. 支援 `op://<vault>/<item>/<field>` 原生路徑語法或 Item 名稱。

### 3. GCP Secret Manager (`gcloud`)
1. 透過 `gcloud auth login` 登入。
2. 設定檔中的 `item` 對應 GCP Secret ID，預設讀取 `latest` 版本。

---

## 🛠 如何擴充新連線器 (How to Add a New Connector)

本專案採用高度解耦的介面設計，若要新增 **GKE (Kubernetes)** 連線器，只需 3 個步驟：

1. 在 `internal/connector/` 建立 `gke.go`：
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
   func (c *GKEConnector) ValidateDependencies() error { /* 檢查 kubectl / gcloud */ return nil }
   func (c *GKEConnector) Connect(ctx context.Context, target config.ResolvedTarget, sec secret.Resolver) error {
       // 執行 gcloud container clusters get-credentials 或 kubectl exec
       return nil
   }
   ```
2. 在 `cmd/hop/main.go` 註冊：
   ```go
   connRegistry := connector.NewRegistry(
       connector.NewGCPVMConnector(),
       connector.NewMySQLConnector(),
       connector.NewMongoConnector(),
       connector.NewGKEConnector(), // 👈 註冊新連線器
   )
   ```
3. 在 `config.yaml` 加上 `gke` 的目標設定，即可直接使用 `hop gke prod cluster-01` 連線！

---

## 📄 授權 (License)

本專案採用 [MIT License](LICENSE) 授權。

