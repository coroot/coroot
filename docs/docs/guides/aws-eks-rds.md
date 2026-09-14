---
sidebar_position: 5
---

# Monitoring Amazon RDS and ElastiCache from EKS

This guide connects Coroot running on Amazon EKS to your RDS and ElastiCache instances. The cluster-agent
authenticates to AWS with an IAM role bound to its pod, and database credentials come from a Kubernetes Secret
referenced in the Coroot custom resource. No access keys and no settings in the Coroot UI are needed.

What you get: RDS and ElastiCache instances in the Service Map, linked to the services that connect to them, with
instance status, OS metrics from Enhanced Monitoring, Postgres logs, and database internals such as query statistics
and locks. The integration is described in detail on the [AWS](/configuration/aws) configuration page.

## Prerequisites

- Coroot deployed on EKS via the [Kubernetes Operator](/installation/k8s-operator)
- The `eks-pod-identity-agent` add-on installed in the cluster
- `aws` CLI and `kubectl` configured, with permissions to manage IAM and EKS

Set these variables once in your shell; every command below uses them:

```bash
export AWS_REGION=us-east-1            # the region of the EKS cluster
export CLUSTER_NAME=my-cluster         # the EKS cluster name
export NAMESPACE=coroot                # the namespace of the Coroot custom resource
export ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
```

## Step 1: Create the IAM policy

Read-only access to RDS and ElastiCache descriptions and tags, RDS log files, and the Enhanced Monitoring log group:

```json title="coroot-aws-policy.json"
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

```bash
aws iam create-policy \
  --policy-name CorootMonitoringReadOnly \
  --policy-document file://coroot-aws-policy.json
```

## Step 2: Bind an IAM role to the cluster-agent

The operator creates the cluster-agent's service account as `<Coroot CR name>-cluster-agent`, so for a CR named
`coroot` it is `coroot-cluster-agent`. Create a role for EKS Pod Identity, attach the policy, and associate the role
with that service account:

```bash
cat > trust-policy.json <<'EOF'
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": { "Service": "pods.eks.amazonaws.com" },
            "Action": ["sts:AssumeRole", "sts:TagSession"]
        }
    ]
}
EOF

aws iam create-role \
  --role-name coroot-cluster-agent \
  --assume-role-policy-document file://trust-policy.json

aws iam attach-role-policy \
  --role-name coroot-cluster-agent \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/CorootMonitoringReadOnly

aws eks create-pod-identity-association \
  --cluster-name ${CLUSTER_NAME} \
  --namespace ${NAMESPACE} \
  --service-account coroot-cluster-agent \
  --role-arn arn:aws:iam::${ACCOUNT_ID}:role/coroot-cluster-agent
```

Credentials are injected into pods when they are created, and a new association takes a few seconds to reach the
webhook. Wait briefly, restart the cluster-agent, and confirm the new pod got the credentials endpoint:

```bash
sleep 15
kubectl rollout restart deployment/coroot-cluster-agent -n ${NAMESPACE}
kubectl rollout status deployment/coroot-cluster-agent -n ${NAMESPACE}
kubectl exec deploy/coroot-cluster-agent -n ${NAMESPACE} -- env | grep AWS_CONTAINER_CREDENTIALS_FULL_URI
```

:::note
If the last command prints nothing, the pod was created before the association propagated: run the restart again.
Clusters without the Pod Identity agent can use IRSA instead, see [Using IRSA](#using-irsa) below.
:::

## Step 3: Enable RDS Enhanced Monitoring

OS-level metrics (CPU, memory, disk and network I/O) come from
[Enhanced Monitoring](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Monitoring.OS.html). Enable it on
each instance with a granularity of 60 seconds or lower. The `rds-monitoring-role` is the standard role RDS uses to
publish these metrics; the AWS console creates it when Enhanced Monitoring is first enabled there.

```bash
aws rds modify-db-instance \
  --db-instance-identifier my-db \
  --monitoring-interval 60 \
  --monitoring-role-arn arn:aws:iam::${ACCOUNT_ID}:role/rds-monitoring-role \
  --apply-immediately
```

## Step 4: Configure database credentials

For query statistics, locks and replication state, the cluster-agent connects to each database directly. Allow
connections from the EKS pods in the RDS security group, and create a monitoring user as described in
[Postgres](/databases/postgres) or [MySQL](/databases/mysql). On RDS Postgres, `CREATE EXTENSION pg_stat_statements`
is all that is needed.

Put the credentials in a Secret and reference it from the Coroot custom resource. RDS instances are referenced by
identifier and ElastiCache clusters by id: the agent takes their endpoints from discovery.

```yaml title="rds-coroot-pg-secret.yaml"
apiVersion: v1
kind: Secret
metadata:
  name: rds-coroot-pg
  namespace: coroot
