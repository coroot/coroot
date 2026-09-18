---
sidebar_position: 7
---

# Monitoring MySQL HeatWave and OCI Cache from OKE

This guide connects Coroot running on Oracle Kubernetes Engine to your MySQL HeatWave, PostgreSQL and OCI Cache
instances. The cluster-agent authenticates to OCI through OKE Workload Identity, and database credentials come from a
Kubernetes Secret referenced in the Coroot custom resource. No API key and no settings in the Coroot UI are needed.

What you get: the DB systems and cache clusters in the Service Map, linked to the services that connect to them, with
instance status, OS metrics from OCI Monitoring, logs, and database internals such as query statistics, locks and
replication. Read replicas and standby instances are discovered along with their primary. The integration is
described in detail on the [OCI](/configuration/oci) configuration page.

## Prerequisites

- Coroot deployed on an OKE **enhanced** cluster via the [Kubernetes Operator](/installation/k8s-operator). Workload
  Identity is not available on basic clusters.
- `oci` and `kubectl` configured, with permissions to manage policies in the compartment of the databases

Set these variables once in your shell; every command below uses them:

```bash
export COMPARTMENT_NAME=my-compartment      # the compartment of the databases
export COMPARTMENT_ID=$(oci iam compartment list --all --query "data[?name=='${COMPARTMENT_NAME}'].id | [0]" --raw-output)
export CLUSTER_ID=ocid1.cluster.oc1...        # the OCID of the OKE cluster
export NAMESPACE=coroot                       # the namespace of the Coroot custom resource
```

## Step 1: Grant the access to the cluster-agent

The operator creates the cluster-agent's service account as `<Coroot CR name>-cluster-agent`, so for a CR named
`coroot` it is `coroot-cluster-agent`. With Workload Identity the policy statements name that service account
directly, no dynamic group is needed:

```bash
WORKLOAD="request.principal.type='workload', request.principal.namespace='${NAMESPACE}', request.principal.service_account='coroot-cluster-agent', request.principal.cluster_id='${CLUSTER_ID}'"

oci iam policy create --compartment-id ${COMPARTMENT_ID} --name coroot-cluster-agent \
  --description "Coroot cluster-agent: discovery of managed databases and their metrics" \
  --statements "[
    \"Allow any-user to read mysql-family in compartment ${COMPARTMENT_NAME} where all {${WORKLOAD}}\",
    \"Allow any-user to read postgres-db-systems in compartment ${COMPARTMENT_NAME} where all {${WORKLOAD}}\",
    \"Allow any-user to read redis-family in compartment ${COMPARTMENT_NAME} where all {${WORKLOAD}}\",
    \"Allow any-user to read metrics in compartment ${COMPARTMENT_NAME} where all {${WORKLOAD}}\",
    \"Allow any-user to inspect log-groups in compartment ${COMPARTMENT_NAME} where all {${WORKLOAD}}\",
    \"Allow any-user to read log-content in compartment ${COMPARTMENT_NAME} where all {${WORKLOAD}}\"
  ]"
```

