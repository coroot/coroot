---
sidebar_position: 1
slug: /configuration/configuration
---

# Configuration

Coroot can be configured using command-line arguments, environment variables, and a configuration file.

Configuration values are evaluated in the following precedence, with items higher on the list taking priority:
1. Command-line arguments
2. Environment variables
3. Configuration file parameters

:::info
Certain configuration values can only be set through command-line flags, while others are available only via configuration file. 
For instance, the `projects` parameter (a list of predefined projects) can only be configured via configuration file.
:::

## Command-line flags

| Argument                             | Environment Variable               | Default Value | Description                                                                                                                                                                     |
|--------------------------------------|------------------------------------|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| --config                             | CONFIG                             | 0.0.0.0:8080  | Configuration file.                                                                                                                                                             |
| --listen                             | LISTEN                             | 0.0.0.0:8080  | Listen address in the format `ip:port` or `:port`.                                                                                                                              |
| --url-base-path                      | URL_BASE_PATH                      | /             | Base URL to run Coroot at a sub-path, e.g., `/coroot/`.                                                                                                                         |
| --data-dir                           | DATA_DIR                           | /data         | Path to the data directory.                                                                                                                                                     |
| --cache-ttl                          | CACHE_TTL                          | 30d           | Metric Cache Time-To-Live (TTL).                                                                                                                                                |
| --cache-gc-interval                  | CACHE_GC_INTERVAL                  | 10m           | Metric Cache Garbage Collection (GC) interval.                                                                                                                                  |
| --traces-ttl                         | TRACES_TTL                         | 7d            | Traces Time-To-Live (TTL).                                                                                                                                                      |
| --logs-ttl                           | LOGS_TTL                           | 7d            | Logs Time-To-Live (TTL).                                                                                                                                                        |
| --profiles-ttl                       | PROFILES_TTL                       | 7d            | Profiles Time-To-Live (TTL).                                                                                                                                                    |
| --pg-connection-string               | PG_CONNECTION_STRING               |               | PostgreSQL connection string (uses SQLite if not set).                                                                                                                          |
| --disable-usage-statistics           | DISABLE_USAGE_STATISTICS           | false         | Disable usage statistics.                                                                                                                                                       |
| --read-only                          | READ_ONLY                          | false         | Enable read-only mode where configuration changes don't take effect.                                                                                                            |
| --do-not-check-slo                   | DO_NOT_CHECK_SLO                   | false         | Do not check Service Level Objective (SLO) compliance.                                                                                                                          |
| --do-not-check-for-deployments       | DO_NOT_CHECK_FOR_DEPLOYMENTS       | false         | Do not check for new deployments.                                                                                                                                               |
| --do-not-check-for-updates           | DO_NOT_CHECK_FOR_UPDATES           | false         | Do not check for new versions.                                                                                                                                                  |
| --auth-anonymous-role                | AUTH_ANONYMOUS_ROLE                |               | Disable authentication and assign one of the following roles to the anonymous user: Admin, Editor, or Viewer.                                                                   |
| --auth-bootstrap-admin-password      | AUTH_BOOTSTRAP_ADMIN_PASSWORD      |               | Password for the default Admin user.                                                                                                                                            |
| --license-key                        | LICENSE_KEY                        |               | License key for Coroot Enterprise Edition.                                                                                                                                      |
| --global-clickhouse-address          | GLOBAL_CLICKHOUSE_ADDRESS          |               | The address of the ClickHouse server to be used for all projects.                                                                                                               |
| --global-clickhouse-user             | GLOBAL_CLICKHOUSE_USER             |               | The username for the ClickHouse server to be used for all projects.                                                                                                             |
| --global-clickhouse-password         | GLOBAL_CLICKHOUSE_PASSWORD         |               | The password for the ClickHouse server to be used for all projects.                                                                                                             |
| --global-clickhouse-initial-database | GLOBAL_CLICKHOUSE_INITIAL_DATABASE |               | The initial database on the ClickHouse server to be used for all projects. Coroot will automatically create and manage a dedicated database for each project within the server. |
| --global-clickhouse-tls-enabled      | GLOBAL_CLICKHOUSE_TLS_ENABLED      | false         | Whether TLS is enabled for the ClickHouse server connection (true or false).                                                                                                    |
| --global-clickhouse-tls-skip-verify  | GLOBAL_CLICKHOUSE_TLS_SKIP_VERIFY  | false         | Whether to skip verification of the ClickHouse server's TLS certificate (true or false).                                                                                        |
| --global-prometheus-url              | GLOBAL_PROMETHEUS_URL              |               | The URL of the Prometheus server to be used for all projects.                                                                                                                   |
| --global-prometheus-tls-skip-verify  | GLOBAL_PROMETHEUS_TLS_SKIP_VERIFY  | false         | Whether to skip verification of the Prometheus server's TLS certificate (true or false).                                                                                        |
| --global-refresh-interval            | GLOBAL_REFRESH_INTERVAL            | 15s           | The interval for refreshing Prometheus data.                                                                                                                                    |
| --global-prometheus-user             | GLOBAL_PROMETHEUS_USER             |               | The username for the Prometheus server to be used for all projects.                                                                                                             |
| --global-prometheus-password         | GLOBAL_PROMETHEUS_PASSWORD         |               | The password for the Prometheus server to be used for all projects.                                                                                                             |
| --global-prometheus-custom-headers   | GLOBAL_PROMETHEUS_CUSTOM_HEADERS   |               | Custom headers to include in requests to the Prometheus server.                                                                                                                 |
| --global-prometheus-remote-write-url | GLOBAL_PROMETHEUS_REMOTE_WRITE_URL |               | The URL for metric ingestion though the Prometheus Remote Write protocol.                                                                                                       |
| --disable-clickhouse-space-manager   | CLICKHOUSE_SPACE_MANAGER_DISABLED  | false         | Disable ClickHouse space manager that automatically cleans up old partitions.                                                                                                   |
| --clickhouse-space-manager-usage-threshold | CLICKHOUSE_SPACE_MANAGER_USAGE_THRESHOLD | 70      | Disk usage percentage threshold for triggering partition cleanup in ClickHouse.                                                                                                 |
| --clickhouse-space-manager-min-partitions | CLICKHOUSE_SPACE_MANAGER_MIN_PARTITIONS | 1        | Minimum number of partitions to keep when cleaning up ClickHouse disk space.                                                                                                    |

