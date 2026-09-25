---
hide_table_of_contents: true
sidebar_position: 20
---

# Conclusion

Every app served the same load, 1,000 requests per second, and every request was one `INCR` call to Valkey.
Here are the [per-language results](/tracing/opentelemetry-overhead) side by side.

## The numbers

CPU usage with the OpenTelemetry SDK on, compared to the same app without it:

| Language                                                          | 100% sampled | 50%    | 20%    | 0% sampled   | Extra CPU cores per 1,000 req/s (100%) |
|-------------------------------------------------------------------|--------------|--------|--------|--------------|-----------------------------------------|
| <img src="/img/docs/tracing/opentelemetry-overhead/rust.svg" class="lang-logo" alt=""/> [Rust](/tracing/opentelemetry-overhead/rust)      | +21%         | +15%   | +7%    | +6%          | 0.009                                   |
| <img src="/img/docs/tracing/opentelemetry-overhead/java.svg" class="lang-logo" alt=""/> [Java](/tracing/opentelemetry-overhead/java) (agent) | +30%      | +25%   | +19%   | +11%         | 0.03                                    |
| <img src="/img/docs/tracing/opentelemetry-overhead/cpp.svg" class="lang-logo" alt=""/> [C++](/tracing/opentelemetry-overhead/cpp)        | +42%         | +33%   | +5%    | +10%         | 0.03                                    |
| <img src="/img/docs/tracing/opentelemetry-overhead/golang.svg" class="lang-logo" alt=""/> [Go](/tracing/opentelemetry-overhead/go) (compile-time) | +35%  | +22%   | +24%   | +13%         | 0.04                                    |
| <img src="/img/docs/tracing/opentelemetry-overhead/golang.svg" class="lang-logo" alt=""/> [Go](/tracing/opentelemetry-overhead/go) (manual SDK) | +50%    | +29%   | +24%   | +20%         | 0.05                                    |
| <img src="/img/docs/tracing/opentelemetry-overhead/ruby.svg" class="lang-logo" alt=""/> [Ruby](/tracing/opentelemetry-overhead/ruby)      | +67%         | +45%   | +28%   | +21%         | 0.16                                    |
| <img src="/img/docs/tracing/opentelemetry-overhead/nodejs.svg" class="lang-logo" alt=""/> [Node.js](/tracing/opentelemetry-overhead/nodejs) | +70%       | +63%   | +57%   | +41%         | 0.09                                    |
| <img src="/img/docs/tracing/opentelemetry-overhead/python.svg" class="lang-logo" alt=""/> [Python](/tracing/opentelemetry-overhead/python) | +160%       | +121%  | +92%   | +74%         | 0.36                                    |
| <img src="/img/docs/tracing/opentelemetry-overhead/dotnet.svg" class="lang-logo" alt=""/> [.NET](/tracing/opentelemetry-overhead/dotnet)  | n/a\*         | n/a\*  | n/a\*  | n/a\*         | n/a\*                                   |

\* .NET: not measurable. The runtime's thread pool spin-waiting varies from run to run more than tracing costs. See the [.NET page](/tracing/opentelemetry-overhead/dotnet).

Memory, latency and network traffic with every request traced:

