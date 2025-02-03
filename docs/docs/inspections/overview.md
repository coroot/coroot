---
sidebar_position: 1
---

# Overview

When you look at any dashboard, you are mentally evaluating whether the metrics are within their acceptable range of values or not.

In order to automate this process, you can configure alerting rules to notify you when a metric exceeds a threshold. 
However, alerts usually have no context, so you have to manually extract issues relevant to a particular app from the alert stream.

<img alt="Alerting Rules" src="/img/docs/inspections_alerting_rules.svg" class="card w-600"/>

Coroot turns the conventional metric analysis inside out. 
It uses a distributed system model to evaluate every inspection within an application's context. 
For instance, the _Network round-trip time_ inspection checks the network latency between a particular app and the services it depends on.

<img alt="Inspections Model" src="/img/docs/inspections_model.svg" class="card w-600"/>

From a UI/UX perspective, each dashboard at the application level has a status which is calculated by checking the corresponding metrics.

<img alt="Inspection Audit report" src="/img/docs/inspections_audit_report.png" class="card w-1200"/>


Each inspection threshold can be easily overridden for a specific application or the entire project.

<img alt="Inspection Config" src="/img/docs/inspections_config.png" class="card w-1200"/>

## Application status

To highlight the status of each application on the overview page, Coroot takes the status of the SLO inspection.
<img alt="App Status" src="/img/docs/inspections_app_status.png" class="card w-1200"/>


