---
sidebar_position: 4
---

# Go profiling

The Go standard library includes the [pprof](https://pkg.go.dev/net/http/pprof) package,
enabling developers to expose profiling data of their Go applications.

`Coroot-cluster-agent` automatically discovers and periodically retrieves profiles from Golang applications.

<img alt="golang pull profiling" src="/img/docs/profiling/golang-profiling.png" class="card w-1200"/>

## Supported profile types

* **CPU** profile is useful to identify functions consuming most CPU time.
* **Memory** profile shows the amount of memory held and allocated by particular functions.
This data is quite useful for troubleshooting memory leaks and GC (Garbage Collection) pressure.
* **Blocking** profile helps identify and analyze goroutines that are blocked, waiting for resources or synchronization,
including unbuffered channels and locks.
* **Mutex** profile allows to identify areas where goroutines contend for access to shared resources protected by mutexes.

## Enabling

To enable collecting profiles of a Go application, you need to expose `pprof` endpoints
and allow Coroot to discover the application pods.

### Step 1: exposing pprof endpoints

Register `/debug/pprof/*` handlers in `http.DefaultServeMux`:

```go
import _ "net/http/pprof"
```

You can register `/debug/pprof/*` handlers to your own `http.ServeMux`:

```go
var mux *http.ServeMux
mux.Handle("/debug/pprof/", http.DefaultServeMux)
```

Or, to `gorilla/mux`:

```go
var router *mux.Router
router.PathPrefix("/debug/pprof").Handler(http.DefaultServeMux)
```

### Step 2: annotating application Pods

Coroot-cluster-agent automatically discovers and fetches profiles from pods
annotated with `coroot.com/profile-scrape` and `coroot.com/profile-port` annotations:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
  labels:
    name: catalog
spec:
  replicas: 2
  selector:
    matchLabels:
      name: catalog
  template:
    metadata:
      annotations:
        coroot.com/profile-scrape: "true"
        coroot.com/profile-port: "8080"
...
```
