# Contributing

Thank you for your interest in contributing to Coroot!
Below are some basic guidelines.


## Requirements
* Go v1.21
* Node v21


## Running locally
Run frontend builder (watcher):
```shell
cd front
npm ci
npm run build-dev
```

Run backend:
```shell
go mod tidy
go run main.go
```

Open http://127.0.0.1:8080 in your browser.

## Remote Debug

How to use Goland and Dlv for remote debug of Golang code on a remote server.

Install dlv：
```shell
go install github.com/go-delve/delve/cmd/dlv@latest
```

Run frontend builder (watcher):
```shell
cd front
npm ci
npm run build-dev
```

Run backend:
```shell
go mod tidy

go build -gcflags "all=-N -l" -o coroot

dlv --headless \
    --api-version=2 \
    --log \
    --listen=:12345 \
    exec ./coroot -- \
    --listen=0.0.0.0:8888 \
    --bootstrap-clickhouse-address=$CLICKHOUSE-ADDRESS \
    --bootstrap-clickhouse-user=$CLICKHOUSE-USER \
    --bootstrap-clickhouse-database=$CLICKHOUSE-DATABASE \
    --bootstrap-clickhouse-password=$CLICKHOUSE-PASSWORD \
    --bootstrap-prometheus-url=$PROMETHEUS-URL \
    --bootstrap-refresh-interval=15s
```

Replace the values in the above environment variables before execution:

- CLICKHOUSE-ADDRESS: clickHouse address
- CLICKHOUSE-DATABASE: clickHouse database
- CLICKHOUSE-USER: clickHouse user 
- CLICKHOUSE-PASSWORD: clickHouse password
- PROMETHEUS-URL: prometheus url

Configure GoLand： Run -> EditConfigurations -> Go Remote, Set host to the server address where the backend is located above, and port to 12345.

Then, start goland and you will see the following coroot log.
```shell
2024-06-12T16:53:26+08:00 warning layer=debugger reading debug_info: concrete subprogram without address range at 0x9c4b90
2024-06-12T16:53:26+08:00 debug layer=debugger Adding target 10436 "/root/mark/coroot/coroot --listen=0.0.0.0:8888 --bootstrap-clickhouse-address=10.31.0.220:13124 --bootstrap-clickhouse-user=default --bootstrap-clickhouse-database=default --bootstrap-clickhouse-password=SH9eDMx3e0 --bootstrap-prometheus-url=http://10.31.0.220:58021 --bootstrap-refresh-interval=15s"
2024-06-12T16:53:46+08:00 debug layer=debugger continuing
2024-06-12T16:53:46+08:00 debug layer=debugger ContinueOnce
I0612 16:53:46.238961   10436 main.go:63] version: unknown, url-base-path: /, read-only: false
I0612 16:53:46.239085   10436 db.go:47] using sqlite database
I0612 16:53:46.241036   10436 db.go:47] using sqlite database
I0612 16:53:46.258643   10436 cache.go:169] loaded from disk in 17ms
I0612 16:53:46.258878   10436 compaction.go:82] compaction worker started
I0612 16:53:46.775754   10436 main.go:171] listening on 0.0.0.0:8888
I0612 16:53:47.188737   10436 constructor.go:126] trdbha8u: got 8 nodes, 209 apps in 413ms
I0612 16:53:47.263657   10436 auditor.go:67] trdbha8u: audited 209 apps in 74ms
I0612 16:53:49.681942   10436 constructor.go:126] trdbha8u: got 8 nodes, 213 apps in 191ms
I0612 16:53:49.732725   10436 updater.go:187] trdbha8u: cache updated in 2.451s
I0612 16:53:50.074162   10436 constructor.go:126] trdbha8u: got 8 nodes, 214 apps in 340ms
I0612 16:53:50.090387   10436 deployments.go:36] trdbha8u: checked 88 apps in 16ms
I0612 16:53:50.151023   10436 auditor.go:67] trdbha8u: audited 214 apps in 76ms
I0612 16:53:50.153154   10436 incidents.go:46] trdbha8u: checked 46 apps in 78ms
I0612 16:53:56.258937   10436 compaction.go:93] compaction iteration started
```
Access coroot and then you can debug the code in goland.

## Pull Request Checklist

* Branch from the main branch and, if needed, rebase to the current main branch before submitting your pull request. If it doesn't merge cleanly with main you may be asked to rebase your changes.
* Commits should be as small as possible, while ensuring that each commit is correct independently (i.e., each commit should compile and pass tests).
* Add tests relevant to the fixed bug or new feature.
* Use `make lint` to run linters and ensure formatting is correct.
* Run the unit tests suite `make test`.


## IDE configuration

### Goland
Enable the following "Actions on Save" for automatic formatting:
![image](https://github.com/coroot/coroot/assets/199054/ca32b935-1bf6-42d6-ad5a-dccfc04aa673)

### VS Code
_TODO_...
