InfraKit.DigitalOcean
=====================

[![CircleCI](https://circleci.com/gh/docker/infrakit.digitalocean.svg?style=svg&circle-token=df12ee3a96564afcccc29468bdb28f8931899581)](https://circleci.com/gh/docker/infrakit.digitalocean)
[![codecov](https://codecov.io/gh/docker/infrakit.digitalocean/branch/master/graph/badge.svg?token=MUiFRcmQVF)](https://codecov.io/gh/docker/infrakit.digitalocean)

[InfraKit](https://github.com/docker/infrakit) plugins for creating and managing resources in Digital ocean.

A [reference](/README.md#reference-implementations) implementation of an Instance Plugin that can accept any
configuration and writes the configuration to disk as `provision`.  It is useful for testing and debugging.

## Instance plugin

An InfraKit instance plugin which creates Digitalocean droplets.

### Building

To build the instance plugin, run `make binaries`. The plugin binary
will be located at `./build/infrakit-instance-digitalocean`.

### Running

```
${PATH_TO_INFRAKIT}/infrakit-flavor-vanilla
${PATH_TO_INFRAKIT}/infrakit-group-default
./build/infrakit-instance-digitalocean --config=[CONFIG_PATH] --region=[GCP_ZONE]

${PATH_TO_INFRAKIT}/infrakit group commit do-example-1.json
```
