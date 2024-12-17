---
sidebar_position: 7
---

# High Availability

Coroot supports high availability by allowing multiple instances to run simultaneously. 
These instances work with the same ClickHouse and Prometheus servers while maintaining independent copies of the [metric cache](/configuration/prometheus#metric-cache).

## Configuration Database

By default, Coroot uses [SQLite](/configuration/database#sqlite-default) to store configuration and incident history. 
However, to run multiple instances, you must use [PostgreSQL](/configuration/database#postgres) as the configuration database. This ensures consistent configuration and incident history across all instances.

## Alerting and Deployment Tracking

Coroot checks SLO compliance and tracks deployments every minute. 
To prevent race conditions and ensure accuracy, a leader election mechanism is implemented using PostgreSQL [advisory locks](https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS).

- **How it works**: Only the instance holding the lock performs the checks.
- **Automatic failover**: If the current leader becomes unavailable, the lock is automatically released, allowing another instance to take over these responsibilities seamlessly.

This approach ensures reliable monitoring and eliminates duplication of effort across instances.

