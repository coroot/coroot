---
sidebar_position: 8
hide_table_of_contents: true
---

# Monitoring a database shared by several Kubernetes clusters

A database often outlives the cluster boundaries around it: one MySQL server on a dedicated host, with applications
in several Kubernetes clusters connected to it. Each cluster is a separate project in Coroot, so each team sees
its own services, but the database is just an external endpoint for all of them: you can see that queries are slow,
but not why.

This guide shows how to monitor such a database once and make it a part of every project that depends on it:

- each cluster reports to its own project,
- the database reports to a dedicated project,
- multi-cluster projects join an application project with the database project, so the service map and the MySQL
  metrics cover the whole path from the application to the database.

## Architecture overview

The setup used in this guide:

- **Cluster 1** runs Coroot and the `app1` application (`storefront` → `orders`).
- **Cluster 2** runs the `app2` application (`backoffice` → `billing`).
- **Database host** runs MySQL with two databases, `app1` and `app2`, used by `orders` and `billing`.

<img alt="A database shared by two Kubernetes clusters" src="/img/docs/guides/shared-database/architecture.svg" class="card w-1200"/>

The database is monitored by **coroot-node-agent** on the database host. It collects host metrics, logs, and the TCP
connections of the MySQL process, and, with the database credentials in its
[configuration file](/configuration/coroot-node-agent#configuration-file), connects to MySQL and collects the database
metrics as well: queries, locks, replication, table sizes, and so on. Nothing has to reach the database from the outside.

It reports to the `shared-db` project. Cluster 1 and cluster 2 keep reporting to `app1` and `app2`.
Then two [multi-cluster projects](/configuration/multi-cluster), `app1-with-db` and `app2-with-db`, combine each
application project with `shared-db`:

<img alt="Projects" src="/img/docs/guides/shared-database/projects.svg" class="card w-1200"/>

A multi-cluster project stores nothing: it's a view over its member projects. When Coroot builds it, the external
endpoint that `orders` connects to is replaced with the actual MySQL instance from `shared-db`, because that instance
listens on the same address.

## Prerequisites

- Two Kubernetes clusters with `kubectl` contexts (the examples below use `cluster-1` and `cluster-2`) and Helm 3.
- Coroot 1.29.0 or later, Coroot Operator 1.10.6 or later, coroot-node-agent 1.39.0 or later.
- Network access from cluster 2 and from the database host to Coroot in cluster 1 (the examples use the node port
  `30001` on `10.10.0.2`).

## Step 1: Install Coroot in cluster 1

Install the Coroot Operator:

```bash
kubectl config use-context cluster-1

helm repo add coroot https://coroot.github.io/helm-charts
helm repo update coroot
helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator
```

Create the Coroot custom resource with three projects, one per telemetry source:

```yaml title="coroot.yaml"
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec:
  service:
    type: NodePort
    nodePort: 30001 # cluster 2 and the database host reach Coroot through this port
  apiKeySecret: # the agents of this cluster report to the app1 project
    name: coroot-api-keys
    key: app1
  projects:
    - name: app1
      apiKeys: # if the secret doesn't exist, the operator creates it and generates the keys
        - description: agents in cluster 1
          keySecret:
            name: coroot-api-keys
            key: app1
    - name: app2
      apiKeys:
        - description: agents in cluster 2
          keySecret:
            name: coroot-api-keys
            key: app2
    - name: shared-db
      apiKeys:
        - description: agents monitoring the shared database
          keySecret:
            name: coroot-api-keys
            key: shared-db
```

```bash
kubectl apply -f coroot.yaml
```

The operator installs Coroot, its storage, and the agents for cluster 1, and generates an API key for each project
in the `coroot-api-keys` secret:

```bash
kubectl get pods -n coroot
```
```bash
NAME                                    READY   STATUS    RESTARTS   AGE
coroot-clickhouse-keeper-0              1/1     Running   0          2m
coroot-clickhouse-keeper-1              1/1     Running   0          2m
coroot-clickhouse-keeper-2              1/1     Running   0          2m
coroot-clickhouse-shard-0-0             1/1     Running   0          2m
coroot-cluster-agent-5cd7bfdff8-zpng9   1/1     Running   0          2m
coroot-coroot-0                         1/1     Running   0          2m
coroot-node-agent-gl2vv                 1/1     Running   0          2m
coroot-operator-5bf4b74445-ljpkw        1/1     Running   0          3m
coroot-prometheus-57c7c4d449-dss7c      1/1     Running   0          2m
```

## Step 2: Install the agents in cluster 2

Cluster 2 only needs the agents. Copy the API key of the `app2` project there:

```bash
kubectl config use-context cluster-1
APP2_KEY=$(kubectl get secret -n coroot coroot-api-keys -o jsonpath='{.data.app2}' | base64 -d)

kubectl config use-context cluster-2
helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator
kubectl create secret generic -n coroot coroot-api-key --from-literal=app2="$APP2_KEY"
```

Create the custom resource in the `agentsOnly` mode:

```yaml title="agents.yaml"
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec:
  agentsOnly:
    corootURL: http://10.10.0.2:30001 # Coroot in cluster 1
  apiKeySecret: # report to the app2 project
    name: coroot-api-key
    key: app2
```

```bash
kubectl apply -f agents.yaml
```

At this point each cluster has its own project. In both of them the database is an external service: Coroot knows
that `orders` talks to MySQL at `10.10.0.4:3306` and measures these requests on the client side, but has no data from
the database itself.

<img alt="The app1 project: the database is an external service" src="/img/docs/guides/shared-database/app1-map.png" class="card w-1200"/>

## Step 3: Install the node-agent on the database host

Database metrics are collected over a regular MySQL connection, from the host itself. Create a user for the agent,
limited to local connections (see [MySQL](/databases/mysql) for what each privilege is used for):

```sql
CREATE USER 'coroot'@'localhost' IDENTIFIED BY '<PASSWORD>';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'coroot'@'localhost';
```

Put the database into the node-agent configuration file. The agent reads it at startup, so create it before installing
the agent:

```bash
mkdir -p /etc/coroot-node-agent
cat > /etc/coroot-node-agent/config.yaml <<EOF
databases:
  - type: mysql
    host: 127.0.0.1
    port: "3306"
    credentials:
      username: coroot
      password: <PASSWORD>
EOF
chmod 600 /etc/coroot-node-agent/config.yaml
```

Get the API key of the `shared-db` project:

```bash
kubectl config use-context cluster-1
kubectl get secret -n coroot coroot-api-keys -o jsonpath='{.data.shared-db}' | base64 -d
```

Install coroot-node-agent on the database host and point it to Coroot using this key:

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot-node-agent/main/install.sh | \
  COLLECTOR_ENDPOINT=http://10.10.0.2:30001 \
  API_KEY=<the API key of the shared-db project> \
  SCRAPE_INTERVAL=15s \
  sh -
```

The agent discovers the MySQL process and the addresses it listens on. This is what lets Coroot recognize later that
the external endpoint `10.10.0.4:3306` seen from the clusters and this MySQL instance are the same thing. It also finds
that the process listens on the configured `127.0.0.1:3306`, connects to it, and attributes the collected MySQL metrics
to the `mysqld` service. The log confirms it:

```bash
journalctl -u coroot-node-agent | grep 'database target'
```
```bash
databases.go:126] loaded 1 database targets from /etc/coroot-node-agent/config.yaml
databases.go:162] database target mysql://127.0.0.1:3306: started exporter for /system.slice/mysqld.service
```

In a few minutes the `shared-db` project contains the database host and the MySQL instance with its metrics.
On its own this project is not very informative: none of the database clients report to it, so its Service Map
doesn't show who uses the database. That's what the next step is for.

## Step 4: Create the multi-cluster projects

Add two more projects to `coroot.yaml`. They have no API keys, only the list of projects they combine:

```yaml title="coroot.yaml"
spec:
  projects:
    # app1, app2, and shared-db stay as they are
    - name: app1-with-db
      memberProjects:
        - app1
        - shared-db
    - name: app2-with-db
      memberProjects:
        - app2
        - shared-db
```

```bash
kubectl apply -f coroot.yaml
```

## The result

Open the `app1-with-db` project. On the Service Map the external endpoint is gone: `orders` now talks to the MySQL
instance from the `shared-db` project.

<img alt="The app1-with-db project: the database is a part of the service map" src="/img/docs/guides/shared-database/app1-with-db-map.png" class="card w-1200"/>

The database is a regular application here, with its clients, instances, host metrics, logs, and the MySQL
dashboard built from the metrics collected by the node-agent on the host. The agent is listed among the clients as well,
since it connects to the database to collect them:

<img alt="MySQL metrics in the app1-with-db project" src="/img/docs/guides/shared-database/app1-with-db-mysql.png" class="card w-1200"/>

The `app2-with-db` project shows the same database from the other side: the services of cluster 2 and their
queries. Neither project contains anything from the other cluster.

<img alt="The app2-with-db project" src="/img/docs/guides/shared-database/app2-with-db-map.png" class="card w-1200"/>

## Notes

- The database is monitored once. Adding another cluster that uses it takes a project for the cluster and
  a multi-cluster project with `shared-db` as a member, with no changes on the database side.
- To monitor several shared databases, install the node-agent on each database host with its own configuration file.
