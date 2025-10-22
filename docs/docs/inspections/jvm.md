---
sidebar_position: 8
---

# JVM

The following inspections are based on the JVM metrics automatically collected by `coroot-node-agent` for each JVM running on the node. 
So, you don't need to configure anything!

* JVM availability: checks that every JVM is up and running
* JVM safepoints: detects situations when a java application has been stopped for a significant amount of time due to safepoint operations

<img alt="JVM" src="/img/docs/jvm.png" class="card w-1200"/>

# Enhanced continuous profiling
For continuous profiling, Coroot uses eBPF as well, capturing execution paths at the kernel level with minimal overhead. However, because the JVM relies heavily on JIT compilation, the generated native code does not include symbolic information by default. In other words, we see memory addresses, but not the actual method names. To make the profiles readable and useful, the JVM needs to expose symbol information. This can be generated using the `jcmd <pid> Compiler.perfmap` command.

Coroot automates this step by periodically calling jcmd in the background. However, the JVM must be started with the `-XX:+PreserveFramePointer` option. This allows for accurate stack traces and proper symbolization of JIT-compiled code, with only a small performance overhead (typically around 1-3%).
