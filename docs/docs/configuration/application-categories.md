---
sidebar_position: 9
---

# Application Categories

Coroot allows you to organize your applications into custom groups called Application Categories. 
These act like scopes, helping you either hide certain applications or focus on specific ones more easily.
Additionally, Application Categories can be used for [notification routing](#notification-routing) — for example, to send alerts to different Slack channels based on category.

<img alt="Application Categories" src="/img/docs/categories.png" class="card w-1200"/>

## Kubernetes annotations

To define a category for a Kubernetes application (Deployment, StatefulSet, DaemonSet, or CronJob),
annotate it with the `coroot.com/application-category` annotation.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: some-app
  namespace: default
  annotations:
    coroot.com/application-category: auxiliary
```

The application category can also be defined using Pod annotations.

## Pattern-based configuration

For non-Kubernetes applications, or in cases where setting annotations is not possible, 
Coroot allows you to configure Application Categories manually by matching applications using patterns.

:::info
Application categories defined via annotations take precedence over those configured manually.
:::

To configure Application Categories, go to the **Project Settings**, click on **Applications**, and adjust the 
built-in categories or create your own custom ones. 
Each category is defined by a set of [glob patterns](https://en.wikipedia.org/wiki/Glob_(programming)) in the `<namespace>/<application_name>` format.

Coroot also includes several pre-defined categories, such as `monitoring` and `control-plane`.

<img alt="Configuring Application Categories" src="/img/docs/categories_configuration.png" class="card w-1200"/>

## Quick links

To make organizing your apps easier, Coroot allows you to define the category for an app directly on the service map:

<img alt="Categories on Service Map" src="/img/docs/category_service_map.png" class="card w-1200"/>

... or application page:

<img alt="Setting Categories from the Application page" src="/img/docs/category_app_page.png" class="card w-1200"/>

## Notification routing

Each category has independent notification settings for three event types: **Incidents** (SLO violations), **Deployments**, and **Alerts** (check-based, log-based, and PromQL-based).

For each event type, you can enable or disable individual integrations (Slack, Microsoft Teams, PagerDuty, OpsGenie, Webhook).
For Slack, you can also override the default channel on a per-category basis.

<img alt="Setting Categories from the Application page" src="/img/docs/category_configuration.png" class="card w-600"/>

When an alert fires for an application, Coroot looks up the application's category and checks whether notifications are enabled for that category. If disabled, the notification is silently skipped.

See [Alerts — Notification routing](/alerting/alerts#notification-routing) for more details.
