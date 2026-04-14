---
sidebar_position: 1
---

# Postgres

Coroot leverages eBPF to monitor Postgres queries between applications and databases, requiring no additional integration. 
While this approach provides a high-level view of database performance, it lacks the visibility needed to understand why issues occur within the database internals.

To bridge this gap, Coroot also collects statistics from Postgres system views such as `pg_stat_statements` and `pg_stat_activity`, complementing the eBPF-based metrics and traces.

## Prerequisites

This integration requires a database user with the `pg_monitor` role and the `pg_stat_statements` extension enabled.

```sql
CREATE ROLE coroot WITH LOGIN PASSWORD '<PASSWORD>';
GRANT pg_monitor TO coroot;
CREATE EXTENSION pg_stat_statements;
```

The `pg_stat_statements` extension must be loaded via the `shared_preload_libraries` server setting.

### Required privileges explained

**pg_monitor role**

The `pg_monitor` role includes `pg_read_all_stats` and `pg_read_all_settings`, which grant read access to all the monitoring views Coroot uses:
- `pg_settings` - server configuration.
- `pg_stat_statements` - query performance statistics (requires the extension).
- `pg_stat_activity` - current connections, active queries, and lock information.
- `pg_stat_wal_receiver` - WAL receiver status on replicas.
- `pg_is_in_recovery()`, `pg_current_wal_lsn()`, `pg_last_wal_receive_lsn()`, `pg_last_wal_replay_lsn()` - replication functions.

**Connection to the postgres database**

Coroot connects to the `postgres` database by default. For schema and size tracking, it also connects to each user database individually (Postgres isolates catalog data per database).

**Schema and size tracking**

Schema tracking queries `pg_catalog` system catalogs (`pg_class`, `pg_namespace`, `pg_attribute`, `pg_attrdef`, `pg_constraint`, `pg_indexes`) and calls `pg_total_relation_size()` for table sizes. The `pg_monitor` role provides sufficient access to these catalogs. Coroot connects to each user database to read its schema, so the monitoring user must have `CONNECT` privilege on the databases it should track (granted to `PUBLIC` by default in Postgres).

:::note
All access is **read-only**. Coroot never modifies any data, schema, or configuration on your Postgres server.
:::

## What data is collected

### Server version and settings

**Always collected.** Coroot reads `pg_settings` on each scrape to collect:

- **`server_version`** - identify the instance.
- All settings with integer, real, or boolean values are exported as metrics (e.g., `max_connections`, `shared_buffers`, `work_mem`).
- **`track_activity_query_size`** - used internally to determine the query text truncation limit.

### Settings change detection

**Enabled by default.** Controlled by `--track-database-changes` / `TRACK_DATABASE_CHANGES` (default: `true`).

Coroot compares successive `pg_settings` snapshots to detect configuration changes and surfaces them in the change timeline. Session-level and client-level overrides are excluded from tracking.

### Query performance

**Always collected.** Coroot reads `pg_stat_statements` (joined with `pg_roles` and `pg_database`) to get per-query statistics:

- **`datname`**, **`rolname`** - associate queries with a database and user.
- **`query`** - normalized query text. Postgres replaces literal values with parameter placeholders (e.g., `SELECT * FROM users WHERE id = $1`), and Coroot performs additional obfuscation to ensure that sensitive query arguments never appear in the collected telemetry data.
- **`queryid`** - unique identifier for the normalized query.
- **`calls`** - query execution rate (calls/sec).
- **`total_plan_time + total_exec_time`** - total execution time rate (seconds/sec). On Postgres < 13, uses `total_time`.
- **`blk_read_time + blk_write_time`** - I/O wait time rate (seconds/sec). On Postgres >= 17, also includes local and temp block I/O. Requires `track_io_timing = on` in the server configuration, otherwise these values are always zero.

The top 20 queries by execution time are reported each scrape interval.

### Connections and locks

**Always collected.** Coroot reads `pg_stat_activity` (joined with `pg_database`) to collect:

- **`datname`**, **`usename`**, **`state`**, **`wait_event_type`** - connection counts broken down by database, user, state, and wait event type.
- **`query`**, **`query_start`** - active query text and start time for latency estimation.
- **`backend_type`** - distinguish client backends from system processes (Postgres >= 10).
- **`pg_blocking_pids()`** - detect lock contention and report the number of queries blocked by each blocking query.

Query text from `pg_stat_activity` is also obfuscated before being stored.

### Replication status

**Always collected.** Coroot calls `pg_is_in_recovery()` to determine the instance role and then collects:

On a **primary**:
- **`pg_current_wal_lsn()`** - current WAL write position.

On a **replica**:
- **`pg_last_wal_receive_lsn()`** - last WAL position received from the primary.
- **`pg_last_wal_replay_lsn()`** - last WAL position replayed.
- **`pg_is_wal_replay_paused()`** - whether replay is paused.
- **`pg_stat_wal_receiver`** - whether the WAL receiver is connected.
- **`primary_conninfo`** from `pg_settings` - identify the primary host and port.

On Postgres < 10, the older `xlog` function names are used automatically.

### Schema tracking

**Enabled by default.** Controlled by `--track-database-changes` / `TRACK_DATABASE_CHANGES` (default: `true`).

Coroot connects to each user database and queries `pg_catalog` to reconstruct table DDL and detect schema changes over time:

- **`pg_class`** + **`pg_namespace`** + **`pg_attribute`** + **`pg_attrdef`** - column name, data type, nullability, default value.
- **`pg_constraint`** - primary keys, foreign keys, unique constraints, check constraints.
- **`pg_indexes`** - index definitions.

System schemas (`pg_catalog`, `information_schema`) are excluded.

### Table and database sizes

**Enabled by default.** Controlled by `--track-database-sizes` / `TRACK_DATABASE_SIZES` (default: `true`).

Coroot reads `pg_database_size()` for per-database sizes and `pg_total_relation_size()` (via `pg_class`) for per-table sizes including indexes and TOAST data. Template databases and databases that don't allow connections are skipped.

### Common options for schema and size tracking

Both schema tracking and size tracking respect these additional flags:

- **`--max-tables-per-database`** / `MAX_TABLES_PER_DATABASE` (default: `1000`) - skip databases with more tables than this limit, protecting against expensive queries on very large schemas.
- **`--exclude-databases`** / `EXCLUDE_DATABASES` (default: `postgres`, `mysql`, `information_schema`, `performance_schema`, `sys`) - databases to exclude from schema and size tracking. The default list is shared with the MySQL integration; for Postgres only `postgres` is relevant (the others don't exist as databases).

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
