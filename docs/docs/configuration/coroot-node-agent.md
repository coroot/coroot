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
| `--disable-json-log-parsing` | `DISABLE_JSON_LOG_PARSING` | `false` | Disable extracting the message, severity, and attributes from JSON-formatted logs |
| `--disable-pinger` | `DISABLE_PINGER` | `false` | Disable ICMP ping to upstreams |
| `--disable-l7-tracing` | `DISABLE_L7_TRACING` | `false` | Disable application-layer (L7) tracing |
| `--disable-gpu-monitoring` | `DISABLE_GPU_MONITORING` | `false` | Disable GPU monitoring (NVML) |
| `--enable-java-tls` | `ENABLE_JAVA_TLS` | `false` | Enable Java TLS instrumentation via dynamic agent loading |
| `--enable-java-async-profiler` | `ENABLE_JAVA_ASYNC_PROFILER` | `false` | Enable Java profiling via async-profiler (CPU, memory allocations, lock contention) |
| `--ebpf-io-profiling` | `EBPF_IO_PROFILING` | `true` | Enable eBPF disk I/O profiling (block I/O time and bytes by code path) |
| `--go-heap-profiler` | `GO_HEAP_PROFILER` | `enabled` | Go heap profiling mode: `disabled`, `enabled` (passive), or `force` (enable profiling in all Go apps) |
| `--instrumentation-delay` | `INSTRUMENTATION_DELAY` | `30s` | Delay before enabling Python GIL and Node.js event loop instrumentation, after a process is started |
| `--container-allowlist` | `CONTAINER_ALLOWLIST` | – | List of allowed containers (regex patterns) |
| `--container-denylist` | `CONTAINER_DENYLIST` | – | List of denied containers (regex patterns) |
| `--skip-systemd-system-services` | `SKIP_SYSTEMD_SYSTEM_SERVICES` | `true` | Skip well-known systemd system services (apt, motd, udev, etc.) |
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
| `--log-patterns-per-container` | `LOG_PATTERNS_PER_CONTAINER` | `256` | Max unique log patterns per container per level |
| `--log-pattern-extraction-limit` | `LOG_PATTERN_EXTRACTION_LIMIT` | `100` | Max log messages per second per container for which patterns are extracted. Over-limit messages are counted under a dedicated `event was sampled` pattern (`0` - unlimited) |
| `--max-fqdns-per-container` | `MAX_FQDNS_PER_CONTAINER` | `50` | Max unique FQDN values per container in `container_dns_requests_total`; extras are bucketed under `~other` |
| `--max-label-length` | `MAX_LABEL_LENGTH` | `4096` | Max metric label length |
| `--collector-endpoint` | `COLLECTOR_ENDPOINT` | – | Unified base URL for telemetry export |
| `--api-key` | `API_KEY` | – | Coroot API key |
| `--metrics-endpoint` | `METRICS_ENDPOINT` | – | Custom URL for metrics export |
| `--traces-endpoint` | `TRACES_ENDPOINT` | – | Custom URL for traces export |
| `--traces-sampling` | `TRACES_SAMPLING` | `1.0` | Trace sampling rate (0.0 to 1.0) |
| `--logs-endpoint` | `LOGS_ENDPOINT` | – | Custom URL for logs export |
| `--profiles-endpoint` | `PROFILES_ENDPOINT` | – | Custom URL for profiles export |
| `--profiles-prune-fraction` | `PROFILES_PRUNE_FRACTION` | `0.0025` | Drop insignificant profile code paths accounting for less than this fraction of the profile total (`0` to disable) |
| `--insecure-skip-verify` | `INSECURE_SKIP_VERIFY` | `false` | Skip TLS certificate verification |
| `--ca-file` | `CA_FILE` | – | Path to the custom CA certificate file |
| `--scrape-interval` | `SCRAPE_INTERVAL` | `15s` | How often to collect internal metrics |
| `--wal-dir` | `WAL_DIR` | `/tmp/coroot-node-agent` | Directory for WAL storage |
| `--max-spool-size` | `MAX_SPOOL_SIZE` | `500MB` | Max size for on-disk spool |
| `--config-file` | `CONFIG_FILE` | `/etc/coroot-node-agent/config.yaml` | Path to the optional YAML configuration file (see [Configuration file](#configuration-file)) |

## Configuration file

Databases running on a node can be monitored by the node agent itself, from inside the node, instead of by
[coroot-cluster-agent](/configuration/coroot-cluster-agent). This is meant for the cases where the cluster agent cannot
reach the database: a MySQL or Postgres server installed as a system service on a VM and listening on `127.0.0.1` only,
or a database in a network the cluster agent has no route to. For everything else, including databases in Kubernetes
and managed cloud databases, use the cluster agent.

The databases to monitor are listed in a YAML file. The agent reads it once at startup from the path given by `--config-file`,
`/etc/coroot-node-agent/config.yaml` by default. The file is optional: when it is missing, the agent runs as usual and
monitors no databases. When the agent runs as a container, mount the file into the container at that path. Environment
variable references (`${VAR}`) in the file are expanded when it is read, so credentials can be kept out of the file.
This feature is not available on Windows.

```yaml
databases:
  - type: mysql                   # postgres, mysql, redis, memcached or mongodb
    host: 127.0.0.1               # an IP address the server listens on, as seen from the node
    port: "3306"
    credentials:
      username: coroot
      password: ${MYSQL_PASSWORD}
    params: {tls: "false"}        # the same per-type parameters as in the cluster agent: tls, sslmode, authSource
  - type: postgres
    host: 127.0.0.1
    port: "5432"
    credentials: {username: coroot, password: ${PG_PASSWORD}}
    params: {sslmode: disable}
  - type: redis
    host: 127.0.0.1
    port: "6379"
```

Every ten seconds the agent looks for a process on the node that listens on the configured address. The address must be
an IP address, not a hostname. A loopback address matches only a server running in the host network namespace, that is a
system service rather than a container. For a server in a container, use one of the container's own IP addresses. Once the
listening process is found, the agent connects to the database from the node and attributes everything it collects to that
process's container (a systemd unit, a Docker container or a pod), so the data shows up on the same application as the
server's CPU, memory, network and log telemetry. If the server is stopped, the collection stops with it and resumes when it
is back.

What is collected is the same as with the cluster agent: the per-type metrics described on the database pages
([Postgres](/databases/postgres), [MySQL](/databases/mysql), [Redis](/databases/redis), [MongoDB](/databases/mongodb),
[Memcached](/databases/memcached)), and the schema and settings changes shown as events on the application, which are sent
through the agent's logs endpoint. The required database privileges are also the same. The MySQL error log is not read
from `performance_schema.error_log` in this mode, since the node agent already collects the server's log file and journal
as container logs.

## Windows

The Windows agent uses the same flags, but only the platform-independent subset. Linux-only capabilities (eBPF L7 tracing and profiling, cgroups, Java/Python/Node.js instrumentation, the ICMP pinger, systemd handling, and trace/profile export) do not apply.

Flag names are identical (for example `--scrape-interval`), but **environment variables are prefixed with `COROOT_`**. On Windows environment variables are global, so the prefix avoids clashing with other software. For example, `SCRAPE_INTERVAL` becomes `COROOT_SCRAPE_INTERVAL`.

| Flag | Env Variable | Description |
|------|--------------|-------------|
| `--collector-endpoint` | `COROOT_COLLECTOR_ENDPOINT` | Base URL of your Coroot instance |
| `--api-key` | `COROOT_API_KEY` | Project API key |
| `--scrape-interval` | `COROOT_SCRAPE_INTERVAL` | Metrics collection interval |
| `--metrics-endpoint` / `--logs-endpoint` | `COROOT_METRICS_ENDPOINT` / `COROOT_LOGS_ENDPOINT` | Custom export URLs |
| `--insecure-skip-verify` | `COROOT_INSECURE_SKIP_VERIFY` | Skip TLS verification of the collector |
| `--ca-file` | `COROOT_CA_FILE` | Path to a custom CA certificate |
| `--disable-log-parsing` | `COROOT_DISABLE_LOG_PARSING` | Disable Windows Event Log and container log collection |
| `--disable-json-log-parsing` | `COROOT_DISABLE_JSON_LOG_PARSING` | Disable extracting the message, severity, and attributes from JSON-formatted logs |
| `--disable-gpu-monitoring` | `COROOT_DISABLE_GPU_MONITORING` | Disable NVIDIA GPU monitoring |
| `--container-allowlist` / `--container-denylist` | `COROOT_CONTAINER_ALLOWLIST` / `COROOT_CONTAINER_DENYLIST` | Regex of services to include or exclude |
| `--provider` / `--region` / `--availability-zone` / `--instance-type` / `--instance-life-cycle` | `COROOT_PROVIDER` and so on | Override `node_cloud_info` labels |
| `--wal-dir` / `--max-spool-size` | `COROOT_WAL_DIR` / `COROOT_MAX_SPOOL_SIZE` | Spool directory and max size |
| `--listen` | `COROOT_LISTEN` | Local `/metrics` listen address |

See the [Windows installation guide](../installation/windows) for setting these via the MSI or machine environment variables.

## Container Environment Variables

You can disable specific functionality for individual containers by setting environment variables within the container:

| Environment Variable | Description |
|---------------------|-------------|
| `COROOT_EBPF_PROFILING=disabled` | Disable eBPF profiling for this container |
| `COROOT_LOG_MONITORING=disabled` | Disable log monitoring and parsing for this container |
| `COROOT_EBPF_TRACES=disabled` | Disable eBPF traces for this container |

These environment variables are read from the container's process environment and allow fine-grained control over which containers are monitored by the agent.
