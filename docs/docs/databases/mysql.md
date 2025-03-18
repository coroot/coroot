---
sidebar_position: 2
---

# MySQL

Coroot leverages eBPF to monitor MySQL queries between applications and databases, requiring no additional integration.
While this approach provides a high-level view of database performance, it lacks the visibility needed to understand why issues occur within the database internals.

To bridge this gap, Coroot also collects statistics from the MySQL Performance Schema, complementing the eBPF-based metrics and traces.

## Prerequisites

This integration requires a database user with the following permissions:

```mysql
CREATE USER 'coroot'@'%' IDENTIFIED BY '<PASSWORD>';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'coroot'@'%';
```

## Kubernetes (pod annotations)

The Kubernetes approach to monitoring databases typically involves running metric exporters as sidecar containers within database instance Pods.
However, this method can be challenging for certain use cases.
Coroot has a dedicated coroot-cluster-agent that can discover and gather metrics from databases without requiring a separate container for each database instance.

Coroot-cluster-agent automatically discovers and collects metrics from pods annotated with `coroot.com/mysql-scrape` annotations.
Coroot can retrieve database credentials from a Secret or be configured with plain-text credentials.

```yaml
coroot.com/mysql-scrape: "true"
coroot.com/mysql-scrape-port: "3306"

# plain-text credentials
coroot.com/mysql-scrape-credentials-username: "coroot"
coroot.com/mysql-scrape-credentials-password: "<PASSWORD>"

# credentials from a secret
coroot.com/mysql-scrape-credentials-secret-name: "mysql-secret"
coroot.com/mysql-scrape-credentials-secret-username-key: "username"
coroot.com/mysql-scrape-credentials-secret-password-key: "password"

# client TLS options: true, false, skip-verify, preferred (default: false)
coroot.com/mysql-scrape-param-tls: "false"
```

Note that Coroot checks only **Pod** annotations, not higher-level Kubernetes objects like Deployments or StatefulSets.

## Non-Kubernetes environments

In non-Kubernetes environments, the MySQL integration can be enabled via the Coroot UI.
In this setup, coroot-cluster-agent retrieves MySQL instance credentials from the Coroot configuration storage.

To configure the integration, go to the `MYSQL` tab and click the `Configure` button.
<img alt="MySQL Configuration" src="/img/docs/databases/mysql/configure.png" class="card w-800"/>

Then, switch to `Manual Configuration`, complete the form, and click `Save`.
<img alt="MySQL Manual Configuration" src="/img/docs/databases/mysql/manual.png" class="card w-600"/>

Coroot-cluster-agent updates its configuration every minute and also takes some time to collect metrics.
Please wait a few minutes for telemetry to appear.

## Troubleshooting

Check the coroot-cluster-agent logs if you encounter any issues.
