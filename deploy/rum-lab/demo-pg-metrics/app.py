#!/usr/bin/env python3
"""Push node-agent–compatible metrics for a lab container (Postgres/Valkey/…)."""
from __future__ import annotations

import os
import socket
import time

from remote_write import remote_write

METRICS_RW_URL = os.environ.get("COROOT_METRICS_URL", "http://coroot:8080/v1/metrics").rstrip("/")
API_KEY = os.environ.get("COROOT_API_KEY", "agent-local-key-0000000000000001")
SERVICE_NAME = os.environ.get("SERVICE_NAME", "demo-service")
LISTEN_HOST = os.environ.get("LISTEN_HOST", SERVICE_NAME)
LISTEN_PORT = os.environ.get("LISTEN_PORT", "5432")
MACHINE_ID = os.environ.get("DEMO_MACHINE_ID", "rumlabdemomachine000000000000099")
SYSTEM_UUID = os.environ.get("DEMO_SYSTEM_UUID", "rumlabdemosystemuuid0000000000099")
CONTAINER_ID = os.environ.get("DEMO_CONTAINER_ID", f"/docker/{SERVICE_NAME}")
HOSTNAME = os.environ.get("DEMO_HOSTNAME", "rum-lab-node")
IMAGE = os.environ.get("CONTAINER_IMAGE", "unknown:local")
APP_TYPE = os.environ.get("APPLICATION_TYPE", "unknown")
INTERVAL = float(os.environ.get("METRICS_INTERVAL_SEC", "10"))

_START = time.monotonic()
_CPU = 0.0


def resolve_listen() -> str:
    infos = socket.getaddrinfo(LISTEN_HOST, int(LISTEN_PORT), type=socket.SOCK_STREAM)
    for family, _, _, _, sockaddr in infos:
        if family == socket.AF_INET:
            return f"{sockaddr[0]}:{LISTEN_PORT}"
    if infos:
        host = infos[0][4][0]
        return f"{host}:{LISTEN_PORT}"
    return f"{LISTEN_HOST}:{LISTEN_PORT}"


def series(listen_addr: str):
    global _CPU
    now = time.monotonic()
    uptime = now - _START
    _CPU += 0.05 + (uptime * 0.001)
    ts_ms = int(time.time() * 1000)
    node = [("machine_id", MACHINE_ID), ("system_uuid", SYSTEM_UUID)]
    common = node + [("container_id", CONTAINER_ID)]
    cores = float(os.cpu_count() or 2)

    def s(name: str, labels: list[tuple[str, str]], value: float):
        return ([("__name__", name)] + labels, [(value, ts_ms)])

    out = [
        s("node_info", node + [("hostname", HOSTNAME), ("kernel_version", "6.8.0-lab")], 1),
        s("node_agent_info", node + [("version", "rum-lab-shim")], 1),
        s("node_uptime_seconds", node, uptime),
        s("node_resources_cpu_logical_cores", node, cores),
        s("node_resources_cpu_usage_seconds_total", node + [("mode", "user")], _CPU),
        s("node_resources_cpu_usage_seconds_total", node + [("mode", "idle")], uptime * max(1.0, cores - 1)),
        s("node_resources_memory_total_bytes", node, 8589934592),
        s("node_resources_memory_available_bytes", node, 4294967296),
        s("node_resources_memory_free_bytes", node, 2147483648),
        s("node_resources_memory_cached_bytes", node, 1073741824),
        s("container_info", common + [("image", IMAGE)], 1),
        s("container_resources_cpu_usage_seconds_total", common, _CPU),
        s("container_resources_cpu_limit_cores", common, 1),
        s("container_resources_cpu_delay_seconds_total", common, _CPU * 0.02),
        s("container_resources_cpu_throttled_seconds_total", common, 0),
        s("container_resources_memory_rss_bytes", common, 80 * 1024 * 1024),
        s("container_resources_memory_cache_bytes", common, 20 * 1024 * 1024),
        s("container_resources_memory_limit_bytes", common, 512 * 1024 * 1024),
        s("container_net_tcp_listen_info", common + [("listen_addr", listen_addr)], 1),
        # POD_IP:0 — same wildcard listen K8s metadata adds; helps merge ExternalService edges.
        s("container_net_tcp_listen_info", common + [("listen_addr", f"{listen_addr.split(':', 1)[0]}:0")], 1),
    ]
    if APP_TYPE and APP_TYPE != "unknown":
        out.append(s("container_application_type", common + [("application_type", APP_TYPE)], 1))
    return out


def main() -> None:
    print(
        f"[demo-shim-metrics] {SERVICE_NAME} type={APP_TYPE} → {METRICS_RW_URL}",
        flush=True,
    )
    while True:
        try:
            listen = resolve_listen()
            remote_write(METRICS_RW_URL, API_KEY, series(listen))
            print(f"[demo-shim-metrics] ok {SERVICE_NAME} listen={listen}", flush=True)
        except Exception as exc:  # noqa: BLE001
            print(f"[demo-shim-metrics] {SERVICE_NAME} failed: {exc}", flush=True)
        time.sleep(INTERVAL)


if __name__ == "__main__":
    main()
