---
sidebar_position: 5
---

# Memory

These memory inspections identify problems with the application's memory:

* Out of Memory: detects situations where an application's container has been terminated by the OOM (Out Of Memory) killer.
* Memory leak: monitors the memory usage of the app over time and detects memory leaks before the OOM killer restarts the application containers.

<img alt="Memory" src="/img/docs/memory.png" class="card w-1200"/>

## Possible failure scenarios

### Out of Memory

There are several reasons why the OOM killer mechanism might terminate a process:
* If a container has reached its memory limit.
* If a top-level cgroup has reached its memory limit. 
For example, Kubernetes limits the `kubepods` cgroup to the size of allocatable memory on the node, so a container can be killed even if it has no memory limit.
* If a node has run out of memory.

Though the terminated container will be restarted by a container runtime, this can affect the application SLIs (Services Level Indicators). 
For example, when a container is terminated, all its in-progress requests will fail.

In the worst cases of node-level OOMs, a node becomes unresponsive due to the low-memory condition.
This means that every application on the node 'freezes', so not only the OOM Killer victim's SLI can be affected.

Coroot utilizes the `container_oom_kills_total` metric to identify which containers have been terminated.

### Memory leak

Memory leaks typically occur when a program allocates memory dynamically during its execution but forgets to release it when it's no longer needed.

Coroot calculates linear regression using the `container_resources_memory_rss_bytes` metric
to determine if a container's memory consumption is increasing over time.

The default threshold for detecting a significant increase in memory consumption is set at 10% per hour.

## Dashboard

### Memory usage

<img alt="Memory usage" src="/img/docs/memory-containers.png" class="card w-600"/>


This chart can help you answer the following questions:

* Are all the application instances consuming the same amount of memory, or are there any outliers?
* How does memory consumption change over time?
* Assess the memory usage of each container in comparison to its limit.


Memory usage is calculated using the `container_resources_memory_rss_bytes` metric,
and does not take into account the amount of page cache memory allocated by the container.

If the application Pods contain more than one container, this chart provides you with both per-container and total views.

The **profile** button opens the memory profiling data,
allowing you to identify and analyze unexpected spikes in memory usage down to the precise line of code.

:::info
Learn more about [Continuous profiling](/profiling/) in Coroot.
:::

### Out of memory events

<img alt="Memory OOM" src="/img/docs/memory-ooms.png" class="card w-600"/>

Based on the `container_oom_kills_total` metric,
this chart shows the number of times application containers have been terminated by the OOM killer and when these terminations occurred.

### Node memory usage (unreclaimable)

<img alt="Memory nodes" src="/img/docs/memory-nodes.png" class="card w-600"/>


This chart allows you to estimate the memory usage of the related nodes.
It does not take into account the page cache size because this memory can be reclaimed for new allocations.

In situations where there is no memory available for allocations,
the OOM killer will terminate certain processes on the node, even if they have no memory limits defined.

:::info
Node memory usage = (`total` - `available`) / `total` * 100%
:::

### Memory consumers

<div class="horizontal-images">
  <img alt="CPU consumers" src="/img/docs/memory-consumers-1.png" class="card" />
  <img alt="CPU consumers" src="/img/docs/memory-consumers-2.png" class="card" />
</div>

When you observe high memory usage on a particular node,
this chart can assist you in identifying the primary memory-consuming applications.
The chart displays the top 5 applications by their peak memory consumption.

The chart is based on the `container_resources_memory_rss_bytes` metric,
which does not include the amount of page cache allocated by the container.
