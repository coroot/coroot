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
 * The agent monitors GitOps tooling by reading [FluxCD](#fluxcd) and [ArgoCD](#argocd) custom resources through its embedded kube-state-metrics and exposing their state as metrics.
 * The agent monitors [Postgres backups](#postgres-backups) of clusters managed by CloudNativePG and the Percona Operator for PostgreSQL, reading their custom resources through the embedded kube-state-metrics.

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

### pg_autovacuum_workers
* **Description**: Number of running autovacuum worker processes
* **Type**: Gauge
* **Source**: `pg_stat_activity` (`backend_type = 'autovacuum worker'`)

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

### WAL size and archiving

### pg_wal_size_bytes
* **Description**: Size of the WAL directory
* **Type**: Gauge
* **Source**: `pg_ls_waldir()`

### pg_replication_slot_retained_wal_bytes
* **Description**: Amount of WAL retained for the replication slot
* **Type**: Gauge
* **Source**: `pg_replication_slots` (`restart_lsn`); `wal_status` is reported on Postgres >= 13
* **Labels**: slot, active, wal_status

### pg_wal_archived_segments_total
* **Description**: Number of WAL files successfully archived
* **Type**: Counter
* **Source**: `pg_stat_archiver`

### pg_wal_archive_failures_total
* **Description**: Number of failed attempts to archive WAL files
* **Type**: Counter
* **Source**: `pg_stat_archiver`

### pg_wal_archiving_status
* **Description**: 1 if the last WAL archive attempt succeeded, 0 if it failed
* **Type**: Gauge
* **Source**: `pg_stat_archiver`

### Checkpoint metrics

The agent tracks checkpoint activity from [`pg_stat_checkpointer`](https://www.postgresql.org/docs/current/monitoring-stats.html#MONITORING-PG-STAT-CHECKPOINTER-VIEW) (Postgres >= 17) or `pg_stat_bgwriter` (older versions). The counters are rebased and accumulated over the agent's lifetime.

### pg_checkpoints_scheduled_total
* **Description**: Number of scheduled checkpoints, including skipped ones
* **Type**: Counter
* **Source**: `pg_stat_checkpointer` (Postgres >= 17) or `pg_stat_bgwriter`
* **Labels**: type (`timed`, `requested`)

### pg_checkpoints_total
* **Description**: Number of checkpoints that have been completed
* **Type**: Counter
* **Source**: `pg_stat_checkpointer` (Postgres >= 17) or `pg_stat_bgwriter`

### pg_restartpoints_total
* **Description**: Number of restartpoints that have been completed on a standby
* **Type**: Counter
* **Source**: `pg_stat_checkpointer.restartpoints_done` (Postgres >= 17)

### pg_buffers_written_total
* **Description**: Total number of dirty buffers flushed to disk
* **Type**: Counter
* **Source**: `pg_stat_checkpointer` (Postgres >= 17) or `pg_stat_bgwriter`
* **Labels**: source (`checkpointer`)

### pg_time_since_last_checkpoint_seconds
* **Description**: Seconds since the last checkpoint observed by the agent
* **Type**: Gauge
* **Source**: Measured by the agent from the checkpoint completion counter

### pg_wal_since_last_checkpoint_bytes
* **Description**: Amount of WAL written since the last completed checkpoint (to be replayed in the case of a crash)
* **Type**: Gauge
* **Source**: `pg_control_checkpoint()` (`redo_lsn`)

### Transaction ID age

### pg_xid_age
* **Description**: Transactions since the oldest unfrozen transaction ID (age of `datfrozenxid`)
* **Type**: Gauge
* **Source**: `pg_database`
* **Labels**: db

### pg_multixact_age
* **Description**: Multixacts since the oldest unfrozen multixact ID (age of `datminmxid`)
* **Type**: Gauge
* **Source**: `pg_database`
* **Labels**: db

### pg_oldest_xmin_age
* **Description**: Age, in transactions, of the oldest transaction ID held back from freezing, by holder
* **Type**: Gauge
* **Source**: `pg_stat_activity`, `pg_replication_slots`, `pg_prepared_xacts` (Postgres >= 10)
* **Labels**: holder (`running_transaction`, `standby_feedback`, `replication_slot`, `prepared_transaction`)

### Change tracking

When `--track-database-changes` is enabled, the agent detects and emits change events for:

* **Schema changes** — The agent periodically snapshots the DDL of every table (columns, constraints, indexes) across all databases. When a table's DDL changes between consecutive snapshots, a change event is emitted with a unified diff. Only schema modifications are tracked (e.g., `ALTER TABLE`, index creation/removal). The snapshot is collected by connecting to each database and querying `pg_catalog` and `information_schema`.
* **Settings changes** — The agent snapshots all `pg_settings` values each cycle. When a setting changes (e.g., after a configuration reload or restart), a change event is emitted with the diff. Session-level and client-level overrides are excluded.

Each change event includes `db.system`, `db.target`, `db.name`, `db_change.object`, and `db_change.type` attributes.

### Size and bloat metrics

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

When `--track-database-bloat` is enabled, the agent also estimates wasted space (bloat) for tables and indexes from planner statistics (`pg_class.reltuples`/`relpages` and `pg_stats` column widths), without scanning table data. TOAST relations are excluded, and only the top tables and indexes by estimated bloat are reported per database. Estimates are approximate and depend on up-to-date `ANALYZE`/autovacuum statistics.

### pg_db_table_bloat_bytes
* **Description**: Estimated wasted space across all tables of the database
* **Type**: Gauge
* **Source**: Estimated from `pg_class` and `pg_stats`
* **Labels**: db

### pg_db_index_bloat_bytes
* **Description**: Estimated wasted space across all indexes of the database
* **Type**: Gauge
* **Source**: Estimated from `pg_class` and `pg_stats`
* **Labels**: db

### pg_table_bloat_bytes
* **Description**: Estimated wasted space in the table heap
* **Type**: Gauge
* **Source**: Estimated from `pg_class` and `pg_stats`
* **Labels**: db, schema, table

### pg_index_bloat_bytes
* **Description**: Estimated wasted space in the index
* **Type**: Gauge
* **Source**: Estimated from `pg_class` and `pg_stats`
* **Labels**: db, schema, table, index

When `--track-database-sizes` is enabled, the agent also reports dead-row statistics — a leading indicator of autovacuum falling behind, distinct from bloat. The dead/live counts let Coroot compute *autovacuum pressure* (how many times past its own autovacuum trigger a table sits), and dead bytes provides materiality. All three are emitted from one query with one top-N ranking (by dead bytes), so every reported table carries the complete set.

### pg_table_dead_tuple_bytes
* **Description**: Estimated size of dead tuples not yet reclaimed by vacuum (heap size × dead fraction)
* **Type**: Gauge
* **Source**: `pg_stat_user_tables` (`n_dead_tup`, `n_live_tup`) and `pg_relation_size()`
* **Labels**: db, schema, table

### pg_table_dead_tuples
* **Description**: Number of dead tuples not yet reclaimed by vacuum
* **Type**: Gauge
* **Source**: `pg_stat_user_tables` (`n_dead_tup`)
* **Labels**: db, schema, table

### pg_table_live_tuples
* **Description**: Estimated number of live tuples
* **Type**: Gauge
* **Source**: `pg_stat_user_tables` (`n_live_tup`)
* **Labels**: db, schema, table

### pg_table_seconds_since_last_autovacuum
* **Description**: Seconds since the last autovacuum of the table. Not reported for tables that have never been autovacuumed.
* **Type**: Gauge
* **Source**: `pg_stat_user_tables` (`now() - last_autovacuum`)
* **Labels**: db, schema, table

### pg_table_setting
* **Description**: Info metric (value always `1`) carrying a table's per-table autovacuum and autoanalyze reloption overrides as labels. Reported only for tables that override at least one setting; an unset override is an empty label. Coroot uses the trigger overrides to compute pressure against the table's own vacuum and analyze triggers, and the cost overrides to pinpoint per-table throttling.
* **Type**: Gauge
* **Source**: `pg_class.reloptions`
* **Labels**: db, schema, table, and the override values: `autovacuum_disabled` (`1` when `autovacuum_enabled=false`, which disables autoanalyze too), `autovacuum_vacuum_scale_factor`, `autovacuum_vacuum_threshold`, `autovacuum_vacuum_cost_delay`, `autovacuum_vacuum_cost_limit`, `autovacuum_analyze_scale_factor`, `autovacuum_analyze_threshold`

### pg_table_vacuum_in_progress
* **Description**: `1` if a vacuum is currently running on the table. Reported only while a vacuum is in progress. Lets Coroot tell a table that has a worker on it (possibly crawling) from one waiting for a free worker.
* **Type**: Gauge
* **Source**: [`pg_stat_progress_vacuum`](https://www.postgresql.org/docs/current/progress-reporting.html#VACUUM-PROGRESS-REPORTING) (Postgres >= 9.6)
* **Labels**: db, schema, table

### pg_table_vacuum_throttled
* **Description**: `1` if the running vacuum is sleeping on the cost-based delay (`VacuumDelay` wait event) at scrape time, `0` otherwise. Reported only while a vacuum is in progress; a high average means the vacuum is being throttled by `autovacuum_vacuum_cost_delay`/`autovacuum_vacuum_cost_limit`.
* **Type**: Gauge
* **Source**: `pg_stat_progress_vacuum` joined with `pg_stat_activity` (`wait_event = 'VacuumDelay'`)
* **Labels**: db, schema, table

### pg_table_mods_since_analyze
* **Description**: Rows modified (inserted, updated, or deleted) since the table's planner statistics were last analyzed. Unlike dead tuples this includes inserts, so it is collected as its own top-N (ranked by analyze pressure) rather than reusing the dead-tuple set. Coroot divides it by the autoanalyze trigger to compute how stale the statistics are.
* **Type**: Gauge
* **Source**: `pg_stat_user_tables` (`n_mod_since_analyze`)
* **Labels**: db, schema, table

### pg_table_reltuples
* **Description**: The planner's estimate of the number of live rows in the table. Used as the denominator of analyze/vacuum pressure. Unlike `n_live_tup` it is persisted in `pg_class`, so it survives a replica promotion (which resets the cumulative `pg_stat_*` counters) and keeps pressure from spiking to a false value on a freshly promoted primary.
* **Type**: Gauge
* **Source**: `pg_class.reltuples`
* **Labels**: db, schema, table

### pg_table_seconds_since_last_analyze
* **Description**: Seconds since the table's planner statistics were last refreshed by `ANALYZE` or autoanalyze. Not reported for tables that have never been analyzed.
* **Type**: Gauge
* **Source**: `pg_stat_user_tables` (`now() - greatest(last_analyze, last_autoanalyze)`)
* **Labels**: db, schema, table

## Postgres backups

Backup state is collected through the agent's embedded kube-state-metrics from the custom resources of [CloudNativePG](https://cloudnative-pg.io/) (`cnpg`) and the [Percona Operator for PostgreSQL](https://docs.percona.com/percona-operator-for-postgresql/) (pgBackRest, `percona`). Every metric carries an `operator` label identifying the source and, via the common `namespace`/`name` labels, correlates to the corresponding `DatabaseCluster` application in Coroot, so a single operator-agnostic set of metrics powers the backup inspection.

### pg_backup_target_info
* **Description**: A configured backup destination. cnpg reports a single object-storage target, and pgBackRest reports one series per repository. Coroot assembles the destination from the explicit `path` or the object-storage sub-fields.
* **Type**: Info
* **Source**: `Cluster.spec.backup` (cnpg), `PerconaPGCluster.spec.backups.pgbackrest.repos` (Percona)
* **Labels**: operator, method, path, endpoint, s3_bucket, s3_endpoint, gcs_bucket, azure_container, schedule, retention_policy

### pg_cluster_status
* **Description**: A cluster status condition (e.g. `ReadyForBackup`, `LastBackupSucceeded`, `ContinuousArchiving`, `PGBackRestRepoHostReady`). Value is 1 for the currently-active series. The reason feeds Coroot's "why backups are failing" hint.
* **Type**: Info
* **Source**: `.status.conditions`
* **Labels**: operator, type, status, reason

### pg_backup_last_successful_timestamp_seconds
* **Description**: Time of the last successful backup, per method. Reported by cnpg. For Percona it is derived from the individual backup runs.
* **Type**: Gauge
* **Source**: `Cluster.status.lastSuccessfulBackupByMethod`
* **Labels**: operator, method

### pg_backup_first_recoverability_point_timestamp_seconds
* **Description**: The oldest point in time the cluster can be restored to (start of the recovery window), per method.
* **Type**: Gauge
* **Source**: `Cluster.status.firstRecoverabilityPointByMethod`
* **Labels**: operator, method

### pg_backup_last_failed_timestamp_seconds
* **Description**: Time of the last failed backup.
* **Type**: Gauge
* **Source**: `Cluster.status.lastFailedBackup`
* **Labels**: operator

### pg_backup_schedule_info
* **Description**: The backup schedule (cron).
* **Type**: Info
* **Source**: `ScheduledBackup.spec.schedule` (cnpg)
* **Labels**: operator, cluster, schedule

### pg_backup_next_scheduled_timestamp_seconds
* **Description**: When the next scheduled backup is due. Coroot also derives an expected next run from the schedule and the last backup, so an overdue schedule is detected even when the operator stops advancing this value.
* **Type**: Gauge
* **Source**: `ScheduledBackup.status.nextScheduleTime` (cnpg)
* **Labels**: operator, cluster

### pg_backup_info
* **Description**: An individual backup run (one series per backup object), used to list recent backups. Carries immutable identity only. The run's phase is in `pg_backup_status`.
* **Type**: Info
* **Source**: `Backup` (cnpg), `PerconaPGBackup` (Percona)
* **Labels**: operator, cluster, method, kind, path

### pg_backup_status
* **Description**: The current phase of a backup run (e.g. `Running`, `Succeeded`, `Failed`, `completed`). Value is 1 for the currently-active series (a run's phase changes over its lifecycle, so only the live series is used).
* **Type**: Info
* **Source**: `Backup.status.phase` (cnpg), `PerconaPGBackup.status.state` (Percona)
* **Labels**: operator, status

### pg_backup_completed_timestamp_seconds
* **Description**: When a backup run completed.
* **Type**: Gauge
* **Source**: `Backup.status.stoppedAt` (cnpg), `PerconaPGBackup.status.completed` (Percona)
* **Labels**: operator, cluster

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

### Lock waits

The agent samples `performance_schema.data_lock_waits` on each scrape and reports the current
number of queries blocked on row locks. Unlike `mysql_top_query_lock_time_per_second` (which is
derived from cumulative counters and only updates once a blocked query completes), these gauges
reflect the live contention at scrape time, which makes them suitable for correlation with client
latency. Waiters are deduplicated by requesting transaction id, so a query blocked by several
holders is counted once. Not collected on MariaDB.

### mysql_locked_queries
* **Description**: Number of queries currently waiting for a lock
* **Type**: Gauge
* **Source**: `performance_schema.data_lock_waits` joined to `performance_schema.threads` and
`performance_schema.events_statements_current` on the requesting thread. The `query` label is the
obfuscated statement digest of the waiting (victim) query.
* **Labels**: schema, query (the query waiting for the lock)

### mysql_lock_awaiting_queries
* **Description**: Number of queries currently awaiting a lock, grouped by the query holding it
* **Type**: Gauge
* **Source**: `performance_schema.data_lock_waits` joined to `performance_schema.threads` and
`performance_schema.events_statements_current` on the blocking thread. The `blocking_query` label
is the obfuscated statement digest of the query holding the lock.
* **Labels**: schema, blocking_query (the query holding the lock)

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
* **Description**: The server info; `flavor` is `percona` for Percona Server for MongoDB, `mongodb` otherwise
* **Type**: Gauge
* **Labels**: server_version, flavor

### mongo_rs_status
* **Description**: Replica set status: 1 if the member is part of a replica set. The member reports its own role (`role` label), which Coroot uses to identify the primary and to derive each secondary's replication lag from `mongo_rs_last_applied_timestamp_ms`.
* **Type**: Gauge
* **Labels**: rs, role

### mongo_rs_last_applied_timestamp_ms
* **Description**: Timestamp of the member's last applied operation, in milliseconds. Coroot computes replication lag as the primary's value minus each secondary's.
* **Type**: Gauge

### mongo_rs_member_config_info
* **Description**: Replica set member configuration
* **Type**: Gauge
* **Source**: `replSetGetConfig`
* **Labels**: rs, member, arbiter, votes

### mongo_rs_config_info
* **Description**: Replica set configuration; Coroot warns when `write_concern_majority_journal_default` is `false` (acknowledged majority writes may be lost if a majority of members crash simultaneously)
* **Type**: Gauge
* **Source**: `replSetGetConfig`
* **Labels**: rs, write_concern_majority_journal_default

### mongo_profiling_level
* **Description**: Database profiling level (0 - off, 1 - slow operations, 2 - all operations). Coroot suggests enabling profiling when it is off everywhere, since it is the source of per-query statistics.
* **Type**: Gauge
* **Source**: the `profile` command (get-only, `profile: -1`)
* **Labels**: db

### mongo_oplog_window_seconds
* **Description**: Time span between the oldest and the newest oplog entries
* **Type**: Gauge
* **Source**: `local.oplog.rs`

### mongo_oplog_size_bytes / mongo_oplog_max_size_bytes
* **Description**: Current size of the oplog data and the configured maximum
* **Type**: Gauge
* **Source**: `collStats` on `local.oplog.rs`

### mongo_connections_current / mongo_connections_active / mongo_connections_max
* **Description**: Number of client connections, connections currently executing operations, and the connection limit
* **Type**: Gauge
* **Source**: `serverStatus.connections`

### mongo_connections_created_total
* **Description**: Total number of connections created
* **Type**: Counter

### mongo_connections_rejected_total
* **Description**: Total number of rejected connections
* **Type**: Counter

### mongo_connections_by_app
* **Description**: Number of connections by client application name (top 20)
* **Type**: Gauge
* **Source**: `$currentOp`
* **Labels**: app

### mongo_opcounters_total
* **Description**: Total number of operations by type (insert, query, update, delete, getmore, command)
* **Type**: Counter
* **Source**: `serverStatus.opcounters`
* **Labels**: op

### mongo_documents_returned_total
* **Description**: Total number of documents returned by queries; compared with the scanned counters to detect inefficient queries
* **Type**: Counter
* **Source**: `serverStatus.metrics.document`

### mongo_op_latency_seconds_total / mongo_op_latency_ops_total
* **Description**: Total operation latency and operation count by type (read, write, command). The average latency is `rate(latency) / rate(ops)`.
* **Type**: Counter
* **Source**: `serverStatus.opLatencies`
* **Labels**: type

### mongo_queued_operations
* **Description**: Number of operations queued waiting for a lock or a WiredTiger ticket
* **Type**: Gauge
* **Source**: `serverStatus.globalLock.currentQueue`
* **Labels**: type

### mongo_wt_tickets_available
* **Description**: Available WiredTiger concurrency tickets by type (read, write)
* **Type**: Gauge
* **Source**: `serverStatus.queues.execution` (7.0+) or `serverStatus.wiredTiger.concurrentTransactions`
* **Labels**: type

### mongo_wt_cache_used_bytes / mongo_wt_cache_dirty_bytes / mongo_wt_cache_max_bytes
* **Description**: WiredTiger cache usage, dirty bytes, and the configured maximum
* **Type**: Gauge
* **Source**: `serverStatus.wiredTiger.cache`

### mongo_wt_pages_evicted_by_app_threads_total
* **Description**: Pages evicted from the WiredTiger cache by application threads - a key cache-pressure signal
* **Type**: Counter

### mongo_wt_app_threads_evicting_seconds_total
* **Description**: Total time application threads spent evicting pages instead of serving queries
* **Type**: Counter

### mongo_wt_cache_bytes_read_into_total
* **Description**: Bytes read from disk into the WiredTiger cache (cache misses) - a direct signal that the working set does not fit in cache
* **Type**: Counter
* **Source**: `serverStatus.wiredTiger.cache`

### mongo_wt_checkpoints_total / mongo_wt_checkpoint_seconds_total
* **Description**: Number of WiredTiger checkpoints completed and the cumulative time spent in them. The rate of the seconds counter is the fraction of wall-time spent checkpointing - sustained high values indicate write-stall risk.
* **Type**: Counter
* **Source**: `serverStatus.wiredTiger` (`transaction` on 7.0, `checkpoint` on 8.0+)

### mongo_wt_journal_bytes_written_total
* **Description**: Bytes written to the WiredTiger journal (the crash-recovery write-ahead log)
* **Type**: Counter
* **Source**: `serverStatus.wiredTiger.log`

### mongo_wt_journal_bytes_since_checkpoint
* **Description**: Bytes written to the journal since the last checkpoint - the amount that would be replayed on crash recovery (MongoDB's analog of Postgres's `pg_wal_since_last_checkpoint_bytes`)
* **Type**: Gauge

### mongo_time_since_last_checkpoint_seconds
* **Description**: Time since the last WiredTiger checkpoint observed by the agent; rises above the checkpoint interval if checkpoints stall
* **Type**: Gauge

### mongo_scanned_keys_total / mongo_scanned_documents_total
* **Description**: Total number of index keys / documents scanned by the query executor. Compared with documents returned to detect inefficient queries.
* **Type**: Counter
* **Source**: `serverStatus.metrics.queryExecutor`

### mongo_collection_scans_total
* **Description**: Total number of queries that performed a collection scan
* **Type**: Counter

### mongo_scan_and_order_total
* **Description**: Total number of queries that performed an in-memory sort
* **Type**: Counter

### mongo_write_conflicts_total
* **Description**: Write conflicts (WiredTiger optimistic-concurrency retries); a rising rate indicates contention on hot documents
* **Type**: Counter
* **Source**: `serverStatus.metrics.operation`

### mongo_ttl_deleted_documents_total
* **Description**: Total number of documents deleted by TTL indexes
* **Type**: Counter

### mongo_flow_control_time_acquiring_seconds_total
* **Description**: Total time write operations spent acquiring flow control tickets; a non-zero rate means flow control is throttling writes on the primary
* **Type**: Counter
* **Source**: `serverStatus.flowControl`

### Per-query metrics

Collected from the database profiler (`system.profile`), which requires `operationProfiling` to be enabled on
the server, so the numbers cover slow (or, with Percona Server for MongoDB's `rateLimit`, sampled) operations.
Query shapes are normalized (literal values replaced with `?`); only the top 20 shapes by total execution time
are reported. Rates are smoothed over a 60-second window.

Unlike the other MongoDB metrics (read from in-memory counters via `serverStatus`), these have a server-side cost:
with profiling enabled, `mongod` writes a `system.profile` document for every captured operation. The agent only
reads new entries incrementally, but the capture overhead is paid by the server — bound it with `slowOpThresholdMs`
/ `mode: slowOp` and, on Percona Server for MongoDB, `rateLimit` sampling. See
[Enabling the profiler](/databases/mongodb) for the trade-off.

### mongo_top_query_calls_per_second / mongo_top_query_time_per_second / mongo_top_query_docs_returned_per_second / mongo_top_query_docs_examined_per_second / mongo_top_query_keys_examined_per_second
* **Description**: Per-query-shape execution rate, time, and documents/keys examined vs returned
* **Type**: Gauge
* **Source**: `system.profile`
* **Labels**: db, collection, query

### Current operations

### mongo_operations_waiting_for_lock
* **Description**: Number of operations waiting for a lock, by database
* **Type**: Gauge
* **Source**: `$currentOp`
* **Labels**: db

### mongo_long_running_operations
* **Description**: Number of operations of this query shape that have been running for at least 10s (their concurrency; top 20 shapes by count). Integrated over time this is the operation-time spent on long-running operations.
* **Type**: Gauge
* **Source**: `$currentOp`
* **Labels**: db, collection, query, plan

### mongo_fsync_locked
* **Description**: 1 if `db.fsyncLock()` is holding a global lock on this member (e.g. a filesystem-snapshot backup). On a secondary this blocks the oplog applier, so it is a direct root cause of replication lag.
* **Type**: Gauge
* **Source**: the `currentOp` command (`fsyncLock`)

### mongo_prepared_transactions
* **Description**: Number of transactions currently in the prepared state. A prepared transaction holds locks on secondaries until its commit/abort replicates, so a stuck or long one can block oplog apply.
* **Type**: Gauge
* **Source**: `serverStatus.transactions` (`currentPrepared`)

### mongo_open_transactions
* **Description**: Number of open (including prepared) transactions broken down by the client application that owns them - a long or prepared one holds locks and can block oplog apply. The client name is normalized (UUID/pod suffixes stripped) and empty names are reported as `unknown`; only the top 20 applications are kept.
* **Type**: Gauge
* **Source**: `$currentOp` (entries with a `transaction` sub-document)
* **Labels**: app

### Cursors

### mongo_cursors_open / mongo_cursors_open_no_timeout / mongo_cursors_timed_out_total
* **Description**: Open cursors, open cursors created with the `noTimeout` option (a leak indicator), and the total number of cursors that timed out (a sign a client stopped iterating results before exhausting the cursor)
* **Type**: Gauge (`mongo_cursors_open`, `mongo_cursors_open_no_timeout`), Counter (`mongo_cursors_timed_out_total`)
* **Source**: `serverStatus.metrics.cursor`

### Replication apply

### mongo_repl_apply_ops_total
* **Description**: Oplog operations applied by this member - the secondary apply throughput; a low rate on a lagging secondary indicates it is apply-bound
* **Type**: Counter
* **Source**: `serverStatus.metrics.repl.apply`

### mongo_repl_buffer_operations / mongo_repl_buffer_bytes
* **Description**: Number and size of oplog operations buffered on a secondary waiting to be applied; a backed-up buffer explains replication lag
* **Type**: Gauge
* **Source**: `serverStatus.metrics.repl.buffer`

### MongoDB backups

When running on Kubernetes, the agent collects backup state from the Percona Operator for MongoDB custom resources
(`PerconaServerMongoDB`, `PerconaServerMongoDBBackup`): `mongo_backup_target_info`, `mongo_backup_schedule_info`,
`mongo_backup_pitr_info`, `mongo_cluster_status`, `mongo_backup_info`, `mongo_backup_status`,
and `mongo_backup_completed_timestamp_seconds`.

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
* **Source**: Computed from consecutive `$collStats` measurements
* **Labels**: db, collection

### mongo_collection_storage_size_bytes
* **Description**: Bytes allocated on disk for documents of the collection
* **Type**: Gauge
* **Source**: `$collStats` (`storageStats.storageSize`)
* **Labels**: db, collection

### mongo_collection_free_storage_bytes
* **Description**: Reusable (fragmented) bytes within the allocated collection storage
* **Type**: Gauge
* **Source**: `$collStats` (`storageStats.freeStorageSize`)
* **Labels**: db, collection

### mongo_collection_documents
* **Description**: Number of documents in the collection
* **Type**: Gauge
* **Source**: `$collStats` (`storageStats.count`)
* **Labels**: db, collection

## FluxCD

The agent's embedded kube-state-metrics reads [FluxCD](https://fluxcd.io/) custom resources (`source.toolkit.fluxcd.io`, `kustomize.toolkit.fluxcd.io`, `helm.toolkit.fluxcd.io`, `fluxcd.controlplane.io`) and exposes their state. This requires the agent's service account to have `get`/`list`/`watch` access to those API groups, which the [Coroot Operator](/installation/k8s-operator) grants automatically.

Every metric below also carries `uid`, `name`, and `namespace` labels identifying the source custom resource. The `*_info` metrics are info-style: their value is always `1` and the useful data is carried in labels. The `*_status` metrics expose [Kubernetes status conditions](https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions): one series per condition, with the value `1` when the condition holds (`status: "True"`) and `0` otherwise (`"False"`/`"Unknown"`).

### fluxcd_git_repository_info / fluxcd_oci_repository_info / fluxcd_helm_repository_info
* **Description**: Information about a `GitRepository` / `OCIRepository` / `HelmRepository` source
* **Type**: Info
* **Source**: the source `spec`
* **Labels**: url, interval, suspended

### fluxcd_git_repository_status / fluxcd_oci_repository_status / fluxcd_helm_repository_status
* **Description**: Status conditions of a `GitRepository` / `OCIRepository` / `HelmRepository` source
* **Type**: Gauge
* **Source**: `status.conditions[]`
* **Labels**: type (condition type, e.g. `Ready`), reason

### fluxcd_helm_release_info
* **Description**: Information about a `HelmRelease`
* **Type**: Info
* **Source**: the HelmRelease `spec`
* **Labels**: suspended, interval, target_namespace, source_kind, source_name, source_namespace, chart, version, chart_ref_kind, chart_ref_name, chart_ref_namespace

### fluxcd_helm_release_status
* **Description**: Status conditions of a `HelmRelease`
* **Type**: Gauge
* **Source**: `status.conditions[]`
* **Labels**: type, reason

### fluxcd_helm_chart_info
* **Description**: Information about a `HelmChart`
* **Type**: Info
* **Source**: the HelmChart `spec`
* **Labels**: chart, version, source_kind, source_name, source_namespace, interval, suspended

### fluxcd_helm_chart_status
* **Description**: Status conditions of a `HelmChart`
* **Type**: Gauge
* **Source**: `status.conditions[]`
* **Labels**: type, reason

### fluxcd_kustomization_info
* **Description**: Information about a `Kustomization`
* **Type**: Info
* **Source**: the Kustomization `spec` and `status`
* **Labels**: suspended, interval, path, source_kind, source_name, source_namespace, target_namespace, last_applied_revision, last_attempted_revision

### fluxcd_kustomization_status
* **Description**: Status conditions of a `Kustomization`
* **Type**: Gauge
* **Source**: `status.conditions[]`
* **Labels**: type, reason

### fluxcd_kustomization_inventory_entry_info
* **Description**: A resource managed by a `Kustomization` (one series per inventory entry)
* **Type**: Info
* **Source**: `status.inventory.entries[]`
* **Labels**: entry_id

### fluxcd_kustomization_dependency_info
* **Description**: A dependency declared by a `Kustomization` (one series per `dependsOn` entry)
* **Type**: Info
* **Source**: `spec.dependsOn[]`
* **Labels**: depends_on_name, depends_on_namespace

### fluxcd_resourceset_info
* **Description**: Information about a `ResourceSet`
* **Type**: Info
* **Source**: the ResourceSet `status`
* **Labels**: last_applied_revision

### fluxcd_resourceset_status
* **Description**: Status conditions of a `ResourceSet`
* **Type**: Gauge
* **Source**: `status.conditions[]`
* **Labels**: type, reason

### fluxcd_resourceset_inventory_entry_info
* **Description**: A resource managed by a `ResourceSet` (one series per inventory entry)
* **Type**: Info
* **Source**: `status.inventory.entries[]`
* **Labels**: entry_id

### fluxcd_resourceset_dependency_info
* **Description**: A dependency declared by a `ResourceSet` (one series per `dependsOn` entry)
* **Type**: Info
* **Source**: `spec.dependsOn[]`
* **Labels**: depends_on_kind, depends_on_name, depends_on_namespace

## ArgoCD

The agent's embedded kube-state-metrics reads [ArgoCD](https://argo-cd.readthedocs.io/) `Application` resources (`argoproj.io/v1alpha1`) and exposes their sync, health, and operation state. This requires the agent's service account to have `get`/`list`/`watch` access to the `argoproj.io` API group, which the [Coroot Operator](/installation/k8s-operator) grants automatically.

Every metric below also carries `uid`, `name`, and `namespace` labels identifying the `Application`. All of these are info-style metrics: their value is always `1` and the meaningful state is carried in labels (such as `sync_status`), so a status that ArgoCD doesn't currently report simply has no series.

### argocd_application_info
* **Description**: Information about an `Application`
* **Type**: Info
* **Source**: the Application `spec` and `status`
* **Labels**: project, source_type, repo, path, chart, target_revision, dest_server, dest_name, dest_namespace, revision

### argocd_application_sync_status
* **Description**: Sync status of an `Application`
* **Type**: Info
* **Source**: `status.sync.status`
* **Labels**: sync_status (e.g. `Synced`, `OutOfSync`, `Unknown`)

### argocd_application_health_status
* **Description**: Health status of an `Application`
* **Type**: Info
* **Source**: `status.health.status`
* **Labels**: health_status (e.g. `Healthy`, `Progressing`, `Degraded`, `Suspended`, `Missing`, `Unknown`)

### argocd_application_operation_status
* **Description**: Phase of the most recent sync operation
* **Type**: Info
* **Source**: `status.operationState.phase`
* **Labels**: operation_phase (e.g. `Running`, `Succeeded`, `Failed`, `Error`, `Terminating`)

### argocd_application_operation_finished_timestamp_seconds
* **Description**: When the most recent sync operation finished, as a Unix timestamp
* **Type**: Gauge
* **Source**: `status.operationState.finishedAt`

### argocd_application_resource_info
* **Description**: A resource managed by an `Application` (one series per resource)
* **Type**: Info
* **Source**: `status.resources[]`
* **Labels**: resource_group, resource_kind, resource_namespace, resource_name

### argocd_application_resource_sync_status
* **Description**: Sync status of a managed resource
* **Type**: Info
* **Source**: `status.resources[].status`
* **Labels**: resource_group, resource_kind, resource_namespace, resource_name, sync_status

### argocd_application_resource_health_status
* **Description**: Health status of a managed resource
* **Type**: Info
* **Source**: `status.resources[].health.status`
* **Labels**: resource_group, resource_kind, resource_namespace, resource_name, health_status

### argocd_application_resource_status
* **Description**: Result for a resource from the most recent sync operation
* **Type**: Info
* **Source**: `status.operationState.syncResult.resources[]`
* **Labels**: resource_group, resource_kind, resource_namespace, resource_name, status (e.g. `Synced`, `Pruned`, `SyncFailed`)
