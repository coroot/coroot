---
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Kubernetes

<Tabs queryString="edition">
  <TabItem value="ce" label="Community Edition (operator)" default>

Add the Coroot helm chart repo:

```bash
helm repo add coroot https://coroot.github.io/helm-charts
helm repo update coroot
```

Next, install the Coroot Operator:

```bash
helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator
```

Install the Coroot Community Edition. This chart creates a minimal [Coroot Custom Resource](/installation/k8s-operator):

```bash
helm install -n coroot coroot coroot/coroot-ce \
  --set "clickhouse.shards=2,clickhouse.replicas=2"
```

The helm chart 

Forward the Coroot port to your machine:

```bash
kubectl port-forward -n coroot service/coroot-coroot 8080:8080
```

Then, you can access Coroot at http://localhost:8080

**Upgrade**

The Coroot Operator for Kubernetes automatically upgrades all components.

**Uninstall**

To uninstall Coroot run the following command:

```bash
helm uninstall coroot -n coroot
helm uninstall coroot-operator -n coroot
```
  </TabItem>

  <TabItem value="ee" label="Enterprise Edition (operator)">

:::info
Coroot Enterprise Edition is a paid subscription (from $1 per CPU core/month) that offers extra features and priority support.
To install the Enterprise Edition, you'll need a license. [Start](https://coroot.com/account) your free trial today.
:::

Add the Coroot helm chart repo:

```bash
helm repo add coroot https://coroot.github.io/helm-charts
helm repo update coroot
```

Next, install the Coroot Operator:

```bash
helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator
```

Install the Coroot Enterprise Edition.This chart creates a minimal [Coroot Custom Resource](/installation/k8s-operator):

```
helm install -n coroot coroot coroot/coroot-ee \
  --set "licenseKey=COROOT-LICENSE-KEY-HERE,clickhouse.shards=2,clickhouse.replicas=2"
```

Forward the Coroot port to your machine:

```
kubectl port-forward -n coroot service/coroot-coroot 8080:8080
```

Then, you can access Coroot at http://localhost:8080

**Upgrade**

The Coroot Operator for Kubernetes automatically upgrades all components.

**Uninstall**

To uninstall Coroot run the following command:

```
helm uninstall coroot -n coroot
helm uninstall coroot-operator -n coroot
```
  </TabItem>

<TabItem value="ce-helm" label="Community Edition (Helm, deprecated)">

:::warning
Installing Coroot via the Helm chart is deprecated. Please use the Coroot Operator instead.
:::

Add the Coroot helm chart repo:

```
helm repo add coroot https://coroot.github.io/helm-charts
helm repo update coroot
```

Next, install the chart that includes:

```
helm install --namespace coroot --create-namespace coroot coroot/coroot
```

Forward the Coroot port to your machine:

```
kubectl port-forward -n coroot service/coroot 8080:8080
```

Then, you can access Coroot at http://localhost:8080

**Upgrade**

To upgrade Coroot to the latest version:

```
helm repo update coroot
helm upgrade coroot --namespace coroot coroot/coroot
```

**Uninstall**

To uninstall Coroot run the following command:

```
helm uninstall coroot -n coroot
```
  </TabItem>



</Tabs>
