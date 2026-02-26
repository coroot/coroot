---
sidebar_position: 4
---

# Kubernetes Operator 

The best way to deploy Coroot into a Kubernetes or OpenShift cluster is by using the [coroot-operator](https://github.com/coroot/coroot-operator). 
The operator simplifies the deployment of all required components and enables scaling as needed. 
It supports the deployment of both Coroot Community and Enterprise editions.

## Operator installation

Add the Coroot helm chart repo:

```bash
helm repo add coroot https://coroot.github.io/helm-charts
helm repo update coroot
```

Next, install the Coroot Operator:

```bash
helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator
```

## Custom image registry

By default, the operator pulls all component images from `ghcr.io/coroot`.
To use a private or self-hosted registry, configure the operator via Helm values:

```bash
helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator \
  --set registry.url=https://registry.example.com/coroot \
  --set registry.pullSecret=coroot-registry-auth \
  --set registry.tlsSkipVerify=false
```

| Parameter | Description |
|-----------|-------------|
| `registry.url` | Full registry URL including scheme and path prefix (default: `https://ghcr.io/coroot`). |
| `registry.pullSecret` | Name of an existing `kubernetes.io/dockerconfigjson` secret. Used for both version discovery and as `imagePullSecret` on all component pods. |
| `registry.tlsSkipVerify` | Skip TLS certificate verification for registries with self-signed certificates. |

The registry must have the same image layout as the default registry
(e.g., `<registry>/coroot`, `<registry>/coroot-node-agent`, `<registry>/clickhouse`, etc.).

For a step-by-step example, see the [Using JFrog Artifactory as a Registry for Coroot](/guides/jfrog-artifactory) guide.

## Coroot CR (Custom Resource)

To deploy Coroot, you need to create a Coroot resource. Below is an example specification of the Coroot custom resource.
The operator continuously monitors these resources and adjusts the configuration if necessary.
Additionally, the operator checks for new versions of Coroot components and automatically updates them unless you specify particular versions.

```yaml
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec:
#  metricsRefreshInterval: 15s # Specifies the metric resolution interval.
#  cacheTTL: 30d # Duration for which Coroot retains the metric cache.
#  authAnonymousRole: # Allows access to Coroot without authentication if set (one of Admin, Editor, or Viewer).
#  authBootstrapAdminPassword:        # Initial admin password for bootstrapping (plain-text).
#  authBootstrapAdminPasswordSecret:  # Secret containing the initial admin password.
#    name: # Name of the secret to select from.
#    key:  # Key of the secret to select from.
#  env: # Environment variables for Coroot.
#    - name:
#      value:
#      valueFrom:
#  nodeSelector: # Restricts scheduling to nodes matching the specified labels.
#    <node label name>: <node label value>
#  affinity: # Affinity rules for Coroot pods.
#  tolerations: # Tolerations for Coroot pods.
#  resources: # Resource requests and limits for Coroot pods.
#  podAnnotations: # Annotations for Coroot pods.
#  storage:
#    size: 10Gi # Volume size for Coroot storage.
#    className: "" # If not set, the default storage class will be used.
#    reclaimPolicy: Delete # Options: Retain (keeps PVC) or Delete (removes PVC on Coroot CR deletion).
#    annotations: # Annotations for PersistentVolumeClaim (PVC).
#  service:
#    type: # Service type (e.g., ClusterIP, NodePort, LoadBalancer).
#    port:      # Service port (default 8080).
#    nodePort:  # Service nodePort (if type is NodePort).
#    grpcPort:      # gRPC port (default 4317).
#    grpcNodePort:  # gRPC nodePort (if type is NodePort).
#    annotations: # Annotations for Service.
#  ingress: # Ingress configuration for Coroot.
#    className: Ingress class name (e.g., nginx, traefik; if not set the default IngressClass will be used).
#    host: # Domain name for Coroot (e.g., coroot.company.com).
#    path: # Path prefix for Coroot (e.g., /coroot).
#    annotations: # Annotations for Ingress.
#    tls: # TLS configuration.
#      hosts: # The array with host names
#      secretName: # The name of secret where TLS certificate and private key would be stored
#  storeMetricsInClickhouse: false # Store metrics in ClickHouse. If enabled, Prometheus will not be installed.
#  grpc: # gRPC settings.
#    disabled: false # Disables gRPC server.
#  tls: # TLS settings (enables TLS for gRPC if defined).
#    certSecret:  # Secret containing TLS certificate (required).
#      name: # Name of the secret to select from.
#      key:  # Key of the secret to select from (e.g., 'tls.crt').
#    keySecret: # Secret containing TLS private key (required).
#      name: # Name of the secret to select from.
#      key:  # Key of the secret to select from (e.g., 'tls.key').

#  disableBuiltinAlerts: false # Disable all built-in alerting rules on startup.

# Coroot stores Traces, Logs, and Profiles in ClickHouse.
# Their retention is managed by setting a Time-To-Live (TTL) for the corresponding Clickhouse tables.  
# The TTLs below are applied during table creation and do not currently affect existing tables.
#  tracesTTL: 7d
#  logsTTL: 7d
#  profilesTTL: 7d

# Configuration for Coroot Community Edition.
#  communityEdition:
#    image: # If unspecified, the operator will automatically update Coroot CE to the latest version from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/coroot:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.

# Configuration for Coroot Enterprise Edition.
#  enterpriseEdition:
#    licenseKey: COROOT-1111-111 # License key for Coroot Enterprise Edition.
#    licenseKeySecret: # Secret containing the license key.
#      name: # Name of the secret to select from.
#      key:  # Key of the secret to select from.
#    image: # If unspecified, the operator will automatically update Coroot EE to the latest version from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/coroot-ee:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.

# Configures the operator to install only the node-agent and cluster-agent.
#  agentsOnly:
#    corootURL: http(s)://COROOT_IP:PORT/ # URL of the Coroot instance to which agents send metrics, logs, traces, and profiles.
#    tlsSkipVerify: false # Whether to skip verification of the Coroot server's TLS certificate.

# The API key used by agents when sending telemetry to Coroot.
#  apiKey: # Plain-text API key. Prefer using `apiKeySecret` for better security.
#  apiKeySecret: # Secret containing the API key.
#    name: # Name of the secret to select from.
#    key:  # Key of the secret to select from.

# Configuration for Coroot Node Agent.
#  nodeAgent:
#    priorityClassName: # Priority class for the node-agent pods.
#    update_strategy: # Update strategy for node-agent pods.
#    nodeSelector: # Restricts scheduling to nodes matching the specified labels.
#    affinity: # Affinity rules for node-agent pods.
#    tolerations: # Tolerations for node-agent pods.
#      - operator: Exists
#    podAnnotations: # Annotations for node-agent pods.
#    resources: # Resource requests and limits for the node-agent pods.
#      requests: 
#        cpu: 100m
#        memory: 200Mi
#      limits: 
#        cpu: 500m
#        memory: 1Gi
#    env: # Environment variables for the node-agent.
#    image: # If unspecified, the operator will automatically update Coroot Node Agent to the latest version from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/coroot-node-agent:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.
#    trackPublicNetworks: ["0.0.0.0/0"] # Allow track connections to the specified IP networks (e.g., Y.Y.Y.Y/mask). By default, Coroot tracks all connections.
#    logCollector:
#      collectLogBasedMetrics: true # Collect log-based metrics. Disables `collectLogEntries` if set to false.
#      collectLogEntries: true      # Collect log entries and store them in ClickHouse.
#    ebpfTracer:
#      enabled: true # Collect traces and store them in ClickHouse.
#      sampling: "1.0" # Trace sampling rate (0.0 to 1.0).
#    ebpfProfiler:
#      enabled: true # Collect profiles and store them in ClickHouse.

# Configuration for Coroot Cluster Agent.
#  clusterAgent:
#    nodeSelector: # Restricts scheduling to nodes matching the specified labels.
#    affinity: # Affinity rules for cluster-agent.
#    tolerations: # Tolerations for cluster-agent.
#    podAnnotations: # Annotations for cluster-agent.
#    resources: # Resource requests and limits for cluster-agent.
#    env: # Environment variables for the cluster-agent.
#    image: # If unspecified, the operator will automatically update Coroot Cluster Agent to the latest version from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/coroot-cluster-agent:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.
#    kubeStateMetrics:
#      image: # If unspecified, the operator will install Kube State Metrics from Coroot's public registry.
#        name:           # Specifies the full image reference (e.g., <private-registry>/kube-state-metrics:<version>)
#        pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#        pullSecrets: [] # The pull secrets for pulling the image from a private registry.

# Configuration for Prometheus managed by the operator.
#  prometheus:
#    nodeSelector: # Restricts scheduling to nodes matching the specified labels.
#    affinity: # Affinity rules for Prometheus.
#    tolerations: # Tolerations for Prometheus.
#    storage:
#      size: 10Gi # Volume size for Prometheus storage.
#      className: "" # If not set, the default storage class will be used.
#      reclaimPolicy: Delete # Options: Retain (keeps PVC) or Delete (removes PVC on Coroot CR deletion).
#      annotations: # Annotations for PersistentVolumeClaim (PVC).
#    resources: # Resource requests and limits for Prometheus.
#    podAnnotations: # Annotations for Prometheus.
#    retention: 2d # Metrics retention time (e.g. 4h, 3d, 2w, 1y).
#    outOfOrderTimeWindow: 1h # The `storage.tsdb.out_of_order_time_window` Prometheus setting.
#    image: # If unspecified, the operator will install Prometheus from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/prometheus:<version>).
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.

# Use an external Prometheus instance instead of deploying one.
# NOTE: Remote write receiver must be enabled in your Prometheus via the `--web.enable-remote-write-receiver` flag.
#  externalPrometheus:
#    url: # http(s)://<IP>:<port> or http(s)://<domain>:<port> or http(s)://<service name>:<port>.
#    tlsSkipVerify: false # Whether to skip verification of the Prometheus server's TLS certificate.
#    basicAuth: # Basic auth credentials.
#      username: # Basic auth username.
#      password: # Basic auth password.
#      passwordSecret: # Secret containing password.
#        name: # Name of the secret to select from.
#        key:  # Key of the secret to select from.
#    customHeaders:  # Custom headers to include in requests to the Prometheus server.
#      <header name>: <header value>
#    # The URL for metric ingestion though the Prometheus Remote Write protocol (optional).
#    # By default, Coroot appends /api/v1/write to the base URL configured above.
#    remoteWriteURL: # (e.g., http://vminsert:8480/insert/0/prometheus/api/v1/write).

# Configuration for Clickhouse managed by the operator.
#  clickhouse:
#    shards: 1 # Number of ClickHouse shards.
#    replicas: 1 # Number of replicas per shard.
#    resources: # Resource requests and limits for Clickhouse pods.
#    storage:
#      size: 10Gi # Volume size for EACH ClickHouse instance.
#      className: "" # If not set, the default storage class will be used.
#      reclaimPolicy: Delete # Options: Retain (keeps PVC) or Delete (removes PVC on Coroot CR deletion).
#      annotations: # Annotations for PersistentVolumeClaim (PVC).
#    nodeSelector: # Restricts scheduling to nodes matching the specified labels.
#    affinity: # Affinity rules for ClickHouse pods.
#    tolerations: # Tolerations for ClickHouse pods.
#    resources: # Resource requests and limits for ClickHouse pods.
#    podAnnotations: # Annotations for Clickhouse pods.
#    image: # If unspecified, the operator will install Clickhouse from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/clickhouse-server:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.
#    logLevel: warning # Log level (fatal, critical, error, warning, notice, information, debug, trace, test, or none).
#    keeper: # Configuration for ClickHouse Keeper.
#      replicas: 3 # Use only during initial setup, as changing the replica count for a running Keeper may cause it to fail.
#      nodeSelector: # Restricts scheduling to nodes matching the specified labels.
#      affinity: # Affinity rules for keeper pods.
#      tolerations: # Tolerations for keeper pods.
#      storage:
#        size: 10Gi # Volume size for keeper storage.
#        className: "" # If not set, the default storage class will be used.
#        annotations: # Annotations for PersistentVolumeClaim (PVC).
#      resources: # Resource requests and limits for keeper pods.
#      podAnnotations: # Annotations for keeper pods.
#      image: # If unspecified, the operator will install Clickhouse Keeper from Coroot's public registry.
#        name:           # Specifies the full image reference (e.g., <private-registry>/clickhouse-keeper:<version>)
#        pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#        pullSecrets: [] # The pull secrets for pulling the image from a private registry.
#      logLevel: warning # Log level (fatal, critical, error, warning, notice, information, debug, trace, test, or none).

# Use an external ClickHouse instance instead of deploying one.
#  externalClickhouse:
#    address: # Address of the external ClickHouse instance.
#    database: # Name of the database to be used.
#    user: # Username for accessing the external ClickHouse.
#    password: # Password for accessing the external ClickHouse (plain-text, not recommended).
#    passwordSecret: # Secret containing a password for accessing the external ClickHouse.
#      name: # Name of the secret to select from.
#      key:  # Key of the secret to select from.
#    tlsEnabled: false # Whether to enable TLS for the connection to ClickHouse.
#    tlsSkipVerify: false # Whether to skip verification of the ClickHouse server's TLS certificate.

#  replicas: 1 # Number of Coroot StatefulSet pods.

# Store configuration in a Postgres DB instead of SQLite (required if `replicas` > 1).
#  postgres:
#    host: # Postgres host or service name.
#    port: # Postgres port (optional, default 5432).
#    database: # Name of the database.
#    user: # Username for accessing Postgres.
#    password: # Password for accessing postgres (plain-text, not recommended).
#    passwordSecret: # Secret containing password.
#      name: # Name of the secret to select from.
#      key:  # Key of the secret to select from.
#    params: # Extra parameters, e.g., sslmode and connect_timeout.
#      sslmode: disable

# The project defined here will be created if it does not exist and will be configured with the provided API keys.
# If a project with the same name already exists (e.g., configured via the UI), its API keys and other settings will be replaced.
#  projects: # Create or update projects.
#    - name:    # Project name (e.g., production, staging; required).
#      # Multi-cluster aggregation: list existing project names to combine (optional).
#      memberProjects:
#        - prod-eu
#        - prod-us
#      # Use another Coroot instance as the data source for this project (optional).
#      remoteCoroot:
#        url: # Base URL of the remote Coroot instance (e.g., https://coroot.example.com).
#        tlsSkipVerify: false # Whether to skip verification of the Coroot server's TLS certificate.
#        apiKey: # API key of the remote project. Prefer using `apiKeySecret` for better security.
#        apiKeySecret: # Secret containing the API key.
#          name: # Name of the secret to select from.
#          key:  # Key of the secret to select from.
#        metricResolution: 15s # Prometheus query resolution/refresh interval.
#      # Project API keys, used by agents to send telemetry data (required unless memberProjects or remoteCoroot is set).
#      apiKeys:
#        - description: # The API key description (optional).
#          key:         # Plain-text API key (a random string or UUID). Must be unique. Prefer using `keySecret` for better security.
#          keySecret:   # Secret containing the API key. Generated automatically if missing.
#            name: # Name of the secret to select from.
#            key:  # Key of the secret to select from.
#      # Project notification integrations.
#      notificationIntegrations:
#        baseURL: # The URL of Coroot instance (required). Used for generating links in notifications.
#        slack:
#          token:        # Slack Bot User OAuth Token (required).
#          tokenSecret:  # Secret containing the Token.
#            name: # Name of the secret to select from.
#            key:  # Key of the secret to select from.
#          defaultChannel:     # Default channel (required).
#          incidents: false    # Notify of incidents (SLO violations).
#          deployments: false  # Notify of deployments.
#          alerts: false       # Notify of alerts.
#        teams:
#          webhookURL:        # Microsoft Teams Webhook URL (required).
#          webhookURLSecret:  # Secret containing the Webhook URL.
#            name: # Name of the secret to select from.
#            key:  # Key of the secret to select from.
#          incidents: false    # Notify of incidents (SLO violations).
#          deployments: false  # Notify of deployments.
#          alerts: false       # Notify of alerts.
#        pagerduty:
#          integrationKey:        # PagerDuty Integration Key (required).
#          integrationKeySecret:  # Secret containing the Integration Key.
#            name: # Name of the secret to select from.
#            key:  # Key of the secret to select from.
#          incidents: false    # Notify of incidents (SLO violations).
#          alerts: false       # Notify of alerts.
#        opsgenie:
#          apiKey:        # Opsgenie API Key (required).
#          apiKeySecret:  # Secret containing the API Key.
#            name: # Name of the secret to select from.
#            key:  # Key of the secret to select from.
#          euInstance: false   # EU instance of Opsgenie.
#          incidents: false    # Notify of incidents (SLO violations).
#          alerts: false       # Notify of alerts.
#        webhook:
#          url:                    # Webhook URL (required).
#          tlsSkipVerify: false    # Whether to skip verification of the Webhook server's TLS certificate.
#          basicAuth:              # Basic auth credentials.
#            username:        # Basic auth username.
#            password:        # Basic auth password.
#            passwordSecret:  # Secret containing password.
#              name: # Name of the secret to select from.
#              key:  # Key of the secret to select from.
#          customHeaders:          # Custom headers to include in requests.
#            - key:
#              value:
#          incidents: false        # Notify of incidents (SLO violations).
#          deployments: false      # Notify of deployments.
#          alerts: false           # Notify of alerts.
#          incidentTemplate: ""    # Incident template (required if `incidents: true`).
#          deploymentTemplate: ""  # Deployment template (required if `deployments: true`).
#          alertTemplate: ""       # Alert template (required if `alerts: true`).
#      # Project application category settings.
#      applicationCategories:
#        - name:               # Application category name (required).
#          customPatterns:     # List of glob patterns in the <namespace>/<application_name> format.
#            - staging/*
#            - test-*/*
#          notificationSettings: # Category notification settings.
#            incidents:          # Notify of incidents (SLO violations).
#              enabled: true
#              slack:
#                enabled: true
#                channel: ops
#              teams:
#                enabled: false
#              pagerduty:
#                enabled: false
#              opsgenie:
#                enabled: false
#              webhook:
#                enabled: false
#            deployments:        # Notify of deployments.
#              enabled: true
#              slack:
#                enabled: true
#                channel: general
#              teams:
#                enabled: false
#              webhook:
#                enabled: false
#            alerts:             # Notify of alerts.
#              enabled: true
#              slack:
#                enabled: true
#                channel: alerts
#              teams:
#                enabled: false
#              pagerduty:
#                enabled: false
#              opsgenie:
#                enabled: false
#              webhook:
#                enabled: false
#      # Project custom applications settings.
#      customApplications:
#        - name: custom-app
#          instancePatterns:
#            - app@node1
#            - app@node2
#      # Alerting rules: adjust built-in rules or define custom ones.
#      # Rules defined here become read-only in the UI (shown with a lock icon).
#      # For built-in rules, only the fields you specify are overridden; unset fields keep their current values.
#      # For custom rules, all required fields (name, source) must be provided.
#      alertingRules:
#        # Adjust a built-in rule (only override severity and description)
#        - id: storage-space          # Required. Built-in rule ID or a custom ID you choose.
#          severity: critical         # One of: warning, critical.
#          templates:
#            description: "Disk space critically low"
#        # Disable a built-in rule
#        - id: memory-pressure
#          enabled: false
#        # Adjust the built-in Kubernetes events rule
#        - id: kubernetes-events
#          source:
#            type: kubernetes_events
#            kubernetesEvents:
#              minCount: 3              # Require at least 3 occurrences before alerting.
#              evaluateWithAi: false    # Disable AI evaluation.
#        # Custom check-based rule
#        - id: custom-postgres-latency
#          name: "Postgres latency (production)"
#          source:
#            type: check              # One of: check, log_patterns, kubernetes_events, promql.
#            check:
#              checkId: postgres_latency
#          selector:
#            type: category           # One of: all, category, applications.
#            categories:
#              - production
#          severity: critical
#          for: 5m                    # How long the condition must be true before firing.
#          keepFiringFor: 5m          # How long to keep firing after condition clears.
#          templates:
#            description: "Postgres latency is critically high in production."
#          enabled: true
#        # Custom PromQL-based rule
#        - id: custom-uptime
#          name: "Instance uptime"
#          source:
#            type: promql
#            promql:
#              expression: "up == 0"
#          severity: warning
#          templates:
#            summary: "Instance {{.instance}} is down"
#          notificationCategory: custom-category # Override the notification category for this rule.
#        # Custom log-based rule
#        - id: custom-log-errors
#          name: "Critical log errors"
#          source:
#            type: log_patterns
#            logPattern:
#              severities:
#                - error
#                - fatal
#              minCount: 5            # Minimum occurrences before alerting.
#              maxAlertsPerApp: 10    # Maximum alerts per application for this rule.
#              evaluateWithAi: true   # Use AI to evaluate log patterns and reduce noise.
#          severity: critical
#        # Custom Kubernetes events-based rule
#        - id: custom-k8s-events
#          name: "K8s scheduling failures"
#          source:
#            type: kubernetes_events
#            kubernetesEvents:
#              minCount: 3              # Minimum occurrences before alerting.
#              maxAlertsPerApp: 10      # Maximum alerts per application for this rule.
#              evaluateWithAi: true     # Use AI to evaluate events and reduce noise.
#          severity: warning
#      # Project inspection overrides.
#      inspectionOverrides:
#        # `applicationId` format: <namespace>:<kind>:<name>
#        sloAvailability:
#          - applicationId: otel-demo:Deployment:cart
#            objectivePercent: 99.9 # The percentage of requests that should be served without errors (e.g., 95, 99, 99.9).
#        sloLatency:
#          - applicationId: otel-demo:Deployment:cart
#            objectivePercent: 99.9     # The percentage of requests that should be served faster than `objectiveThreshold` (e.g., 95, 99, 99.9).
#            objectiveThreshold: 100ms  # The latency threshold (e.g., 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s).
#          - applicationId: external:ExternalService:api.github.com:443
#            objectivePercent: 99 
#            objectiveThreshold: 2s

# Coroot Cloud integration.
#  corootCloud:
#    # Coroot Cloud API key (required). Can be obtained from the UI after connecting to Coroot Cloud.
#    apiKey:
#    apiKeySecret: # Secret containing the API key.
#      name: # Name of the secret to select from.
#      key:  # Key of the secret to select from.
#    # Root Cause Analysis (RCA) configuration.
#    rca:
#      # If 'true', incidents will not be investigated automatically.
#      disableIncidentsAutoInvestigation: false

# Single Sign-on configuration (Coroot Enterprise edition only).
# Define either `saml` or `oidc` section to enable SSO.
#  sso:
#    enabled: true
#    defaultRole: Viewer # Default role for authenticated users (Admin, Editor, Viewer, or a custom role).
#    # SAML configuration (define this section to use SAML SSO).
#    saml:
#      metadata: |  # SAML Identity Provider Metadata XML.
#        <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://www.okta.com/exkk72*********n5d7">
#          ...
#        </md:EntityDescriptor>
#      metadataSecret:  # Secret containing the Metadata XML.
#        name: # Name of the secret to select from.
#        key:  # Key of the secret to select from.
#    # OIDC configuration (define this section to use OIDC SSO).
#    oidc:
#      issuerURL:     # OIDC provider issuer URL (e.g., https://accounts.google.com).
#      clientID:      # OAuth client ID.
#      clientSecret:  # OAuth client secret (plain-text). Prefer using `clientSecretSecret` for better security.
#      clientSecretSecret:  # Secret containing the client secret (recommended).
#        name: # Name of the secret to select from.
#        key:  # Key of the secret to select from.
  
# AI configuration (Coroot Enterprise edition only).
#  ai:
#    provider: # AI model provider (one of: anthropic, openai, or openai_compatible).
#    anthropic:
#      apiKey:        # Anthropic API key (required).
#      apiKeySecret:  # Secret containing the API key.
#        name: # Name of the secret to select from.
#        key:  # Key of the secret to select from.
#    openai:
#      apiKey:        # OpenAI API key (required).
#      apiKeySecret:  # Secret containing the API key.
#        name: # Name of the secret to select from.
#        key:  # Key of the secret to select from.
#    openaiCompatible:
#      apiKey:        # API key (required).
#      apiKeySecret:  # Secret containing the API key.
#        name: # Name of the secret to select from.
#        key:  # Key of the secret to select from.
#      baseURL:  # Base URL (e.g., https://generativelanguage.googleapis.com/v1beta/openai).
#      model:    # Model name (e.g., gemini-2.5-pro-preview-06-05).
```

## Operator upgrade

```bash
helm repo update coroot
helm upgrade -n coroot coroot-operator coroot/coroot-operator
```
