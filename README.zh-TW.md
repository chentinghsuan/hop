# hop

> ⚡ **Fast, Secure & Extensible Multi-Environment Connection CLI for Engineering Teams**  
> 一鍵秒連 MongoDB、MySQL、GCP VM 等多環境與服務，具備零明文資安架構與智慧權杖路由。

[English](README.md) | 繁體中文

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 🌟 核心特色 (Key Features)

- 🔒 **AI Agent 安全防護與零明文設定 (Zero-Plaintext & Zero-Credential)**：本機設定檔完全不存放任何密碼。機敏資訊在連線瞬間直接由 **Bitwarden**、**1Password**、**GCP Secret Manager**、**GCP Cloud KMS** 或 **環境變數 / `.env`** 注入記憶體。當 Secret 以 JSON `{"username": "...", "password": "..."}` 儲存時，系統自動解析帳號與密碼，設定檔可完全不寫帳號密碼，連本機 AI Agent 與讀檔腳本都無法窺探。
- ⚡ **雙模式高效存取 (SDK & CLI Dual Mode)**：針對 GCP Secret Manager 與 KMS，支援原生高吞吐量 **Go 官方 SDK**（低延遲、免開 Python 子進程）與本機 **gcloud CLI** 雙模式，可在設定檔中彈性切換 `mode: "sdk"` 或 `mode: "gcloud"`。
- 🎯 **智慧權杖路由 (Smart Token Matcher)**：支援無序快速連線，輸入 `hop mongo prod user` 或 `hop prod user mongo` 順序任意，系統自動解析環境、服務與實例；若有多筆相符則開啟選單過濾。
- ⚡ **預設屬性繼承 (Service Defaults Inheritance)**：支援在服務層級設定 `defaults`（例如共用的 `port`、`user`、`auth_source`），各 Cluster / Database 只要專注定義差異欄位即可。
- 🎨 **現代互動選單 (Interactive TUI Menu)**：
  - **預設階層選單**：依序鑽取選擇（環境 Environment ➔ 服務 Service ➔ 目標 Instance），支援 **`← 返回上一步 (Back)`** 隨時退回重選。
  - **游標懸停預取 (Hover Prefetching)**：游標懸停選項時背景自動非同步預先暖機快取 Secret，按下 Enter 連線零等待。
  - **扁平模糊搜尋**：搭配 `-F` / `--flat` 即時模糊過濾所有目標。
