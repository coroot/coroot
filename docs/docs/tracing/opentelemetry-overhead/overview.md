---
hide_table_of_contents: true
sidebar_position: 1
slug: /tracing/opentelemetry-overhead
---

# OpenTelemetry SDK overhead

Tracing is not free. For every request, the OpenTelemetry SDK creates spans, fills them with attributes, passes the trace context along,
and then serializes the spans and ships them to a backend. How much all of this costs depends a lot on the language, so we measured it.

<div class="lang-cards">
  <a href="/tracing/opentelemetry-overhead/go"><img src="/img/docs/tracing/opentelemetry-overhead/golang.svg" alt=""/>Go</a>
  <a href="/tracing/opentelemetry-overhead/java"><img src="/img/docs/tracing/opentelemetry-overhead/java.svg" alt=""/>Java</a>
  <a href="/tracing/opentelemetry-overhead/python"><img src="/img/docs/tracing/opentelemetry-overhead/python.svg" alt=""/>Python</a>
  <a href="/tracing/opentelemetry-overhead/rust"><img src="/img/docs/tracing/opentelemetry-overhead/rust.svg" alt=""/>Rust</a>
  <a href="/tracing/opentelemetry-overhead/dotnet"><img src="/img/docs/tracing/opentelemetry-overhead/dotnet.svg" alt=""/>.NET</a>
  <a href="/tracing/opentelemetry-overhead/nodejs"><img src="/img/docs/tracing/opentelemetry-overhead/nodejs.svg" alt=""/>Node.js</a>
  <a href="/tracing/opentelemetry-overhead/ruby"><img src="/img/docs/tracing/opentelemetry-overhead/ruby.svg" alt=""/>Ruby</a>
  <a href="/tracing/opentelemetry-overhead/cpp"><img src="/img/docs/tracing/opentelemetry-overhead/cpp.svg" alt=""/>C++</a>
</div>

Every language went through the same test, so the results are comparable. The [conclusion](/tracing/opentelemetry-overhead/conclusion) puts them side by side.

## How we measured

### The app

We wrote the same tiny service in every language, using the stack most people would pick for it: an HTTP server with one endpoint.
The endpoint runs `INCR counter` in [Valkey](https://valkey.io/) and returns the number. That's all it does.

This makes the overhead as visible as it can be. A real application does much more work per request, so its *relative* overhead will be lower than what you see here.

Tracing is switched on with an environment variable, so the very same build runs with and without the SDK:

* we used what a typical developer would use: the official zero-code agent where there is one (Java, Python, Node.js) or the official SDK with its instrumentation libraries. Rust and C++ have no instrumentation for their HTTP servers and Redis clients, so there the two spans are created by hand with the SDK API;
* traces only: OpenTelemetry metrics and logs are off;
* every request produces **exactly two spans**: a `SERVER` span for the HTTP request and a `CLIENT` span for the Valkey call. Where an instrumentation adds more spans by default, we turned them off;
* spans go to Coroot over OTLP/HTTP (protobuf) through the batch span processor with its default settings.

### Five modes

Each app serves the same load, **1,000 requests per second** over 100 keep-alive connections, in five modes:

| Mode | OpenTelemetry SDK | What is traced |
|------|-------------------|----------------|
| off  | not loaded        | nothing        |
| 100% | on                | every request (the default sampler) |
| 50%  | on                | `OTEL_TRACES_SAMPLER=parentbased_traceidratio`, `OTEL_TRACES_SAMPLER_ARG=0.5` |
| 20%  | on                | `OTEL_TRACES_SAMPLER_ARG=0.2` |
| 0%   | on                | `OTEL_TRACES_SAMPLER_ARG=0`: the SDK is active, but not a single span is recorded or sent |

The 0% mode is the interesting one. It shows what the instrumentation itself costs, before any span is even created.

For each mode we start the app from scratch, warm it up for 90 seconds, and then measure for 4 minutes.
To make sure the results are reproducible, we go through all five modes twice: the numbers in the tables are the average of the two passes.
The charts show only the second pass, to keep them readable.

### The lab

Four virtual machines in one private network, so nothing competes for resources:

* the app: 4 dedicated vCPUs, 16GB RAM;
* Valkey 8: 2 dedicated vCPUs;
* the load generator: [wrk2](https://github.com/giltene/wrk2), which holds the request rate constant (`-t4 -c100 -R1000`);
* Coroot, which receives the traces and does the measuring.

CPU and memory of the app come from the container metrics collected by coroot-node-agent. Network traffic is measured on the app node.
We turned off coroot-node-agent's own eBPF tracing on the app node, so it doesn't get in the way.
Latency comes from wrk2, and we count the spans in Coroot's storage to make sure none got lost on the way.

## Results

* [Go](/tracing/opentelemetry-overhead/go)
* [Java](/tracing/opentelemetry-overhead/java)
* [Python](/tracing/opentelemetry-overhead/python)
* [Rust](/tracing/opentelemetry-overhead/rust)
* [.NET](/tracing/opentelemetry-overhead/dotnet)
* [Node.js](/tracing/opentelemetry-overhead/nodejs)
* [Ruby](/tracing/opentelemetry-overhead/ruby)
* [C++](/tracing/opentelemetry-overhead/cpp)
* [Conclusion](/tracing/opentelemetry-overhead/conclusion): all of them side by side
