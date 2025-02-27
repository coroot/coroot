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
#  cacheTTL: 7d # Duration for which Coroot retains the metric cache.
#  authAnonymousRole: # Allows access to Coroot without authentication if set (one of Admin, Editor, or Viewer).
#  authBootstrapAdminPassword: # Initial admin password for bootstrapping.
#  env: # Environment variables for Coroot.
#    - name:
#      value:
#      valueFrom:
#  affinity: # Affinity rules for Coroot pods.
#  tolerations: # Tolerations for Coroot pods.
#  resources: # Resource requests and limits for Coroot pods.
#  podAnnotations: # Annotations for Coroot pods.
#  storage:
#    size: 10Gi # Volume size for Coroot storage.
#    className: "" # If not set, the default storage class will be used.
#    reclaimPolicy: Delete # Options: Retain (keeps PVC) or Delete (removes PVC on Coroot CR deletion).
#  service:
#    type: # Service type (e.g., ClusterIP, NodePort, LoadBalancer).
#    port: # Service port number.
#    nodePort: # NodePort number (if type is NodePort).
#  ingress: # Ingress configuration for Coroot.
#    className: Ingress class name (e.g., nginx, traefik; if not set the default IngressClass will be used).
#    host: # Domain name for Coroot (e.g., coroot.company.com).
#    path: # Path prefix for Coroot (e.g., /coroot).
#    annotations: # Annotations for Ingress.
#    tls: # TLS configuration.
#      hosts: # The array with host names
#      secretName: # The name of secret where TLS certificate and private key would be stored

# Configuration for Coroot Community Edition.
#  communityEdition:
#    image: # If unspecified, the operator will automatically update Coroot CE to the latest version from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/coroot:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.

# Configuration for Coroot Enterprise Edition.
#  enterpriseEdition:
#    licenseKey: COROOT-1111-111 # License key for Coroot Enterprise Edition.
#    image: # If unspecified, the operator will automatically update Coroot EE to the latest version from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/coroot-ee:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.

# Configures the operator to install only the node-agent and cluster-agent.
#  agentsOnly:
#    corootURL: http://COROOT_IP:PORT/ # URL of the Coroot instance to which agents send metrics, logs, traces, and profiles.

#  apiKey: # The API key used by agents when sending telemetry to Coroot.

# Configuration for Coroot Node Agent.
#  nodeAgent:
#    priorityClassName: # Priority class for the node-agent pods.
#    update_strategy: # Update strategy for node-agent pods.
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

# Configuration for Coroot Cluster Agent.
#  clusterAgent:
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
#    affinity: # Affinity rules for Prometheus.
#    tolerations: # Tolerations for Prometheus.
#    storage:
#      size: 10Gi # Volume size for Prometheus storage.
#      className: "" # If not set, the default storage class will be used.
#      reclaimPolicy: Delete # Options: Retain (keeps PVC) or Delete (removes PVC on Coroot CR deletion).
#    resources: # Resource requests and limits for Prometheus.
#    podAnnotations: # Annotations for Prometheus.
#    retention: 2d # Metrics retention time (e.g. 4h, 3d, 2w, 1y)
#    image: # If unspecified, the operator will install Prometheus from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/prometheus:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.

# Use an external Prometheus instance instead of deploying one.
# NOTE: Remote write receiver must be enabled in your Prometheus via the --web.enable-remote-write-receiver flag.
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
#    affinity: # Affinity rules for ClickHouse pods.
#    tolerations: # Tolerations for ClickHouse pods.
#    resources: # Resource requests and limits for ClickHouse pods.
#    podAnnotations: # Annotations for Clickhouse pods.
#    image: # If unspecified, the operator will install Clickhouse from Coroot's public registry.
#      name:           # Specifies the full image reference (e.g., <private-registry>/clickhouse-server:<version>)
#      pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#      pullSecrets: [] # The pull secrets for pulling the image from a private registry.
#    keeper: # Configuration for ClickHouse Keeper.
#      affinity: # Affinity rules for keeper pods.
#      tolerations: # Tolerations for keeper pods.
#      storage:
#        size: 10Gi # Volume size for keeper storage.
#        className: "" # If not set, the default storage class will be used.
#      resources: # Resource requests and limits for keeper pods.
#      podAnnotations: # Annotations for keeper pods.
#      image: # If unspecified, the operator will install Clickhouse Keeper from Coroot's public registry.
#        name:           # Specifies the full image reference (e.g., <private-registry>/clickhouse-keeper:<version>)
#        pullPolicy:     # The image pull policy (e.g., Always, IfNotPresent, Never).
#        pullSecrets: [] # The pull secrets for pulling the image from a private registry.

# Use an external ClickHouse instance instead of deploying one.
#  externalClickhouse:
#    address: # Address of the external ClickHouse instance.
#    database: # Name of the database to be used.
#    user: # Username for accessing the external ClickHouse.
#    password: # Password for accessing the external ClickHouse (plain-text, not recommended).
#    passwordSecret: # Secret containing password.
#      name: # Name of the secret to select from.
#      key:  # Key of the secret to select from.

#  replicas: 1 # Number of Coroot StatefulSet pods.

# Store configuration in a Postgres DB instead of SQLite (required if replicas > 1).
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
# If a project with the same name already exists (e.g., configured via the UI), its API keys will be replaced.
#  projects: # Create or update projects.
#    - name:    # Project name (e.g., production, staging; required).
#      apiKeys: # Project API keys, used by agents to send telemetry data (required).
#        - key:         # Random string or UUID (must be unique; required).
#          description: # The API key description (optional).
```
