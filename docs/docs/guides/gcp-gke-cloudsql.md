---
sidebar_position: 6
---

# Monitoring Cloud SQL and Memorystore from GKE

This guide connects Coroot running on Google Kubernetes Engine to your Cloud SQL and Memorystore instances. The
cluster-agent authenticates to GCP through Workload Identity, and database credentials come from a Kubernetes Secret
referenced in the Coroot custom resource. No service account keys and no settings in the Coroot UI are needed.

What you get: Cloud SQL and Memorystore instances in the Service Map, linked to the services that connect to them,
with instance status, OS metrics from Cloud Monitoring, database logs in the Logs tab, and database internals such
as query statistics and locks. The integration is described in detail on the [GCP](/configuration/gcp) configuration
page.

## Prerequisites

- Coroot deployed on a GKE Standard cluster via the [Kubernetes Operator](/installation/k8s-operator). Autopilot
  clusters don't allow the privileged node-agent.
- [Workload Identity Federation for GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
  enabled on the cluster (`--workload-pool=<project>.svc.id.goog`)
- `gcloud` and `kubectl` configured, with permissions to manage IAM on the project

Set these variables once in your shell; every command below uses them:

```bash
export PROJECT_ID=my-project            # the GCP project of the cluster and the databases
export NAMESPACE=coroot                 # the namespace of the Coroot custom resource
export PROJECT_NUMBER=$(gcloud projects describe ${PROJECT_ID} --format='value(projectNumber)')
```

## Step 1: Grant the roles to the cluster-agent

The operator creates the cluster-agent's service account as `<Coroot CR name>-cluster-agent`, so for a CR named
`coroot` it is `coroot-cluster-agent`. With Workload Identity Federation the IAM roles are granted directly to that
Kubernetes service account, no Google service account and no annotation are needed:

```bash
MEMBER="principal://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${PROJECT_ID}.svc.id.goog/subject/ns/${NAMESPACE}/sa/coroot-cluster-agent"

for role in roles/cloudsql.viewer roles/redis.viewer roles/memcache.viewer roles/memorystore.viewer roles/monitoring.viewer roles/logging.viewer; do
  gcloud projects add-iam-policy-binding ${PROJECT_ID} --member="${MEMBER}" --role=${role} --condition=None
done
```

The roles are read-only: listing Cloud SQL instances and Memorystore instances of all three products, and reading
Cloud Monitoring metrics and Cloud Logging entries.

## Step 2: Configure database credentials

For query statistics, locks and replication state, the cluster-agent connects to each database directly. Cloud SQL
instances with a private IP are reachable from the cluster's VPC; create a monitoring user as described in
[Postgres](/databases/postgres) or [MySQL](/databases/mysql):

```bash
gcloud sql users create coroot --instance=my-db --password='<PASSWORD>' --project ${PROJECT_ID}
```

Then grant the monitoring privileges from a database session (for Postgres, `GRANT pg_monitor TO coroot` and
`CREATE EXTENSION pg_stat_statements`).

Put the credentials in a Secret and reference it from the Coroot custom resource. Cloud SQL instances are referenced
by name and Memorystore instances by name: the agent takes their addresses from discovery.

```yaml title="cloudsql-coroot-secret.yaml"
apiVersion: v1
kind: Secret
metadata:
  name: cloudsql-coroot
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
    gcp:
      cloudsqlLabelFilters:              # optional: discover only instances with matching labels
        team: payments
    databases:
      - type: postgres
        cloudsql: my-db                  # Cloud SQL instance name
        credentials:
          usernameSecret:
            name: cloudsql-coroot
            key: username
          passwordSecret:
            name: cloudsql-coroot
            key: password
        params:
          sslmode: require               # Cloud SQL requires SSL by default
      - type: redis
        memorystore: my-cache            # Memorystore instance name; no credentials unless AUTH is enabled
  clickhouse:
    storage:
      size: 100Gi
  prometheus:
    storage:
      size: 50Gi
```

```bash
kubectl apply -f cloudsql-coroot-secret.yaml
kubectl apply -f coroot.yaml
```

The operator restarts the cluster-agent with the new configuration. Within a couple of minutes the agent logs the
targets, and the Postgres and Redis tabs of the Cloud SQL and Memorystore applications show data:

```bash
kubectl logs deploy/coroot-cluster-agent -n ${NAMESPACE} | grep "GCP \|new target"
```

```
GCP integration: project=my-project, region=us-central1, credentials=metadata server (workload identity / service account of the VM)
GCP discovery (project=my-project, region=us-central1): 1 Cloud SQL instances, 1 Memorystore instances
new target: postgres://10.10.0.3:5432 (cloudsql:my-db)
new target: redis://10.20.0.4:6379 (memorystore:my-cache)
```

## Troubleshooting

- **`403` errors in the discovery line right after Step 1**: IAM changes take up to a minute to propagate. The agent
  retries every minute and recovers on its own.
- **`credentials=application default credentials` with a `could not find default credentials` error**: the cluster
  has no Workload Identity pool, or the pod runs on a node pool created without the GKE metadata server. Enable
  Workload Identity Federation on the node pool.
- **Instances discovered but no database metrics**: check the Cloud SQL instance has a private IP in the cluster's
  VPC, that the Secret exists in the CR's namespace, and that the user can log in. The agent logs a warning for every
  database it cannot reach.