- 🔌 **極度易擴充 (Pluggable Connectors)**：基於 Go 介面抽象與工廠註冊模式，未來擴充 **GKE、Redis、PostgreSQL** 只要實作單一檔案即可無痛接入。相容 TiDB、MariaDB、AWS Aurora 等 MySQL 協定資料庫。

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
│ - 1Password (`op`)       │                 │ - MySQL / TiDB (`mysql`) │
│ - GCP Secret Manager     │                 │ - MongoDB Atlas (`mongosh)
│ - GCP Cloud KMS          │                 │ - [Extensible: GKE, ...] │
│ - Environment (`env`)    │                 │                          │
└──────────────────────────┘                 └──────────────────────────┘
```

---

## 🚀 快速開始 (Quick Start)

### 1. 安裝 (Installation)

#### 🍏 macOS / 🐧 Linux:
```bash
# Clone 專案
git clone https://github.com/chentinghsuan/hop.git
cd hop

# 編譯並安裝至 ~/.local/bin (請確認 ~/.local/bin 已加入系統 PATH)
make install
```
> 💡 若需安裝到其他目錄，可指定 `INSTALL_DIR`，例如：`make install INSTALL_DIR=/usr/local/bin`

#### 🪟 Windows (PowerShell / CMD):
```powershell
# Clone 專案
git clone https://github.com/chentinghsuan/hop.git
cd hop

# 編譯出 hop.exe
go build -ldflags="-s -w" -o hop.exe ./cmd/hop
```
> 💡 若有安裝 `make`，也可直接執行 `make build-windows`。編譯後的 `hop.exe` 可放置於任何已加入系統 `PATH` 的目錄中。

### 2. 初始化設定檔 (Initialize Config)

```bash
hop init
```
- **macOS / Linux**：預設建立於 `~/.config/hop/config.yaml`。
- **Windows**：預設建立於 `%APPDATA%\hop\config.yaml`（同時自動向下相容 `%USERPROFILE%\.config\hop\config.yaml`）。
- 若欲覆蓋現有檔案，可加上 `--force` / `-f`。

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

    # MySQL / MariaDB 設定
    # 連線指令: hop mysql prod user
    mysql:
      user:
        description: "User microservice primary database"
        host: "user-db.company.internal"
        port: 3306
        user: "app_user"
        database: "user_service"
        extra_args: ["--ssl-mode=VERIFY_IDENTITY"] # 選填：自訂 CLI 附加參數
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

### 零帳密自動推導 (Zero-Credential Auto-Inference)
當保險庫或 Secret Manager 儲存的內容同時包含使用者名稱與密碼時，`hop` 會自動解析帳號與密碼，您在 `config.yaml` 中可直接省略 `user:`，達到本機設定檔完全零帳號、零密碼的最高資安規格。

### 1. Bitwarden (`bw`)
1. 安裝 CLI：`brew install bitwarden-cli`（Windows: `winget install Bitwarden.CLI`）
2. 登入帳號：`bw login`
3. 工作階段解鎖：
   ```bash
   export BW_SESSION=$(bw unlock --raw)
   # Windows PowerShell:
   # $env:BW_SESSION = (bw unlock --raw)
   ```
   > 💡 只要設定 `BW_SESSION`，連線時密碼會在記憶體向 `bw` 取得，不會在硬碟產生任何明文檔案。若 `config.yaml` 未設定 `user`，系統會自動抓取項目中的帳號名稱。

### 2. 1Password (`op`)
1. 安裝 CLI：`brew install --cask 1password-cli`
2. 支援 `op://<vault>/<item>/<field>` 原生路徑語法或 Item 名稱。若省略 `user`，系統亦會自動獲取使用者名稱。

### 3. GCP Secret Manager (`gcp`)
1. 驗證認證：
   - **Go SDK 模式（預設）**：執行 `gcloud auth application-default login`，享有低延遲、無子進程的高效能存取。
   - **gcloud CLI 模式**：執行 `gcloud auth login`（設定檔中指定 `mode: "gcloud"`）。
2. 設定檔中的 `item` 對應 GCP Secret ID（預設讀取 `latest` 版本）。
3. 若 Secret 內容為 JSON 格式：
   ```json
   {
     "username": "my_db_user",
     "password": "my_super_secret_password"
   }
   ```
   `hop` 會自動同時解析帳號與密碼，設定檔完全不需要指定 `user:` 或 `field:`。

### 4. GCP Cloud KMS (`gcp_kms`)
若希望將密鑰直接加密後貼在 `config.yaml`（僅擁有 GCP KMS 解密權限的工程師能連線）：
1. 使用 `hop encrypt` 加密機敏字串：
   ```bash
   hop encrypt --key projects/<PROJECT>/locations/<LOCATION>/keyRings/<RING>/cryptoKeys/<KEY> --plaintext "my_secret_password"
   ```
2. 將輸出的密文貼入設定檔：
   ```yaml
   secret:
     provider: gcp_kms
     key: "projects/<PROJECT>/locations/<LOCATION>/keyRings/<RING>/cryptoKeys/<KEY>"
     ciphertext: "CiQA...=="
   ```

### 5. 環境變數與 `.env` (`env`)
直接從本機環境變數或 `.env` 檔案載入機敏資料：
```yaml
secret:
  provider: env
  item: "MY_DATABASE_SECRET" # 讀取 $MY_DATABASE_SECRET
```
- 自動搜尋執行目錄下的 `.env` 或 `~/.config/hop/.env`（Windows: `%APPDATA%\hop\.env`）。
- 同樣支援 JSON `{"username": "...", "password": "..."}` 格式以省略 `user:`。

---

## 🤝 參與貢獻 (Contributing)

非常歡迎提交 Issue、建議或 Pull Request！  
若您想為 hop 新增服務連線器（如 Redis、PostgreSQL、GKE 等）或改善功能，請參考完整的 [貢獻指南 (CONTRIBUTING.md)](CONTRIBUTING.md)。

---

## 📄 授權 (License)

本專案採用 [MIT License](LICENSE) 授權。

