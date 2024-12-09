---
sidebar_position: 16
---

# Deployment tracking

Coroot automatically discovers and monitors every application rollout in your Kubernetes cluster.

<img alt="Deployments" src="/img/docs/deployments.png" class="card w-1200"/>

This inspection uses metrics gathered by `kube-state-metrics` to detect that an application (Kubernetes Deployment) has been updated.
Using Kubernetes metadata to detect rollouts has two main advantages.
Firstly, this is the most up-to-date information about the state of applications.
Secondly, this approach eliminates the need to integrate Coroot with CI/CD tools.

<img alt="Deployments Rollout Detection" src="/img/docs/deployments-rollout-detection.png" class="card w-800"/>

Upon identifying a new rollout, Coroot immediately starts monitoring its status.
The possible states of a rollout are as follows:

<img alt="Deployments Rollout Statuses" src="/img/docs/deployments-rollout-statuses.svg" class="card w-800"/>

A rollout is considered "Stuck" if it lasts for more than 3 minutes (the threshold can be easily adjusted for any application or an entire project).
This can occur due to issues related to container images or if a new pod cannot be scheduled on a cluster node.


Coroot audits each application 30 minutes after its new version is deployed to evaluate its performance in comparison to the previous version, using the following criteria:

* SLOs (Availability & Latency)
* The number of errors in the application logs
* Container restarts
* CPU consumption
* Memory leaks

Notifications of all notable changes are sent to your Slack workspace to ensure that you don't miss even the slightest performance degradation.

<img alt="Deployments Slack" src="/img/docs/deployments-slack.png" class="card w-800"/>

You can turn off such notifications for any particular [application category](/configuration/application-categories):

<img alt="Deployments disable notifications" src="/img/docs/deployments-disable-notification.png" class="card w-600"/>

