---
sidebar_position: 1
---

# Postgres

Coroot leverages eBPF to monitor Postgres queries between applications and databases, requiring no additional integration. 
While this approach provides a high-level view of database performance, it lacks the visibility needed to understand why issues occur within the database internals.

To bridge this gap, Coroot also collects statistics from Postgres system views such as `pg_stat_statements` and `pg_stat_activity`, complementing the eBPF-based metrics and traces.

## Prerequisites

This integration requires a database user with the `pg_monitor` role and the `pg_stat_statements` extension enabled. 

```postgresql
create role coroot with login password '<PASSWORD>';
grant pg_monitor to coroot;
create extension pg_stat_statements;
```

The `pg_stat_statements` extension should be loaded via the `shared_preload_libraries` server setting.

## Kubernetes (pod annotations)

The Kubernetes approach to monitoring databases typically involves running metric exporters as sidecar containers within database instance Pods.
However, this method can be challenging for certain use cases.
Coroot has a dedicated coroot-cluster-agent that can discover and gather metrics from databases without requiring a separate container for each database instance.

Coroot-cluster-agent automatically discovers and collects metrics from pods annotated with `coroot.com/postgres-scrape` annotations.
Coroot can retrieve database credentials from a Secret or be configured with plain-text credentials.

```yaml
coroot.com/postgres-scrape: "true"
coroot.com/postgres-scrape-port: "5432"

# plain-text credentials
coroot.com/postgres-scrape-credentials-username: "coroot"
coroot.com/postgres-scrape-credentials-password: "<PASSWORD>"

# credentials from a secret
coroot.com/postgres-scrape-credentials-secret-name: "postgres-secret"
coroot.com/postgres-scrape-credentials-secret-username-key: "username"
coroot.com/postgres-scrape-credentials-secret-password-key: "password"

# client SSL options: disable, require, verify-ca (default: disable)
coroot.com/postgres-scrape-param-sslmode: "disable"
```

Note that Coroot checks only **Pod** annotations, not higher-level Kubernetes objects like Deployments or StatefulSets.

## Non-Kubernetes environments

In non-Kubernetes environments, the Postgres integration can be enabled via the Coroot UI.
In this setup, coroot-cluster-agent retrieves Postgres instance credentials from the Coroot configuration storage.

To configure the integration, go to the `POSTGRES` tab and click the `Configure` button. 
<img alt="Postgres Configuration" src="/img/docs/databases/postgres/configure.png" class="card w-800"/>

Then, switch to `Manual Configuration`, complete the form, and click `Save`.
<img alt="Postgres Manual Configuration" src="/img/docs/databases/postgres/manual.png" class="card w-600"/>

Coroot-cluster-agent updates its configuration every minute and also takes some time to collect metrics.
Please wait a few minutes for telemetry to appear.

## Troubleshooting

Check the coroot-cluster-agent logs if you encounter any issues.
