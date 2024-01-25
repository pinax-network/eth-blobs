# Golang Template

This is an opinionated template for Golang services and APIs. It includes handling config files, Prometheus metrics,
a predefined logger, a version endpoint, as well as an optional gRPC server including health checks.

It also includes Github workflows for testing code formats, building the binary, do linting and building + publishing
Docker images to the Github container registry.

## How to create a new project from this template

Create a new Github repository from this template by clicking on _Use this template_ and then create a repository from
it.

### Prerequisites

In case you never have worked with Pinax Go projects, you should ensure that you have [Go](https://go.dev/doc/install)
and [Taskfile](https://taskfile.dev/installation/) installed. To run the linter locally, you also need to have
[golangci-lint](https://golangci-lint.run/usage/install/#local-installation) installed.

### First step

Before you do anything on the new project, you should **always** clone it to your local machine and then run:

```zsh
task init
```

This will rewrite your Git history by rebasing this project onto the template branch, which is necessary that you are
able to merge changes from the template repository into your project in the future. It will also automatically rename
your project.

After this is done, you need to force push those changes once to your project's repository (please avoid using `--force`
at later stages):

```zsh
git push --force
```

Now you are able to build and run your project:

```zsh
task build && task start:service
```

### Keep your project in sync with the template

You should occasionally check in with the template and make sure to keep your project in sync. New features that are
generally useful should always be added to this template repository and then synced to the projects rather than adding
them to the projects directly. This allows other projects to benefit from new changes and generally keeps all projects
in a similar feature state.

To sync your project with the template, first fetch all the Git remotes:

```zsh
git fetch --all
```

In case Git found changes on `template/main` you want to merge those into your project by running:

```zsh
git merge template/main
```

### Recommended Github settings

As this template also includes a bunch of [Github workflows](#github-workflows) it's recommended to protect your main
branch and require pull requests for changes. This can be done under `Settings -> Branches -> Add rule`. The following
settings should be enabled:

* _Branch name pattern_ (use the name of your primary branch, for example `main`).
* _Require a pull request before merging_
* _Require status checks to pass before merging_ (enable `build (1.21.x, ubuntu-latest)` and
  `lint (1.21.x, ubuntu-latest)` here)
* _Do not allow bypassing the above settings_

> [!NOTE]
> It's recommended to use one primary branch for development (ideally `main`). This branch should be protected as above.
> Everything pushed to that branch is considered ready for development and can be auto deployed to staging environments.
> To release for production, create tagged releases. Go to Releases on your Github repository and then chose
> `Draft a new release`, choose a tag like `v1.0.0` and then just click on `Generate release notes` to have Github
> handle the rest.

## Tasks

This template uses [Taskfile](https://taskfile.dev/) to automate some common tasks. For example, to build projects we
provide a `task build` command, that automatically injects build flags like the version (based on Git tags) and commit
hash into the application.

To view all available tasks, just run `task --list`:

```
task: Available tasks for this project:
* build:               Builds the Go binary. You can pass through arguments to the Go compiler by appending --, for example: 'task build -- -tags my_feature'. To set compiler flags use the BUILD_FLAGS environment variable.
* init:                Initializes the project by renaming it from the template and properly rebasing it. This should be run first before anything else is done.
* build:docker:        Builds the Docker image
* print:env:           Print all environment variables set within this Taskfile.
* run:format:          Runs Golang's code formatter gofmt
* run:lint:            Runs Golang's linter
* start:service:       Starts a local instance of the service. You can pass through arguments to the binary by appending --, for example 'task start -- -debug'
* test:format:         Checks Golang's code formatter
```

Note that the Taskfile works in a generic way by extracting all necessary data from the current repository it's in and
writing it into environment variables. That means new projects created from this template don't need to adapt any tasks,
but can just execute them right away.

You can see those environment variables by running `task print:env`:

```
PACKAGE:        golang-service-template
TAG:            untagged
SHORT_COMMIT:   5fd4b1e
COMMIT:         5fd4b1e723f5d6dc9eec1f4b210d1a369b7fc7e7
REPOSITORY:     golang-service-template
ORGANISATION:   pinax-network
REGISTRY:       ghcr.io
```

## Github workflows

The available Github workflows are:

* `go.yml` - ensures the Go binary can be build and runs `go test` to execute available Unit tests. It is executed
  on pushes and pull requests to `main` or `develop*` branches. It is able to include dependencies from private Pinax
  repositories (it gets the necessary SSH key injected).
* `golangci-lint.yml` - executes a Go linter and ensures the code is properly formatted. You can run the linter locally
  using `task run:lint` in case this workflow fails which will link you to the actual files that contain issues. It is
  also executed on pushes and pull requests to `main` or `develop*` branches.
* `docker.yml` - builds and pushes Docker images to the Github container registry. It's run on pushes to `main` or
  whenever new tags are created in the form of `v*`.

## Template features

### Default endpoints

This template by default starts a gRPC and an HTTP server. To disable either, you just need to remove
the `grpc_host` or `http_host` config in your config.yaml.

The default gRPC methods are:

* `grpc.health.v1.Health` - health method.
* `grpc.reflection.v1.ServerReflection` and `grpc.reflection.v1alpha.ServerReflection` - reflection methods to query the
  available methods.

Example usage:

```zsh
➜ grpcurl -plaintext localhost:9000 grpc.health.v1.Health/Check
{
  "status": "SERVING"
}
```

The default HTTP endpoints are:

* `GET /version` - returns the version (based on Git tags), the commit hash and potentially enabled features. All those
  flags are injected automatically when building this project using `task build`.
* `GET /metrics` - exposes Prometheus metrics.

Example:

```zsh
➜ curl http://localhost:8080/version
{"data":{"version":"untagged","commit":"10b2e6c","enabled_features":[]}}
```

### Configuration handling

The template includes loading a configuration and validating it based on
[validator](https://github.com/go-playground/validator) rules. To extend the configuration add more structs to
`config/config.go` as necessary.

### Feature flags

To enable or disable functionality based on feature flags, you can define new features in `flags/features.go`:

```go 
const MyFeature Feature = "my_feature"
```

Then create a `flags/my_feature.go` file that is only included based on the `my_feature` build tag:

```go
//go:build my_feature

package flags

func init() {
	enableFeature(MyFeature)
}
```

In your code, you can then check for the feature flag:

```go
if flags.MyFeature.IsEnabled() {
// include my feature code here
}
```

To enable the feature you just need to pass it in the build step: `task build -- -tags my_feature`. 
