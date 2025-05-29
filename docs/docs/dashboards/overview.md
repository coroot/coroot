---
sidebar_position: 1
---

# Overview

Coroot is an opinionated observability solution out of the box. 
This means it comes with a wide range of predefined inspections and dashboards that help you quickly identify and troubleshoot common issues without any manual configuration.

However, every environment is unique, and sometimes you need to go beyond the built-in views. 
That's where custom dashboards come in. Coroot allows you to create your own dashboards to visualize any metrics that matter to you, turning Coroot into a true single pane of glass for all your observability needs.

Whether you're tracking business KPIs, third-party metrics, or application-specific performance indicators, custom dashboards help you extend Coroot's built-in capabilities.

To learn how to gather custom metrics in Coroot, follow this [guide](/metrics/custom-metrics).

This page will walk you through how to create a new dashboard, add and organize panels, and build effective dashboards tailored to your environment.

Let's get started.

## Create a dashboard

1. Navigate to **Dashboards** and click **Add dashboard**.
2. Provide a name for your dashboard and, optionally, a description.
   <img alt="Coroot Dashboards - Create Dashboard" src="/img/docs/dashboards/dashboard-create.png" class="card w-1200"/>
3. Click **Save**.

## Add a panel

:::info
Currently, only the `Time series chart` panel type is supported.
:::

1. Click **Add panel**.
2. Enter a **Name** and optionally a **Description**.
3. Choose or create a panel **Group**.
4. Enter a PromQL expression in the **Query #1** field.
5. Optionally, provide a **Legend** for the query. You can reference label values using the format `{{ label_name }}`.
   <img alt="Coroot Dasboards - Add Panel" src="/img/docs/dashboards/panel-add.png" class="card w-1200"/>
6. You can add additional PromQL queries if needed.
7. Click **Apply**.
8. Adjust the panel’s size by dragging its bottom-right corner, and move it by dragging the top-right corner.
   <img alt="Coroot Dashboards - Save Dashboard" src="/img/docs/dashboards/dashboard-save.png" class="card w-1200"/>
9. Click **Save** to save the dashboard.

## Panel groups

Panel groups let you organize related panels under a shared title. 
They make it easier to keep things tidy and focus on specific parts of your system, such as resource usage, database metrics, or custom business indicators.

You can collapse groups by default to reduce visual clutter, which is especially useful in larger dashboards. 
Groups are easy to reorder with `↑` and `↓` buttons, and you can move panels between them whenever needed to keep everything organized.
<img alt="Coroot Dashboards - Panel Groups" src="/img/docs/dashboards/groups.png" class="card w-1200"/>

## Dashboard Permissions
Dashboards in Coroot follow role-based access control (RBAC). 
In the Community edition, only Admins and Editors can create or edit dashboards. Viewers can access all dashboards in read-only mode.

The [Enterprise edition](https://coroot.com/enterprise/) offers more control with fine-grained permissions. 
You can define who can view or edit specific dashboards, making it easy to manage access across teams and environments.