| Language                                                          | Memory (RSS)    | Latency, p50     | Latency, p99   | Trace export traffic | Bytes per span |
|-------------------------------------------------------------------|-----------------|------------------|----------------|----------------------|----------------|
| <img src="/img/docs/tracing/opentelemetry-overhead/rust.svg" class="lang-logo" alt=""/> [Rust](/tracing/opentelemetry-overhead/rust)      | 4.5 → 7 MB      | 0.58 → 0.58 ms   | 1.6 → 1.5 ms   | 2.9 Mbit/s           | ~180           |
| <img src="/img/docs/tracing/opentelemetry-overhead/cpp.svg" class="lang-logo" alt=""/> [C++](/tracing/opentelemetry-overhead/cpp)        | 4 → 6 MB        | 0.55 → 0.64 ms   | 1.9 → 2.9 ms   | 2.9 Mbit/s           | ~180           |
| <img src="/img/docs/tracing/opentelemetry-overhead/golang.svg" class="lang-logo" alt=""/> [Go](/tracing/opentelemetry-overhead/go)        | 13 → 15 MB      | 0.56 → 0.56 ms   | 1.3 → 1.0 ms   | 5.6 Mbit/s           | ~350           |
| <img src="/img/docs/tracing/opentelemetry-overhead/java.svg" class="lang-logo" alt=""/> [Java](/tracing/opentelemetry-overhead/java)      | 310 → 459 MB    | 0.63 → 0.64 ms   | 1.6 → 1.8 ms   | 6.3 Mbit/s           | ~400           |
| <img src="/img/docs/tracing/opentelemetry-overhead/dotnet.svg" class="lang-logo" alt=""/> [.NET](/tracing/opentelemetry-overhead/dotnet)  | 41 → 75 MB      | 0.58 → 0.61 ms   | 1.3 → 1.5 ms   | 2.8 Mbit/s           | ~280           |
| <img src="/img/docs/tracing/opentelemetry-overhead/ruby.svg" class="lang-logo" alt=""/> [Ruby](/tracing/opentelemetry-overhead/ruby)      | 170 → 231 MB    | 0.76 → 0.79 ms   | 2.7 → 10.9 ms  | 0.7 Mbit/s (gzip)    | ~45            |
| <img src="/img/docs/tracing/opentelemetry-overhead/nodejs.svg" class="lang-logo" alt=""/> [Node.js](/tracing/opentelemetry-overhead/nodejs) | 147 → 688 MB  | 0.59 → 0.72 ms   | 1.3 → 4.0 ms   | 4.7 Mbit/s           | ~290           |
| <img src="/img/docs/tracing/opentelemetry-overhead/python.svg" class="lang-logo" alt=""/> [Python](/tracing/opentelemetry-overhead/python) | 201 → 304 MB   | 0.75 → 1.04 ms   | 2.0 → 13.3 ms  | 5.1 Mbit/s           | ~320           |

## What it means

* **The cost is paid per request, and it varies 40x between languages.** Two spans cost 0.01 ms of CPU time in Rust, 0.03-0.05 ms in C++, Java and Go, 0.09 ms in Node.js, 0.16 ms in Ruby and 0.36 ms in Python.
  Multiply by your request rate and you get the CPU cores you'll need. Our test app does nothing else, so the percentages (+21%...+160%) are the worst case:
  a real application does much more per request, and its relative overhead will be lower.
* **Sampling lowers the cost but doesn't remove it.** At 0% sampling, when not one span leaves the process, the apps still use 6-74% more CPU than without the SDK.
  Instrumented handlers, context creation and propagation, and the sampling decision itself run on every request. Sampling only skips recording, serializing and sending spans.
  Java and Node.js also keep their full memory footprint at 0%.
* **Interpreted runtimes pay the most.** In Python, Ruby and Node.js the instrumentation runs in the same interpreter as your code.
  Python's CPU usage grows 2.6 times, and in all three the tail latency goes up: every span is a handful of objects for the garbage collector.
  On a smaller machine the Python app ran out of CPU with tracing on and stopped keeping up with the load, so leave CPU headroom.
* **Memory is the main cost in Java and Node.js.** The Java agent adds about 150MB whatever the sampling rate, and the Node.js SDK adds 130MB+ per worker process.
* **Compile-time instrumentation for Go is a bit cheaper than wiring the SDK by hand** (+35% vs. +50%), and it needs no code changes.
* **Check that nothing gets dropped.** The batch span processor drops spans silently when its queue is full. It happened to .NET in this benchmark:
  its Redis instrumentation delivers spans in 10-second bursts, and most of them were lost with the default settings. Java lost about 3% of its spans at 100% sampling.
  Compare the spans your backend receives with the requests you serve, and raise `OTEL_BSP_MAX_QUEUE_SIZE` if they don't match.
* **Network traffic is modest**: 180-400 bytes per span, 3-6 Mbit/s for 2,000 spans per second, and it goes down in step with the sampling rate.
  Ruby's exporter compresses with gzip by default and sends only ~45 bytes per span; the others send uncompressed protobuf.

## The alternative: eBPF

Coroot's [eBPF-based tracing](/tracing/ebpf-based-tracing) sees the same requests and Valkey calls from the kernel, with no code in the application.
The app spends no CPU or memory on it, sampling can be changed without a redeploy, and the cost is paid once per node by coroot-node-agent instead of once per process.
It doesn't replace OpenTelemetry where you need in-process context, like custom spans, business attributes or async work.
But for the request/response protocols it understands, it gives you the same request-level visibility for free from the application's point of view.
