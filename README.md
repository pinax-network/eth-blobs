# golang-service-template

This is an opinionated template for Golang services for Pinax. It includes handling config files, Prometheus metrics, 
a predefined logger, a version endpoint, as well as an optional gRPC server including health checks. 

It also includes Github workflows for testing code formats, building the binary and to do linting.

## Prerequisites

Ensure that you have [Go](https://go.dev/doc/install) and [Taskfile](https://taskfile.dev/installation/) installed. 
To run the linter locally, you also need to have [golangci-lint](https://golangci-lint.run/usage/install/#local-installation) installed.

Copy over the `config.example.yaml` to `config.yaml`.

## Tasks

See the available tasks:

```
# task --list
task: Available tasks for this project:
* build:               Builds the Go binary. You can pass through arguments to the Go compiler by appending --, for example: 'task build -- -tags my_feature'
* run:format:          Runs Golang's code formatter gofmt
* run:lint:            Runs Golang's linter
* start:service:       Starts a local instance of the service. You can pass through arguments to the API by appending --, for example 'task start -- -debug'
* test:format:         Checks Golang's code formatter
```

## Default endpoints

* `GET /version` - returns the build version, commit hash and enabled features
* `GET /metrics` - Prometheus metrics exporter
