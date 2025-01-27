---
sidebar_position: 5
---

# OpenShift

Coroot Enterprise Edition is available in the OperatorHub catalog and is fully certified for OpenShift, 
making the installation process seamless and straightforward. 
Follow the steps below to install and configure Coroot Enterprise Edition on your OpenShift cluster.


## Prerequisites

Before you begin, ensure the following:
1. **OpenShift Cluster**:
    - You must have an OpenShift 4.x cluster running.
    - You must have cluster administrator privileges.
2. **Subscription**:
    - You need an active Coroot Enterprise Edition license. If you don't have one, you can get a license and start a free trial at [Coroot Customer Portal](https://coroot.com/account).
3. **Internet Access**:
    - Ensure your cluster can access the Red Hat Ecosystem Catalog and the Coroot OperatorHub repository.
    - Ensure your cluster can access `coroot.com` for license verification.


## Step#1: Install Coroot Operator

Launch OpenShift web console. Using the `Administrator` view, navigate to `Operators` > `OperatorHub`. 
Search for `Coroot`.
<img alt="Operator Search" src="/img/docs/installation/openshift/operator-search.png" class="card w-1200"/>

Click `Install` to install the operator.
<img alt="Operator Install" src="/img/docs/installation/openshift/operator-install.png" class="card w-1200"/>

Configure the Operator installation as shown:
:::info
We recommend selecting `Automatic` for `Upgrade Approval`.
This setting automatically upgrades the operator whenever upgrades are available.
:::
<img alt="Operator Configure" src="/img/docs/installation/openshift/operator-configure.png" class="card w-1200"/>

On successful installation, a message like the following appears:
<img alt="Operator Ready" src="/img/docs/installation/openshift/operator-ready.png" class="card w-1200"/>

Select `View Operator` to verify the Operator details. You will see the following details:
<img alt="Operator Details" src="/img/docs/installation/openshift/operator-details.png" class="card w-1200"/>


## Step#2: Install Coroot

Select or create an appropriate `project` (aka `namespace`):
<img alt="Coroot Namespace" src="/img/docs/installation/openshift/coroot-namespace.png" class="card w-1200"/>

Select the `Coroot` tab and click `Create Coroot`.
<img alt="Coroot Create" src="/img/docs/installation/openshift/coroot-create.png" class="card w-1200"/>

Fill out the necessary parameters, including:
* License key.
* Storage configuration.
* Ingress configuration.

:::info
Fill in the `licenseKey` parameter with a valid Coroot Enterprise license, 
which can be acquired from the [Customer Portal](https://coroot.com/account).
:::
Then click `Create`.

<img alt="Coroot Configure" src="/img/docs/installation/openshift/coroot-configure.png" class="card w-1200"/>


## Step#3: Verify the installation

* Navigate to `Workloads` > `Pods` and check the namespace where Coroot Enterprise Edition was deployed.
* Verify that all Coroot pods are running without issues.
  <img alt="Coroot Pods" src="/img/docs/installation/openshift/coroot-pods.png" class="card w-1200"/>
* Access Coroot in your browser by using the configured Ingress host, such as http://coroot.example.com.
