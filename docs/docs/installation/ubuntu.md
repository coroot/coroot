---
sidebar_position: 8
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Ubuntu & Debian

<Tabs queryString="edition">
  <TabItem value="ce" label="Community Edition" default>

**Step #1: Installing ClickHouse**

```bash
sudo apt install -y apt-transport-https ca-certificates curl gnupg
curl -fsSL 'https://packages.clickhouse.com/rpm/lts/repodata/repomd.xml.key' | sudo gpg --dearmor -o /usr/share/keyrings/clickhouse-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/clickhouse-keyring.gpg] https://packages.clickhouse.com/deb stable main" | sudo tee /etc/apt/sources.list.d/clickhouse.list
sudo apt update
sudo DEBIAN_FRONTEND=noninteractive apt install -y clickhouse-server clickhouse-client
sudo service clickhouse-server start
```

**Step #2: Installing Prometheus**

Coroot requires Prometheus with support for Remote Write Receiver, which has been available since v2.25.0.

```bash
sudo apt install -y prometheus
sudo service prometheus start
```

Enable Remote Write Receiver in Prometheus by adding the `--enable-feature=remote-write-receiver` argument to the `/etc/default/prometheus` file:

```bash
# Set the command-line arguments to pass to the server.
# Due to shell escaping, to pass backslashes for regexes, you need to double
# them (\\d for \d). If running under systemd, you need to double them again
# (\\\\d to mean \d), and escape newlines too.
ARGS="--enable-feature=remote-write-receiver"
```

Restart Prometheus:

```bash
sudo service prometheus restart
```

**Step #3: Installing Coroot**

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot/main/deploy/install.sh | \
  BOOTSTRAP_PROMETHEUS_URL="http://127.0.0.1:9090" \
  BOOTSTRAP_REFRESH_INTERVAL=15s \
  BOOTSTRAP_CLICKHOUSE_ADDRESS=127.0.0.1:9000 \
  sh -
```

**Step #4: Installing coroot-node-agent**

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot-node-agent/main/install.sh | \
  COLLECTOR_ENDPOINT=http://127.0.0.1:8080 \
  SCRAPE_INTERVAL=15s \
  sh -
```

**Step #5: Accessing Coroot**

Access Coroot at: http://NODE_IP:8080.

**Uninstall Coroot**

To uninstall Coroot run the following command:

```bash
/usr/bin/coroot-uninstall.sh
```

Uninstall coroot-node-agent:

```bash
/usr/bin/coroot-node-agent-uninstall.sh
```
  </TabItem>

  <TabItem value="ee" label="Enterprise Edition">

:::info
Coroot Enterprise Edition is a paid subscription (from $1 per CPU core/month) that offers extra features and priority support.
To install the Enterprise Edition, you'll need a license. [Start](https://coroot.com/account) your free trial today.
:::

**Step #1: Installing ClickHouse**

```bash
sudo apt install -y apt-transport-https ca-certificates curl gnupg
curl -fsSL 'https://packages.clickhouse.com/rpm/lts/repodata/repomd.xml.key' | sudo gpg --dearmor -o /usr/share/keyrings/clickhouse-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/clickhouse-keyring.gpg] https://packages.clickhouse.com/deb stable main" | sudo tee /etc/apt/sources.list.d/clickhouse.list
sudo apt update
sudo DEBIAN_FRONTEND=noninteractive apt install -y clickhouse-server clickhouse-client
sudo service clickhouse-server start
```

**Step #2: Installing Prometheus**

Coroot requires Prometheus with support for Remote Write Receiver, which has been available since v2.25.0.

```bash
sudo apt install -y prometheus
sudo service prometheus start
```

Enable Remote Write Receiver in Prometheus by adding the `--enable-feature=remote-write-receiver` argument to the `/etc/default/prometheus` file:

```bash
# Set the command-line arguments to pass to the server.
# Due to shell escaping, to pass backslashes for regexes, you need to double
# them (\\d for \d). If running under systemd, you need to double them again
# (\\\\d to mean \d), and escape newlines too.
ARGS="--enable-feature=remote-write-receiver"
```

Restart Prometheus:

```bash
sudo service prometheus restart
```

**Step #3: Installing Coroot**

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/install.sh | \
  LICENSE_KEY="COROOT-LICENSE-KEY-HERE" \
  BOOTSTRAP_PROMETHEUS_URL="http://127.0.0.1:9090" \
  BOOTSTRAP_REFRESH_INTERVAL=15s \
  BOOTSTRAP_CLICKHOUSE_ADDRESS=127.0.0.1:9000 \
  sh -
```

**Step #4: Installing coroot-node-agent**

```bash
curl -sfL https://raw.githubusercontent.com/coroot/coroot-node-agent/main/install.sh | \
  COLLECTOR_ENDPOINT=http://127.0.0.1:8080 \
  SCRAPE_INTERVAL=15s \
  sh -
```

**Step #5: Accessing Coroot**

Access Coroot at: http://NODE_IP:8080.

**Uninstall Coroot**

To uninstall Coroot run the following command:

```bash
/usr/bin/coroot-ee-uninstall.sh
```

Uninstall coroot-node-agent:

```bash
/usr/bin/coroot-node-agent-uninstall.sh
```
</TabItem>
</Tabs>
