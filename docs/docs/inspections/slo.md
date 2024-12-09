---
sidebar_position: 2
---

# Service Level Objectives (SLOs)

These inspections allow you to monitor the _Availability_ and _Latency_ SLOs (Service Level Objectives) for every application. 
By default, Coroot tracks the application layer metrics gathered by coroot-node-gent, but you can replace them with your custom Prometheus metrics.

<img alt="SLO" src="/img/docs/inspections_slo.png" class="card w-1200"/>

## Availability

The predefined `Availability` SLO: 99% of requests should be server without errors.
You can easily adjust the objective:

<img alt="Availability SLO" src="/img/docs/inspections_slo_availability.png" class="card w-600"/>

...or configure Coroot to rack your custom Prometheus metrics:

<img alt="Custom Availability SLO" src="/img/docs/inspections_slo_availability_custom.png" class="card w-600"/>

## Latency

The predefined `Latency` SLO: 99% of requests should be served in less that 500ms.

<img alt="Latency SLO" src="/img/docs/inspections_slo_latency.png" class="card w-600"/>

You can also define any Prometheus [histogram](https://prometheus.io/docs/practices/histograms/) to be used instead of the built-in metrics:

<img alt="Custom Latency SLO" src="/img/docs/inspections_slo_latency_custom.png" class="card w-600"/>


