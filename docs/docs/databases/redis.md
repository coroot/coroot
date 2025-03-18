---
sidebar_position: 3
---

# Redis

Coroot leverages eBPF to monitor Redis queries between applications and databases, requiring no additional integration.
While this approach provides a high-level view of database performance, it lacks the visibility needed to understand why issues occur within the database internals.

To bridge this gap, Coroot also collects telemetry using the Redis INFO command, complementing the eBPF-based metrics and traces.

## Kubernetes (pod annotations)

The Kubernetes approach to monitoring databases typically involves running metric exporters as sidecar containers within database instance Pods.
However, this method can be challenging for certain use cases.
Coroot has a dedicated coroot-cluster-agent that can discover and gather metrics from databases without requiring a separate container for each database instance.

Coroot-cluster-agent automatically discovers and collects metrics from pods annotated with `coroot.com/redis-scrape` annotations.
Coroot can retrieve database credentials from a Secret or be configured with plain-text credentials.

```yaml
coroot.com/redis-scrape: "true"
coroot.com/redis-scrape-port: "6379"

# plain-text credentials
coroot.com/redis-scrape-credentials-password: "<PASSWORD>"

# credentials from a secret
coroot.com/redis-scrape-credentials-secret-name: "redis-secret"
coroot.com/redis-scrape-credentials-secret-password-key: "password"
```

Note that Coroot checks only **Pod** annotations, not higher-level Kubernetes objects like Deployments or StatefulSets.

## Non-Kubernetes environments

In non-Kubernetes environments, the Redis integration can be enabled via the Coroot UI.
In this setup, coroot-cluster-agent retrieves Redis instance credentials from the Coroot configuration storage.

To configure the integration, go to the `REDIS` tab and click the `Configure` button.
<img alt="Redis Configuration" src="/img/docs/databases/redis/configure.png" class="card w-800"/>

Then, switch to `Manual Configuration`, complete the form, and click `Save`.
<img alt="Redis Manual Configuration" src="/img/docs/databases/redis/manual.png" class="card w-600"/>

Coroot-cluster-agent updates its configuration every minute and also takes some time to collect metrics.
Please wait a few minutes for telemetry to appear.

## Troubleshooting

Check the coroot-cluster-agent logs if you encounter any issues.
