---
sidebar_position: 1
---

# Overview

Coroot's Logs monitoring enables you to effortlessly analyze your application logs and correlate them with traces, metrics, and profiles. 
All logs are grouped by application, eliminating the need for manual navigation.

<img alt="Coroot Log Monitoring" src="/img/docs/logs_demo.gif" class="card w-1200"/>

Coroot's node-agent automatically discovers and gathers logs from all containers on a node, then transmits them to Coroot. 
Additionally, it performs low-overhead log analysis right on the node to identify message severities and recurring patterns. 
This process is seamless and compatible with a wide range of log formats, providing valuable meta-information for quick and easy log analysis.

## Log patterns
To quickly understand what types of errors appeared in the logs at a particular time, you can switch to the "Patterns" mode.

<img alt="Log patterns" src="/img/docs/logs_patterns.png" class="card w-1200"/>

By clicking on any pattern, you can view the message distribution across application instances and navigate to the original messages that match this pattern (Show Messages).

<img alt="Log pattern details" src="/img/docs/logs_pattern_details.png" class="card w-1200"/>

<img alt="Log pattern messages" src="/img/docs/logs_pattern_messages.png" class="card w-1200"/>

## Event details
Clicking on a specific event from the list allows you to access its details, including the full message text, severity, and OpenTelemetry attributes. You can also jump to similar messages that match the same pattern.

<img alt="Log message details" src="/img/docs/logs_message_details.png" class="card w-1200"/>

## Correlating logs and traces

If you instrument your apps with the OpenTelemetry SDK to send logs to Coroot's OpenTelemetry collector along with the tracing context, 
you can instantly navigate to the corresponding trace with just one click.

<img alt="Correlating logs and traces" src="/img/docs/logs_to_traces.gif" class="card w-1200"/>



