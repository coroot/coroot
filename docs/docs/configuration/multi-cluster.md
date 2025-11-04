---
sidebar_position: 6.1
---

# Multi-cluster

Coroot multi-cluster projects let you combine several existing projects into a single, aggregated view. Use them to monitor the same application that runs across multiple Kubernetes clusters, regions, or data centers without duplicating ingestion pipelines.

<img alt="Prometheus Configuration" src="/img/docs/multi-cluster-settings.png" class="card w-1200"/>

## How multi-cluster projects work

- A multi-cluster project references one or more member projects and reads their metrics, logs, traces, and profiles on demand. It never ingests telemetry directly.
- Prometheus and ClickHouse integrations, as well as API keys, live on the member projects. The corresponding tabs are disabled for a multi-cluster project.
- Members must be regular projects. You cannot nest one multi-cluster project inside another.

## Configure from the UI

1. Open `Settings → Project` and create a new project (or edit an existing one).
2. Enter a unique project name that will represent the aggregated view, for example `prod-global`.
3. In **Member projects**, select the projects you want to aggregate.
4. Save the changes. The new multi-cluster project appears in the project selector and renders a combined view of its members.

When you return to the configuration screen, Coroot shows a banner confirming that the project aggregates telemetry from its members. Because the project no longer receives data directly, the **Project API keys**, **Prometheus**, and **ClickHouse** tabs remain disabled.

## Configure via `configuration.yaml`

```yaml
projects:
  - name: prod-eu
    # regular project definition omitted for brevity

  - name: prod-us
    # regular project definition omitted for brevity

  - name: prod-global
    memberProjects:
      - prod-eu
      - prod-us
```

During startup Coroot creates (or updates) the `prod-global` project and links it to the `prod-eu` and `prod-us` projects. Each member must already exist either in the database or earlier in the same configuration file.

## Configure via Coroot Operator

When you manage Coroot with the Coroot Operator, declare multi-cluster projects in the `spec.projects` section of the `Coroot` custom resource. Set `memberProjects` on the project and omit API keys—the operator recognises it as an aggregated project.

```yaml
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
spec:
  projects:
    - name: prod-eu
      apiKeys:
        - key: ${PROD_EU_API_KEY}
    - name: prod-us
      apiKeys:
        - key: ${PROD_US_API_KEY}
    - name: prod-global
      memberProjects:
        - prod-eu
        - prod-us
```

When the configuration is applied, Coroot automatically creates the multi-cluster view on startup.

## Limitations

- Multi-cluster projects are read-only from a data-ingestion standpoint: they cannot issue API keys or receive telemetry.
- Only non multi-cluster projects can be selected as members.
