---
sidebar_position: 4
---

# Installing Coroot with ArgoCD

This guide walks through installing Coroot (Community or Enterprise edition) in a Kubernetes cluster using
[ArgoCD](https://argo-cd.readthedocs.io/), following a GitOps workflow.

The setup has two parts:

1. **The Coroot Operator** is installed from its Helm chart via an ArgoCD `Application`.
2. **The Coroot custom resource (CR)** that describes your installation is stored in Git and deployed by a second
   ArgoCD `Application`.

Both Applications, along with the CR itself, live in Git.

## How these Applications get created

ArgoCD has no auto-discovery: an `Application` only exists once something creates it in the cluster. How you create
Coroot's Applications depends on whether you already run a cluster-wide bootstrap:

- **You already have a global app-of-apps or ApplicationSet.** Then you don't bootstrap anything new. Just commit
  Coroot's `Application` manifests into the directory that parent already watches, and it creates them on its next sync.
  Adjust the `repoURL`/`targetRevision`/`path` in those manifests to match your repo, and you can skip Step 5.
- **You don't have one yet (greenfield).** Then you apply a small parent Application once, by hand, to break the
  chicken-and-egg (ArgoCD can't sync an Application it doesn't know about). That parent points at your `apps/` directory
  and creates Coroot's Applications from there; everything afterwards is driven by `git commit`. This is what Step 5
  sets up.

Either way, the manifests are identical, the only difference is who applies the first one. The steps below build the
greenfield layout; if you already have a parent, reuse its directory instead of `bootstrap/`.

:::tip
This guide is about *installing* Coroot through ArgoCD. If you instead want Coroot to *monitor* the applications that
ArgoCD (or FluxCD) delivers in your cluster, see [GitOps monitoring](/gitops/overview), which works out of the box with no
configuration.
:::

## Prerequisites

- A Kubernetes cluster with [ArgoCD](https://argo-cd.readthedocs.io/en/stable/getting_started/) installed.
- A Git repository that ArgoCD can read, where you'll keep the manifests below.
- For a highly available (HA) deployment: an external PostgreSQL database reachable from the cluster.

## Step 1: Prepare the Git repository

Everything ArgoCD manages lives in your Git repository, in two directories:

```
gitops-repo/
├── bootstrap/
│   └── coroot-root.yaml      # parent Application: committed, but applied by hand once (Step 5)
├── apps/                     # one ArgoCD Application per file
│   ├── coroot-operator.yaml  #   installs the operator (Helm)
│   └── coroot.yaml           #   deploys the Coroot CR (points at ../coroot)
└── coroot/                   # the Coroot CR
    └── coroot.yaml
```

This is the standard [app-of-apps](https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/)
pattern. Each child app in `apps/` is its own Argo CD `Application` manifest (one file per app), and the parent
Application in `bootstrap/` uses a directory source pointing at `apps/`. Argo CD renders that directory, finds the two
`Application` manifests, and creates them as the `coroot-operator` and `coroot` Applications, which then deploy their
respective sources. Adding another app later is just another file in `apps/` plus a commit.

## Step 2: Define the operator Application

The operator is installed straight from the Coroot Helm repository:

```yaml title="apps/coroot-operator.yaml"
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: coroot-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://coroot.github.io/helm-charts
    chart: coroot-operator
    targetRevision: "*"  # or pin a specific chart version, e.g. 1.2.3
  destination:
    server: https://kubernetes.default.svc
    namespace: coroot
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

## Step 3: Define the Coroot Application

This Application deploys the Coroot CR from the `coroot/` directory of your repository. We'll create that CR in the next
step.

```yaml title="apps/coroot.yaml"
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: coroot
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/your-gitops-repo
    targetRevision: main
    path: coroot
  destination:
    server: https://kubernetes.default.svc
    namespace: coroot
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## Step 4: Define the Coroot CR

The CR lives in the `coroot/` directory as a plain manifest. The Coroot Application from Step 3 points at this directory,
and ArgoCD applies every manifest it finds there as-is, so no Kustomize or templating is involved.

Pick **one** of the two options below for `coroot/coroot.yaml`.

:::note Enterprise Edition
The examples below default to the Community Edition. To run Coroot Enterprise, create a secret with your license key and
reference it from the CR (shown commented out in both options). Keep the license out of Git, just like the Postgres
password:

```bash
kubectl create secret generic coroot-license -n coroot \
  --from-literal=license-key='<your-license-key>'
```
:::

:::note Project and API key
Both options below create a `default` project and point the bundled agents (node-agent and cluster-agent) at it for
telemetry ingestion. They share a single API key stored in the `coroot-api-key` secret: the operator generates the key
into that secret if it doesn't exist yet, and the same secret is referenced by the agents, so no key is committed to
Git.
:::

### Option A: Single instance with SQLite

This is the simplest setup: a single Coroot pod that stores its configuration in SQLite on a persistent volume.
It's a great fit for small to medium installations.

```yaml title="coroot/coroot.yaml"
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec:
  # replicas defaults to 1, a single instance backed by SQLite.
  storage:
    size: 10Gi
    # className: ""  # uses the default storage class if not set
  clickhouse:
    shards: 1          # single ClickHouse shard
    replicas: 1        # single replica (no redundancy)
    storage:
      size: 100Gi      # volume size for the ClickHouse instance
  apiKeySecret:        # agents authenticate with this key (generated into the secret if missing)
    name: coroot-api-key
    key: api-key
  projects:
    - name: default    # created automatically with the API key below
      apiKeys:
        - description: default ingestion key
          keySecret:
            name: coroot-api-key
            key: api-key

  # For Enterprise Edition, reference your license key from a secret:
  # enterpriseEdition:
  #   licenseKeySecret:
  #     name: coroot-license
  #     key: license-key

  # Expose Coroot (optional):
  # ingress:
  #   host: coroot.company.com
  #   className: traefik
```

Because SQLite cannot be shared across pods, this option is limited to a single replica.

### Option B: Highly available with external PostgreSQL

For HA, Coroot must store its configuration in PostgreSQL instead of SQLite. This is **required** whenever `replicas`
is greater than 1, since multiple Coroot pods need a shared, concurrent configuration store.

First, create a secret with the Postgres password. Keep it out of Git by creating it directly, or manage it with a tool
like [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) or
[External Secrets](https://external-secrets.io/):

```bash
kubectl create secret generic coroot-postgres -n coroot \
  --from-literal=password='<your-postgres-password>'
```

Then reference it from the CR:

```yaml title="coroot/coroot.yaml"
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec:
  replicas: 2  # run multiple Coroot instances for high availability
  postgres:
    host: postgres.example.com
    port: 5432
    database: coroot
    user: coroot
    passwordSecret:
      name: coroot-postgres
      key: password
    params:
      sslmode: require
  storage:
    size: 50Gi
  storeMetricsInClickhouse: true  # store metrics in the HA ClickHouse cluster instead of Prometheus
  clickhouse:
    shards: 2          # spread telemetry across 2 shards
    replicas: 2        # 2 replicas per shard for redundancy
    storage:
      size: 100Gi      # volume size for EACH ClickHouse instance
  apiKeySecret:        # agents authenticate with this key (generated into the secret if missing)
    name: coroot-api-key
    key: api-key
  projects:
    - name: default    # created automatically with the API key below
      apiKeys:
        - description: default ingestion key
          keySecret:
            name: coroot-api-key
            key: api-key

  # For Enterprise Edition, reference your license key from a secret:
  # enterpriseEdition:
  #   licenseKeySecret:
  #     name: coroot-license
  #     key: license-key

  # Expose Coroot (optional):
  # ingress:
  #   host: coroot.company.com
  #   className: traefik
```

:::note
Coroot stores traces, logs, and profiles in ClickHouse. The example also sets `storeMetricsInClickhouse: true`, so
metrics go to the same ClickHouse cluster and Prometheus is not installed, keeping the whole telemetry backend on one
HA store. The operator deploys ClickHouse for you; the example above runs it as a HA cluster of 2 shards with 2 replicas
each. Note that `clickhouse.storage.size` applies to **each** instance, so the layout above provisions 4 × 100Gi volumes.
Alternatively, you can point Coroot at an `externalClickhouse`. See the
[Kubernetes Operator](/installation/k8s-operator) reference for all available fields.
:::

Commit and push the repository.

## Step 5: Bootstrap with the parent Application

:::info This step is optional
Only needed for a greenfield setup. **If you already run a global app-of-apps or ApplicationSet, do nothing here**, it
already watches `apps/` (or wherever you committed Coroot's Applications) and will create them on its next sync. The
parent below is just for clusters that don't yet have a root app to hang things off.
:::

Add the parent Application under `bootstrap/`. It points at the `apps/` directory and is the seed that lets ArgoCD
discover the operator and Coroot Applications from Git. Commit it like everything else, but it's the one manifest you
apply by hand to get things rolling.

```yaml title="bootstrap/coroot-root.yaml"
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: coroot-root
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/your-gitops-repo
    targetRevision: main
    path: apps
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

Apply it once:

```bash
kubectl apply -f bootstrap/coroot-root.yaml
```

ArgoCD now creates the operator and Coroot Applications from Git, the operator installs into the `coroot` namespace and
registers the `Coroot` CRD, and the CR is applied so the operator can deploy all components.

From here on you never touch the cluster directly: switching from SQLite to Postgres, scaling replicas, enabling ingress,
or adding a license key is just a commit to your repository.

## A note on ordering

The operator's Helm chart registers the `Coroot` CRD, and the Coroot CR can't be applied until that CRD exists. ArgoCD
automatically retries failed syncs, so the Coroot Application simply fails until the operator has registered the CRD,
then succeeds on the next attempt. No manual intervention is needed.

## Step 6: Verify the installation

```bash
# The operator and Coroot components should be running:
kubectl get pods -n coroot

# The Coroot CR should be present:
kubectl get coroot -n coroot
```

All three Applications (`coroot-root`, `coroot-operator`, and `coroot`) should report `Synced` and `Healthy` in the
ArgoCD UI.
