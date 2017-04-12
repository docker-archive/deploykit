InfraKit.DigitalOcean
=====================

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
