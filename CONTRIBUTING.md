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


## Pull Request Checklist

* Branch from the main branch and, if needed, rebase to the current main branch before submitting your pull request. If it doesn't merge cleanly with main you may be asked to rebase your changes.
* Commits should be as small as possible, while ensuring that each commit is correct independently (i.e., each commit should compile and pass tests).
* Add tests relevant to the fixed bug or new feature.
* Use `make lint` to run linters and ensure formatting is correct.
* Run the unit tests suite `make test`.


## IDE configuration

### Goland
Enable the following "Actions on Save" for automatic formatting:
![image](https://github.com/coroot/coroot/assets/199054/9c5e62e9-7bdf-47e0-97b2-2ea56c9b620d)

### VS Code
_TODO_...
