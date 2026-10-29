package config

// DefaultExampleConfig provides a clean, simplified YAML configuration.
const DefaultExampleConfig = `# ==============================================================================
# hop Configuration File
#
# Location: ~/.config/hop/config.yaml
# Format:   environments.<env>.<service>.<instance>
# ==============================================================================

version: "1"

# Default secret provider when not explicitly set on a target.
# Supported: bitwarden, 1password, gcp
default_secret_provider: bitwarden

environments:
  # ----------------------------------------------------------------------------
  # Production Environment
  # ----------------------------------------------------------------------------
  prod:
    name: "Production (線上正式環境)"

    # MongoDB / Atlas Clusters
    # Connect via: hop mongo prod user
    mongo:
      user:
        description: "User profile cluster"
        host: "user-mongo.company.internal"
        port: 27017
        user: "admin"
        auth_source: "admin"
        secret:
          item: "prod-mongo-user" # Bitwarden item name

      order:
        description: "Order & transaction cluster"
        host: "order-mongo.company.internal"
        port: 27017
        user: "admin"
        auth_source: "admin"
        secret:
          item: "prod-mongo-order"

      analytics:
        description: "Atlas Analytics Cluster"
        host: "analytics-prod.abcde.mongodb.net"
        srv: true # Enable mongodb+srv://
        user: "dw_reader"
        database: "dw_analytics"
        secret:
          item: "prod-mongo-analytics"

    # MySQL Databases
    # Connect via: hop mysql prod user
    mysql:
      user:
        description: "User microservice primary database"
        host: "user-db.company.internal"
        port: 3306
        user: "app_user"
        database: "user_service"
        secret:
          provider: "gcp_secret_manager" # or "gcp"
          item: "prod-mysql-credentials"
          project: "company-prod-gcp"
          field: "password" # JSON field extraction
          mode: "sdk"       # "sdk" (fastest, default) or "gcloud"

      billing:
        description: "Billing database protected by Cloud KMS"
        host: "billing-db.company.internal"
        port: 3306
        user: "billing_app"
        database: "billing"
        secret:
          provider: "gcp_kms" # Encrypted locally with Cloud KMS
          key: "projects/company-prod-gcp/locations/global/keyRings/db-ring/cryptoKeys/db-key"
          ciphertext: "CiQA5n3sX...==" # Generated via 'hop encrypt --key ...'
          mode: "sdk"

    # GCP Compute Engine VMs
    # Connect via: hop gcp prod api
    gcp:
      api:
        description: "Production API gateway"
        project: "company-prod-gcp"
        zone: "asia-east1-b"
        instance: "prod-api-server"
        tunnel_through_iap: true

  # ----------------------------------------------------------------------------
  # UAT Environment
  # ----------------------------------------------------------------------------
  uat:
    name: "UAT (測試驗證環境)"

    mongo:
      user:
        description: "UAT User Mongo"
        host: "uat-mongo.company.internal"
        port: 27017
        user: "uat_admin"
        secret:
          item: "uat-mongo-user"

    mysql:
      user:
        description: "UAT User MySQL"
        host: "uat-db.company.internal"
        port: 3306
        user: "uat_user"
        database: "user_service_uat"
        secret:
          item: "uat-mysql-user"

    gcp:
      testbox:
        description: "UAT Testing VM"
        project: "company-uat-gcp"
        zone: "asia-east1-a"
        instance: "uat-testbox-01"
`
