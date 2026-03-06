---
sidebar_position: 3
---

# Using S3 Storage with ClickHouse

This guide walks through configuring ClickHouse to use S3-compatible object storage for Coroot's telemetry data (logs, traces, profiles, and metrics).

## Overview

By default, ClickHouse stores all data on local disks. With S3 storage enabled, you can:

- **Reduce storage costs** by moving older data to cheaper object storage
- **Scale storage independently** from compute — no need to resize PVCs
- **Store more data** without being limited by local disk capacity

Coroot's operator supports two S3 storage modes:

| Mode | Description | Best for |
|------|-------------|----------|
| **Tiered** | Recent data on local SSD, older data automatically moved to S3 | Best query performance on recent data with cost-effective long-term storage |
| **S3-only** | All data on S3, local disk used only for caching | Minimal local storage requirements, cost optimization |

## Prerequisites

- Coroot deployed via the [Kubernetes Operator](/installation/k8s-operator)
- An S3-compatible bucket (AWS S3, MinIO, Ceph, etc.)
- S3 credentials (access key + secret key) or IAM/IRSA configured

## Step 1: Create an S3 bucket

Create a dedicated bucket for ClickHouse data. Do not share this bucket with other applications — ClickHouse manages its own file lifecycle, and external lifecycle policies can cause data loss.

```bash
aws s3 mb s3://my-coroot-clickhouse --region us-east-1
```

## Step 2: Create the credentials secret

```bash
kubectl create secret generic clickhouse-s3-creds \
  --from-literal=access_key_id=YOUR_ACCESS_KEY \
  --from-literal=secret_access_key=YOUR_SECRET_KEY \
  -n coroot
```

:::tip
If your Kubernetes cluster uses **IAM Roles for Service Accounts (IRSA)** or **workload identity**, you can skip creating the secret and omit the `credentials` section entirely. ClickHouse is configured to resolve credentials from environment variables and instance metadata automatically.
:::

## Step 3: Configure the Coroot CR

### Tiered mode (recommended)

Keeps recent data on fast local disks for best query performance. Older data is automatically moved to S3 when local disk usage exceeds the threshold.

```yaml
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec:
  communityEdition:
  nodeAgent:
  clusterAgent:
  clickhouse:
    shards: 1
    replicas: 2
    storage:
      size: 100Gi          # local disk per replica
    s3:
      endpoint: https://s3.us-east-1.amazonaws.com/my-coroot-clickhouse/
      region: us-east-1
      credentials:
        accessKeyId:
          name: clickhouse-s3-creds
          key: access_key_id
        secretAccessKey:
          name: clickhouse-s3-creds
          key: secret_access_key
      cacheSize: 10Gi       # local cache for S3 reads
      mode: tiered
      moveFactor: "0.1"     # move data to S3 when <10% free space
```

With these settings and 100Gi local disk, ClickHouse will start moving the oldest data to S3 when less than 10Gi of free space remains on local disk.

### S3-only mode

All data is stored on S3. Local disk is used only for caching reads and temporary merge operations. This minimizes local storage requirements.

```yaml
    s3:
      endpoint: https://s3.us-east-1.amazonaws.com/my-coroot-clickhouse/
      region: us-east-1
      credentials:
        accessKeyId:
          name: clickhouse-s3-creds
          key: access_key_id
        secretAccessKey:
          name: clickhouse-s3-creds
          key: secret_access_key
      cacheSize: 20Gi       # larger cache recommended for s3only mode
      mode: s3only
```

In S3-only mode, you can reduce the `storage.size` since local disk is only needed for cache and temporary operations.

## Step 4: Apply the configuration

```bash
kubectl apply -f coroot.yaml
```

The operator will update the ClickHouse StatefulSets with the S3 storage configuration. Pods will be restarted to pick up the new config.

## How it works

### Data isolation

Each ClickHouse shard and replica writes to a unique S3 path prefix. Even replicas within the same shard get separate paths. ClickHouse does have a "zero-copy replication" feature that allows replicas to share S3 objects, but it is **experimental and disabled by default since version 22.8** due to [known data corruption issues](https://github.com/ClickHouse/ClickHouse/issues/45346). The operator explicitly disables it and uses separate S3 paths per replica to ensure data safety.

Paths follow this pattern:

```
s3://my-coroot-clickhouse/{shard}/{replica}/
```

For example, with 2 shards and 2 replicas:
- `s3://my-coroot-clickhouse/shard-0/coroot-clickhouse-shard-0-0/`
- `s3://my-coroot-clickhouse/shard-0/coroot-clickhouse-shard-0-1/`
- `s3://my-coroot-clickhouse/shard-1/coroot-clickhouse-shard-1-0/`
- `s3://my-coroot-clickhouse/shard-1/coroot-clickhouse-shard-1-1/`

### Caching

A local cache layer sits between ClickHouse and S3. Frequently accessed data is cached on local disk, reducing S3 API calls and read latency. The cache also pre-populates on writes (`cache_on_write_operations`), so recently written data is immediately available from cache.

### Space Manager

When S3 storage is configured, Coroot's [Space Manager](/configuration/clickhouse#space-manager) is automatically disabled. Instead of deleting old partitions to free disk space, ClickHouse moves data to S3, preserving it for the full TTL period.

## Using MinIO or other S3-compatible storage

Any S3-compatible storage can be used. Simply set the `endpoint` to your storage URL:

```yaml
    s3:
      endpoint: https://minio.example.com/coroot-clickhouse/
      credentials:
        accessKeyId:
          name: minio-creds
          key: access_key_id
        secretAccessKey:
          name: minio-creds
          key: secret_access_key
```

## Troubleshooting

### Verify S3 disks are configured

Connect to a ClickHouse pod and run:

```sql
SELECT name, path, type, free_space, total_space FROM system.disks
```

You should see `ObjectStorage` type disks for `s3_disk` and `s3_cache` alongside the `Local` default disk.

### Verify the storage policy

```sql
SELECT * FROM system.storage_policies
```

You should see an `s3_tiered` or `s3_s3only` policy depending on your mode.

### Check data distribution across disks

```sql
SELECT disk_name, sum(bytes_on_disk) as bytes
FROM system.parts
WHERE active = 1
GROUP BY disk_name
```
