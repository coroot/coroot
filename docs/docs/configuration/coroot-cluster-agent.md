---
sidebar_position: 12
---

# Coroot-cluster-agent

Coroot-cluster-agent connects to Coroot to receive configuration and collects cluster-level telemetry data,
including metrics from databases, Kubernetes state metrics, and Kubernetes events.

## Cluster Agent Configuration

You can configure coroot-cluster-agent using command-line flags or environment variables.

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--listen` | `LISTEN` | `127.0.0.1:10301` | Listen address - ip:port or :port |
| `--coroot-url` | `COROOT_URL` | – | Coroot URL (required) |
| `--api-key` | `API_KEY` | – | Coroot API key |
| `--config-update-interval` | `CONFIG_UPDATE_INTERVAL` | `60s` | Interval between configuration updates from Coroot |
| `--config-update-timeout` | `CONFIG_UPDATE_TIMEOUT` | `10s` | Timeout for configuration update requests |
| `--config-file` | `CONFIG_FILE` | – | Path to a YAML file with static configuration (see [Configuration file](#configuration-file)), merged with the configuration received from Coroot |
| `--metrics-scrape-interval` | `METRICS_SCRAPE_INTERVAL` | – | Interval between metrics scrapes |
| `--metrics-scrape-timeout` | `METRICS_SCRAPE_TIMEOUT` | `10s` | Timeout for metrics scrape requests |
| `--metrics-wal-dir` | `METRICS_WAL_DIR` | `/tmp` | Directory for the metrics write-ahead log |
| `--profiles-scrape-interval` | `PROFILES_SCRAPE_INTERVAL` | `60s` | Interval between profiling scrapes |
| `--profiles-scrape-timeout` | `PROFILES_SCRAPE_TIMEOUT` | `10s` | Timeout for profiling scrape requests |
| `--kube-state-metrics-listen-address` | `KUBE_STATE_METRICS_LISTEN_ADDRESS` | `127.0.0.1:10303` | Listen address for the kube-state-metrics endpoint |
| `--insecure-skip-verify` | `INSECURE_SKIP_VERIFY` | `false` | Skip TLS certificate verification |
| `--ca-file` | `CA_FILE` | – | Path to the custom CA certificate file |
| `--collect-kubernetes-events` | `COLLECT_KUBERNETES_EVENTS` | `true` | Collect and forward Kubernetes events |
| `--collect-gcp-logs` | `COLLECT_GCP_LOGS` | `true` | Forward the logs of Cloud SQL instances discovered through the [GCP integration](/configuration/gcp#logs) |
| `--collect-aws-logs` | `COLLECT_AWS_LOGS` | `true` | Forward the logs of RDS Postgres and MySQL instances discovered through the [AWS integration](/configuration/aws#logs) |
| `--track-database-changes` | `TRACK_DATABASE_CHANGES` | `true` | Track schema and settings changes in databases |
| `--track-database-sizes` | `TRACK_DATABASE_SIZES` | `true` | Collect per-database and per-table size metrics |
| `--track-database-bloat` | `TRACK_DATABASE_BLOAT` | `true` | Estimate per-database, per-table and per-index bloat (PostgreSQL only) |
| `--max-tables-per-database` | `MAX_TABLES_PER_DATABASE` | `1000` | Skip databases with more tables than this limit |
| `--exclude-databases` | `EXCLUDE_DATABASES` | `rdsadmin,cloudsqladmin,mysql,information_schema,performance_schema,sys` | Databases to exclude from monitoring (applies to both PostgreSQL and MySQL). For Postgres no connection, query, transaction ID age, schema or size metrics are collected for them; for MySQL they are excluded from schema and size tracking. MongoDB system databases (admin, config, local) are always excluded regardless of this setting. |

## Configuration file

Besides the configuration it receives from Coroot, the agent can read a static YAML file given by `--config-file`.
This is how the [Kubernetes Operator](/installation/k8s-operator) passes the `clusterAgent.aws` and
`clusterAgent.databases` sections of the Coroot custom resource to the agent. Environment variable references
(`${VAR}`) in the file are expanded when it is read, so credentials can be kept out of the file.

```yaml
# AWS integration settings. When present, they replace the settings made in the Coroot UI.
aws:
  region: us-east-1               # Optional: defaults to the region the agent runs in.
  accessKeyId: ${AWS_KEY_ID}      # Optional: leave both keys out to use the IAM role of the pod or the EC2 instance profile.
  secretAccessKey: ${AWS_KEY}
  rdsTagFilters: {team: payments}
  elasticacheTagFilters: {}

# GCP integration settings (Cloud SQL and Memorystore discovery).
gcp:
  projectId: my-project           # Optional: defaults to the project of the credentials or of the cluster.
  region: us-central1             # Optional: defaults to the cluster's region, "all" scans every region of the project.
  credentialsJson: ${GCP_KEY}     # Optional: a service account key; leave out to use Workload Identity or the VM's service account.
  cloudsqlLabelFilters: {team: payments}
  memorystoreLabelFilters: {}

# Databases to collect metrics from, in addition to those configured in Coroot. Exactly one of host, rds, elasticache, cloudsql or memorystore per entry.
databases:
  - type: postgres                # postgres, mysql, redis, memcached or mongodb.
    rds: my-db                    # An RDS instance discovered by the AWS integration: its endpoint is used.
    credentials: {username: coroot, password: ${PG_PASSWORD}}
    params: {sslmode: require}
  - type: redis
    elasticache: my-cache         # An ElastiCache cluster: every node is monitored.
  - type: postgres
    cloudsql: my-db               # A Cloud SQL instance discovered by the GCP integration.
    credentials: {username: coroot, password: ${PG_PASSWORD}}
    params: {sslmode: require}
  - type: redis
    memorystore: my-cache         # A Memorystore instance discovered by the GCP integration.
  - type: mysql
    host: mysql.example.internal  # Re-resolved on every configuration update; every resolved IP address is monitored.
    port: "3306"
    credentials: {username: coroot, password: ${MYSQL_PASSWORD}}
```

Targets defined in the file take precedence over the same `ip:port` targets configured in Coroot. Metrics are
attributed to applications by address, so a database configured here shows up under the RDS, ElastiCache, or external
service application that Coroot already sees clients connecting to.

