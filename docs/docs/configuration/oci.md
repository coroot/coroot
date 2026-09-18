---
sidebar_position: 5.7
---

# OCI

The OCI integration lets Coroot discover the managed databases of one or more Oracle Cloud Infrastructure
compartments, MySQL HeatWave DB systems (with their read replicas), Database with PostgreSQL DB systems (with their
standby instances) and OCI Cache clusters (Valkey), and collect their telemetry.
Discovered instances appear in the Service Map as separate applications, linked to the services that connect to them
through the eBPF-based metrics collected on the client side.

The integration adds:

* **Instance inventory and status**: engine, version, shape, region and availability domain
* **OS-level metrics** from [OCI Monitoring](https://docs.oracle.com/en-us/iaas/Content/Monitoring/home.htm): CPU,
  memory, storage, network and disk I/O of the DB systems, CPU, memory and network of the cache clusters
* **Logs**: the PostgreSQL DB system and OCI Cache logs from [OCI Logging](https://docs.oracle.com/en-us/iaas/Content/Logging/home.htm)
  when their service logs are enabled, and the MySQL HeatWave error log read from the server, see [Logs](#logs)

Database internals (query statistics, locks, replication) are collected separately, see
[Database credentials](#database-credentials).

All OCI API calls are made by the [cluster-agent](/installation/architecture), not by the Coroot server. The agent
polls the MySQL, PostgreSQL and OCI Cache APIs once a minute. Like the [GCP integration](/configuration/gcp), the OCI
integration is configured only as code: in the Coroot custom resource when Coroot is deployed by the
[Kubernetes Operator](/installation/k8s-operator), or in the cluster-agent's
[configuration file](/configuration/coroot-cluster-agent#configuration-file). The **Cloud integrations** page of the
project settings shows the discovery status and the discovered instances, and warns when the cluster runs on OCI
without the integration configured.

## IAM permissions

Whatever identity the agent uses, it needs read access to these resource types in the compartments:

| Resource type | Used for |
|------|----------|
| `mysql-family` | Listing MySQL HeatWave DB systems and their read replicas |
| `postgres-db-systems` | Listing Database with PostgreSQL DB systems and reading their instances and endpoints |
| `redis-family` | Listing OCI Cache clusters |
| `metrics` | Reading the metrics from OCI Monitoring |
| `log-groups` (inspect) | Finding the service logs of the instances |
| `log-content` | Reading the logs from OCI Logging |

## Credentials

On OKE the recommended identity is [Workload Identity](https://docs.oracle.com/en-us/iaas/Content/ContEng/Tasks/contenggrantingworkloadaccesstoresources.htm),
available on enhanced clusters: the policy statements grant the access to the agent's Kubernetes service account
directly, as shown in the [Monitoring MySQL HeatWave and OCI Cache from OKE](/guides/oci-oke-mysql) guide. Outside
OKE the agent uses the instance principal of the VM it runs on, which needs a dynamic group with the same statements.

An API key can be provided instead through `apiKeySecret` in the custom resource, a Secret with the `tenancy_id`,
`user_id`, `fingerprint` and `private_key` keys, for installations outside OCI.

## Compartments and region

`compartmentIds` lists the compartments to discover instances in. With OKE Workload Identity it defaults to the
compartment of the cluster, taken from the identity token; with an API key or the instance principal it must be set.
The compartment APIs don't descend into child compartments, so list every compartment explicitly; a policy granted
on a parent compartment covers its children. The region defaults to the one the cluster runs in, taken from the node
labels, and can be set explicitly with `region`.

## Replicas

MySQL HeatWave read replicas and the standby instances of a Database with PostgreSQL DB system are discovered along
with their primary, whatever their own tags, and grouped with it into one application in Coroot. Declaring the
primary in `databases` is enough: its replicas are monitored with the same credentials, so replication lag and the
role of each instance come from the databases themselves.

## Tag filters

`dbTagFilters` and `cacheTagFilters` restrict discovery to instances whose freeform tags match `tag: value` pairs.
[Glob patterns](https://en.wikipedia.org/wiki/Glob_(programming)) are supported in the value part. An instance is
discovered only if all listed tags match.

## Database credentials

The APIs provide instance status and OS metrics. Query statistics, locks and replication state come from the database
itself, so the cluster-agent connects to each instance directly over the VCN:

1. The DB systems and cache clusters have private endpoints in their subnet; its security list or network security
   group must allow the traffic from the OKE worker subnet on the database ports (3306 for MySQL, 5432 for PostgreSQL,
   6379 for OCI Cache).
2. Create a monitoring user as described in [Postgres](/databases/postgres) or [MySQL](/databases/mysql). On
   Database with PostgreSQL, `CREATE EXTENSION pg_stat_statements` is refused until the extension is allowed in the
   DB system's configuration: create a configuration with `oci.admin_enabled_extensions` set to `pg_stat_statements`
   and apply it to the DB system (this restarts it), then create the extension.
3. Declare the instance in `clusterAgent.databases` of the custom resource with `ocidb: <display name>` and the
   credentials referenced from a Secret. Database with PostgreSQL requires SSL, so set `sslmode: require`.

OCI Cache clusters need no credentials; declare them with `ocicache: <display name>` and `type: redis`. OCI Cache
requires in-transit encryption, and its certificate is issued for the cluster's FQDN while the agent connects to the
IP address, so the agent uses TLS without certificate verification for `ocicache:` targets (the `tls` parameter of
[Redis](/databases/redis) targets, set to `skip-verify`).

## Logs

Database with PostgreSQL and OCI Cache publish their logs to [OCI Logging](https://docs.oracle.com/en-us/iaas/Content/Logging/Concepts/service_logs.htm)
as service logs, which are disabled by default. Enable them in the OCI console (**Logging → Logs → Enable service log**)
or with the CLI, choosing the resource and the log category `postgresql_database_logs` (service `postgresql`) or
`oci-cache-engine-logs` (service `oci-cache`):

```bash
oci logging log create --log-group-id <log group OCID> --log-type SERVICE --display-name my_db_logs \
  --configuration '{"source": {"sourceType": "OCISERVICE", "service": "postgresql", "resource": "<DB system OCID>", "category": "postgresql_database_logs"}}'
```

The cluster-agent finds the enabled service logs of the discovered instances in the log groups of the compartments
(`inspect log-groups`) and reads their new entries every 30 seconds (`read log-content`), extracts the repeated
patterns and forwards the messages to Coroot, so they appear in the **Logs** section of the instance's application.
The service log of a PostgreSQL DB system carries the entries of all its instances; the agent attributes them to the
primary and the standby instances. The log entries are stored in Coroot, not read from OCI Logging on demand. OCI
Logging bills the ingested volume, the first 10 GB per month are free, and the searches are free.

MySQL HeatWave doesn't publish its logs to OCI Logging. The cluster-agent reads the error log of the MySQL DB systems
declared in `databases` (and of their read replicas) from the server itself, through the
`performance_schema.error_log` table (MySQL 8.0.22+; `SELECT` on `performance_schema` is part of the
[monitoring user's grants](/databases/mysql)), and forwards it the same way. When the table is not available, the
agent logs a warning once and stops reading it.

`--collect-oci-logs=false` (`COLLECT_OCI_LOGS`) on the cluster-agent disables the forwarding: the OCI Logging readers
keep counting the messages by pattern, the MySQL error log is not read at all.

## Configuration as code

```yaml
spec:
  clusterAgent:
    oci:
      compartmentIds: [ocid1.compartment.oc1..aaaa]   # optional on OKE: defaults to the cluster's compartment
      dbTagFilters: {team: payments}
    databases:
      - type: mysql
        ocidb: my-db
        credentials:
          usernameSecret: {name: my-db-coroot, key: username}
          passwordSecret: {name: my-db-coroot, key: password}
      - type: redis
        ocicache: my-cache
```

## Costs

The MySQL, PostgreSQL and OCI Cache API reads are free. [OCI Monitoring](https://www.oracle.com/devops/monitoring/pricing/)
bills metric retrieval per datapoint, with the first billion datapoints per month free: the agent retrieves about five
datapoints per metric and instance a minute, roughly 0.5 million per instance per month, so even large compartments
stay within the free allowance.

## Troubleshooting

The cluster-agent logs the compartment, region and credential source when the integration is configured, followed by
a discovery summary:

```
OCI integration: compartments=ocid1.compartment.oc1..aaaa, region=us-ashburn-1, credentials=OKE workload identity
OCI discovery (compartments=ocid1.compartment.oc1..aaaa, region=us-ashburn-1): 2 DB systems, 1 cache clusters
```

Discovery and API errors, including those of the log readers, are exposed as the `oci_discovery_error` metric and shown
on the **Cloud integrations** page.
A `NotAuthorizedOrNotFound` error means the policy statements don't cover the resource type or the compartment. The full
list of collected metrics is in the [cluster-agent metrics reference](/metrics/cluster-agent#oci).
