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
| `--metrics-scrape-interval` | `METRICS_SCRAPE_INTERVAL` | – | Interval between metrics scrapes |
| `--metrics-scrape-timeout` | `METRICS_SCRAPE_TIMEOUT` | `10s` | Timeout for metrics scrape requests |
| `--metrics-wal-dir` | `METRICS_WAL_DIR` | `/tmp` | Directory for the metrics write-ahead log |
| `--profiles-scrape-interval` | `PROFILES_SCRAPE_INTERVAL` | `60s` | Interval between profiling scrapes |
| `--profiles-scrape-timeout` | `PROFILES_SCRAPE_TIMEOUT` | `10s` | Timeout for profiling scrape requests |
| `--kube-state-metrics-listen-address` | `KUBE_STATE_METRICS_LISTEN_ADDRESS` | `127.0.0.1:10303` | Listen address for the kube-state-metrics endpoint |
| `--insecure-skip-verify` | `INSECURE_SKIP_VERIFY` | `false` | Skip TLS certificate verification |
| `--collect-kubernetes-events` | `COLLECT_KUBERNETES_EVENTS` | `true` | Collect and forward Kubernetes events |
| `--track-database-changes` | `TRACK_DATABASE_CHANGES` | `true` | Track schema and settings changes in databases |
| `--track-database-sizes` | `TRACK_DATABASE_SIZES` | `true` | Collect per-database and per-table size metrics |
| `--max-tables-per-database` | `MAX_TABLES_PER_DATABASE` | `1000` | Skip databases with more tables than this limit |
| `--exclude-databases` | `EXCLUDE_DATABASES` | `postgres,mysql,information_schema,performance_schema,sys` | Databases to exclude from schema and size tracking (applies to both PostgreSQL and MySQL). MongoDB system databases (admin, config, local) are always excluded regardless of this setting. |
