---
sidebar_position: 10
---

# Custom Applications

Coroot groups individual containers into applications using the following approach:

* Kubernetes metadata: Pods are grouped into Deployments, StatefulSets, etc.
* Non-Kubernetes containers: Containers such as Docker containers or Systemd units are grouped into applications by their names. For example, Systemd services named mysql on different hosts are grouped into a single application called mysql.

This default approach works well in most cases. However, since no one knows your system better than you do, 
Coroot allows you to manually adjust application groupings to better fit your specific needs. 
You can match desired application instances by defining [glob patterns](https://en.wikipedia.org/wiki/Glob_(programming)) for `instance_name`. Note that this is not applicable to Kubernetes applications.

For example, if you have 10 Apache HTTPD instances running on 10 nodes as systemd services, 
Coroot typically groups them into one application by their unit name. 
If this grouping isn't accurate for your setup, you can create custom applications and define the instance mapping to better match your system design.

To configure Custom Application, go to the **Project Settings**, and click on **Applications**.

<img alt="Configuring Custom Applications" src="/img/docs/custom_apps.png" class="card w-1200"/>

## Quick links
To make organizing your apps easier, Coroot allows you to create or configure the custom application for an app directly on the application page:

<img alt="Configuring Custom Applications on the Application page" src="/img/docs/custom_apps_app_page.png" class="card w-1200"/>

