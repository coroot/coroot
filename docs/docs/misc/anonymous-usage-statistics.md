---
sidebar_position: 1
---

# Anonymous usage statistics

To improve Coroot we collect anonymous usage statistics. The collection of statistics is enabled by default, but you can opt out at any time.

## What exactly is being collected

The following is an example of the reported payload:

```json
{
    "instance": {
        "uuid": "4423595b-9d97-4e01-a19f-8e3d60c83b2a", // generated upon the first startup and stored in `data-dir/instance.uuid`
        "version": "0.20.0", // Coroot version
        "database_type": "sqlite" // the type of database being used
    },
    "integration": {
        "prometheus": true, // shows whether a Prometheus integration has been configured or not
        "node_agent": true, // shows whether Coroot has seen the metrics gathered by `node-agent` or not
        "kube_state_metrics": true, // shows whether Coroot has seen the metrics gathered by `kube-state-metrics` or not
        "inspection_overrides": { // the number of overridden inspection thresholds
            "CPUNode": {"project_level": 1, "application_level": 2},
            "NetworkRTT": {"project_level": 0, "application_level": 1}
        },
        "application_categories": 3, // the number of configured application categories
        "alerting_integrations": ["slack"], // configured alerting integration types
        "cloud_costs": true, // shows if prices are available for the cloud in use
        "clickhouse": true, // whether a Clickhouse integration is configured
        "tracing": true, // whether Distributed Tracing is enabled
        "logs": true, // whether Logs Monitoring is enabled
        "profiles": true // whether Continuous Profiling is enabled
    },
    "stack": {
        "clouds": ["aws", "hetzner", "ovh"], // based on the `node_cloud_info` metric
        "services": ["pgbouncer", "postgres", "redis"], // based on the `container_application_type` metric
        "instrumented_service": ["postgres", "redis"] // the services monitored by the relevant exporters, such as `pg-agent` or `redis_exporter`
    },
    "infra": {
        "projects": 2, // the number of configured projects
        "applications": 18, // the total number of applications
        "nodes": 10, // the total number of nodes
        "instances": 50, // the total number of application instances (containers)
        "deployments": 1, // the number of application rollouts
        "deployment_summaries": {"-CPU": 1, "+Logs": 1, "-Memory": 1} // the number of notable changes during new deployments by type
    },
    "ux": {
        "users_by_screen_size": { // the number of browsers by display breakpoints (https://vuetifyjs.com/en/features/breakpoints/)
            "xs": 1,
            "lg": 7,
            "xl": 5
        },
        "users": [ // browser UUIDs
            "20174a56-4a1e-41ca-a90d-b657d1fa873e"
        ],
        "page_views": { // the number of page views by type
            "/p/$projectId": 1,
            "/p/$projectId/app/$id/SLO": 3
        },
        "world_load_time_avg": 0.053, // the average time to load telemetry from the Prometheus cache
        "audit_time_avg": 0.011, // the average time to audit telemetry
        "sent_notifications": {"slack": 4} // the number of sent notifications by destination
    },
    "performance": {
        "constructor": {
            "stages": { // the maximum time to load and process metrics from the Prometheus cache
                "query": 0.022014374,
                "load_rds": 0.000004882,
                "load_nodes": 0.000139763,
                "join_db_cluster": 0.000008697,
                "load_containers": 0.017340304,
                "enrich_instances": 0.002621411,
                "load_k8s_metadata": 0.000543033
            },
            "queries": { // per Prometheus query statistics: result set size, latency and status
                "node_info": {
                    "failed": false,
                    "query_time": 0.012892758,
                    "metrics_count": 4
                },
                // ...
                "container_net_latency": {
                    "failed": false,
                    "query_time": 0.015887205,
                    "metrics_count": 130
                }
            }
        },
        "cpu_usage": [0.079, 0,071, ...], // CPU usage of the Coroot process
        "memory_usage": [27086848, 27086848, ...], // memory usage of the Coroot process
    },
    "profile": {
        "from": 1678113877,
        "to": 1678114877,
        "cpu": "...",   // base64-encoded CPU profile in the pprof format
        "memory": "..." // base64-encoded memory profile in the pprof format
    }
}
```

As you can see, the data is absolutely anonymous.

## How we process the data

Anonymous usage statistics are reported to our collector at https://coroot.com/ce/usage-statistics. 
Coroot Inc uses the described statistics for its own purposes (improving the product) and does not share the data with any third parties.

## Disable usage statistics
You can disable the collecting of usage statistics by using the `--disable-usage-statistics` command line argument.

Docker:

```bash
docker run ... ghcr.io/coroot/coroot --disable-usage-statistics
```

Kubernetes:

```yaml
...
spec:
containers:
- name: coroot
  image: ghcr.io/coroot/coroot
  args: ["--disable-usage-statistics"]
  ...
```
