---
sidebar_position: 7
---

# Database

Coroot requires a database to store its configuration, such as projects and Prometheus connection details.

## SQLite (default)

By default, Coroot uses an embedded sqlite database. For production installations, we recommend users to use a 
robust database, such as Postgres. This allows you to run several Coroot replicas for high availability and backup the database.

## Postgres

Create role and database:

```sql
CREATE ROLE coroot WITH LOGIN PASSWORD 'password';
CREATE DATABASE coroot WITH OWNER = coroot;
```

You can configure Coroot to use Postgres by setting the `--pg-connection-string` command line argument or the `PG_CONNECTION_STRING` environment variable:

```bash
docker run -d --name coroot \
  -p 8080:8080 \
  -e PG_CONNECTION_STRING="postgres://coroot:password@127.0.0.1:5432/coroot?sslmode=disable" \
  ghcr.io/coroot/coroot
``` 

Here is an example of how to format the `PG_CONNECTION_STRING` variable using a Kubernetes secret:

```yaml
...
env:
- name: PGPASSWORD
  valueFrom: { secretKeyRef: { name: coroot.pg.credentials, key: password } }
- name: PG_CONNECTION_STRING
  value: "host=coroot-db user=coroot password=$(PGPASSWORD) dbname=coroot sslmode=require connect_timeout=1"
  ...
```
  
To learn more about the connection string format follow the [Postgres documentation](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING).

