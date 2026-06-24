---
sidebar_position: 9.5
---

# Windows

The Coroot Windows agent monitors Windows hosts, including Windows Services and Docker (Windows) containers, and ships metrics, logs, and DNS/network telemetry to Coroot. It connects out to an existing Coroot instance, so before you start you need Coroot's URL and a project API key (**Project Settings → API keys**).

:::note
Only Coroot agents run on Windows. Coroot itself and its components (the Coroot server, ClickHouse, Prometheus) are not supported on Windows. Run them on Linux, and point the Windows agent at that instance.
:::

## Requirements

- Windows Server 2016 or later (2016, 2019, 2022, 2025), x64 or arm64
- Administrator privileges (the installer registers a Windows service)
- Outbound TCP access from the host to your Coroot instance

## Install

Run this in an **elevated** PowerShell (Run as administrator). It downloads the latest agent and installs it as the `coroot-windows-agent` service via the MSI package. Re-running it upgrades an existing installation.

```powershell
$env:COROOT_COLLECTOR_ENDPOINT = 'http://COROOT_URL:8080'
$env:COROOT_API_KEY = '<API_KEY>'
iwr -useb https://raw.githubusercontent.com/coroot/coroot-node-agent/main/install.ps1 | iex
```

(`SCRAPE_INTERVAL` defaults to `15s`. Set `$env:COROOT_SCRAPE_INTERVAL` to override. You can also download `install.ps1` and run it with `-CollectorEndpoint`/`-ApiKey` flags instead of env vars.)

### Manual MSI install

Download the MSI for your architecture from the [releases page](https://github.com/coroot/coroot-node-agent/releases/latest) (`coroot-windows-agent-amd64.msi` for x64, `coroot-windows-agent-arm64.msi` for arm64) and install it silently:

```powershell
msiexec /i coroot-windows-agent-amd64.msi /qn COLLECTOR_ENDPOINT=http://COROOT_URL:8080 API_KEY=<API_KEY> SCRAPE_INTERVAL=15s
```

| MSI property | Description | Default |
| --- | --- | --- |
| `COLLECTOR_ENDPOINT` | Base URL of your Coroot instance | _(required)_ |
| `API_KEY` | Project API key | _(required)_ |
| `SCRAPE_INTERVAL` | How often metrics are gathered | `15s` |

## Configuration

The agent is configured with command-line flags. The installer sets the three above. To change any other option, set a **machine** environment variable with the `COROOT_` prefix and restart the service. On Windows environment variables are global, so the agent namespaces its variables with `COROOT_` to avoid clashing with other software.

```powershell
[Environment]::SetEnvironmentVariable("COROOT_SCRAPE_INTERVAL", "30s", "Machine")
Restart-Service coroot-windows-agent
```

Common variables:

| Environment variable | Flag | Description |
| --- | --- | --- |
| `COROOT_COLLECTOR_ENDPOINT` | `--collector-endpoint` | Base URL of your Coroot instance |
| `COROOT_API_KEY` | `--api-key` | Project API key |
| `COROOT_SCRAPE_INTERVAL` | `--scrape-interval` | Metrics gathering interval |
| `COROOT_INSECURE_SKIP_VERIFY` | `--insecure-skip-verify` | Skip TLS certificate verification of the collector |
| `COROOT_CA_FILE` | `--ca-file` | Path to a custom CA certificate |
| `COROOT_DISABLE_LOG_PARSING` | `--disable-log-parsing` | Disable Windows Event Log / container log collection |
| `COROOT_CONTAINER_DENYLIST` | `--container-denylist` | Regex patterns of services/containers to ignore |

For the complete list of supported flags and the platform notes, see the [Coroot-node-agent configuration reference](../configuration/coroot-node-agent#windows).

## Managing the service

```powershell
Get-Service coroot-windows-agent
Restart-Service coroot-windows-agent
Get-WinEvent -ProviderName coroot-windows-agent -MaxEvents 50   # agent log
```

## Upgrade

Re-run the install script (or install a newer MSI). The MSI performs an in-place upgrade.

## Uninstall

Remove **Coroot Windows Agent** from **Settings → Apps**, or run:

```powershell
Get-Package "Coroot Windows Agent" | Uninstall-Package
```
