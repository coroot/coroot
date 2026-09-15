---
sidebar_position: 5.5
---

# AWS

The AWS integration lets Coroot discover Amazon RDS instances and ElastiCache nodes and collect their telemetry.
Discovered instances appear in the Service Map as separate applications, linked to the services that connect to them
through the eBPF-based metrics collected on the client side.

The integration adds:

* **Instance inventory and status**: engine, version, instance type, storage, Multi-AZ, read replicas, backup retention
* **OS-level metrics** from [RDS Enhanced Monitoring](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Monitoring.OS.html): CPU, memory, disk I/O, network
* **Database logs** of RDS Postgres and MySQL instances (including Aurora), read through the RDS API: shipped to Coroot and grouped by message pattern
* **Cost estimates** for RDS and ElastiCache instances in the [Costs](/costs/overview) report

Database internals (query statistics, locks, replication) are collected separately, see
[Database credentials](#database-credentials).

All AWS API calls are made by the [cluster-agent](/installation/architecture), not by the Coroot server. The agent polls
the RDS and ElastiCache APIs once a minute, so a new instance shows up within about a minute of being created.

To configure the integration, go to the **Project Settings**, click on **AWS**, and fill in the form described below.
The page also shows the discovery status, the last errors reported by the agent, and the list of discovered instances.

## IAM permissions

Whatever identity the agent uses, it needs the following read-only permissions: describing RDS and ElastiCache instances
and their tags, reading RDS log files, and reading the `RDSOSMetrics` CloudWatch Logs group written by Enhanced Monitoring.

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "rds:DescribeDBInstances",
                "rds:DescribeDBLogFiles",
                "rds:DownloadDBLogFilePortion",
                "rds:ListTagsForResource",
                "elasticache:DescribeCacheClusters",
                "elasticache:ListTagsForResource"
            ],
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "logs:GetLogEvents"
            ],
            "Resource": "arn:aws:logs:*:*:log-group:RDSOSMetrics:log-stream:*"
        }
    ]
}
```

## Credentials

The agent supports three ways of authenticating to AWS. Leave the **Access Key ID** and **Secret Access Key** fields
empty for the first two.

| Method | Where the agent runs | Credential lifetime |
|--------|----------------------|---------------------|
| **IAM role of the pod** (EKS Pod Identity or IRSA) | Amazon EKS | Temporary, rotated automatically |
| **EC2 instance profile** | An EC2 instance outside EKS | Temporary, rotated automatically |
| **Static access key** | Anywhere, including outside AWS | Long-lived |

With the keys empty the agent uses the AWS SDK's default credential chain: environment variables, EKS Pod Identity,
IRSA, shared config files, and finally the EC2 instance profile. Binding an IAM role to the cluster-agent on EKS is
described step by step in the [Monitoring Amazon RDS and ElastiCache from EKS](/guides/aws-eks-rds) guide.

For a static key, create an [IAM user](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_users_create.html) with
programmatic access, attach the policy above, and enter its Access Key ID and Secret Access Key in the form. Coroot
stores the key in the project settings and passes it to the cluster-agent along with the rest of the configuration.

## Region

Coroot discovers RDS and ElastiCache instances in a single region. Leave the **Region** field empty to use the region
the cluster-agent runs in: the agent takes it from the `AWS_REGION` environment variable, then from the
`topology.kubernetes.io/region` label of the Kubernetes nodes, and finally from the EC2 instance metadata service.
Set the field explicitly to monitor instances in a different region than the one the agent runs in.

## Tag filters

By default every RDS instance and ElastiCache cluster in the region is discovered. Use **RDS tag filters** and
**ElastiCache tag filters** to restrict discovery to instances whose tags match `tag=value` pairs, separated by
commas. [Glob patterns](https://en.wikipedia.org/wiki/Glob_(programming)) are supported in the value part:

```
team=payments,env=prod*
```

An instance is discovered only if all listed tags match.

## Enhanced Monitoring

OS-level metrics come from the `RDSOSMetrics` CloudWatch Logs group, which exists only for instances with
Enhanced Monitoring enabled. Enable it with a granularity of 60 seconds or lower on each instance you want these
metrics for; instances without it still get inventory, status, and log metrics.

## Logs

The cluster-agent tails the log files of RDS instances with the `postgres`, `aurora-postgresql`, `mysql`, `mariadb` and
`aurora-mysql` engines through the RDS API and forwards every message to Coroot as an OpenTelemetry log record, the
same way coroot-node-agent forwards container logs. All log files the RDS API lists for the instance are read: for
Postgres that is the server log, for MySQL the error log plus the slow query and general logs when they are written
to files (`log_output=FILE` in the parameter group). Messages
are grouped by automatically extracted patterns, and multi-line messages are joined. The records carry the
`pattern.hash` attribute and `service.name` set to `/aws/rds/<region>/<DBInstanceIdentifier>`, which Coroot uses to
show them in the **Logs** tab of the RDS application. Log files are polled every 30 seconds, so a record's timestamp
can lag the time in the log line by up to that much.

Log collection needs the `rds:DescribeDBLogFiles` and `rds:DownloadDBLogFilePortion` permissions from the policy
above. Forwarding can be disabled with the cluster-agent's `--collect-aws-logs=false` flag, in which case only the
pattern-based `aws_rds_log_messages_total` metric is collected. ElastiCache logs are not collected.

## Database credentials

The AWS API provides instance status and OS metrics. Query statistics, locks, and replication state come from the
database itself, so the cluster-agent connects to each instance directly:

1. Allow inbound connections from the cluster-agent to the database port in the instance's security group.
2. Create a monitoring user as described in [Postgres](/databases/postgres) or [MySQL](/databases/mysql).
3. Open the instance's application in Coroot, go to the **Postgres** or **MySQL** tab, click **Configure**, and enter
   the credentials. RDS Postgres 15 and later rejects unencrypted connections by default, so set **sslmode** to
   `require`.

ElastiCache Redis, Valkey and Memcached nodes need no credentials unless AUTH is enabled. Enable collection the same way,
from the **Redis** or **Memcached** tab of the node's application.

The `rdsadmin` database that Amazon RDS creates on every Postgres instance rejects all connections and is excluded
from schema and size tracking by default, see [`--exclude-databases`](/configuration/coroot-cluster-agent).

## Configuration as code

When Coroot is deployed by the [Kubernetes Operator](/installation/k8s-operator), the AWS integration and the database
credentials can be declared in the Coroot custom resource instead of the UI. Secrets are referenced, never stored in
the resource, and the settings take precedence over the UI:

```yaml
spec:
  clusterAgent:
    aws:
      rdsTagFilters: {team: payments}
    databases:
      - type: postgres
        rds: my-db
        credentials:
          usernameSecret: {name: my-db-coroot, key: username}
          passwordSecret: {name: my-db-coroot, key: password}
        params: {sslmode: require}
      - type: redis
        elasticache: my-cache
```

The operator renders these into the cluster-agent's [configuration file](/configuration/coroot-cluster-agent#configuration-file),
which can also be used directly for installations without the operator.

## Troubleshooting

The cluster-agent logs the effective region, the credential source, and the IAM identity it runs as whenever the
integration is configured or changed, followed by a discovery summary:

```
AWS integration: region=us-east-1, credentials=CredentialsEndpointProvider, identity=arn:aws:sts::123456789012:assumed-role/coroot-cluster-agent/...
AWS discovery (region=us-east-1): 3 RDS instances, 2 ElastiCache nodes
```

`CredentialsEndpointProvider` means EKS Pod Identity, `WebIdentityCredentials` means IRSA, `EC2RoleProvider` means the
instance profile, and `StaticCredentials` means the key from the integration settings. Discovery errors are shown on
the integration page and exposed as the `aws_discovery_error` metric. The full list of collected metrics is in the
[cluster-agent metrics reference](/metrics/cluster-agent#aws).
