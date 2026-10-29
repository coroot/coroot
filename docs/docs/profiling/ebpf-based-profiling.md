---
sidebar_position: 2
---

# eBPF-based profiling

Coroot’s agent includes a built-in eBPF-based profiler. It continuously profiles all processes running on a node, 
associates them with container metadata, and sends the results to the collector.
It collects CPU profiles and [disk I/O profiles](#disk-io-profiling) from the same stack traces.

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

For richer Java profiling (CPU without `PreserveFramePointer`, memory allocations, and lock contention),
see [Java profiling with async-profiler](/profiling/java-profiling).

## Node.js

Node.js also supports generating perf map files. To enable it, start the Node.js process with the following options:

```bash
--perf-basic-prof-only-functions --interpreted-frames-native-stack
```

With these flags, the Node.js process will maintain the perf map file automatically. 
Coroot’s agent will detect and use it to improve symbolization.

## Disk I/O profiling

Besides CPU time, the agent attributes block device I/O to the code paths that issue it.
Every I/O profile carries three values per stack trace:

* **Disk I/O time**: the time the block requests issued by this code path spent between submission and completion,
  summed over all requests. Ten requests in flight for 1ms each add 10ms, the same way CPU time adds up across threads.
* **Disk read bytes** and **Disk write bytes**: the amount of data the code path read from or wrote to block devices.

The profiles are available in the **Profiling** tab under the **I/O** category, and the **Storage** section of the
application page links to them. The chart above the flame graph shows the I/O load, reads or writes of the volumes
the application's instances use, so a spike on the chart can be matched with the code path behind it.

<img alt="Disk I/O time profile" src="/img/docs/profiling/io-profile.png" class="card w-1200"/>

The screenshot shows a MySQL cluster whose writer node shares its data volume with another workload that saturates it.
The incident is selected on the chart and compared with the period before it, for the affected instance `mysql-pxc-0`.
Red frames grew, green frames shrank: the I/O time of `fdatasync` in InnoDB's doublewrite completion path is up by 40%,
while the time spent by the client threads committing transactions is down by 48%, since fewer transactions get through.
The slow volume stalls page flushing, not the queries themselves.

### How it works

The agent attaches eBPF programs to the kernel's block layer entry point (`submit_bio`) and completion point (`bio_endio`).
At submission it captures the user-space stack trace of the process issuing the request, its size and direction, and at
completion it adds the elapsed time to that stack. This works for every way an application can reach the disk:
synchronous reads and writes with `O_DIRECT`, page cache misses, memory-mapped file faults, `fsync` and `fdatasync`,
`libaio` and `io_uring` submissions.

Because the request is attributed to the process that submits it, some I/O lands on kernel threads instead of the
application:

* Buffered writes are written back later by the kernel's writeback threads, so a write-heavy application that never
  calls `fsync` shows little I/O time. Its `fsync` calls, however, are attributed correctly.
* Journal commits of ext4 and XFS are issued by the journaling threads.
* `io_uring` with `SQPOLL` and its worker threads submit on behalf of the application, so those requests show up with
  an unresolved stack.

Discards, flushes and write-zeroes requests are not counted as data.

### Overhead

The profiler samples the block layer instead of recording every request. On each CPU, at most one request per
millisecond is recorded, weighted by the number of requests seen since the previous sample, so the reported totals stay
exact while the cost per request stays flat. A request that is not sampled costs about 100ns at submission plus a map
lookup at completion, around 1.6% of one CPU core at 700,000 IOPS.

We measured the impact with `pgbench` on PostgreSQL: a select-only workload at 74,000 transactions per second lost
between 0 and 1.6% of throughput, and a TPC-B workload at 1,900 transactions per second showed no measurable change.
A `fio` `io_uring` workload at 700,000 IOPS showed no measurable change either. The agent's memory usage grows by about 1MB.

### Disabling disk I/O profiling

Disk I/O profiling is enabled by default. To disable it while keeping CPU profiling, start the agent with the
`--ebpf-io-profiling=false` flag or set the `EBPF_IO_PROFILING=false` environment variable.

## Disabling profiling for specific applications

To exclude specific applications from eBPF-based profiling, set the following environment variable for the process:

```bash
COROOT_EBPF_PROFILING=disabled
```

Coroot checks the /proc/&lt;pid&gt;/environ file for each process and skips profiling when this variable is set.


