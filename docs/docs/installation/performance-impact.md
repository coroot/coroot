---
sidebar_position: 10
---

# Performance Impact

Coroot leverages eBPF to collect telemetry data, such as metrics and traces. 
This approach involves running small observer programs in the kernel space. 
The Linux kernel guarantees that eBPF programs will not significantly interrupt kernel code execution by verifying each program before it runs.

Program must have a finite complexity. The verifier will evaluate all possible execution paths and must be capable of completing the analysis within the limits of the configured upper complexity limit.
At Coroot, we assess the performance of every component to ensure our users enjoy observability without any negative impact. This page presents the benchmark results for Coroot's monitoring agent.

## Lab

We run all tests on an `m5.2xlarge` instance and use a simple Go HTTP server along with `wrk2` to generate the load.

All components are configured to use separate CPU cores, reducing competition for CPU time. 
To achieve this, we use the `--cpuset-cpus` Docker parameter or the taskset utility for non-containerized apps.

### HTTP server
We're not testing the app's performance but rather how capturing requests for observability affects latency. 
We'll use a Go app that responds with a 1KB payload in a constant 5ms.

```go 
package main

import (
  "net/http"
  "bytes"
  "time"
)

var payload = bytes.Repeat([]byte("0"), 1024)

func handler(w http.ResponseWriter, req *http.Request) {
  time.Sleep(5*time.Millisecond)
  w.Write(payload)
}

func main() {
  http.HandleFunc("/", handler)
  http.ListenAndServe(":8090", nil)
}
```

```bash
# CPU cores #2-3
taskset -c 2-3 go run app.go
```

### wrk2 (load generation)

It's important to note that tests for maximum throughput can be impacted by any additional CPU-consuming processes on the node. 
Our approach involves measuring a baseline latency under a fixed number of requests per second (10,000 RPS) and then repeating 
the experiment with the Coroot's agent enabled.

```bash
# threads:4, connections: 100, test duration: 5 minute, CPU cores #4-7
docker run --rm --cpuset-cpus 4-7 -ti cylab/wrk2 -t4 -c100 -d300s -R10000 --u_latency http://172.17.0.1:8090/
```

### coroot-node-agent

```bash
# CPU cores #0-1
docker run -d --name coroot-node-agent \
  --cpuset-cpus 0-1 \
  --privileged --pid host \
  -v /sys/kernel/debug:/sys/kernel/debug:rw \
  -v /sys/fs/cgroup:/host/sys/fs/cgroup:ro \
  ghcr.io/coroot/coroot-node-agent --cgroupfs-root=/host/sys/fs/cgroup
```

## Test Results

![Agent Performance Test](/img/docs/agent_performance_test.png)

The latency difference with and without coroot-node-agent enabled falls within the margin of measurement error. 
During the test the agent consumed 200m CPU (20% of one CPU core).

It's essential to understand that eBPF ensures that the observer program cannot impact kernel operations, 
even during slowdowns caused by factors like CPU resource limitations. In such situations, some events sent from the 
kernel to the agent may be lost due to the limited capacity of the underlying ring buffers. 
In other words, this might result in some statistics not being entirely accurate, but the application performance will not be affected.

## Conclusion

If you are running loads around 10,000 requests per second, you can be confident that Coroot will have no noticeable 
impact on your application's performance or response time. In this scenario, the Coroot agent's CPU consumption will 
be approximately 20% of a single CPU core.

If your workloads are significantly larger, we highly recommend conducting a similar load test. 
The Coroot team is here to assist you with this, please feel free to reach out to us.

