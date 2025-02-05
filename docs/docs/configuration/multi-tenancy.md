---
sidebar_position: 6
---

# Multi-tenancy

A single Coroot instance can monitor multiple projects. Each project represents a distinct infrastructure, such as a 
Dev, Staging, or Production environment, a regional cluster, or a cluster dedicated to specific products or features.

## Configuration

By default, Coroot requires a dedicated ClickHouse and Prometheus configuration for each project.

To enable multi-tenancy mode, specify a global ClickHouse and Prometheus by using the [GLOBAL_CLICKHOUSE_ADDRESS](/configuration/configuration) 
and [GLOBAL_PROMETHEUS_URL](/configuration/configuration) environment variables or their corresponding CLI arguments.  
**Note**: When these parameters are set, ClickHouse and Prometheus configurations will no longer be editable in the UI.

## ClickHouse

When multi-tenancy mode is enabled, Coroot automatically creates a dedicated database for each project. 
Telemetry data pushed by Coroot agents (`coroot-node-agent` and `coroot-cluster-agent`) is stored in their 
respective project databases, ensuring data isolation and enabling efficient querying for individual projects.

## Prometheus

Multi-tenancy mode requires Coroot's agent to operate in push mode for metrics using the Prometheus Remote Write Protocol. 
This functionality is enabled by default when Coroot is deployed on Kubernetes using the [Coroot Operator](/installation/k8s-operator), 
Docker, or virtual machines (VMs).

Coroot automatically appends the `coroot_project_id` label to each metric and uses `{coroot_project_id="XXXX"}` as an 
additional selector when querying metrics for a specific project. This ensures precise data segmentation and retrieval per project.
