InfraKit Group Plugin
=====================

This is the default implementation of the Group Plugin that can manage collections of resources.
This plugin works in conjunction with the Instance and Flavor plugins, which separately define
the properties of the physical resource (Instance plugin) and semantics or nature  of the node
(Flavor plugin).


## Building

When you do `make -k all` in the top level directory, the CLI binary will be built and can be
found as `./infrakit/cli` from the project's top level directory.

## Usage

```
$ ./infrakit/group -h
Group server

Usage:
  ./infrakit/group [flags]
  ./infrakit/group [command]

Available Commands:
  version     print build version information

Flags:
      --listen string            listen address (unix or tcp) for the control endpoint (default "unix:///run/infrakit/plugins/group.sock")
      --log int                  Logging level. 0 is least verbose. Max is 5 (default 4)
      --poll-interval duration   Group polling interval (default 10s)

Use "./infrakit/group [command] --help" for more information about a command.
```

The plugin can be started without any arguments and will default to using unix socket in
`/run/infrakit/plugins` for communications with the CLI and other plugins:

```
$ ./infrakit/group --log=5
INFO[0000] Starting discovery
DEBU[0000] Opening: /run/infrakit/plugins
DEBU[0000] Discovered plugin at unix:///run/infrakit/plugins/flavor-swarm.sock
DEBU[0000] Discovered plugin at unix:///run/infrakit/plugins/flavor-zookeeper.sock
DEBU[0000] Discovered plugin at unix:///run/infrakit/plugins/instance-file.sock
INFO[0000] Starting plugin
INFO[0000] Starting
INFO[0000] Listening on: unix:///run/infrakit/plugins/group.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/group.sock err= <nil>
```

### Default Directory for Plugin Discovery

All InfraKit plugins will by default open the unix socket located at `/run/infrakit/plugins`.
Make sure this directory exists on your host:

```
mkdir -p /run/infrakit/plugins
chmod 777 /run/infrakit/plugins
```

See the [CLI Doc](../cli/README.md) for details on accessing the group plugin via CLI.
