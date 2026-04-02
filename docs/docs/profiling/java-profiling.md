---
sidebar_position: 3
---

# Java profiling

Coroot can profile Java applications using [async-profiler](https://github.com/async-profiler/async-profiler),
a low-overhead profiler for HotSpot JVMs. When enabled, it captures CPU, memory allocation, and lock contention
profiles without requiring any changes to the Java application.

## How it works

Coroot's node agent dynamically loads async-profiler into running Java processes using the JVM Attach API.
No JVM flags, no application restarts, and no Java agent JARs are needed.

The profiling lifecycle:

1. **Detection**: The agent discovers HotSpot JVMs by scanning `/proc/<pid>/maps` for `libjvm.so`.
2. **Deployment**: The agent deploys `libasyncProfiler.so` (~600KB) into the container's filesystem at `/tmp/coroot/`.
3. **Start**: The agent loads the native library into the JVM via the attach protocol, starting CPU, allocation, and lock profiling.
4. **Collection**: Every 60 seconds, the agent stops the profiler (finalizing the JFR output file), reads the data, 
   parses it, and immediately starts a new session. The gap is ~4ms.
5. **Upload**: Parsed profiles are uploaded to Coroot in pprof format.

## Supported profile types

| Profile | Description | Event |
|---------|-------------|-------|
| **Java CPU** | CPU time attribution per stack trace | `itimer` (no safepoint bias) |
| **Java Memory (alloc_space)** | Bytes allocated per stack trace | TLAB allocation events |
| **Java Memory (alloc_objects)** | Object count per stack trace | TLAB allocation events |
| **Java Lock (delay)** | Time spent waiting for locks | `JavaMonitorEnter` (threshold: 10ms) |
| **Java Lock (contentions)** | Number of lock contentions | `JavaMonitorEnter` (threshold: 10ms) |

## Enabling

Set the `--enable-java-async-profiler` flag (or `ENABLE_JAVA_ASYNC_PROFILER=true` environment variable) on the node agent.

In the Coroot custom resource:

```yaml
apiVersion: coroot.com/v1
kind: Coroot
spec:
  nodeAgent:
    env:
      ENABLE_JAVA_ASYNC_PROFILER: "true"
```

## Metrics

When async-profiler is enabled, the following Prometheus metrics are exported per JVM:

| Metric | Type | Description |
|--------|------|-------------|
| `container_jvm_alloc_bytes_total` | Counter | Total bytes allocated |
| `container_jvm_alloc_objects_total` | Counter | Total objects allocated |
| `container_jvm_lock_contentions_total` | Counter | Total lock contentions |
| `container_jvm_lock_time_seconds_total` | Counter | Total time waiting for locks |
| `container_jvm_profiling_status` | Gauge | 1 if async-profiler is enabled |

These metrics are derived from the same profiling data and provide time-series visibility into allocation rates
and lock contention. Use them alongside the flamegraph profiles to spot anomalies and drill into the details.

## Compatibility

- **JVM**: HotSpot-based JVMs (OpenJDK, Oracle JDK, Amazon Corretto, etc.). OpenJ9 is not supported.
- **JDK version**: 8+ (11+ recommended)
- **Architecture**: x86_64 and ARM64
- **Privileges**: No special capabilities required (no `perf_event_open`, no `CAP_PERFMON`)

## Conflict detection

If another tool (e.g., Pyroscope Java agent, Datadog) has already loaded async-profiler into a JVM,
Coroot's agent detects this by scanning `/proc/<pid>/maps` and skips that process to avoid conflicts.

## Overhead

- **CPU overhead**: ~1-2% per profiled JVM
- **Memory**: async-profiler uses an in-memory JFR buffer (bounded)
- **Agent-side**: Parsing ~1MB of JFR data takes ~200-400ms per JVM per collection cycle
- **Profiling gap**: ~4ms every 60 seconds (during stop/start transition)