The statements are read-only: listing the DB systems and cache clusters, reading their metrics from OCI Monitoring
and their logs from OCI Logging (see [Logs](/configuration/oci#logs) for enabling the service logs).

## Step 2: Configure database credentials

For query statistics, locks and replication state, the cluster-agent connects to each database directly. The DB
systems must be reachable from the worker nodes: allow the worker subnet on port 3306 (MySQL) or 5432 (PostgreSQL) in
the security list of the database subnet. Create a monitoring user as described in [MySQL](/databases/mysql) or
[Postgres](/databases/postgres), for example with the MySQL admin account:

```sql
CREATE USER 'coroot'@'%' IDENTIFIED BY '<PASSWORD>';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'coroot'@'%';
```

For a PostgreSQL DB system, allow the `pg_stat_statements` extension first: it is refused until it is listed in the
DB system's configuration. Create a flexible configuration (valid for any OCPU and memory size of the shape) with the
extension allowed, apply it (the DB system restarts), then create the extension:

```bash
export PG_DB_SYSTEM_ID=ocid1.postgresqldbsystem.oc1...
CONFIG_ID=$(oci psql configuration create --compartment-id ${COMPARTMENT_ID} --display-name coroot-pg-stat-statements \
  --db-version 16 --shape VM.Standard.E5.Flex --is-flexible true \
  --db-configuration-overrides '{"items":[{"configKey":"oci.admin_enabled_extensions","overridenConfigValue":"pg_stat_statements"}]}' \
  --query 'data.id' --raw-output)
oci psql db-system update --db-system-id ${PG_DB_SYSTEM_ID} --force \
  --db-configuration-params "{\"configId\": \"${CONFIG_ID}\", \"applyConfig\": \"RESTART\"}" --wait-for-state SUCCEEDED
```

```sql
CREATE EXTENSION pg_stat_statements;
```

The `--db-version` and `--shape` values must match the DB system's.

Put the credentials in a Secret and reference it from the Coroot custom resource. DB systems and cache clusters are
referenced by their display names: the agent takes their addresses from discovery, and monitors the read replicas and
standby instances of a DB system with the same credentials.

```yaml title="ocidb-coroot-secret.yaml"
apiVersion: v1
kind: Secret
metadata:
  name: ocidb-coroot
  namespace: coroot
stringData:
  username: coroot
  password: <PASSWORD>
```

```yaml title="coroot.yaml"
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec:
  communityEdition:
  nodeAgent:
  clusterAgent:
    oci:
      compartmentIds: [ocid1.compartment.oc1...]   # the compartments of the databases (default: the cluster's compartment)
      dbTagFilters:                            # optional: discover only instances with matching freeform tags
        team: payments
    databases:
      - type: mysql
        ocidb: my-db                           # display name of the MySQL HeatWave DB system
        credentials:
          usernameSecret:
            name: ocidb-coroot
            key: username
          passwordSecret:
            name: ocidb-coroot
            key: password
      - type: redis
        ocicache: my-cache                     # display name of the OCI Cache cluster; no credentials
  clickhouse:
    storage:
      size: 100Gi
  prometheus:
    storage:
      size: 50Gi
```

```bash
kubectl apply -f ocidb-coroot-secret.yaml
kubectl apply -f coroot.yaml
```

The operator restarts the cluster-agent with the new configuration. Within a couple of minutes the agent logs the
targets, and the MySQL and Redis tabs of the discovered applications show data:

```bash
kubectl logs deploy/coroot-cluster-agent -n ${NAMESPACE} | grep "OCI \|new target"
```

```
OCI integration: compartments=ocid1.compartment.oc1..., region=us-ashburn-1, credentials=OKE workload identity
OCI discovery (compartments=ocid1.compartment.oc1..., region=us-ashburn-1): 1 DB systems, 1 cache clusters
new target: mysql://10.0.9.48:3306 (ocidb:my-db)
new target: redis://10.0.9.20:6379 (ocicache:my-cache)
```

## Step 3 (optional): Enable the logs

MySQL HeatWave error logs are read from the servers with the credentials above, nothing to enable. Database with
PostgreSQL and OCI Cache publish their logs to OCI Logging only when a service log is enabled for the resource, see
[Logs](/configuration/oci#logs); the policy of step 1 already lets the agent find and read them.

## Troubleshooting

- **`NotAuthorizedOrNotFound` errors in the discovery line**: the policy doesn't match. Check the cluster OCID, the
  namespace and the service account name in the statements; a policy takes up to a minute to propagate, and the agent
  retries every minute.
- **`failed to obtain credentials`**: the cluster is a basic OKE cluster, which has no Workload Identity. Upgrade it to
  an enhanced cluster or use an API key through `apiKeySecret`.
- **Instances discovered but no database metrics**: check that the security list of the database subnet allows the
  worker subnet on the database port, that the Secret exists in the CR's namespace, and that the user can log in. The
  agent logs a warning for every database it cannot reach.
