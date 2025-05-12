---
sidebar_position: 2
---

# Application logs

In the Application view, Coroot allows you to analyze and correlate application telemetry (availability, latency, CPU metrics, etc.) with raw logs and recurring log patterns.
Logs are pre-filtered by application, eliminating the need to locate it manually in the main Logs view.

<img alt="Coroot Log Monitoring" src="/img/docs/logs/application.png" class="card w-1200"/>


## Log patterns
To quickly understand what types of errors appeared in the logs at a particular time, you can switch to the "Patterns" mode.

<img alt="Log patterns" src="/img/docs/logs/patterns.png" class="card w-1200"/>

By clicking on any pattern, you can navigate to the original messages that match this pattern (Show Messages).

<img alt="Log pattern details" src="/img/docs/logs/pattern-details.png" class="card w-1200"/>

<img alt="Log pattern messages" src="/img/docs/logs/pattern-messages.png" class="card w-1200"/>


## Event details
Clicking on a specific event from the list allows you to access its details, including the full message text, severity, and OpenTelemetry attributes. You can also jump to similar messages that match the same pattern.

<img alt="Log message details" src="/img/docs/logs/event-details.png" class="card w-1200"/>
