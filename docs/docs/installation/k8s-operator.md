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
#  projects: # Projects configuration.
#    - name: # Project name (e.g., production, staging; required).
#      apiKeys: # Project API keys, used by agents to send telemetry data (required).
#        - key: # Random string or UUID (must be unique; required).
#          description: # API key description (optional).

#  affinity: # Affinity rules for Coroot pods.
#  tolerations: # Tolerations for Coroot pods.
#  resources: # Resource requests and limits for Coroot pods.
#  podAnnotations: # Annotations for Coroot pods.
#  storage:
#    size: 10Gi # Volume size for Coroot storage.
#    className: "" # If not set, the default storage class will be used.
#  service:
#    type: # Service type (e.g., ClusterIP, NodePort, LoadBalancer).
#    port: # Service port number.
#    nodePort: # NodePort number (if type is NodePort).
#  ingress: # Ingress configuration for Coroot.
#    className: Ingress class name (e.g., nginx, traefik; if not set the default IngressClass will be used).
#    host: # Domain name for Coroot (e.g., coroot.company.com).
#    path: # Path prefix for Coroot (e.g., /coroot).
#    tls: # TLS configuration.
#      - hosts:
#        secretName:

#  communityEdition:
#    version: x.y.z # If unspecified, the operator will automatically update Coroot CE to the latest version.

#  enterpriseEdition: # Configurations for Coroot Enterprise Edition.
#    version: x.y.z # If unspecified, the operator will automatically update Coroot EE to the latest version.
#    licenseKey: COROOT-1111-111 # License key for Coroot Enterprise Edition.

#  agentsOnly: # Configures the operator to install only the node-agent and cluster-agent.
#    corootUrl: http://COROOT_IP:PORT/ # URL of the Coroot instance to which agents send metrics, logs, traces, and profiles.

#  apiKey: # The API key used by agents when sending telemetry to Coroot.

#  nodeAgent:
#    version: x.y.z # If unspecified, the operator will automatically update the node-agent to the latest version.
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

#  clusterAgent:
#    affinity: # Affinity rules for cluster-agent.
#    tolerations: # Tolerations for cluster-agent.
#    podAnnotations: # Annotations for cluster-agent.
#    resources: # Resource requests and limits for cluster-agent.
#    env: # Environment variables for the cluster-agent.

#  prometheus:
#    affinity: # Affinity rules for Prometheus.
#    tolerations: # Tolerations for Prometheus.
#    storage:
#      size: 10Gi # Volume size for Prometheus storage.
#      className: "" # If not set, the default storage class will be used.
#    resources: # Resource requests and limits for Prometheus.
#    podAnnotations: # Annotations for Prometheus.

#  clickhouse:
#    shards: 1 # Number of ClickHouse shards.
#    replicas: 1 # Number of replicas per shard.
#    resources: # Resource requests and limits for Clickhouse pods.
#    storage:
#      size: 10Gi # Volume size for EACH ClickHouse instance.
#      className: "" # If not set, the default storage class will be used.
#    affinity: # Affinity rules for ClickHouse pods.
#    tolerations: # Tolerations for ClickHouse pods.
#    resources: # Resource requests and limits for ClickHouse pods.
#    podAnnotations: # Annotations for Clickhouse pods.
#    keeper: # Configuration for ClickHouse Keeper.
#      affinity: # Affinity rules for keeper pods.
#      tolerations: # Tolerations for keeper pods.
#      storage:
#        size: 10Gi # Volume size for keeper storage.
#        className: "" # If not set, the default storage class will be used.
#      resources: # Resource requests and limits for keeper pods.
#      podAnnotations: # Annotations for keeper pods.

#  externalClickhouse: # Use an external ClickHouse instance instead of deploying one.
#    address: # Address of the external ClickHouse instance.
#    database: # Name of the database to be used.
#    user: # Username for accessing the external ClickHouse.
#    password: # Password for accessing the external ClickHouse (plain-text, not recommended).
#    passwordSecret: # Secret containing password.
#      name: # Name of the secret to select from.
#      key:  # Key of the secret to select from.

#  replicas: 1 # Number of Coroot StatefulSet pods.

#  postgres: # Store configuration in a Postgres db instead of SQLite (required if replicas > 1).
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
```
