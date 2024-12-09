---
sidebar_position: 2
slug: /quick-start/enterprise
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Enterprise Edition

This guide provides a quick overview of launching Coroot Enterprise Edition with default options. For more details and customization options check out the Installation section.

:::info
Coroot Enterprise Edition is a paid subscription (from $1 per CPU core/month) that offers extra features and priority support. 
To install the Enterprise Edition, you'll need a license. [Start](https://coroot.com/account) your free trial today.
:::


<Tabs queryString="env">
  <TabItem value="kubernetes" label="Kubernetes" default>

Add the Coroot helm chart repo:

```bash
helm repo add coroot https://coroot.github.io/helm-charts
helm repo update coroot
```

Next, install the Coroot Operator:

```bash
helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator
```

Install the Coroot Enterprise Edition. This chart creates a minimal [Coroot Custom Resource](/installation/k8s-operator):

```bash
helm install -n coroot coroot coroot/coroot-ee \
  --set "licenseKey=COROOT-LICENSE-KEY-HERE,clickhouse.shards=2,clickhouse.replicas=2"
```

Forward the Coroot port to your machine:

```bash
kubectl port-forward -n coroot service/coroot-coroot 8080:8080
```

Then, you can access Coroot at http://localhost:8080
  </TabItem>

  <TabItem value="docker" label="Docker">
To deploy Coroot using Docker Compose, run the following command. 
Before applying it, you can review the configuration file in Coroot's GitHub repository: docker-compose.yaml

```bash
curl -fsS https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/docker-compose.yaml | \
  LICENSE_KEY="COROOT-LICENSE-KEY-HERE" docker compose -f - up -d
```

If you installed Coroot on your desktop machine, you can access it at http://localhost:8080/. 
If Coroot is deployed on a remote node, replace `NODE_IP_ADDRESS` with the IP address of the node in the following URL: http://NODE_IP_ADDRESS:8080/.

  </TabItem>

  <TabItem value="docker-swarm" label="Docker Swarm">

Deploy the Coroot stack to your cluster by running the following command on the manager node. 
Before applying, you can review the configuration file in Coroot's GitHub repository: docker-swarm-stack.yaml

```bash
curl -fsS https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/docker-swarm-stack.yaml | \
  LICENSE_KEY="COROOT-LICENSE-KEY-HERE" docker stack deploy -c - coroot-ee
```

Since Docker Swarm doesn't support privileged containers, you'll have to manually deploy coroot-node-agent on each cluster node. 
Just replace `NODE_IP` with any node's IP address in the Docker Swarm cluster.

```bash
docker run --detach --name coroot-node-agent \
  --pull=always \
  --privileged --pid host \
  -v /sys/kernel/tracing:/sys/kernel/tracing:rw \
  -v /sys/kernel/debug:/sys/kernel/debug:rw \
  -v /sys/fs/cgroup:/host/sys/fs/cgroup:ro \
  ghcr.io/coroot/coroot-node-agent \
  --cgroupfs-root=/host/sys/fs/cgroup \
  --collector-endpoint=http://NODE_IP:8080
```
Access Coroot through any node in your Docker Swarm cluster using its published port: http://NODE_IP:8080.
  </TabItem>

  <TabItem value="ubuntu" label="Ubuntu & Debian">

Coroot requires a Prometheus server with the Remote Write Receiver enabled, along with a ClickHouse server. 
For detailed steps on installing all the necessary components on an Ubuntu/Debian node, refer to the [full instructions](/installation/ubuntu?edition=ee).

To install Coroot, run the following command:

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/install.sh | \
  LICENSE_KEY="COROOT-LICENSE-KEY-HERE" \
  BOOTSTRAP_PROMETHEUS_URL="http://127.0.0.1:9090" \
  BOOTSTRAP_REFRESH_INTERVAL=15s \
  BOOTSTRAP_CLICKHOUSE_ADDRESS=127.0.0.1:9000 \
  sh -
```

Install coroot-node-agent to every node within your infrastructure:

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot-node-agent/main/install.sh | \
  COLLECTOR_ENDPOINT=http://COROOT_NODE_IP:8080 \
  SCRAPE_INTERVAL=15s \
  sh -
```
Access Coroot at: http://COROOT_NODE_IP:8080.
  </TabItem>
<TabItem value="rhel" label="RHEL & CentOS">

Coroot requires a Prometheus server with the Remote Write Receiver enabled, along with a ClickHouse server. 
For detailed steps on installing all the necessary components on an Ubuntu/Debian node, refer to the [full instructions](/installation/rhel?edition=ee).

To install Coroot, run the following command:

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/install.sh | \
  LICENSE_KEY="COROOT-LICENSE-KEY-HERE" \
  BOOTSTRAP_PROMETHEUS_URL="http://127.0.0.1:9090" \
  BOOTSTRAP_REFRESH_INTERVAL=15s \
  BOOTSTRAP_CLICKHOUSE_ADDRESS=127.0.0.1:9000 \
  sh -
```

Install coroot-node-agent to every node within your infrastructure:

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot-node-agent/main/install.sh | \
  COLLECTOR_ENDPOINT=http://COROOT_NODE_IP:8080 \
  SCRAPE_INTERVAL=15s \
  sh -
```

Access Coroot at: http://COROOT_NODE_IP:8080.
</TabItem>


</Tabs>
