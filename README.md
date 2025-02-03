<img width="200" src="https://coroot.com/static/logo_u.png">

![](https://github.com/coroot/coroot/actions/workflows/ci.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/coroot/coroot)](https://goreportcard.com/report/github.com/coroot/coroot)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![](https://img.shields.io/badge/slack-coroot-brightgreen.svg?logo=slack)](https://join.slack.com/t/coroot-community/shared_invite/zt-1gsnfo0wj-I~Zvtx5CAAb8vr~r~vecyw)

### [Features](#features) | [Installation](https://docs.coroot.com/) | [Documentation](https://docs.coroot.com/) | [Community & Support](#community--support) | [Live demo](https://demo.coroot.com/) | [Coroot Enterprise](https://coroot.com/enterprise/) 


## Open-source observability augmented with actionable insights

Collecting metrics, logs, and traces alone doesn't make your applications observable.
Coroot turns that data into actionable insights for you!

## Features

### Zero-instrumentation observability

* Metrics, logs, traces, and profiles are gathered automatically by using eBPF
* Coroot provides you with a Service Map that covers 100% of your system with no blind spots
* Predefined inspections audit each application without any configuration

<p align="center">
<img width="775" src="https://user-images.githubusercontent.com/194465/235189673-833066d1-b18f-4c7a-8b81-81b37f966daf.png">
</p>


### Application Health Summary

* Easily understand the status of your services, even when dealing with hundreds of them
* Gain insight into application logs without the need to manually inspect each one
* SLOs (Service Level Objectives) tracking

<p align="center">
<img width="773" src="https://github.com/coroot/coroot/assets/194465/6cef06d4-0dcc-4908-85a3-7ec140bd444f">
</p>

### Explore any outlier requests with distributed tracing

* Investigate any anomaly with just one click
* Vendor-neutral instrumentation with OpenTelemetry
* Are you unable to instrument legacy or third-party services?
Coroot's eBPF-based instrumentation can capture requests without requiring any code changes.

<p align="center">
<img width="1352" src="https://github.com/coroot/coroot/assets/194465/f5a4342f-776d-48b1-a3b8-03ccbdf43b5e">
</p>

### Grasp insights from logs with just a quick glance

* Log patterns: out-of-the-box event clustering
* Seamless logs-to-traces correlation
* Lightning-fast search based on ClickHouse

<p align="center">
<img width="777" src="https://github.com/coroot/coroot/assets/194465/14abefdb-4737-4991-9d48-c7efec42fefd">
</p>

### Profile any application in 1 click

* Analyze any unexpected spike in CPU or memory usage down to the precise line of code
* Don't make assumptions, know exactly what the resources were spent on
* Easily investigate any anomaly by comparing it to the system's baseline behavior

<p align="center">
<img width="773" src="https://user-images.githubusercontent.com/194465/235190071-21256cbe-6201-4d16-97f3-6565f7256f98.png">
</p>

### Built-in expertise

* Coroot can automatically identify over 80% of issues
* If an app is not meeting its Service Level Objectives (SLOs), Coroot will send a single alert that includes the results of all relevant inspections
* You can easily adjust any inspection for a particular application or an entire project

<p align="center">
  <img width="778" src="https://github.com/coroot/coroot/assets/194465/3590a492-8895-4cc6-94df-a880656a330a">
</p>

### Deployment Tracking

* Coroot discovers and monitors every application rollout in your Kubernetes cluster
* Requires no integration with your CI/CD pipeline
* Each release is automatically compared with the previous one, so you'll never miss even the slightest performance degradation
* With integrated Cost Monitoring, developers can track how each change affects their cloud bill

<p align="center">
<img width="772" src="https://user-images.githubusercontent.com/194465/235190275-a6541063-1b26-4ae3-8c20-87787d2e928d.png">
</p>

### Cost Monitoring

* Understand your cloud costs down to the specific application
* Doesn't require access to you cloud account or any other configurations
* AWS, GCP, Azure

<p align="center">
<img width="771" src="https://user-images.githubusercontent.com/194465/235190425-a7f33c7f-33ef-4ef5-9dc1-525ff7524e93.png">
</p>


## Installation

You can run Coroot as a Docker container or deploy it into any Kubernetes cluster.
Check out the [Installation guide](https://docs.coroot.com/).

## Documentation

The Coroot documentation is available at [docs.coroot.com/docs](https://docs.coroot.com/).

## Live demo

A live demo of Coroot is available at [demo.coroot.com](https://demo.coroot.com/)

## Community & Support

* [Community Slack](https://join.slack.com/t/coroot-community/shared_invite/zt-1gsnfo0wj-I~Zvtx5CAAb8vr~r~vecyw)
* [GitHub Discussions](https://github.com/coroot/coroot/discussions)
* [GitHub Issues](https://github.com/coroot/coroot/issues)
* Twitter: [@coroot_com](https://twitter.com/coroot_com)


## Contributing
To start contributing, check out our [Contributing Guide](https://github.com/coroot/coroot/blob/main/CONTRIBUTING.md).

## License

Coroot is licensed under the [Apache License, Version 2.0](https://github.com/coroot/coroot/blob/main/LICENSE).





