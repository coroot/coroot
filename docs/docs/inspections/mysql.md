---
sidebar_position: 13
---

# MySQL

Coroot inspects MySQL instances using metrics gathered by the
[cluster-agent](/metrics/cluster-agent#mysql), which connects to each discovered instance and
reads its status counters and `performance_schema` / `information_schema` views
(`SHOW GLOBAL STATUS`, `events_statements_summary_by_digest`, `events_statements_current`,
`data_lock_waits`, `innodb_trx`, `INNODB_METRICS`, and others). Backup state is collected from the
Percona XtraDB Cluster operator custom resources.

The report opens with a set of automated checks. Each check raises an alert when it crosses a
threshold and comes with supporting charts. Thresholds are configurable per project and can be
overridden per application.

Below the checks is a per-instance overview showing status, queries per second, latency,
replication status and lag, database size, and version. For a Galera or Group Replication
cluster the **Status** column shows the node's replication state (`Synced`, `Joining`,
`Recovering`, ...), which is what you want to look at first when a node is misbehaving but still
reachable.

## Availability

Detects MySQL instances that are unreachable or not accepting connections. The agent verifies the
connection on every scrape (`mysql_up`), and the check reports instances that are down.

This is also how a fenced Galera node shows up: a node that loses quorum stops listening
entirely, so it appears here as unavailable rather than as a replication problem.

## Latency

Flags instances whose **average query latency** exceeds the threshold (default 0.1s). Latency is
measured from the eBPF-observed client traffic, so it reflects what applications actually
experience.

When the check fires it lists the likely causes, ranked, so you don't have to read the charts to
form a hypothesis:

* **The most time-consuming query.** Taken from `events_statements_summary_by_digest` combined
  with in-flight statements from `events_statements_current`, so a long analytical query is
  attributed even while it is still running.
* **Queries blocked on InnoDB row locks.** The slowdown is contention, not the server being slow.
* **A long-running transaction.** Holds a read view and possibly row locks; often an
  `idle in transaction` session.
* **Spilling to on-disk temporary tables.** Large sorts or joins that no longer fit in memory.
* **InnoDB buffer pool pressure.** Either the pool is full of dirty pages and writes are waiting
  for a free page, or the hit rate has dropped and reads are going to disk because the working
  set no longer fits in `innodb_buffer_pool_size`.
* **Redo log stalls.** Writes waiting for the log buffer to be flushed - `innodb_log_buffer_size`
  is too small or the log disk is slow.
* **InnoDB purge lagging.** A long transaction is holding undo records, bloating undo and slowing
  reads.
* **Table lock waits.** `LOCK TABLES`, MyISAM tables, or DDL metadata locks.
* **CPU-bound.** More queries running concurrently than the instance has CPU cores.

## Replication status

Detects replicas whose IO or SQL replication thread is not running. The message carries the
replica's own error text (from `SHOW REPLICA STATUS`) when there is one, which usually names the
failing statement or connection problem directly.

If the instances also have no network connectivity to their replication peer, that is reported
alongside, so a broken replica caused by a network partition is not mistaken for a MySQL fault.

## Replication lag

Detects replicas that have fallen too far behind the primary (default 30s), based on
`Seconds_Behind_Source`.

Note that this check covers **asynchronous replication only**. Galera and Group Replication do not
report replication lag this way - they surface as flow control and applier queues in their own
checks below.

## Connections

Fires when an instance is using more than the threshold percentage (default 90%) of
`max_connections`, or when MySQL has already started refusing connections because the limit was
reached. Running out of connections makes the database unreachable for applications even though
it is healthy, so the check reports both the approaching limit and the refusals.

## Connection errors

Fires when clients are failing to connect (default: more than 1 aborted connection attempt per
second, from `Aborted_connects`). Unlike the Connections check above, the server has capacity -
the connections are being rejected or dropped during the handshake, typically because of bad
credentials, TLS errors, or handshake timeouts.

## Galera replication

Covers Percona XtraDB Cluster and MariaDB Galera. Fires when a node's replication is degraded in
a way that affects writes:

* **Flow control.** A node that cannot keep up makes the whole cluster pause writes until it
  catches up. The check fires when writes are paused more than the threshold share of the time
  (default 10%), and reports the node's receive queue so you can see which node is holding the
  cluster back.
* **Certification conflicts.** The same rows are being written on more than one node at once, so
  transactions are rolled back (`wsrep_local_cert_failures`, `wsrep_local_bf_aborts`). The fix is
  usually to route all writes through a single node.
* **Node state.** A node in a non-Primary component (lost quorum), not ready, or still joining
  (receiving a state transfer) cannot serve queries.

Charts show flow control, the write-set queues, cluster size, and conflicts.

## Group Replication

Covers MySQL Group Replication. Fires when a member is not `ONLINE` (`ERROR`, `OFFLINE`,
`RECOVERING`, or `UNREACHABLE`), or when its transaction queues exceed the threshold
(default 1000 transactions).

A member that is falling behind on applying transactions throttles writes for the whole group, so
a growing applier queue is a group-wide problem, not a local one. The check also reports when the
group has lost redundancy - for example, 2 of 3 members online means one more failure costs
quorum.

Cluster-wide member counts are derived from Coroot's own per-instance view rather than from a
single member's perception of the group, because a partitioned member sees only itself.

## Backups

Reports Percona XtraDB Cluster backups that are failing or stale (default: no successful backup
in the last 24h), along with the configured schedule, destination, and point-in-time recovery
state. A cluster whose backups quietly stopped is a recoverability risk that nothing else
surfaces.

## InnoDB, storage, and other charts

Beyond the checks, the report carries charts that DBAs use to reason about the instance:

* **InnoDB** - buffer pool size in bytes (used vs free) and hit rate, dirty pages, disk reads,
  row operations, transactions (commit/rollback), row lock waits and average wait time, disk I/O
  and fsyncs, redo log throughput and waits, and sort merge passes.
* **Locks** - locked queries, blocking queries, long-running transactions, deadlocks and
  lock-wait timeouts, InnoDB history list length (unpurged undo), and table lock waits.
* **Storage** - database and table sizes, plus **binary log** and **InnoDB undo tablespace**
  sizes. Binary logs and undo grow independently of table data and are a common cause of a full
  data volume, so they are also reported as growth sources in the node's disk usage
  investigation.
