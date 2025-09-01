---
sidebar_position: 11
---

# Coroot-node-agent

Coroot-node-agent is a Prometheus- and OpenTelemetry-compatible agent that gathers comprehensive telemetry data about
all containers running on a node and the node itself.

It collects and exports the following telemetry:

- **Metrics**: Exported in Prometheus format or sent using the Prometheus Remote Write protocol.
- **Traces**: eBPF-based network and application traces sent via OTLP/HTTP (OpenTelemetry protocol).
- **Logs**: Discovers container logs and sends them via OTLP/HTTP.
- **Profiles**: Uses the Pyroscope eBPF profiler to collect CPU profiles and sends them via a custom HTTP-based protocol.

## Node Agent Configuration

You can configure coroot-node-agent using command-line flags or environment variables.

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--listen` | `LISTEN` | `0.0.0.0:80` | HTTP listen address |
| `--cgroupfs-root` | `CGROUPFS_ROOT` | `/sys/fs/cgroup` | Path to the host's cgroup filesystem root |
| `--disable-log-parsing` | `DISABLE_LOG_PARSING` | `false` | Disable container log parsing |
| `--disable-pinger` | `DISABLE_PINGER` | `false` | Disable ICMP ping to upstreams |
| `--disable-l7-tracing` | `DISABLE_L7_TRACING` | `false` | Disable application-layer (L7) tracing |
| `--container-allowlist` | `CONTAINER_ALLOWLIST` | – | List of allowed containers (regex patterns) |
| `--container-denylist` | `CONTAINER_DENYLIST` | – | List of denied containers (regex patterns) |
| `--exclude-http-requests-by-path` | `EXCLUDE_HTTP_REQUESTS_BY_PATH` | – | Exclude HTTP paths from metrics/traces |
| `--track-public-network` | `TRACK_PUBLIC_NETWORK` | `0.0.0.0/0` | Public IP networks to track |
| `--ephemeral-port-range` | `EPHEMERAL_PORT_RANGE` | `32768-60999` | TCP ports to exclude from tracking |
| `--provider` | `PROVIDER` | – | `provider` label for `node_cloud_info` |
| `--region` | `REGION` | – | `region` label for `node_cloud_info` |
| `--availability-zone` | `AVAILABILITY_ZONE` | – | `availability_zone` label for `node_cloud_info` |
| `--instance-type` | `INSTANCE_TYPE` | – | `instance_type` label for `node_cloud_info` |
| `--instance-life-cycle` | `INSTANCE_LIFE_CYCLE` | – | `instance_life_cycle` label for `node_cloud_info` |
| `--log-per-second` | `LOG_PER_SECOND` | `10.0` | Rate limit for logs per second |
| `--log-burst` | `LOG_BURST` | `100` | Max burst for log rate limiting |
| `--max-label-length` | `MAX_LABEL_LENGTH` | `4096` | Max metric label length |
| `--collector-endpoint` | `COLLECTOR_ENDPOINT` | – | Unified base URL for telemetry export |
| `--api-key` | `API_KEY` | – | Coroot API key |
| `--metrics-endpoint` | `METRICS_ENDPOINT` | – | Custom URL for metrics export |
| `--traces-endpoint` | `TRACES_ENDPOINT` | – | Custom URL for traces export |
| `--traces-sampling` | `TRACES_SAMPLING` | `1.0` | Trace sampling rate (0.0 to 1.0) |
| `--logs-endpoint` | `LOGS_ENDPOINT` | – | Custom URL for logs export |
| `--profiles-endpoint` | `PROFILES_ENDPOINT` | – | Custom URL for profiles export |
| `--insecure-skip-verify` | `INSECURE_SKIP_VERIFY` | `false` | Skip TLS certificate verification |
| `--scrape-interval` | `SCRAPE_INTERVAL` | `15s` | How often to collect internal metrics |
| `--wal-dir` | `WAL_DIR` | `/tmp/coroot-node-agent` | Directory for WAL storage |
| `--max-spool-size` | `MAX_SPOOL_SIZE` | `500MB` | Max size for on-disk spool |

## Container Environment Variables

You can disable specific functionality for individual containers by setting environment variables within the container:

| Environment Variable | Description |
|---------------------|-------------|
| `COROOT_EBPF_PROFILING=disabled` | Disable eBPF profiling for this container |
| `COROOT_LOG_MONITORING=disabled` | Disable log monitoring and parsing for this container |
| `COROOT_EBPF_TRACES=disabled` | Disable eBPF traces for this container |

These environment variables are read from the container's process environment and allow fine-grained control over which containers are monitored by the agent.
