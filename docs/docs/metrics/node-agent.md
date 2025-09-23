---
sidebar_position: 1
toc_max_heading_level: 2
---

# Node-agent

This page describes metrics gathered by [coroot-node-agent](https://github.com/coroot/coroot-node-agent).

Each container metric has the `container_id` label. This is a compound identifier and its format varies between container types, e.g.,
`/docker/upbeat_borg`, `k8s/namespace-1/pod-2/container-3` or `/system.slice/nginx.service`.

## CPU

### container_resources_cpu_limit_cores
* **Description**: CPU limit of the container
* **Type**: Gauge
* **Source**: [CPU](https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt) cgroup, the `cpu.cfs_quota_us` and `cpu.cfs_period_us` files

### container_resources_cpu_usage_seconds_total
* **Description**: Total CPU time consumed by the container
* **Type**: Counter
* **Source**: [CPU accounting](https://www.kernel.org/doc/Documentation/cgroup-v1/cpuacct.txt) cgroup, the `cpuacct.usage` file

### container_resources_cpu_throttled_seconds_total
* **Description**: Total time duration the container has been throttled for
* **Type**: Counter
* **Source**: [CPU cgroup](https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt), the `cpu.stat` file

### container_resources_cpu_delay_seconds_total
* **Description**: Total time duration the container has been waiting for a CPU (while being runnable)
* **Type**: Counter
* **Source**:  [Delay accounting](https://www.kernel.org/doc/html/latest/accounting/delay-accounting.html)

### container_resources_cpu_pressure_waiting_seconds_total
* **Description**: Total time in seconds that the container were delayed due to CPU pressure
* **Type**: Counter
* **Source**: [PSI](https://www.kernel.org/doc/html/latest/accounting/psi.html) (Pressure Stall Information)
* **Labels**: kind

## Memory

### container_resources_memory_limit_bytes
* **Description**: Memory limit of the container
* **Type**: Gauge
* **Source**: [Memory](https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt) cgroup, file `memory.limit_in_bytes`

### container_resources_memory_rss_bytes
* **Description**: Amount of physical memory used by the container (doesn't include page cache)
* **Type**: Gauge
* **Source**: [Memory](https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt) cgroup, file `memory.stats`

### container_resources_memory_cache_bytes
* **Description**: Amount of page cache memory allocated by the container
* **Type**: Gauge
* **Source**: [Memory](https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt) cgroup, file `memory.stats`

### container_oom_kills_total
* **Description**: Total number of times the container has been terminated by the OOM killer
* **Type**: Counter
* **Source**: eBPF: tracepoint/oom/mark_victim

### container_resources_memory_pressure_waiting_seconds_total
* **Description**: Total time in seconds that the container were delayed due to memory pressure
* **Type**: Counter
* **Source**: [PSI](https://www.kernel.org/doc/html/latest/accounting/psi.html) (Pressure Stall Information)
* **Labels**: kind

## Disk

### container_resources_disk_delay_seconds_total
* **Description**:  Total time duration the container has been waiting for I/Os to complete
* **Type**: Counter
* **Source**: [Delay accounting](https://www.kernel.org/doc/html/latest/accounting/delay-accounting.html)

### container_resources_io_pressure_waiting_seconds_total
* **Description**: Total time in seconds that the container were delayed due to I/O pressure
* **Type**: Counter
* **Source**: [PSI](https://www.kernel.org/doc/html/latest/accounting/psi.html) (Pressure Stall Information)
* **Labels**: kind

### container_resources_disk_size_bytes
* **Description**: Total capacity of the volume
* **Type**: Gauge
* **Source**: [statfs()](https://man7.org/linux/man-pages/man2/statfs.2.html)
* **Labels**:
   * **mount_point** - path in the mount namespace of the container
   * **device** - device name, e.g., `vda`, `nvme1n1`
   * **volume** - [Persistent Volume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (Kubernetes only)
  
### container_resources_disk_used_bytes
* **Description**: Used capacity of the volume.
* **Type**: Gauge
* **Source**: [statfs()](https://man7.org/linux/man-pages/man2/statfs.2.html)
* **Labels**: mount_point, device, volume


### container_resources_disk_(reads|writes)_total
* **Description**: Total number of reads or writes completed successfully by the container
* **Type**: Counter
* **Source**: [Blkio](https://www.kernel.org/doc/Documentation/cgroup-v1/blkio-controller.txt) cgroup, the `blkio.throttle.io_serviced` file
* **Labels**: mount_point, device, volume

### container_resources_disk_(read|written)_bytes_total
* **Description**: Total number of bytes read from the disk or written to the disk by the container
* **Type**: Counter
* **Source**: [Blkio](https://www.kernel.org/doc/Documentation/cgroup-v1/blkio-controller.txt) cgroup, the `blkio.throttle.io_service_bytes` file
* **Labels**: mount_point, device, volume

## GPU

### container_resources_gpu_usage_percent
* **Description**: Percent of GPU compute resources used by the container
* **Type**: Gauge
* **Source**: NVIDIA Management Library (NVML)
* **Labels**: gpu_uuid

### container_resources_gpu_memory_usage_percent
* **Description**: Percent of GPU memory used by the container
* **Type**: Gauge
* **Source**: NVIDIA Management Library (NVML)
* **Labels**: gpu_uuid

## Network

### container_net_tcp_listen_info
* **Description**: A TCP listen address of the container
* **Type**: Gauge
* **Source**: eBPF: `tracepoint/sock/inet_sock_set_state`, `/proc/<pid>/net/tcp`, `/proc/<pid>/net/tcp6`
* **Labels**: listen_addr (ip:port), proxy

### container_net_tcp_successful_connects_total
* **Description**: Total number of successful TCP connection attempts
* **Type**: Counter
* **Source**: eBPF: `tracepoint/sock/inet_sock_set_state`
* **Labels**: `destination`, `actual_destination`. The IP and port of the connection’s destination. For example, a container might be establishing a connection to port 80 of a Kubernetes Service IP (e.g., 10.96.1.1). This destination address may be translated by iptables to the actual Pod IP (e.g., 10.40.1.5). In this case, the actual_destination would be 10.40.1.5:80.

### container_net_tcp_retransmits_total
* **Description**: Total number of retransmitted TCP segments. This metric is collected only for outbound TCP connections.
* **Type**: Counter
* **Source**: eBPF: `tracepoint/tcp/tcp_retransmit_skb`
* **Labels**: `destination`, `actual_destination`

### container_net_tcp_failed_connects_total
* **Description**: Total number of failed TCP connects to a particular endpoint. The agent takes into account only TCP failures, so this metric doesn't reflect DNS errors
* **Type**: Counter
* **Source**: eBPF: `tracepoint/sock/inet_sock_set_state`
* **Labels**: `destination`

### container_net_tcp_active_connections
* **Description**: Number of active outbound connections between the container and a particular endpoint
* **Type**: Gauge
* **Source**: eBPF: `tracepoint/sock/inet_sock_set_state`
* **Labels**: `destination`, `actual_destination`

### container_net_latency_seconds
* **Description**: Round-trip time between the container and a remote IP
* **Type**: Gauge
* **Source**: The agent measures the round-trip time of an ICMP request sent to IP addresses the container is currently working with
* **Labels**: `destination_ip`

### container_net_latency_seconds
* **Description**: Round-trip time between the container and a remote IP
* **Type**: Gauge
* **Source**: The agent measures the round-trip time of an ICMP request sent to IP addresses the container is currently working with
* **Labels**: `destination_ip`

## Application layer protocol metrics

### container_http_requests_total
* **Description**: Total number of outbound HTTP requests made by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_http_requests_duration_seconds_total
* **Description**: Histogram of the response time for each outbound HTTP request
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_postgres_queries_total
* **Description**: Total number of outbound Postgres queries made by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_postgres_queries_duration_seconds_total
* **Description**: Histogram of the response time for each outbound Postgres query
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_redis_queries_total
* **Description**: Total number of outbound Redis queries made by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_redis_queries_duration_seconds_total
* **Description**: Histogram of the response time for each outbound Redis query
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_memcached_queries_total
* **Description**:  Total number of outbound Memcached queries made by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_memcached_queries_duration_seconds_total
* **Description**:  Histogram of the response time for each outbound Memcached query
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_mysql_queries_total
* **Description**: Total number of outbound Mysql queries made by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_mysql_queries_duration_seconds_total
* **Description**: Histogram of the response time for each outbound Mysql query
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_mongo_queries_total
* **Description**: Total number of outbound Mongo queries made by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_mongo_queries_duration_seconds_total
* **Description**: Histogram of the response time for each outbound Mongo query
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_kafka_requests_total
* **Description**: Total number of outbound Kafka requests made by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_kafka_requests_duration_seconds_total
* **Description**: Histogram of the response time for each outbound Kafka request
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_cassandra_queries_total
* **Description**:  Total number of outbound Cassandra queries made by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_cassandra_queries_duration_seconds_total
* **Description**: Histogram of the response time for each outbound Cassandra query
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_rabbitmq_messages_total
* **Description**: Total number of Rabbitmq messages produced or consumed by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`, `method`

### container_nats_messages_total
* **Description**: Total number of NATS messages produced or consumed by the container
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `method`

### container_dubbo_requests_total
* **Description**: Total number of outbound DUBBO requests
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_dubbo_requests_duration_seconds_total
* **Description**: Histogram of the response time for each outbound DUBBO request
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_dns_requests_total
* **Description**: Total number of outbound DNS requests
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `domain`, `request_type`, `status`

### container_dns_requests_duration_seconds_total
* **Description**: Histogram of the response time for each outbound DNS request
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `le`

### container_clickhouse_requests_total
* **Description**: Total number of outbound ClickHouse queries
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_clickhouse_requests_duration_seconds_total
* **Description**: Histogram of the response time for each outbound ClickHouse query
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

### container_zookeeper_requests_total
* **Description**: Total number of outbound ZooKeeper requests
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `status`

### container_zookeeper_requests_duration_seconds_total
* **Description**: Histogram of the response time for each outbound ZooKeeper request
* **Type**: Counter
* **Source**: eBPF
* **Labels**: `destination`, `actual_destination`, `le`

## JVM

Each JVM metric has the `jvm` label which refers to the main class or path to the `.jar` file.

### container_jvm_info
* **Description**: Meta information about the JVM
* **Type**: Gauge
* **Source**: `hsperfdata`
* **Labels**: `jvm`, `java_version`

### container_jvm_heap_size_bytes
* **Description**: Total heap size in bytes
* **Type**: Gauge
* **Source**: `hsperfdata`
* **Labels**: `jvm`

### container_jvm_heap_used_bytes
* **Description**: Used heap size in bytes
* **Type**: Gauge
* **Source**: `hsperfdata`
* **Labels**: `jvm`

### container_jvm_gc_time_seconds
* **Description**: Time spent in the given JVM garbage collector in seconds
* **Type**: Counter
* **Source**: `hsperfdata`
* **Labels**: `jvm`, `gc`

### container_jvm_safepoint_time_seconds
* **Description**: Time the application has been stopped for safepoint operations in seconds
* **Type**: Counter
* **Source**: `hsperfdata`
* **Labels**: `jvm`

### container_jvm_safepoint_sync_time_seconds
* **Description**: Time spent getting to safepoints in seconds
* **Type**: Counter
* **Source**: `hsperfdata`
* **Labels**: `jvm`

## Node.js runtime

### container_nodejs_event_loop_blocked_time_seconds_total
* **Description**: Total time the Node.js event loop spent blocked
* **Type**: Counter
* **Source**: eBPF uprobes

## .NET runtime

Each .NET runtime metric has the `application` label, which allows distinguishing multiple applications within the same container.

### container_dotnet_info
* **Description**:  Meta information about the Common Language Runtime (CLR)
* **Type**: Gauge
* **Source**: .NET diagnostic port
* **Labels**: `application`, `runtime_version`

### container_dotnet_memory_allocated_bytes_total
* **Description**: The number of bytes allocated
* **Type**: Counter
* **Source**: .NET diagnostic port
* **Labels**: `application`

### container_dotnet_exceptions_total
* **Description**: The number of exceptions that have occurred
* **Type**: Counter
* **Source**: .NET diagnostic port
* **Labels**: `application`

### container_dotnet_memory_heap_size_bytes
* **Description**: Total size of the heap generation in bytes
* **Type**: Gauge
* **Source**: .NET diagnostic port
* **Labels**: `application`, `generation`

### container_dotnet_gc_count_total
* **Description**: The number of times GC has occurred for the generation
* **Type**: Counter
* **Source**: .NET diagnostic port
* **Labels**: `application`, `generation`

### container_dotnet_heap_fragmentation_percent
* **Description**: The heap fragmentation
* **Type**: Gauge
* **Source**: .NET diagnostic port
* **Labels**: `application`

### container_dotnet_monitor_lock_contentions_total
* **Description**: The number of times there was contention when trying to take the monitor's lock
* **Type**: Gauge
* **Source**: .NET diagnostic port
* **Labels**: `application`

### container_dotnet_thread_pool_completed_items_total
* **Description**: The number of work items that have been processed in the ThreadPool
* **Type**: Counter
* **Source**: .NET diagnostic port
* **Labels**: `application`

### container_dotnet_thread_pool_queue_length
* **Description**: The number of work items that are currently queued to be processed in the ThreadPool
* **Type**: Gauge
* **Source**: .NET diagnostic port
* **Labels**: `application`

### container_dotnet_thread_pool_size
* **Description**: The number of thread pool threads that currently exist in the ThreadPool
* **Type**: Gauge
* **Source**: .NET diagnostic port
* **Labels**: `application`

## Other

### container_info
* **Description**: Meta information about the container
* **Type**: Gauge
* **Source**: dockerd, containerd
* **Labels**: `image`

### container_restarts_total
* **Description**: Number of times the container has been restarted
* **Type**: Counter
* **Source**: eBPF: `tracepoint/task/task_newtask`, `tracepoint/sched/sched_process_exit`

### container_application_type
* **Description**: Type of application running in the container (e.g., memcached, postgres, mysql)
* **Type**: Gauge
* **Source**: `/proc/<pid>/cmdline` of the processes running within the container
* **Labels**: `application_type`

## Logs

### container_log_messages_total
* **Description**: The number of messages grouped by the automatically extracted repeated patterns
* **Type**: Counter
* **Source**: The container's log. The following logging methods are supported: 
  * stdout/stderr: streams are captured by Dockerd (json file driver) or Containerd (CRI)
  * Journald
  * `/var/log/*`
* **Labels**: 
  * source: `journald`, `stdout/stderr`, or path to the file in the `/var/log` directory.
  * level: < unknown | debug | info | warning | error | critical >
  * pattern_hash: the ID of the automatically extracted repeated pattern
  * sample: a sample message of the group

## Node metrics

### node_resources_cpu_usage_seconds_total
* **Description**: Amount of CPU time spent in each mode
* **Type**: Counter
* **Source**: `/proc/stat`
* **Labels**: mode: < user | nice | system | idle | iowait | irq | softirq | steal >

### node_resources_cpu_logical_cores
* **Description**: Number of logical CPU cores
* **Type**: Counter
* **Source**: `/proc/stat`

### node_resources_memory_total_bytes
* **Description**: Total amount of physical memory
* **Type**: Gauge
* **Source**: `/proc/meminfo`

### node_resources_memory_free_bytes
* **Description**: Amount of unassigned memory
* **Type**: Gauge
* **Source**: `/proc/meminfo`

### node_resources_memory_available_bytes
* **Description**: An estimate of how much memory is available for allocations, without swapping. 
Roughly speaking, this is the sum of the `free` memory and a part of the `page cache` that can be reclaimed.
You can learn more about how this estimate is calculated [here](https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=34e431b0ae398fc54ea69ff85ec700722c9da773)
* **Type**: Gauge
* **Source**: `/proc/meminfo`

### node_resources_memory_cached_bytes
* **Description**: Amount of memory used as [page cache](https://en.wikipedia.org/wiki/Page_cache). The memory used for page cache might be reclaimed on memory pressure. This can increase the number of disk reads
* **Type**: Gauge
* **Source**: `/proc/meminfo`

### node_resources_disk_(reads|writes)_total
* **Description**:  Total number of reads or writes completed successfully.
Any disk has the maximum IOPS it can serve. Below are the reference values for the different storage types:

| Type                          | Max IOPS     |
|-------------------------------|--------------|
| Amazon EBS sc1               | 250          |
| Amazon EBS st1               | 500          |
| Amazon EBS gp2/gp3           | 16,000       |
| Amazon EBS io1/io2           | 64,000       |
| Amazon EBS io2 Block Express | 256,000      |
| HDD                          | 200          |
| SATA SSD                     | 100,000      |
| NVMe SSD                     | 10,000,000   |
* **Type**: Counter
* **Source**: `/proc/diskstats`
* **Labels**: device

### node_resources_disk_(read|written)_bytes_total
* **Description**:  Total number of bytes read from the disk or written to the disk respectively
In additional to the maximum number of IOPS a disk can serve, there is a throughput limit. For example,

| Type                          | Max throughput |
|-------------------------------|----------------|
| Amazon EBS sc1               | 250 MB/s       |
| Amazon EBS st1               | 500 MB/s       |
| Amazon EBS gp2               | 250 MB/s       |
| Amazon EBS gp3               | 1,000 MB/s     |
| Amazon EBS io1/io2           | 1,000 MB/s     |
| Amazon EBS io2 Block Express | 4,000 MB/s     |
| SATA                         | 600 MB/s       |
| SAS                          | 1,200 MB/s     |
| NVMe                         | 4,000 MB/s     |

* **Type**: Counter
* **Source**: `/proc/diskstats`
* **Labels**: device

### node_resources_disk_(read|write)_time_seconds_total
* **Description**:  Total number of seconds spent reading and writing respectively, including queue wait.
To get the average I/O latency, the sum of these two should be normalized by the number of the executed I/O requests.
Below is the reference average I/O latency for the different storage types:

| Type                          | Avg latency             |
|-------------------------------|--------------------------|
| Amazon EBS gp2/gp3/io1/io2   | "single-digit millisecond" |
| Amazon EBS io2 Block Express | "sub-millisecond"        |
| HDD                          | 2–4ms                   |
| NVMe SSD                     | 0.1–0.3ms               |

* **Type**: Counter
* **Source**: `/proc/diskstats`
* **Labels**: device

### node_resources_disk_io_time_seconds_total
* **Description**: Total number of seconds the disk spent doing I/O. It doesn't include queue wait, only service time.
E.g., if the derivative of this metric for a minute interval is 60s, this means that the disk was busy 100% of that interval.
* **Type**: Counter
* **Source**: `/proc/diskstats`
* **Labels**: device

### node_net_received_(bytes|packets)_total
* **Description**: Total number of bytes and packets received
* **Type**: Counter
* **Labels**: interface

### node_net_transmitted_(bytes|packets)_total
* **Description**: Total number of bytes and packets transmitted
* **Type**: Counter
* **Labels**: interface

### node_net_interface_up
* **Description**: Status of the interface (0: down, 1:up)
* **Type**: Gauge
* **Labels**: interface

### node_net_interface_ip
* **Description**: IP address assigned to the interface
* **Type**: Gauge
* **Labels**: interface, ip

### node_net_interface_ip
* **Description**: IP address assigned to the interface
* **Type**: Gauge
* **Labels**: interface, ip

### node_gpu_info
* **Description**: Meta information about the GPU
* **Type**: Gauge
* **Labels**: gpu_uuid, name

### node_resources_gpu_memory_total_bytes
* **Description**: Total memory available on the GPU in bytes
* **Type**: Gauge
* **Labels**: gpu_uuid

### node_resources_gpu_memory_used_bytes
* **Description**: GPU memory currently in use in bytes
* **Type**: Gauge
* **Labels**: gpu_uuid

### node_resources_gpu_memory_utilization_percent_avg
* **Description**: Average GPU memory utilization (percentage) over the collection interval
* **Type**: Gauge
* **Labels**: gpu_uuid

### node_resources_gpu_memory_utilization_percent_peak
* **Description**: Peak GPU memory utilization (percentage) over the collection interval
* **Type**: Gauge
* **Labels**: gpu_uuid

### node_resources_gpu_utilization_percent_avg
* **Description**: Average GPU core utilization (percentage) over the collection interval
* **Type**: Gauge
* **Labels**: gpu_uuid

### node_resources_gpu_utilization_percent_peak
* **Description**: Peak GPU core utilization (percentage) over the collection interval
* **Type**: Gauge
* **Labels**: gpu_uuid

### node_resources_gpu_temperature_celsius
* **Description**: Current temperature of the GPU in Celsius
* **Type**: Gauge
* **Labels**: gpu_uuid

### node_resources_gpu_power_usage_watts
* **Description**: Current power usage of the GPU in watts
* **Type**: Gauge
* **Labels**: gpu_uuid

### node_uptime_seconds
* **Description**: Uptime of the node in seconds
* **Type**: Gauge

### node_info
* **Description**: Meta information about the node
* **Type**: Gauge
* **Labels**: hostname, kernel_version, agent_version

### node_cloud_info
* **Description**: Meta information about the cloud instance
* **Type**: Gauge
* **Source**: The agent detects the cloud provider using [sysfs](https://man7.org/linux/man-pages/man5/sysfs.5.html).
Then it uses cloud-specific metadata services to retrieve additional information about the instance [AWS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html),
[GCP](https://cloud.google.com/compute/docs/metadata/overview), [Azure](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service?tabs=linux),
Hetzner, Scaleway, DigitalOcean, Alibaba.
For unsupported providers, you can use the `--provider`, `--region`, and `--availability-zone` command line arguments of the agent to define the labels manually.
* **Labels**: 
  * provider: < aws | gcp, azure | hetzner | scaleway | digitalocean | alibaba >
  * account_id: `account_id` for AWS, `project_id` for GCP, `subscription_id` for azure
  * instance_id
  * instance_type
  * instance_life_cycle: < on-demand | spot | preemtible > (always empty for Azure instances)
  * region
  * availability_zone
  * availability_zone_id: [ID](https://docs.aws.amazon.com/ram/latest/userguide/working-with-az-ids.html) of the availability zone (AWS only)
  * local_ipv4
  * public_ipv4
