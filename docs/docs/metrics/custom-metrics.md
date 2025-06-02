---
sidebar_position: 3
toc_max_heading_level: 2
---

# Custom metrics

Coroot-cluster-agent can scrape custom metrics exposed by an application in the Prometheus format.
So far, it supports only Kubernetes service discovery.

## Kubernetes service discovery

If a pod exposes metrics on a specific endpoint (like `/metrics`), you can annotate the pod to enable scraping by coroot-cluster-agent.

For example, to enable metrics scraping, add the following annotations to your pod:

```yaml
metadata:
  annotations:
    coroot.com/scrape-metrics: 'true'
    coroot.com/metrics-port: '8080'
    coroot.com/metrics-path: '/metrics' # optional
    coroot.com/metrics-scheme: 'http' # optional
```

This configuration tells coroot-cluster-agent to scrape metrics from port `8080` and the `/metrics` path.

Each scraped metric will be annotated with the `pod` and `namespace` labels, allowing you to filter and aggregate metrics efficiently.
