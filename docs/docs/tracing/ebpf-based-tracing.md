---
sidebar_position: 2
---

# eBPF-based tracing

While it is ideal to instrument every service with OpenTelemetry, doing so can be challenging, expensive, or simply not 
feasible in certain cases (e.g., with legacy or third-party services). 
In situations where an application lacks OpenTelemetry instrumentation, [coroot-node-agent](https://github.com/coroot/coroot-node-agent) can help by capturing outbound 
requests at the eBPF level and exporting them as OpenTelemetry tracing spans.

Although eBPF-based spans may not provide complete traces, they offer significant value in troubleshooting services that 
have not yet been instrumented. The agent can auto-instrument protocols including HTTP, Postgres, MySQL, Redis, MongoDB, 
and Memcached, making the captured traces invaluable for troubleshooting database issues.

In the example below, you can see specific MongoDB queries that exceed the latency objective, and this is 
achieved without any code changes:

<img alt="eBPF-based Tracing" src="/img/docs/tracing_mongo_spans.png" class="card w-1200"/>