## Configuration file

Use the `--config` flag to specify the configuration file to load. The file must be in YAML format.

```yaml
listen_address: 0.0.0.0:8080 # Listen address in the format `ip:port` or `:port`. 
url_base_path: /             # Base URL to run Coroot at a sub-path, e.g., `/coroot/`.
data_dir: /data              # Path to the data directory. 

cache:
  ttl: 30d        # Metric Cache Time-To-Live (TTL).
  gc_interval: 10m # Metric Cache Garbage Collection (GC) interval. 

# Coroot stores Traces, Logs, and Profiles in ClickHouse.  
# Their retention is managed by setting a Time-To-Live (TTL) for the corresponding Clickhouse tables.  
# The TTLs below are applied during table creation and do not currently affect existing tables.
traces:
  ttl: 7d
logs:
  ttl: 7d
profiles:
  ttl: 7d

postgres: # Store configuration in a Postgres DB instead of SQLite
  # URI form: "postgres://coroot:password@127.0.0.1:5432/coroot?sslmode=disable" 
  # KV form: "host=127.0.0.1 user=coroot password=password port=5432 dbname=coroot ssl_mode=disable"
  # https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
  connection_string: 

global_prometheus: # The Prometheus server to be used for all projects.
  url:                   # http(s)://IP:Port/ or http(s)://Domain:Port/
  refresh_interval: 15s  # The interval for refreshing Prometheus data.
  tls_skip_verify: false # Whether to skip verification of the Prometheus server's TLS certificate.
  user:                  # The basic-auth username.
  password:              # The basic-auth password.
  custom_headers:        # Custom headers to include in requests to the Prometheus server.
#    header_name: header_value
  remote_write_url:      # The URL for metric ingestion though the Prometheus Remote Write protocol.

global_clickhouse: # The ClickHouse server to be used for all projects.
  address:               # IP:Port or Domain:Port.
  user:                  # The username for the ClickHouse server.
  password:              # The password for the ClickHouse server.
  database:              # The initial database on the ClickHouse server.
  tls_enable: false      # Whether TLS is enabled for the ClickHouse server connection.
  tls_skip_verify: false # Whether to skip verification of the ClickHouse server's TLS certificate.

clickhouse_space_manager: # Automatically manage ClickHouse disk space by cleaning up old partitions.
  enabled: true                    # Enable space manager (default: true).
  usage_threshold_percent: 70      # Disk usage percentage threshold for triggering cleanup (default: 70).
  min_partitions: 1               # Minimum number of partitions to keep per table (default: 1).

auth:
  anonymous_role:           # Disables authentication if set (one of Admin, Editor, or Viewer).
  bootstrap_admin_password: # Password for the default Admin user.

do_not_check_for_deployments: false # Do not check for new deployments.
do_not_check_for_updates: false     # Do not check for new versions.
disable_usage_statistics: false     # Disable anonymous usage statistics.

license_key: # License key for Coroot Enterprise Edition.

# The project defined here will be created if it does not exist 
#  and will be configured with the provided API keys.
# If a project with the same name already exists (e.g., configured via the UI), 
#  its API keys and other settings will be replaced.
projects: # Create or update projects (configuration file only).
  - name:     # Project name (e.g., production, staging; must be unique; required).
    # Project API keys, used by agents to send telemetry data (required).
    apiKeys:
      - key:         # Random string or UUID (must be unique; required).
        description: # The API key description (optional).
    # Project notification integrations.
    notificationIntegrations:
      baseURL: # The URL of Coroot instance (required). Used for generating links in notifications.
      slack:
        token:              # Slack Bot User OAuth Token (required).
        defaultChannel:     # Default channel (required).
        incidents: false    # Notify of incidents (SLO violations).
        deployments: false  # Notify of deployments.
      teams:
        webhookURL:         # Microsoft Teams Webhook URL (required).
        incidents: false    # Notify of incidents (SLO violations).
        deployments: false  # Notify of deployments.
      pagerduty:
        integrationKey:     # PagerDuty Integration Key (required).
        incidents: false    # Notify of incidents (SLO violations).
      opsgenie:
        apiKey:             # Opsgenie API Key (required).
        euInstance: false   # EU instance of Opsgenie.
        incidents: false    # Notify of incidents (SLO violations).
      webhook:
        url:                    # Webhook URL (required).
        tlsSkipVerify: false    # Whether to skip verification of the Webhook server's TLS certificate.
        basicAuth:              # Basic auth credentials.
          username:
          password:
        customHeaders:          # Custom headers to include in requests.
          - key:
            value:
        incidents: false        # Notify of incidents (SLO violations).
        deployments: false      # Notify of deployments.
        incidentTemplate: ""    # Incident template (required if `incidents: true`).
        deploymentTemplate: ""  # Deployment template (required if `deployments: true`).
    # Project application category settings.
    applicationCategories:
      - name:               # Application category name (required).
        customPatterns:     # List of glob patterns in the <namespace>/<application_name> format.
          - staging/*
          - test-*/*
        notificationSettings: # Category notification settings.
          incidents:          # Notify of incidents (SLO violations).
            enabled: true
            slack:
              enabled: true
              channel: ops
            teams:
              enabled: false
            pagerduty:
              enabled: false
            opsgenie:
              enabled: false
            webhook:
              enabled: false
          deployments:        # Notify of deployments.
            enabled: true
            slack:
              enabled: true
              channel: general
            teams:
              enabled: false
            webhook:
              enabled: false
    # Project custom applications settings.
    customApplications:
      - name: custom-app
        instancePatterns:
          - app@node1
          - app@node2

# Single Sign-on configuration (Coroot Enterprise edition only).
sso:
  enabled: false
  defaultRole: Viewer # Default role for authenticated users (Admin, Editor, Viewer, or a custom role).
  saml:
    # SAML Identity Provider Metadata XML (required).
    metadata: |
      <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://www.okta.com/exkk72*********n5d7">
        ...
      </md:EntityDescriptor>

# AI configuration (Coroot Enterprise edition only).
ai:
  provider: # AI model provider (one of: anthropic, openai, or openai_compatible).
  anthropic:
    apiKey: # Anthropic API key. 
  openai:
    apiKey: # OpenAI API key.
  openaiCompatible:
    apiKey:   # API key.
    baseUrl:  # Base URL (e.g., https://generativelanguage.googleapis.com/v1beta/openai).
    model:    # Model name (e.g., gemini-2.5-pro-preview-06-05).
```
