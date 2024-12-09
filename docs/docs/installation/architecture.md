---
sidebar_position: 2
---

# Architecture
![Architecture Diagram](/img/docs/architecture.svg)

## Coroot-node-agent

Coroot-node-agent is an open-source observability agent powered by eBPF. It collects metrics, logs, traces, and profiles from all containers running on a node.

The agent supports both pull and push modes for metrics: it exposes metrics in Prometheus format and can also send metrics directly to Coroot using the Prometheus Remote Write protocol. 
Logs and traces are sent to Coroot via the OpenTelemetry protocol, while profiles are transmitted using a custom HTTP-based protocol.

To ensure full coverage, the agent needs to be installed on every node in your cluster. 
If you’re using Kubernetes, it’s deployed as a DaemonSet, so it will automatically be added to each node.

## Coroot-cluster-agent

Coroot-cluster-agent is a dedicated tool for collecting cluster-wide telemetry data:

* It gathers database metrics by discovering databases through Coroot's Service Map. Using the credentials provided by Coroot, the agent connects to the identified databases such as Postgres, MySQL, Redis, Memcached, and MongoDB, collects database-specific metrics, and sends them to Coroot using the Prometheus Remote Write protocol.
* In addition to the eBPF-based continuous profiler embedded in coroot-node-agent, Coroot also supports application-level profiling. The Cluster Agent can discover Go applications annotated with coroot.com/profile-scrape and coroot.com/profile-port, and gather CPU and memory profiles from their instances.
* The agent can be integrated with AWS to discover RDS and ElastiCache clusters and collect their telemetry data.

## OpenTelemetry

Coroot supports the OpenTelemetry protocol (OTLP over HTTP) for logs and traces. If your applications are instrumented with OpenTelemetry SDKs, 
you can configure them to send data directly to Coroot or route it through the OpenTelemetry Collector.

## Prometheus

Coroot uses Prometheus for storing metrics and is compatible with any Prometheus-compatible time series databases such as VictoriaMetrics, Thanos, or Grafana Mimir.

For faster access, Coroot maintains its own on-disk metric cache, continuously retrieving metrics from Prometheus. 
As a result, Coroot treats the time series database as a source for updating its cache. 
This allows you to configure Prometheus with a shorter retention period, such as a few hours.

## ClickHouse

Coroot uses ClickHouse for storing logs, traces, and profiles. 
Thanks to the efficient data compression implemented by ClickHouse, you can expect a compression ratio of 10x or more for this telemetry data.
