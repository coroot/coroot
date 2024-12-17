---
sidebar_position: 5
---

# ClickHouse

Coroot uses ClickHouse to store Logs, Traces, and Profiles. 
To integrate Coroot with ClickHouse, go to the **Project Settings**, click on **Clickhouse**, and configure the ClickHouse 
address and credentials as shown in the following example:

<img alt="ClickHouse configuration" src="/img/docs/clickhouse_configuration.png" class="card w-1200"/>

Coroot handles its own schema in ClickHouse, so you don’t need to do anything manually.

## Clustered ClickHouse
If Coroot is set up to work with a distributed ClickHouse cluster (sharded and/or replicated), 
it automatically detects it using the `SHOW CLUSTERS` command.

Here’s how Coroot chooses a cluster:

* If no clusters are set up, it creates the table on the connected ClickHouse instance (single-node mode)
* If there’s only one cluster, it uses that
* If there are multiple clusters, it chooses the coroot cluster, or default if coroot isn’t available

## Multi-tenancy mode

Coroot supports a multi-tenancy mode, enabling a single ClickHouse instance to store logs, metrics, and profiles for multiple projects (or clusters).

In this mode, Coroot automatically creates a dedicated database for each project. 
Telemetry data pushed by Coroot agents (coroot-node-agent and coroot-cluster-agent) are stored in their respective project databases, 
ensuring isolation and efficient querying for individual projects.

## TTL (Time To Live)

ClickHouse allows you to set a retention policy for tables when they are created. 
Currently, Coroot uses a hardcoded TTL of 7 days and doesn't yet support changing it through the UI. 
However, you can manually adjust it by running the [ALTER TABLE ... MODIFY TTL](https://clickhouse.com/docs/en/sql-reference/statements/alter/ttl) query.

