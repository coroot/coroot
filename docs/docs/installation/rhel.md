---
sidebar_position: 9
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# RHEL & CentOS

<Tabs queryString="edition">
  <TabItem value="ce" label="Community Edition" default>

**Step #1: Installing ClickHouse**

```bash
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://packages.clickhouse.com/rpm/clickhouse.repo
sudo yum install -y clickhouse-server clickhouse-client
sudo systemctl enable clickhouse-server
sudo systemctl start clickhouse-server
```

**Step #2: Installing Prometheus**

Coroot requires Prometheus with support for Remote Write Receiver, which has been available since v2.25.0. 
Unfortunately, there is no official Prometheus RPM package. 
Here is a step-by-step guide on how to install Prometheus manually, including creating a systemd service file for it.

Create a Prometheus user:

```bash
sudo useradd --no-create-home --shell /bin/false prometheus
```

Navigate to the Prometheus download page, and find the URL for the latest version of Prometheus for Linux and your architecture. 
Use wget to download it:

```bash
wget https://github.com/prometheus/prometheus/releases/download/v2.51.2/prometheus-2.51.2.linux-amd64.tar.gz
```

Extract the downloaded tarball and move the binary to `/usr/local/bin/`:

```bash
sudo yum -y install tar
tar xvfz prometheus-2.51.2.linux-amd64.tar.gz
sudo cp prometheus-2.51.2.linux-amd64/prometheus /usr/local/bin/
```

Create directories for Prometheus configuration files and data:

```bash
sudo mkdir /etc/prometheus
sudo mkdir /var/lib/prometheus
sudo chown -R prometheus:prometheus /etc/prometheus /var/lib/prometheus
```

Create a Prometheus configuration file:

```sh
sudo cat <<EOF >/etc/prometheus/prometheus.yml
global:
scrape_interval: 15s
EOF
```

Create a systemd service file for Prometheus:

```sh
sudo cat <<EOF >/etc/systemd/system/prometheus.service
[Unit]
Description=Prometheus
After=network.target

[Service]
User=prometheus
Group=prometheus
Type=simple
ExecStart=/usr/local/bin/prometheus --config.file /etc/prometheus/prometheus.yml --storage.tsdb.path /var/lib/prometheus --enable-feature=remote-write-receiver

[Install]
WantedBy=multi-user.target
EOF
```

Reload systemd to load the new service and start Prometheus:

```
sudo systemctl daemon-reload
sudo systemctl start prometheus
sudo systemctl enable prometheus
```

**Step #3: Installing Coroot**

```
curl -sfL https://raw.githubusercontent.com/coroot/coroot/main/deploy/install.sh | \
  BOOTSTRAP_PROMETHEUS_URL="http://127.0.0.1:9090" \
  BOOTSTRAP_REFRESH_INTERVAL=15s \
  BOOTSTRAP_CLICKHOUSE_ADDRESS=127.0.0.1:9000 \
  sh -
```

**Step #4: Installing coroot-node-agent**

```
curl -sfL https://raw.githubusercontent.com/coroot/coroot-node-agent/main/install.sh | \
  COLLECTOR_ENDPOINT=http://127.0.0.1:8080 \
  SCRAPE_INTERVAL=15s \
  sh -
```

**Step #5: Accessing Coroot**

Access Coroot at: http://NODE_IP:8080.

**Uninstall Coroot**

To uninstall Coroot run the following command:

```
/usr/bin/coroot-uninstall.sh
```

Uninstall coroot-node-agent:

```
/usr/bin/coroot-node-agent-uninstall.sh
```
</TabItem>

  <TabItem value="ee" label="Enterprise Edition">

:::info
Coroot Enterprise Edition is a paid subscription (from $1 per CPU core/month) that offers extra features and priority support.
To install the Enterprise Edition, you'll need a license. [Start](https://coroot.com/account) your free trial today.
:::

**Step #1: Installing ClickHouse**

```
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://packages.clickhouse.com/rpm/clickhouse.repo
sudo yum install -y clickhouse-server clickhouse-client
sudo systemctl enable clickhouse-server
sudo systemctl start clickhouse-server
```

**Step #2: Installing Prometheus**

Coroot requires Prometheus with support for Remote Write Receiver, which has been available since v2.25.0. 
Unfortunately, there is no official Prometheus RPM package. 
Here is a step-by-step guide on how to install Prometheus manually, including creating a systemd service file for it.

Create a Prometheus user:

```
sudo useradd --no-create-home --shell /bin/false prometheus
```

Navigate to the Prometheus download page, and find the URL for the latest version of Prometheus for Linux and your architecture. Use wget to download it:

```
wget https://github.com/prometheus/prometheus/releases/download/v2.51.2/prometheus-2.51.2.linux-amd64.tar.gz
```

Extract the downloaded tarball and move the binary to `/usr/local/bin/`:

```
sudo yum -y install tar
tar xvfz prometheus-2.51.2.linux-amd64.tar.gz
sudo cp prometheus-2.51.2.linux-amd64/prometheus /usr/local/bin/
```

Create directories for Prometheus configuration files and data:

```
sudo mkdir /etc/prometheus
sudo mkdir /var/lib/prometheus
sudo chown -R prometheus:prometheus /etc/prometheus /var/lib/prometheus
```

Create a Prometheus configuration file:

```sh
sudo cat <<EOF >/etc/prometheus/prometheus.yml
global:
scrape_interval: 15s
EOF
```

Create a systemd service file for Prometheus:

```sh
sudo cat <<EOF >/etc/systemd/system/prometheus.service
[Unit]
Description=Prometheus
After=network.target

[Service]
User=prometheus
Group=prometheus
Type=simple
ExecStart=/usr/local/bin/prometheus --config.file /etc/prometheus/prometheus.yml --storage.tsdb.path /var/lib/prometheus --enable-feature=remote-write-receiver

[Install]
WantedBy=multi-user.target
EOF
```

Reload systemd to load the new service and start Prometheus:

```
sudo systemctl daemon-reload
sudo systemctl start prometheus
sudo systemctl enable prometheus
```

**Step #3: Installing Coroot**

```
curl -sfL https://raw.githubusercontent.com/coroot/coroot-ee/main/deploy/install.sh | \
  LICENSE_KEY="COROOT-LICENSE-KEY-HERE" \
  BOOTSTRAP_PROMETHEUS_URL="http://127.0.0.1:9090" \
  BOOTSTRAP_REFRESH_INTERVAL=15s \
  BOOTSTRAP_CLICKHOUSE_ADDRESS=127.0.0.1:9000 \
  sh -
```

**Step #5: Accessing Coroot**

Access Coroot at: http://NODE_IP:8080.

**Uninstall Coroot**

To uninstall Coroot run the following command:

```
/usr/bin/coroot-uninstall.sh
```

Uninstall coroot-node-agent:

```
/usr/bin/coroot-node-agent-uninstall.sh
```
</TabItem>
</Tabs>
