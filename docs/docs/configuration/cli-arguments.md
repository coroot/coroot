---
sidebar_position: 3
---

# CLI arguments

| Argument               | Environment Variable | Default Value | Description                                                                                          |
|------------------------|----------------|-----|------------------------------------------------------------------------------------------------------|
| --listen               | LISTEN         | 0.0.0.0:8080| Listen address in the format `ip:port` or `:port`.                                                   |
| --url-base-path        | URL_BASE_PATH  | /   | Base URL to run Coroot at a sub-path, e.g., `/coroot/`.                                              |
| --data-dir             | DATA_DIR       | /data | Path to the data directory.                                                                          |
| --cache-ttl            | CACHE_TTL      | 720h | Cache Time-To-Live (TTL).                                                                            |
| --cache-gc-interval    | CACHE_GC_INTERVAL | 10m | Cache Garbage Collection (GC) interval.                                                              |
| --pg-connection-string | PG_CONNECTION_STRING |     | PostgreSQL connection string (uses SQLite if not set).                                               |
| --disable-usage-statistics | DISABLE_USAGE_STATISTICS | false | Disable usage statistics.                                                                            |
| --read-only            | READ_ONLY      | false | Enable read-only mode where configuration changes don't take effect.                                 |
| --do-not-check-slo     | DO_NOT_CHECK_SLO | false | Do not check Service Level Objective (SLO) compliance.                                               |
| --do-not-check-for-deployments | DO_NOT_CHECK_FOR_DEPLOYMENTS | false | Do not check for new deployments.                                                                    |
| --do-not-check-for-updates | DO_NOT_CHECK_FOR_UPDATES | false | Do not check for new versions.                                                                       |
| --auth-anonymous-role  | AUTH_ANONYMOUS_ROLE |     | Disable authentication and assign one of the following roles to the anonymous user: Admin, Editor, or Viewer. |
| --auth-bootstrap-admin-password | AUTH_BOOTSTRAP_ADMIN_PASSWORD |     | Password for the default Admin user.                                                                 |
| --license-key          | LICENSE_KEY    |     | License key for Coroot Enterprise Edition.                                                           |
