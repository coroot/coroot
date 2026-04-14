---
sidebar_position: 2
---

# MySQL

Coroot leverages eBPF to monitor MySQL queries between applications and databases, requiring no additional integration.
While this approach provides a high-level view of database performance, it lacks the visibility needed to understand why issues occur within the database internals.

To bridge this gap, Coroot also collects statistics from the MySQL Performance Schema, complementing the eBPF-based metrics and traces.

## Prerequisites

This integration requires a database user with the following permissions:

```sql
CREATE USER 'coroot'@'%' IDENTIFIED BY '<PASSWORD>';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'coroot'@'%';
```

### Minimal permissions

If you don't need schema and size tracking for user databases, you can use narrower grants:

```sql
CREATE USER 'coroot'@'%' IDENTIFIED BY '<PASSWORD>';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'coroot'@'%';
GRANT SELECT ON performance_schema.* TO 'coroot'@'%';
```

Query performance, table I/O waits, replication, and settings change detection will all work fully - `performance_schema` data is not filtered by database-level privileges.
Schema and size tracking will not error but will only cover system databases, since MySQL filters `information_schema` views based on the user's privileges.

### Required privileges explained

**SELECT ON \*.\***

Coroot reads from:
- `performance_schema` tables (`events_statements_summary_by_digest`, `table_io_waits_summary_by_table`, `variables_info`) for query statistics, table I/O waits, and settings change detection.
- `information_schema` tables (`tables`, `columns`, `statistics`, `key_column_usage`) for schema and size tracking.

