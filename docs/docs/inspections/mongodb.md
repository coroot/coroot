---
sidebar_position: 12
---

# Mongodb

Coroot inspects MongoDB replica sets using metrics gathered by the
[cluster-agent](/metrics/cluster-agent#mongodb), which connects to each discovered `mongod` instance
and reads `serverStatus`, `replSetGetStatus`, `$queryStats`, `$currentOp`, `$collStats`, and the oplog.
Backup state is collected from the Percona Operator for MongoDB custom resources.

The report opens with a set of automated checks. Each check raises an alert when it crosses a
threshold and comes with supporting charts. Thresholds are configurable per project and can be
overridden per application. Where possible, a firing check explains the likely root cause,
not just the symptom.

<img alt="Mongodb" src="/img/docs/mongodb.png" class="card w-1200"/>

## Availability

Fires when a MongoDB instance is unreachable, or when a replica set has **no primary** while some
members are still up (majority lost or an election deadlock). The details also surface members stuck
in abnormal states (`RECOVERING`, `ROLLBACK`, `STARTUP2`, `DOWN`).

## Latency

Fires when the average operation latency (from `serverStatus.opLatencies`) exceeds the threshold
(0.1s by default). The check ranks the likely causes: exhausted WiredTiger tickets, cache pressure
(application threads evicting pages), and the most time-consuming query shapes. Coroot also measures
client-side latency via eBPF, so server-side and client-side views can be compared directly.

## Replication lag

Fires when a secondary falls behind the primary by more than the threshold (30s by default), based on
the primary's view of per-member optimes. A lagging secondary serves stale reads to
`secondaryPreferred` clients and risks data loss on failover. The details explain why the member is
lagging, distinguishing oplog apply that is **blocked** from apply that is merely **slow**:

- **blocked** (the apply rate drops to ~0 while the apply buffer backs up) — a held lock is stopping the
  applier: `db.fsyncLock()` on the member (typically a filesystem-snapshot backup), a prepared transaction
  holding locks, or a long-running operation on the member holding a lock;
- **slow** — saturation on the secondary (no tickets, cache pressure), a slow data disk, or flow control
  engaged on the primary;
- an abnormal member state (e.g., initial sync), or lag approaching the oplog window — at which point the
  member will require a full resync.

## Oplog window

Fires when the time span covered by the oplog falls below the threshold (1 hour by default) and the
oplog is already full. A small oplog window means any member that lags or stays down for longer than
the window must perform a full initial sync, and it narrows the point-in-time recovery window.
The details report the current churn rate (how fast the oplog is being rewritten) and suggest
increasing the oplog size.

## Connections

Fires when an instance approaches its connection limit (90% of `net.maxIncomingConnections` by
default). Once the limit is reached, new connections are rejected and dependent applications fail.
The details show whether connections are already being rejected and which client application holds
the most connections — usually a sign of a missing or misconfigured connection pool.

## Saturation

Fires when operations are queuing because the WiredTiger concurrency tickets are exhausted.
Ticket exhaustion stalls all operations regardless of their own cost. The details list long-running
operations (with their query shapes and plans) that may be holding tickets, lock waits, and long
WiredTiger checkpoints.

## Storage fragmentation

Fires when a large share of the storage allocated for a collection is reclaimable free space
(over 50% and at least 1 GiB by default) — typically left behind by mass deletions. The space is not
returned to the operating system until `compact` is run (rolling, secondaries first).

## Backups

For clusters managed by the Percona Operator for MongoDB, this check fires when no successful backup
has completed within the threshold (24h by default), the last backup attempt failed, or a scheduled
backup is overdue. The report shows the backup destination, schedule, point-in-time recovery status,
and recent backup runs.

## Configuration hints

In addition to the checks, the MongoDB report can show a configuration-hint banner (not an alert - an
advisory, like the Postgres `track_io_timing` hint). Coroot warns when the **database profiler is
disabled**, since the profiler is the source of per-query statistics and without it the top-queries view
stays empty. Enable `operationProfiling` (on Percona Server for MongoDB with `rateLimit` sampling) to
populate it.
