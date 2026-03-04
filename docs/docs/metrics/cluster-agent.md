---
sidebar_position: 2
toc_max_heading_level: 2
---

# Cluster-agent

This page describes metrics gathered by [coroot-cluster-agent](https://github.com/coroot/coroot-cluster-agent).

Coroot-cluster-agent is a dedicated tool for collecting cluster-wide telemetry data:
 * It gathers database metrics by discovering databases through Coroot's Service Map and Kubernetes control-plane.
Using the credentials provided by Coroot or via Kubernetes annotations, the agent connects to the identified databases such as Postgres, MySQL, Redis, Memcached, and MongoDB, collects database-specific metrics, and sends them to Coroot using the Prometheus Remote Write protocol.
 * When `--track-database-changes` is enabled, the agent tracks schema and configuration changes in databases. Change events are sent to Coroot as OpenTelemetry log records under the `DatabaseChanges` service name.
 * The agent can be integrated with AWS to discover RDS and ElastiCache clusters and collect their telemetry data.
 * The agent discovers and scrapes [custom metrics](/metrics/custom-metrics) from annotated pods.

## Postgres

### pg_up
* **Description**: Whether the Postgres server is reachable or not
* **Type**: Gauge
* **Source**: The agent checks that a connection to the server is still alive on each scrape

### pg_probe_seconds
* **Description**: How long it took to execute an empty SQL query (`;`) on the server. This metric shows the round-trip time between the agent and the server
* **Type**: Gauge
* **Source**: The time spent executing `db.Ping()`

### pg_scrape_error
* **Description**: Whether a scrape error occurred
* **Type**: Gauge
* **Labels**: error, warning

### pg_info
* **Description**: The server info
* **Type**: Gauge
* **Source**: [`pg_settings.server_version`](https://www.postgresql.org/docs/current/view-pg-settings.html)
* **Labels**: server_version

### pg_setting
* **Description**: Value of the pg_setting variable
* **Type**: Gauge
* **Source**: [`pg_settings`](https://www.postgresql.org/docs/current/view-pg-settings.html). The agent only collects variables of the following types: `integer`, `real` and `bool`
* **Labels**: name, unit

### pg_connections
* **Description**: The number of the database connections
* **Type**: Gauge
* **Source**: [`pg_stat_activity`](https://www.postgresql.org/docs/current/monitoring-stats.html#MONITORING-PG-STAT-ACTIVITY-VIEW)
* **Labels**:
  * db
  * user
  * state: current state of the connection, < active | idle | idle in transaction >
  * wait_event_type: [type](https://www.postgresql.org/docs/current/monitoring-stats.html#WAIT-EVENT-TABLE) of event that the connection is waiting for.
  * query - If the state of a connection is `active`, this is the currently executing query.
  For `idle in transaction` connections, this is the last executed query. This label holds a normalized and obfuscated query.

### pg_latency_seconds
* **Description**: Query execution time
* **Type**: Gauge
* **Source**: [`pg_stat_activity`](https://www.postgresql.org/docs/current/monitoring-stats.html#MONITORING-PG-STAT-ACTIVITY-VIEW), [`pg_stat_statements`](https://www.postgresql.org/docs/current/pgstatstatements.html)
* **Labels**:
  * summary: < avg | max | p50 | p75 | p95 | p99 >

### pg_db_queries_per_second
* **Description**: Number of queries executed in the database
* **Type**: Gauge
* **Source**: Aggregation of `pg_stat_activity.state = 'Active'` and `pg_stat_statements.calls`
* **Labels**: db

### pg_lock_awaiting_queries
* **Description**: Number of queries awaiting a lock
* **Type**: Gauge
* **Source**: Number of connections with `pg_stat_activity.wait_event_type = 'Lock'`.
The `blocking_query` label is calculated using the [pg_blocking_pids](https://www.postgresql.org/docs/current/functions-info.html) function
* **Labels**: db, user, blocking_query (the query holding the lock)

### Query Metrics

The [pg_stat_statements](https://www.postgresql.org/docs/current/pgstatstatements.html) view shows statistics only for queries that have been completed.
So, to provide comprehensive statistics, the agent extends this with data about the currently active queries from the [pg_stat_activity](https://www.postgresql.org/docs/current/monitoring-stats.html#MONITORING-PG-STAT-ACTIVITY-VIEW) view.

Collecting stats about each query would produce metrics with very high cardinality.
However, the primary purpose of such metrics is to show the most resource-consuming queries.
So, the agent collects these metrics only for TOP-20 queries by total execution time.


Each metric described below has `query`, `db` and `user` labels.
`Query` is a normalized and obfuscated query from pg_stat_statements.query, and pg_stat_activity.query.

For example, the following queries:

```sql
SELECT * FROM tbl WHERE id='1';
SELECT * FROM tbl WHERE id='2';
```

will be grouped to

```sql
SELECT * FROM tbl WHERE id=?;
```

### pg_top_query_calls_per_second
* **Description**: Number of times the query has been executed
* **Type**: Gauge
* **Source**: `pg_stat_statements.calls` and `pg_stat_activity.state = 'Active'`
* **Labels**: db, user, query

### pg_top_query_time_per_second
* **Description**: Time spent executing the query
* **Type**: Gauge
* **Source**: `clock_timestamp()-pg_stat_activity.query_start` and `pg_stat_statements.total_time`
* **Labels**: db, user, query

### pg_top_query_io_time_per_second
* **Description**: Time the query spent awaiting I/O
* **Type**: Gauge
* **Source**: `pg_stat_activity.wait_event_type = 'IO'`, `pg_stat_statements.blk_read_time` and `pg_stat_statements.blk_write_time`
* **Labels**: db, user, query

### Replication metrics

### pg_wal_receiver_status
* **Description**: WAL receiver status: 1 if the receiver is connected, otherwise 0
* **Type**: Gauge
* **Source**: `pg_stat_wal_receiver` and `pg_settings[primary_conninfo]`
* **Labels**: sender_host, sender_port

### pg_wal_replay_paused
* **Description**: Whether WAL replay paused or not
* **Type**: Gauge
* **Source**: `pg_is_wal_replay_paused()` or `pg_is_xlog_replay_paused()`

### pg_wal_current_lsn
* **Description**: Current WAL sequence number
* **Type**: Counter
* **Source**:  `pg_current_wal_lsn()` or `pg_current_xlog_location()`

### pg_wal_receive_lsn
* **Description**: WAL sequence number that has been received and synced to disk by streaming replication.
* **Type**: Counter
* **Source**:  `pg_last_wal_receive_lsn()` or `pg_last_xlog_receive_location()`

### pg_wal_reply_lsn
* **Description**: WAL sequence number that has been replayed during recovery
* **Type**: Counter
* **Source**:  `pg_last_wal_replay_lsn()` or `pg_last_xlog_replay_location()`

### Change tracking

When `--track-database-changes` is enabled, the agent detects and emits change events for:

* **Schema changes** — The agent periodically snapshots the DDL of every table (columns, constraints, indexes) across all databases. When a table's DDL changes between consecutive snapshots, a change event is emitted with a unified diff. Only schema modifications are tracked (e.g., `ALTER TABLE`, index creation/removal). The snapshot is collected by connecting to each database and querying `pg_catalog` and `information_schema`.
* **Settings changes** — The agent snapshots all `pg_settings` values each cycle. When a setting changes (e.g., after a configuration reload or restart), a change event is emitted with the diff. Session-level and client-level overrides are excluded.

Each change event includes `db.system`, `db.target`, `db.name`, `db_change.object`, and `db_change.type` attributes.

### Size metrics

The agent collects database and table size metrics. For table sizes, only the top 20 largest tables across all databases are reported.

### pg_database_size_bytes
* **Description**: Total size of the database in bytes
* **Type**: Gauge
* **Source**: [`pg_database_size()`](https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADMIN-DBSIZE)
* **Labels**: db

### pg_table_size_bytes
* **Description**: Total size of the table in bytes including indexes and TOAST
* **Type**: Gauge
* **Source**: [`pg_total_relation_size()`](https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADMIN-DBSIZE)
* **Labels**: db, schema, table

### pg_table_size_growth_bytes_per_second
* **Description**: Table size growth rate in bytes per second. Only the top 20 fastest growing tables across all databases are reported. Requires at least two collection cycles to compute.
* **Type**: Gauge
* **Source**: Computed from consecutive `pg_total_relation_size()` measurements
* **Labels**: db, schema, table

## MySQL

### mysql_up
* **Description**: Whether the MySQL server is reachable or not
* **Type**: Gauge

### mysql_scrape_error
* **Description**: Whether a scrape error occurred
* **Type**: Gauge
* **Labels**: error, warning

### mysql_info
* **Description**: The server info
* **Type**: Gauge
* **Labels**: server_version, server_id, server_uuid

### mysql_top_query_calls_per_second
* **Description**: Number of times the query has been executed
* **Type**: Gauge
* **Source**: `performance_schema.events_statements_summary_by_digest`
* **Labels**: schema, query

### mysql_top_query_time_per_second
* **Description**: Time spent executing the query
* **Type**: Gauge
* **Source**: `performance_schema.events_statements_summary_by_digest`
* **Labels**: schema, query

### mysql_top_query_lock_time_per_second
* **Description**: Time the query spent waiting for locks
* **Type**: Gauge
* **Source**: `performance_schema.events_statements_summary_by_digest`
* **Labels**: schema, query

### Replication metrics

### mysql_replication_io_status
* **Description**: Whether the replication IO thread is running
* **Type**: Gauge
* **Labels**: source_server_id, source_server_uuid, state, last_error

### mysql_replication_sql_status
* **Description**: Whether the replication SQL thread is running
* **Type**: Gauge
* **Labels**: source_server_id, source_server_uuid, state, last_error

### mysql_replication_lag_seconds
* **Description**: Seconds behind master
* **Type**: Gauge
* **Labels**: source_server_id, source_server_uuid

### Connection metrics

### mysql_connections_max
* **Description**: Maximum number of allowed connections
* **Type**: Gauge
* **Source**: `SHOW GLOBAL VARIABLES` (`max_connections`)

### mysql_connections_current
* **Description**: Current number of connected threads
* **Type**: Gauge
* **Source**: `SHOW GLOBAL STATUS` (`Threads_connected`)

### mysql_connections_total
* **Description**: Total number of connections since server start
* **Type**: Counter
* **Source**: `SHOW GLOBAL STATUS` (`Connections`)

### mysql_connections_aborted_total
* **Description**: Total number of aborted connection attempts
* **Type**: Counter
* **Source**: `SHOW GLOBAL STATUS` (`Aborted_connects`)

### Traffic metrics

### mysql_traffic_received_bytes_total
* **Description**: Total bytes received by the server
* **Type**: Counter
* **Source**: `SHOW GLOBAL STATUS` (`Bytes_received`)

### mysql_traffic_sent_bytes_total
* **Description**: Total bytes sent by the server
* **Type**: Counter
* **Source**: `SHOW GLOBAL STATUS` (`Bytes_sent`)

### mysql_queries_total
* **Description**: Total number of queries executed
* **Type**: Counter
* **Source**: `SHOW GLOBAL STATUS` (`Questions`)

### mysql_slow_queries_total
* **Description**: Total number of slow queries
* **Type**: Counter
* **Source**: `SHOW GLOBAL STATUS` (`Slow_queries`)

### mysql_top_table_io_wait_time_per_second
* **Description**: Time spent on table I/O operations
* **Type**: Gauge
* **Source**: `performance_schema.table_io_waits_summary_by_table`
* **Labels**: schema, table, operation

### Change tracking

When `--track-database-changes` is enabled, the agent detects and emits change events for:

* **Schema changes** — The agent periodically snapshots the DDL of every table (columns, indexes, foreign keys) across all databases by querying `information_schema`. When a table's DDL changes between consecutive snapshots, a change event is emitted with a unified diff.
* **Settings changes** — The agent snapshots all `SHOW GLOBAL VARIABLES` values each cycle. When a variable changes (e.g., after a `SET GLOBAL` or restart), a change event is emitted with the diff.

Each change event includes `db.system`, `db.target`, `db.name`, `db_change.object`, and `db_change.type` attributes.

### Size metrics

The agent collects database and table size metrics from `information_schema.tables`. For table sizes, only the top 20 largest tables across all databases are reported.

### mysql_database_size_bytes
* **Description**: Total size of the database in bytes, computed as the sum of `data_length + index_length` for all tables
* **Type**: Gauge
* **Source**: `information_schema.tables`
* **Labels**: db

### mysql_table_size_bytes
* **Description**: Total size of the table in bytes (`data_length + index_length`)
* **Type**: Gauge
* **Source**: `information_schema.tables`
* **Labels**: db, table

### mysql_table_size_growth_bytes_per_second
* **Description**: Table size growth rate in bytes per second. Only the top 20 fastest growing tables across all databases are reported. Requires at least two collection cycles to compute.
* **Type**: Gauge
* **Source**: Computed from consecutive `information_schema.tables` measurements
* **Labels**: db, table

## MongoDB

### mongo_up
* **Description**: Whether the MongoDB server is reachable or not
* **Type**: Gauge

### mongo_scrape_error
* **Description**: Whether a scrape error occurred
* **Type**: Gauge
* **Labels**: error, warning

### mongo_info
* **Description**: The server info
* **Type**: Gauge
* **Labels**: server_version

### mongo_rs_status
* **Description**: Replica set status: 1 if the member is part of a replica set
* **Type**: Gauge
* **Labels**: rs, role

### mongo_rs_last_applied_timestamp_ms
* **Description**: Timestamp of the last applied operation in milliseconds
* **Type**: Gauge

### Change tracking

When `--track-database-changes` is enabled, the agent detects and emits change events for:

* **Index changes** — MongoDB is schemaless, so instead of DDL the agent tracks indexes per collection. Each cycle it snapshots all indexes (name and key fields) for every collection. When indexes are added, removed, or modified, a change event is emitted with a unified diff.
* **Settings changes** — The agent snapshots all server parameters via `getParameter: "*"` each cycle. When a parameter changes (e.g., after `setParameter` or a restart), a change event is emitted with the diff.

MongoDB system databases (admin, config, local) are always excluded from index tracking.

Each change event includes `db.system`, `db.target`, `db.name`, `db_change.object`, and `db_change.type` attributes.

### Size metrics

The agent collects database and collection size metrics. For collection sizes, only the top 20 largest collections across all databases are reported. MongoDB system databases (admin, config, local) are always excluded.

### mongo_database_size_bytes
* **Description**: Total size of the database in bytes
* **Type**: Gauge
* **Source**: `listDatabases` command (`sizeOnDisk`)
* **Labels**: db

### mongo_collection_size_bytes
* **Description**: Total size of the collection in bytes (data + indexes + storage overhead)
* **Type**: Gauge
* **Source**: `collStats` command (`totalSize`)
* **Labels**: db, collection

### mongo_collection_size_growth_bytes_per_second
* **Description**: Collection size growth rate in bytes per second. Only the top 20 fastest growing collections across all databases are reported. Requires at least two collection cycles to compute.
* **Type**: Gauge
* **Source**: Computed from consecutive `collStats` measurements
* **Labels**: db, collection
