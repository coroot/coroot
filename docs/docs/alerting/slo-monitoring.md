---
sidebar_position: 1
---

# Service Level Objective (SLO) monitoring

An SLO (Service Level Objective) is a specific goal that defines how well a service should perform, 
such as its uptime or response time. It sets the standard for reliability or performance that a service needs to meet to keep users happy.

The most common SLOs focus on availability and latency:
* Availability SLO: This tracks how often a service is available to users. For example, an SLO might require that 99.9% of all requests are successful.
* Latency SLO: This measures how fast a service responds. An example SLO might state that 99% of requests should be completed in under 500 milliseconds.

Coroot uses eBPF to automatically gather performance data for each service. It also comes with preset SLOs that you can easily adjust, so it can start monitoring your services right after installation.

To avoid violating SLOs, Coroot alerts your team when the error budget is being consumed too quickly. 
It uses multi-window burn rate thresholds to trigger alerts:

| Severity  | Long Window | Short Window | Burn Rate Threshold | Monthly Error Budget Consumed | Time to Exhaustion |
|-----------|-------------|--------------|---------------------|-------------------------------|--------------------|
| Critical  | 1 hour      | 5 minutes    | 14.4                | 2%                            | ≤ 50 hours         |
| Critical  | 6 hours     | 30 minutes   | 6                   | 5%                            | ≤ 5 days           |
| Warning   | 24 hours    | 2 hours      | 3                   | 10%                           | ≤ 10 days          |


An incident will be triggered if the burn rate exceeds the threshold in both the long and short windows. 
The short window helps ensure the error budget is still being actively used.

To prevent false positive alerts, Coroot only calculates the burn rate if at least half of the window contains valid data. 
This is especially useful for services with low traffic.

:::info
The detailed explanation of SLO-based alerting you can find in [The SRE Workbook](https://sre.google/workbook/alerting-on-slos/).
:::

When an application significantly violates its SLOs, Coroot triggers an incident and notifies the team through the configured integrations:
* [Slack](/alerting/slack)
* [Microsoft Teams](/alerting/teams)
* [Pagerduty](/alerting/pagerduty)
* [OpsGenie](/alerting/opsgenie)
* [Webhook](/alerting/webhook)

