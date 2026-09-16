---
sidebar_position: 5.6
---

# GCP

The GCP integration lets Coroot discover Cloud SQL instances (Postgres, MySQL and SQL Server, including read
replicas) and Memorystore instances (Redis, Memcached and Valkey) and collect their telemetry.
Discovered instances appear in the Service Map as separate applications, linked to the services that connect to them
through the eBPF-based metrics collected on the client side.

The integration adds:

* **Instance inventory and status**: engine, version, tier, availability type, region and zone
* **OS-level metrics** from [Cloud Monitoring](https://cloud.google.com/sql/docs/postgres/admin-api/metrics): CPU, memory, disk and network of Cloud SQL instances, CPU, memory and network of Memorystore nodes
* **Database logs** of Cloud SQL Postgres and MySQL instances from [Cloud Logging](https://cloud.google.com/sql/docs/postgres/logging): shipped to Coroot and grouped by message pattern

Database internals (query statistics, locks, replication) are collected separately, see
[Database credentials](#database-credentials).

All GCP API calls are made by the [cluster-agent](/installation/architecture), not by the Coroot server. The agent
polls the Cloud SQL Admin and Memorystore APIs once a minute. Unlike the [AWS integration](/configuration/aws), the GCP
integration is configured only as code: in the Coroot custom resource when Coroot is deployed by the
[Kubernetes Operator](/installation/k8s-operator), or in the cluster-agent's
[configuration file](/configuration/coroot-cluster-agent#configuration-file). The **Cloud integrations** page of the
project settings shows the discovery status and the discovered instances, and warns when the cluster runs on GCP
without the integration configured.

## IAM permissions

Whatever identity the agent uses, it needs these predefined roles on the project:

| Role | Used for |
|------|----------|
| `roles/cloudsql.viewer` | Listing Cloud SQL instances |
| `roles/redis.viewer` | Listing Memorystore for Redis instances |
| `roles/memcache.viewer` | Listing Memorystore for Memcached instances |
| `roles/memorystore.viewer` | Listing Memorystore for Valkey instances |
| `roles/monitoring.viewer` | Reading Cloud SQL and Memorystore metrics from Cloud Monitoring |
| `roles/logging.viewer` | Reading Cloud SQL logs from Cloud Logging |

## Credentials

The agent uses Google's application default credentials. In order, it checks the `GOOGLE_APPLICATION_CREDENTIALS`
environment variable, the metadata server (GKE Workload Identity or the service account of the VM), and the gcloud
application default credentials of the user running it. No key is needed on GKE with Workload Identity: the roles
above are granted directly to the agent's Kubernetes service account, as shown in the
[Monitoring Cloud SQL and Memorystore from GKE](/guides/gcp-gke-cloudsql) guide.

A service account key can be provided instead through `credentialsSecret` in the custom resource, for installations
outside GCP.

## Project and region

The project defaults to the one of the credentials, or of the GKE cluster or VM the agent runs on, and can be set
explicitly with `projectId`. Discovery is limited to the region the cluster runs in, taken from the node labels or the
metadata server, unless `region` names another region or is set to `all` to scan every region of the project.

## Label filters

`cloudsqlLabelFilters` and `memorystoreLabelFilters` restrict discovery to instances whose labels match `label: value`
pairs. [Glob patterns](https://en.wikipedia.org/wiki/Glob_(programming)) are supported in the value part. An instance
is discovered only if all listed labels match. Cloud SQL read replicas don't inherit the labels of their primary, so
a replica is discovered whenever its primary matches the filters.

## Logs

The cluster-agent reads the logs of Cloud SQL Postgres and MySQL instances from Cloud Logging every 30 seconds and
forwards every entry to Coroot as an OpenTelemetry log record, the same way coroot-node-agent forwards container
logs. Entries are fetched by the time Cloud Logging received them, so lines that Cloud SQL ships with a delay are not
missed. Messages are grouped by automatically extracted patterns. The records keep the timestamp and severity of the
Cloud Logging entry, carry the `pattern.hash` attribute, and have `service.name` set to
`/gcp/cloudsql/<project>/<instance>`, which Coroot uses to show them in the **Logs** tab of the Cloud SQL application.
Forwarding can be disabled with the cluster-agent's `--collect-gcp-logs=false` flag, in which case only the
pattern-based `gcp_cloudsql_log_messages_total` metric is collected. Memorystore logs are not collected.

## Database credentials

The APIs provide instance status and OS metrics. Query statistics, locks, and replication state come from the
database itself, so the cluster-agent connects to each instance directly over the VPC:

1. Cloud SQL instances with a private IP are reachable from GKE nodes in the same VPC through private services
   access. Public-IP-only instances need the agent's egress address in the authorized networks.
2. Create a monitoring user as described in [Postgres](/databases/postgres) or [MySQL](/databases/mysql). A user
   created with `gcloud sql users create` on a MySQL instance gets an empty host, so grant its privileges to
   `'coroot'@''` from a database session.
3. Declare the instance in `clusterAgent.databases` of the custom resource with `cloudsql: <instance name>` and the
   credentials referenced from a Secret. Cloud SQL requires SSL by default, so set `sslmode: require` for Postgres.

Memorystore instances need no credentials unless AUTH is enabled; declare them with `memorystore: <instance name>`
and `type: redis` (also for Valkey) or `type: memcached`. Every node of a Memcached instance becomes a target.

Cloud SQL read replicas are discovered as instances of their primary's application, so a primary with two replicas
shows up as one application with three instances, like an RDS or Aurora cluster. A `cloudsql:` entry covers the
instance and its read replicas: the same monitoring user exists on the replicas, so they are monitored with the
primary's credentials without being declared separately.

The `cloudsqladmin` database that Cloud SQL creates on every Postgres instance is used by the service itself, so it is
excluded from monitoring by default: its maintenance connections and queries don't show up among yours, see
[`--exclude-databases`](/configuration/coroot-cluster-agent).

## Configuration as code

```yaml
spec:
  clusterAgent:
    gcp:
      cloudsqlLabelFilters: {team: payments}
    databases:
      - type: postgres
        cloudsql: my-db
        credentials:
          usernameSecret: {name: my-db-coroot, key: username}
          passwordSecret: {name: my-db-coroot, key: password}
        params: {sslmode: require}
      - type: redis
        memorystore: my-cache
```

## Costs

The Cloud SQL Admin, Memorystore and Cloud Logging reads are free. Cloud Monitoring reads are
[billed per time series returned](https://cloud.google.com/products/observability/pricing): the agent fetches every metric
once a minute, about 16 series per Cloud SQL instance and 2 to 4 per Memorystore node, which is about 0.7 million series
per Cloud SQL instance and 0.2 million per Memorystore node per month. The first million per billing account and month
are free, then $0.01 per 1,000, so a project with ten Cloud SQL instances costs roughly $60 a month. Reading Google's own metrics incurs no ingestion charge.

## Troubleshooting

The cluster-agent logs the project, region scope and credential source when the integration is configured, followed
by a discovery summary:

```
GCP integration: project=my-project, region=us-central1, credentials=metadata server (workload identity / service account of the VM)
GCP discovery (project=my-project, region=us-central1): 2 Cloud SQL instances, 1 Memorystore instances
```

Discovery and API errors are exposed as the `gcp_discovery_error` metric. A `403` on the first cycle right after
granting the roles is normal: IAM changes take a minute to propagate, and the agent retries every minute. The full list
of collected metrics is in the [cluster-agent metrics reference](/metrics/cluster-agent#gcp).
