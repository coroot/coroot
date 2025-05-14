---
sidebar_position: 2
---

# Cluster-agent

This page describes metrics gathered by [coroot-cluster-agent](https://github.com/coroot/coroot-cluster-agent).

Coroot-cluster-agent is a dedicated tool for collecting cluster-wide telemetry data:
 * It gathers database metrics by discovering databases through Coroot's Service Map and Kubernetes control-plane. 
Using the credentials provided by Coroot or via Kubernetes annotations, the agent connects to the identified databases such as Postgres, MySQL, Redis, Memcached, and MongoDB, collects database-specific metrics, and sends them to Coroot using the Prometheus Remote Write protocol.
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
