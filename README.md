<img width="200" src="https://coroot.com/static/logo_u.png">

Coroot is a monitoring and troubleshooting tool for microservice architectures. 

![](https://github.com/coroot/coroot/actions/workflows/build.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/coroot/coroot)](https://goreportcard.com/report/github.com/coroot/coroot)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

<p align="center"><img width="800" src="https://user-images.githubusercontent.com/194465/187667684-224cfa32-96cd-44f0-87f7-0528b3dd7bb9.gif"></p>


## Features

### eBPF-based service mapping
Thanks to eBPF, Coroot shows you a comprehensive [map of your services](https://coroot.com/blog/building-a-service-map-using-ebpf) without any code changes.

<p align="center"><img width="800" src="./front/public/readme/service-map.png"></p>

### Log analysis without storage costs

[Node-agent](https://github.com/coroot/coroot-node-agent) turns terabytes of logs into just a few dozen metrics by extracting 
[repeated patterns](https://coroot.com/blog/mining-logs-from-unstructured-logs) right on the node. 
Using these metrics allows you to quickly and cost-effectively find the errors relevant to a particular outage.

<p align="center"><img width="800" src="./front/public/readme/logs.png"></p>

### Cloud topology awareness

Coroot uses [cloud metadata](https://coroot.com/blog/cloud-metadata) to show which regions and availability zones 
each application runs in.
This is very important to known, because:
 * Network latency between availability zones within the same region can be higher than within one particular zone.
 * Data transfer between availability zones in the same region is paid, while data transfer within a zone is free.

<p align="center"><img width="800" src="./front/public/readme/topology.png"></p>

### Advanced Postgres observability
 
Coroot [makes](https://coroot.com/blog/pg-agent) troubleshooting Postgres-related issues easier not only for experienced DBAs but also for engineers not specialized in databases.

<p align="center"><img width="800" src="./front/public/readme/postgres.png"></p> 


### Integration into your existing monitoring stack

Coroot uses Prometheus as a Time-Series Database (TSDB):
* The agents are Prometheus-compatible exporters 
* Coroot itself is a Prometheus client (like Grafana)

<p align="center"><img width="600" src="./front/public/readme/prometheus.svg"></p>

### Built-in Prometheus cache

The built-in Prometheus cache allows Coroot to provide you with a blazing fast UI without overloading your Prometheus.


## Installation

You can run Coroot as a Docker container or deploy it into any Kubernetes cluster.
Check out the [Installation guide](https://coroot.com/docs/coroot-community-edition/getting-started/installation).

## Documentation

The Coroot documentation is available at [coroot.com/docs/coroot-community-edition](https://coroot.com/docs/coroot-community-edition).

## License

Coroot is licensed under the [Apache License, Version 2.0](https://github.com/coroot/coroot/blob/main/LICENSE).