stringData:
  username: coroot
  password: <PASSWORD>
```

```yaml title="coroot.yaml"
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec:
  communityEdition:
  nodeAgent:
  clusterAgent:
    aws:
      rdsTagFilters:                     # optional: discover only instances with matching tags
        team: payments
    databases:
      - type: postgres
        rds: my-db                       # DBInstanceIdentifier
        credentials:
          usernameSecret:
            name: rds-coroot-pg
            key: username
          passwordSecret:
            name: rds-coroot-pg
            key: password
        params:
          sslmode: require               # RDS Postgres 15+ rejects unencrypted connections
      - type: redis
        elasticache: my-cache            # CacheClusterId; no credentials unless AUTH is enabled
  clickhouse:
    storage:
      size: 100Gi
  prometheus:
    storage:
      size: 50Gi
```

```bash
kubectl apply -f rds-coroot-pg-secret.yaml
kubectl apply -f coroot.yaml
```

The operator restarts the cluster-agent with the new configuration. Within a couple of minutes the agent logs the
targets, and the Postgres and Redis tabs of the RDS and ElastiCache applications show data:

```bash
kubectl logs deploy/coroot-cluster-agent -n ${NAMESPACE} | grep "AWS \|new target"
```

```
AWS integration: region=us-east-1, credentials=CredentialsEndpointProvider, identity=arn:aws:sts::123456789012:assumed-role/coroot-cluster-agent/...
AWS discovery (region=us-east-1): 1 RDS instances, 1 ElastiCache nodes
new target: postgres://10.0.12.34:5432 (rds:my-db)
new target: redis://10.0.45.67:6379 (elasticache:my-cache)
```

Settings in the custom resource take precedence over anything configured in the Coroot UI.

## Using IRSA

On clusters without the Pod Identity agent, replace the role creation and association in Step 2 with a role trusted
by the cluster's OIDC provider, and annotate the service account:

```bash
OIDC_PROVIDER=$(aws eks describe-cluster --name ${CLUSTER_NAME} \
  --query "cluster.identity.oidc.issuer" --output text | sed 's|https://||')

cat > trust-policy.json <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": { "Federated": "arn:aws:iam::${ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}" },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "${OIDC_PROVIDER}:aud": "sts.amazonaws.com",
                    "${OIDC_PROVIDER}:sub": "system:serviceaccount:${NAMESPACE}:coroot-cluster-agent"
                }
            }
        }
    ]
}
EOF

aws iam create-role \
  --role-name coroot-cluster-agent \
  --assume-role-policy-document file://trust-policy.json

aws iam attach-role-policy \
  --role-name coroot-cluster-agent \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/CorootMonitoringReadOnly

kubectl annotate serviceaccount coroot-cluster-agent -n ${NAMESPACE} \
  eks.amazonaws.com/role-arn=arn:aws:iam::${ACCOUNT_ID}:role/coroot-cluster-agent

kubectl rollout restart deployment/coroot-cluster-agent -n ${NAMESPACE}
kubectl rollout status deployment/coroot-cluster-agent -n ${NAMESPACE}
kubectl exec deploy/coroot-cluster-agent -n ${NAMESPACE} -- env | grep AWS_WEB_IDENTITY_TOKEN_FILE
```

The operator keeps the annotation across reconciliations. A freshly created role can take up to a minute to become
assumable; the agent's first discovery cycle may log `Not authorized to perform sts:AssumeRoleWithWebIdentity`, and
it recovers on the next cycle without a restart.

## Troubleshooting

The agent's log has everything needed. The `AWS integration` line shows the region, the credential source and the
IAM identity in use:

| `credentials=` | Meaning |
|----------------|---------|
| `CredentialsEndpointProvider` | Pod Identity, as expected after Step 2 |
| `WebIdentityCredentials` | IRSA |
| `EC2RoleProvider` | The pod got no credentials of its own and fell back to the node's instance role. Restart the deployment so the webhook injects them |

- **`AccessDenied` in the discovery line**: the policy is not attached to the role, or the role is not the one bound to
  the service account.
- **No Enhanced Monitoring metrics**: the `RDSOSMetrics` log group only exists once Enhanced Monitoring is enabled on
  at least one instance (Step 3).
- **Instances discovered but no database metrics**: check the security group, that the Secret exists in the CR's
  namespace, and that the user can log in. The agent logs a warning for every database it cannot reach.
- **`failed to discover the AWS region`**: set `AWS_REGION` on the cluster-agent through `spec.clusterAgent.env`.
