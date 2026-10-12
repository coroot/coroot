---
sidebar_position: 7.5
---

# Amazon ECS on EC2

Coroot supports Amazon ECS clusters with the EC2 launch type.
The node agent runs as an ECS daemon service: ECS runs one agent task on every node (container instance, in ECS terms) of the cluster, including the nodes added later.
Containers are grouped into applications by ECS service, with one instance per task.

This page covers the agent only. Coroot itself runs outside of the ECS cluster, for example on an EC2 instance with [Docker](/installation/docker) or in [Kubernetes](/installation/kubernetes).
The cluster nodes must be able to open TCP connections to it.

:::info
AWS Fargate is not supported: Fargate doesn't allow privileged containers, which the eBPF-based node agent requires.
:::

**Step #1: Register the task definition**

Download the task definition from Coroot's GitHub repository: coroot-node-agent.json.
Replace `COROOT_URL` with the address of your Coroot instance and `API_KEY` with a project API key (Project settings → API keys), then register it:

```bash
curl -fsS https://raw.githubusercontent.com/coroot/coroot/main/deploy/ecs/coroot-node-agent.json | \
  sed -e 's|COROOT_URL|http://coroot.example.com:8080|' -e 's|API_KEY|YOUR_API_KEY|' > coroot-node-agent.json
aws ecs register-task-definition --cli-input-json file://coroot-node-agent.json
```

The task runs in the host network and PID namespaces as a privileged container, mounts the cgroup and tracing file systems of the host, and keeps its write-ahead log in a Docker volume on the node.

**Step #2: Create a daemon service**

```bash
aws ecs create-service --cluster CLUSTER_NAME --service-name coroot-node-agent \
  --task-definition coroot-node-agent --launch-type EC2 --scheduling-strategy DAEMON
```

**Step #3: Validate the deployment**

The running count must match the number of nodes in the cluster:

```bash
aws ecs describe-services --cluster CLUSTER_NAME --services coroot-node-agent \
  --query 'services[0].[desiredCount,runningCount]'
```

The agent log is available on any node:

```bash
docker logs $(docker ps -q --filter name=ecs-coroot-node-agent)
```

The ECS services of the cluster appear in Coroot a minute after the agents start.

**Upgrade the agent**

The task definition references the `latest` image tag, so a new deployment pulls the latest version on every instance:

```bash
aws ecs update-service --cluster CLUSTER_NAME --service coroot-node-agent --force-new-deployment
```

To change the agent configuration, register a new revision of the task definition and point the service to it:

```bash
aws ecs register-task-definition --cli-input-json file://coroot-node-agent.json
aws ecs update-service --cluster CLUSTER_NAME --service coroot-node-agent --task-definition coroot-node-agent
```

**Uninstall the agent**

```bash
aws ecs delete-service --cluster CLUSTER_NAME --service coroot-node-agent --force
```
