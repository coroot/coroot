---
sidebar_position: 1
---

# Requirements

 * Coroot relies heavily on eBPF, therefore, the minimum supported Linux kernel version is 4.16.
 * eBPF-based continuous profiling utilizes CO-RE. CO-RE is supported by most modern Linux distributions, including:
   * Ubuntu 20.10 and above 
   * Debian 11 and above 
   * RHEL 8.2 and above 
 * Coroot gathers metrics, logs, traces, and profiles, with each telemetry signal associated with containers. In this context, a container refers to a group of processes running within a dedicated cgroup. The following container types are supported:
   * Kubernetes Pods using Docker, Containerd, or CRI-O as their runtime environment 
   * Standalone containers: Docker, Containerd, CRI-O 
   * Docker Swarm 
   * Systemd units: any systemd service is also considered as a container
 * Supported container orchestrators include:
   * Kubernetes: Self-managed, EKS (including basic support for AWS Fargate), GKE, AKS, OKE
   * OpenShift
   * K3s
   * MicroK8s
   * Docker Swarm
 * Limitations:
   * Coroot doesn't support Docker-in-Docker environments such as MiniKube due to eBPF limitations
   * WSL1 (Windows Subsystem for Linux) is not supported
