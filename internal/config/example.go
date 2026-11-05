package config

// DefaultExampleConfig provides a clean, simplified YAML configuration.
const DefaultExampleConfig = `# ==============================================================================
# hop Configuration File
#
# Location: ~/.config/hop/config.yaml (Windows: %APPDATA%\hop\config.yaml)
# Format:   environments.<env>.<service>.<instance>
# ==============================================================================

version: "1"

# Default secret provider when not explicitly set on a target.
# Supported: bitwarden, 1password, gcp, gcp_kms, env
default_secret_provider: bitwarden

environments:
  # ----------------------------------------------------------------------------
  # 1. Production - MongoDB Sample
  # Connect via: hop prod mongo sample
  # ----------------------------------------------------------------------------
  prod:
    name: "Production"
    mongo:
      sample:
        description: "Sample MongoDB cluster"
        host: "mongo.company.internal"
        port: 27017
        user: "admin"
        auth_source: "admin"
        secret:
          item: "prod-mongo-sample"

  # ----------------------------------------------------------------------------
  # 2. Pre-Production - MySQL Sample
  # Connect via: hop pre-prod mysql sample
  # ----------------------------------------------------------------------------
  pre-prod:
    name: "Pre-Production"
    mysql:
      sample:
        description: "Sample MySQL database"
        host: "mysql-preprod.company.internal"
        port: 3306
        user: "app_user"
        database: "sample_db"
        secret:
          item: "preprod-mysql-sample"

  # ----------------------------------------------------------------------------
  # 3. UAT - GCP VM Sample
  # Connect via: hop uat gcp sample
  # ----------------------------------------------------------------------------
  uat:
    name: "UAT"
    gcp:
      sample:
        description: "Sample GCP Compute Engine VM"
        project: "company-uat-gcp"
        zone: "asia-east1-a"
        instance: "uat-sample-vm"
        tunnel_through_iap: true
`
