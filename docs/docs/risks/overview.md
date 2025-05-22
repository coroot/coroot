---
sidebar_position: 1
---

# Overview

Risk management is a big part of how SREs think about reliability. It’s not about fixing every possible issue, it’s about constantly asking “what if?” and being prepared for things that could go wrong.

Some risks are fine to tolerate, maybe they’re low impact, unlikely to happen, too expensive to fix, or just not a priority right now. 
Others are quick wins and worth addressing. 
But as systems grow more complex and change rapidly, it becomes hard to track risks manually. That’s where automation helps.

Coroot Risk Monitoring automatically detects availability and some security risks across your infrastructure.

<img alt="Risks monitoring" src="/img/docs/risks/risks.png" class="card w-1200"/>


## Availability

Availability risks are potential situations that can lead to service unavailability or even data loss.  
Coroot uses a model of your system to simulate failure scenarios and identify weak spots.

Below are the currently supported scenarios that Coroot validates for each application:

### Single-instance application

<img alt="Single-instance application" src="/img/docs/risks/single_instance.png" class="card w-1200"/>

Even in Kubernetes, a node failure can temporarily make a service unavailable.  
It takes some time for the control plane to detect the failure and reschedule pods. During this period, your app may be unreachable.

To avoid excessive noise, Coroot doesn’t trigger this risk for:
* Apps that don’t communicate with other services
* Single-node clusters

You can also dismiss the risk manually if needed.

### All instances on one node

<img alt="All instances on one node" src="/img/docs/risks/one_node.png" class="card w-1200"/>

If your app has multiple replicas, they might all end up scheduled on the same node.  
If that node fails, your service will go down despite having multiple instances.

Coroot excludes this risk for:
* Single-node clusters
* Standalone applications

### All instances in one Availability Zone

<img alt="All instances in one Availability Zone" src="/img/docs/risks/one_az.png" class="card w-1200"/>

To survive an Availability Zone (AZ) failure, important applications should have instances spread across multiple AZs.

Running in a single AZ is a valid trade-off in many cases, cross-AZ setups can increase latency and data transfer costs.  
So Coroot only evaluates this risk if your cluster spans multiple AZs.

### All instances on Spot nodes

<img alt="All instances on Spot nodes" src="/img/docs/risks/spot_only.png" class="card w-1200"/>

Spot nodes are cheaper and increasingly used even for user-facing services.  
However, they can be terminated at any time with little notice, so your app must be resilient to that.

A common pattern is to mix Spot and On-Demand nodes. This way, even if Spot instances are lost, On-Demand instances can keep the app running.

Coroot flags this risk only if your cluster includes On-Demand nodes.  
For Spot-only clusters, Coroot assumes the setup is intentional and doesn’t report this as a risk.

### Unreplicated databases

For stateful apps like databases, the failure of a node using local storage can result in data loss.  
If the database uses network-attached storage like AWS EBS, the volume can be reattached to another node, but this takes time.

EBS volumes are highly durable (AWS claims 99.999% durability), but reattachment delays can still impact availability.

To mitigate these risks:
- Use backups to prevent data loss
- Use replication to reduce downtime

:::warning
Replication isn’t a replacement for backups, accidental deletions or unexpected changes will be copied to all replicas.
:::

Coroot can't currently verify backups but can detect whether a database is replicated.  
It checks whether the database service has multiple instances or communicates with another DB (implying replication).

<img alt="Unreplicated databases" src="/img/docs/risks/unreplicated_db.png" class="card w-1200"/>

This doesn't validate replication health or data consistency, but it's a useful starting point for identifying at-risk databases.

As always, you can dismiss this risk for any database with one click.

## Security

:::warning
Currently, Coroot validates only one security risk, so don't consider it as a replacement for other Security audit tools.
:::

### Publicly Exposed Databases

<img alt="Publicly Exposed Databases" src="/img/docs/risks/db_exposure.png" class="card w-1200"/>


Since Coroot automatically detects the type of every application or container, it can distinguish between database servers and stateless apps. 
It supports a wide range of open-source databases, including PostgreSQL, MySQL, Redis (and its alternatives), Memcached, MongoDB, Elasticsearch, 
OpenSearch, ClickHouse, Prometheus, VictoriaMetrics, Kafka, RabbitMQ, and more.

However, databases accepting connections on public IPs are only part of the problem. 
On Kubernetes, services can be exposed through a NodePort or LoadBalancer, making them accessible from the internet. 
Coroot already collects data about Kubernetes Services, so we’ve covered those scenarios as well.

Of course, some databases are intentionally exposed, for example, when access is controlled via firewalls, AWS Security Groups, 
or built-in database security mechanisms. If that’s the case, you can simply dismiss the risk.

## Dismissing risks

If a risk isn’t relevant, you can dismiss it by clicking the three-dot menu next to the risk:

<img alt="Dismiss" src="/img/docs/risks/dismiss.png" class="card w-1200"/>

Once dismissed, the risk will be hidden from the main list but still recorded.
To view dismissed risks, enable the **Show dismissed** checkbox at the top of the page:

<img alt="Show dismissed risks" src="/img/docs/risks/show_dismissed.png" class="card w-1200"/>

Dismissed risks are shown in a lighter gray color and include a note with the dismissal reason and timestamp. For example:

> _Dismissed by Admin (2025-05-22 14:09:03) as "tolerable for this project"_

This makes it easy to track which risks were reviewed and why they were dismissed, helping ensure transparency and accountability in decision-making.

You can re-enable any dismissed risk at any time by clicking the same three-dot menu and selecting **Mark as Active**.

<img alt="Mark as active" src="/img/docs/risks/mark_as_active.png" class="card w-1200"/>