The grant must be `ON *.*` because MySQL filters `information_schema` views to only show objects the user has privileges on. A narrower grant would cause schema and size queries to return empty results for user databases. See [Minimal permissions](#minimal-permissions) if you don't need schema or size tracking.

**PROCESS**

Allows `SHOW GLOBAL STATUS` to return the full set of server status counters. Without this privilege, some counters are hidden.

**REPLICATION CLIENT**

Allows `SHOW REPLICA STATUS` (or `SHOW SLAVE STATUS` on older versions) to monitor replication health and lag. Not needed if the instance is not a replica.

:::note
All access is **read-only**. Coroot never modifies any data, schema, or configuration on your MySQL server.
:::

## What data is collected

### Server status and configuration

**Always collected.** Coroot runs `SHOW GLOBAL VARIABLES` and `SHOW GLOBAL STATUS` on each scrape to collect:

- **`version`**, **`server_id`**, **`server_uuid`** - identify the instance and display server info.
- **`max_connections`** - the configured connection limit.
- **`Threads_connected`**, **`Connections`**, **`Aborted_connects`** - current, total, and failed connection counts.
- **`Bytes_received`**, **`Bytes_sent`** - network traffic to and from the server.
- **`Questions`**, **`Slow_queries`** - query throughput and slow query rate.

### Settings change detection

**Enabled by default.** Controlled by `--track-database-changes` / `TRACK_DATABASE_CHANGES` (default: `true`).

Coroot compares successive `SHOW GLOBAL VARIABLES` snapshots to detect configuration changes (e.g., someone adjusts `innodb_buffer_pool_size`) and surfaces them in the change timeline. To determine which variables are writable it reads `performance_schema.variables_info` (MySQL) or `information_schema.SYSTEM_VARIABLES` (MariaDB).

### Query performance

**Always collected.** Coroot reads `performance_schema.events_statements_summary_by_digest` to get per-query statistics:

- **`SCHEMA_NAME`** - associate queries with a database.
- **`DIGEST`**, **`DIGEST_TEXT`** - normalized query text. MySQL's Performance Schema already replaces literal values with placeholders (e.g., `SELECT * FROM users WHERE id = ?`), but Coroot performs additional obfuscation to ensure that sensitive query arguments never appear in the collected telemetry data.
- **`COUNT_STAR`** - query execution rate (calls/sec).
- **`SUM_TIMER_WAIT`** - total execution time rate (seconds/sec).
- **`SUM_LOCK_TIME`** - lock wait time rate (seconds/sec).

The top 20 queries by execution time are reported each scrape interval.

### Table I/O waits

**Always collected.** Coroot reads `performance_schema.table_io_waits_summary_by_table`:

- **`OBJECT_SCHEMA`**, **`OBJECT_NAME`** - the database and table.
- **`SUM_TIMER_READ`**, **`SUM_TIMER_WRITE`** - cumulative read and write I/O wait time.

The top 20 tables by total I/O wait time are reported, broken down by read and write operations.

### Replication status

**Always collected** (requires the `REPLICATION CLIENT` privilege; safe to skip the privilege if the instance is not a replica).

Coroot runs `SHOW REPLICA STATUS` (falling back to `SHOW SLAVE STATUS` on MySQL &lt; 8.0.22):

- **IO/SQL thread running state and last error** - whether the replica is receiving and applying events.
- **`Seconds_Behind_Source`** - replication lag.
- **Source server ID and UUID** - identify the replication source.

### Schema tracking

**Enabled by default.** Controlled by `--track-database-changes` / `TRACK_DATABASE_CHANGES` (default: `true`).

Coroot queries `information_schema` to reconstruct table DDL and detect schema changes over time:

- **`information_schema.columns`** - column name, type, nullability, default, extra attributes.
- **`information_schema.statistics`** - index name, uniqueness, column list.
- **`information_schema.key_column_usage`** - foreign key name, columns, referenced table and columns.

### Table and database sizes

**Enabled by default.** Controlled by `--track-database-sizes` / `TRACK_DATABASE_SIZES` (default: `true`).

Coroot reads `information_schema.tables` (`data_length + index_length`) to track per-table and per-database sizes and detect growth trends.

### Common options for schema and size tracking

Both schema tracking and size tracking respect these additional flags:

- **`--max-tables-per-database`** / `MAX_TABLES_PER_DATABASE` (default: `1000`) - skip databases with more tables than this limit, protecting against expensive queries on very large schemas.
- **`--exclude-databases`** / `EXCLUDE_DATABASES` (default: `mysql`, `information_schema`, `performance_schema`, `sys`) - databases to exclude from schema and size tracking.

## Kubernetes (pod annotations)

The Kubernetes approach to monitoring databases typically involves running metric exporters as sidecar containers within database instance Pods.
However, this method can be challenging for certain use cases.
Coroot has a dedicated coroot-cluster-agent that can discover and gather metrics from databases without requiring a separate container for each database instance.

Coroot-cluster-agent automatically discovers and collects metrics from pods annotated with `coroot.com/mysql-scrape` annotations.
Coroot can retrieve database credentials from a Secret or be configured with plain-text credentials.

```yaml
coroot.com/mysql-scrape: "true"
coroot.com/mysql-scrape-port: "3306"

# plain-text credentials
coroot.com/mysql-scrape-credentials-username: "coroot"
coroot.com/mysql-scrape-credentials-password: "<PASSWORD>"

# credentials from a secret
coroot.com/mysql-scrape-credentials-secret-name: "mysql-secret"
coroot.com/mysql-scrape-credentials-secret-username-key: "username"
coroot.com/mysql-scrape-credentials-secret-password-key: "password"

# client TLS options: true, false, skip-verify, preferred (default: false)
coroot.com/mysql-scrape-param-tls: "false"
```

Note that Coroot checks only **Pod** annotations, not higher-level Kubernetes objects like Deployments or StatefulSets.

## Non-Kubernetes environments

In non-Kubernetes environments, the MySQL integration can be enabled via the Coroot UI.
In this setup, coroot-cluster-agent retrieves MySQL instance credentials from the Coroot configuration storage.

To configure the integration, go to the `MYSQL` tab and click the `Configure` button.
<img alt="MySQL Configuration" src="/img/docs/databases/mysql/configure.png" class="card w-800"/>

Then, switch to `Manual Configuration`, complete the form, and click `Save`.
<img alt="MySQL Manual Configuration" src="/img/docs/databases/mysql/manual.png" class="card w-600"/>

Coroot-cluster-agent updates its configuration every minute and also takes some time to collect metrics.
Please wait a few minutes for telemetry to appear.

## Troubleshooting

Check the coroot-cluster-agent logs if you encounter any issues.
