---
sidebar_position: 6
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Docker

<Tabs queryString="edition">
  <TabItem value="ce" label="Community Edition" default>

**Step #1: Install Docker Compose (if not installed)**

Use the following commands to install Docker Compose on Ubuntu:

```bash
apt update
apt install docker-compose-v2
```

**Step #2: Deploy Coroot**

To deploy Coroot using Docker Compose, run the following command. Before applying it, you can review the configuration file in Coroot's GitHub repository: docker-compose.yaml

```bash
curl -fsS https://raw.githubusercontent.com/coroot/coroot/main/deploy/docker-compose.yaml | \
  docker compose -f - up -d
```

**Step #3: Validate the deployment**

Ensure that the Coroot containers are running by executing the following command:

```bash
docker ps
```

You should see an output similar to this if the deployment is successful:

```bash
CONTAINER ID   IMAGE                                 COMMAND                  CREATED         STATUS         PORTS                                                 NAMES
b018f1cf6e09   ghcr.io/coroot/coroot-cluster-agent   "coroot-cluster-agen…"   5 seconds ago   Up 3 seconds                                                         coroot-cluster-agent-1
10b4bc2eef63   ghcr.io/coroot/coroot                 "/opt/coroot/coroot …"   5 seconds ago   Up 3 seconds   0.0.0.0:8080->8080/tcp, :::8080->8080/tcp             coroot-coroot-1
d0143aea889b   clickhouse/clickhouse-server:24.3     "/entrypoint.sh"         5 seconds ago   Up 4 seconds   8123/tcp, 9009/tcp, 127.0.0.1:9000->9000/tcp          coroot-clickhouse-1
4cbae2f36c1c   ghcr.io/coroot/coroot-node-agent      "coroot-node-agent -…"   5 seconds ago   Up 4 seconds                                                         coroot-node-agent-1
a6618978d560   prom/prometheus:v2.45.4               "/bin/prometheus --c…"   5 seconds ago   Up 4 seconds   127.0.0.1:9090->9090/tcp                              coroot-prometheus-1
```

**Step #4: Accessing Coroot**

If you installed Coroot on your desktop machine, you can access it at http://localhost:8080/.
If Coroot is deployed on a remote node, replace `NODE_IP_ADDRESS` with the IP address of the node in the following URL: 
http://NODE_IP_ADDRESS:8080/.

**Uninstall Coroot**

To uninstall Coroot run the following command:

```bash
curl -fsS https://raw.githubusercontent.com/coroot/coroot/main/deploy/docker-compose.yaml | \
  docker compose rm -f -s -v
```
  </TabItem>

  <TabItem value="ee" label="Enterprise Edition">

:::info
Coroot Enterprise Edition is a paid subscription (from $1 per CPU core/month) that offers extra features and priority support.
To install the Enterprise Edition, you'll need a license. [Start](https://coroot.com/account) your free trial today.
:::

**Step #1: Install Docker Compose (if not installed)**

Use the following commands to install Docker Compose on Ubuntu:

```bash
apt update
apt install docker-compose-v2
```

**Step #2: Deploy Coroot**

To install Coroot Enterprise Edition, you'll need a license (from $1 per CPU core/month). Start your free trial today.

To deploy Coroot using Docker Compose, run the following command. Before applying it, 
you can review the configuration file in Coroot's GitHub repository: docker-compose.yaml

```
curl -fsS https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/docker-compose.yaml | \
  LICENSE_KEY="COROOT-LICENSE-KEY-HERE" docker compose -f - up -d
```

**Step #3: Validate the deployment**

Ensure that the Coroot containers are running by executing the following command:

```
docker ps
```

You should see an output similar to this if the deployment is successful:

```
CONTAINER ID   IMAGE                                 COMMAND                  CREATED              STATUS              PORTS                                          NAMES
870119cb6859   ghcr.io/coroot/coroot-cluster-agent   "coroot-cluster-agen…"   29 seconds ago       Up 16 seconds                                                      coroot-ee-cluster-agent-1
6f3b8f1c821c   ghcr.io/coroot/coroot-ee:1.5.4        "/opt/coroot/coroot-…"   42 seconds ago       Up 16 seconds       0.0.0.0:8080->8080/tcp, :::8080->8080/tcp      coroot-ee-coroot-1
320e9154a8ba   clickhouse/clickhouse-server:24.3     "/entrypoint.sh"         About a minute ago   Up About a minute   8123/tcp, 9009/tcp, 127.0.0.1:9000->9000/tcp   coroot-ee-clickhouse-1
76b5968068f0   prom/prometheus:v2.45.4               "/bin/prometheus --c…"   About a minute ago   Up About a minute   127.0.0.1:9090->9090/tcp                       coroot-ee-prometheus-1
51e91e09e58a   ghcr.io/coroot/coroot-node-agent      "coroot-node-agent -…"   About a minute ago   Up About a minute                                                  coroot-ee-node-agent-1
```

**Step #4: Accessing Coroot**

If you installed Coroot on your desktop machine, you can access it at http://localhost:8080/.
If Coroot is deployed on a remote node, replace `NODE_IP_ADDRESS` with the IP address of the node in the following URL: 
http://NODE_IP_ADDRESS:8080/.

**Uninstall Coroot**

To uninstall Coroot run the following command:

```
curl -fsS https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/docker-compose.yaml | \
  docker compose rm -f -s -v
```
</TabItem>
</Tabs>
