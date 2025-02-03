---
sidebar_position: 7
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Docker Swarm

<Tabs queryString="edition">
  <TabItem value="ce" label="Community Edition" default>

**Step #1: Initialize Docker Swarm**

If you haven't already initialized Docker Swarm on your manager node, run the following command on the manager node:

```bash
docker swarm init
```

This initializes a new Docker Swarm and joins the current node as a manager.

**Step #2: Deploy the Coroot Stack**

Deploy the Coroot stack to your cluster by running the following command on the manager node. 
Before applying, you can review the configuration file in Coroot's GitHub repository: docker-swarm-stack.yaml

```bash
curl -fsS https://raw.githubusercontent.com/coroot/coroot/main/deploy/docker-swarm-stack.yaml | \
  docker stack deploy -c - coroot
```

**Step #3: Validate the deployment**

After deploying the stack, you can use docker stack ls to list the deployed stacks in your Docker Swarm cluster. 
Here's an example of how the output might look:

```bash
NAME      SERVICES
coroot    3
```

**Step #4: Installing coroot-node-agent**

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

**Step #5: Accessing Coroot**

Access Coroot through any node in your Docker Swarm cluster using its published port: http://NODE_IP:8080.

**Uninstall Coroot**

To uninstall Coroot run the following command:

```bash
docker stack rm coroot
```
  </TabItem>

  <TabItem value="ee" label="Enterprise Edition">

:::info
Coroot Enterprise Edition is a paid subscription (from $1 per CPU core/month) that offers extra features and priority support.
To install the Enterprise Edition, you'll need a license. [Start](https://coroot.com/account) your free trial today.
:::

**Step #1: Initialize Docker Swarm**

If you haven't already initialized Docker Swarm on your manager node, run the following command on the manager node:

```
docker swarm init
```

This initializes a new Docker Swarm and joins the current node as a manager.

**Step #2: Deploy the Coroot Stack**

Deploy the Coroot stack to your cluster by running the following command on the manager node. Before applying, you can review the configuration file in Coroot's GitHub repository: docker-swarm-stack.yaml

```
curl -fsS https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/docker-swarm-stack.yaml | \
  LICENSE_KEY="COROOT-LICENSE-KEY-HERE" docker stack deploy -c - coroot-ee
```

**Step #3: Validate the deployment**

After deploying the stack, you can use docker stack ls to list the deployed stacks in your Docker Swarm cluster. 
Here's an example of how the output might look:

```
NAME        SERVICES
coroot-ee   4
```

**Step #4: Installing coroot-node-agent**

Since Docker Swarm doesn't support privileged containers, you'll have to manually deploy coroot-node-agent on each cluster node. 
Just replace `NODE_IP` with any node's IP address in the Docker Swarm cluster.

```
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

**Step #5: Accessing Coroot**

Access Coroot through any node in your Docker Swarm cluster using its published port: http://NODE_IP:8080.

**Uninstall Coroot**

To uninstall Coroot run the following command:

```
docker stack rm coroot-ee
```
</TabItem>
</Tabs>
