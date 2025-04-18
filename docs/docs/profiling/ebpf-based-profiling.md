---
sidebar_position: 2
---

# eBPF-based profiling

Coroot’s agent includes a built-in eBPF-based CPU profiler. It continuously profiles all processes running on a node, 
associates them with container metadata, and sends the results to the collector.

In most cases, the profiler works out of the box with no configuration. 
However, for certain runtimes, additional integration steps can improve symbolization quality.

## Java

For JVM-based applications, accurate stack traces require exposing JIT-compiled symbols. 
Coroot supports this automatically, but the JVM must be started with the following flag:

```bash
-XX:+PreserveFramePointer
```

When this flag is set, Coroot’s agent will detect it and periodically invoke the JVM to dump the perf map file (once per minute). 
This works seamlessly with containerized applications.

## Node.js

Node.js also supports generating perf map files. To enable it, start the Node.js process with the following options:

```bash
--perf-basic-prof-only-functions --interpreted-frames-native-stack --perf-basic-prof
```

With these flags, the Node.js process will maintain the perf map file automatically. 
Coroot’s agent will detect and use it to improve symbolization.

## Disabling profiling for specific applications

To exclude specific applications from eBPF-based profiling, set the following environment variable for the process:

```bash
COROOT_EBPF_PROFILING=disabled
```

Coroot checks the /proc/&lt;pid&gt;/environ file for each process and skips profiling when this variable is set.


