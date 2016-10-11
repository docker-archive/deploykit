InfraKit CLI
============

This is a CLI for working with various infrakit plugins.  In the simplest form, an InfraKit plugin
is simply a daemon that communicates over unix domain sockets.  InfraKit plugins can find each
other by the socket files in a common directory.  The CLI uses the common directory as a discovery
mechanism and offers various subcommands for working with plugins. In general, plugin methods are
exposed as verbs and configuration JSON can be read from local file or standard input.

## Building

When you do `make binaries` in the top level directory, the CLI binary will be built and can be
found as `./build/cli` from the project's top level directory.

## Usage

```
$ build/cli -h
infrakit cli

Usage:
  build/cli [command]

Available Commands:
  flavor      Access flavor plugin
  group       Access group plugin
  instance    Access instance plugin
  plugin      Manage plugins
  version     Print build version information

Flags:
      --dir string   Dir path for plugin discovery (default "/run/infrakit/plugins")
      --log int      Logging level. 0 is least verbose. Max is 5 (default 4)

Use "build/cli [command] --help" for more information about a command.
```

### Default Directory for Plugin Discovery

All InfraKit plugins will by default open the unix socket located at `/run/infrakit/plugins`.
Make sure this directory exists on your host:

```
mkdir -p /run/infrakit/plugins
chmod 777 /run/infrakit/plugins
```

### List Plugins

```
$ build/cli plugin ls
Plugins:
NAME                	LISTEN
flavor-swarm        	unix:///run/infrakit/plugins/flavor-swarm.sock
flavor-zookeeper    	unix:///run/infrakit/plugins/flavor-zookeeper.sock
group               	unix:///run/infrakit/plugins/group.sock
instance-file       	unix:///run/infrakit/plugins/instance-file.sock
```

Once you know the plugins by name, you can make calls to them.  For example, the instance plugin
`instance-file` is a simple plugin that "provisions" an instance by writing the instructions to
a file in a local directory.

You can access the following plugins and their methods via command line:

  + instance
  + group
  + flavor

### Working with Instance Plugin

```
$ build/cli instance -h
Access instance plugin

Usage:
  build/cli instance [command]

Available Commands:
  describe    describe the instances
  destroy     destroy the resource
  provision   provision the resource instance
  validate    validate input

Flags:
      --name string   Name of plugin

Global Flags:
      --dir string   Dir path for plugin discovery (default "/run/infrakit/plugins")
      --log int      Logging level. 0 is least verbose. Max is 5 (default 4)

Use "build/cli instance [command] --help" for more information about a command.
```

For example, using the plugin `instance-file` as an example:

`describe` calls the `DescribeInstances` endpoint of the plugin:

```
$ build/cli instance --name instance-file describe
ID                            	LOGICAL                       	TAGS
instance-1474850397           	  -                           	group=test,instanceType=small
instance-1474850412           	  -                           	group=test2,instanceType=small
instance-1474851747           	logic2                        	instanceType=small,group=test2
```

Validate - send the config JSON via stdin:

```
$ build/cli instance --name instance-file validate << EOF
> {
>     "Properties" : {
>         "version" : "v0.0.1",
>         "groups" : {
>             "managers" : {
>                 "driver" : "infrakit/quorum"
>             },
>             "small" : {
>                 "driver" : "infrakit/scaler",
>                 "properties" : {
>                     "size" : 3
>                 }
>             },
>             "large" : {
>                 "driver" : "infrakit/scaler",
>                 "properties" : {
>                     "size" : 3
>                 }
>             }
>         }
>     },
>     "Tags" : {
>         "instanceType" : "small",
>         "group" : "test2"
>     },
>     "Init" : "#!/bin/sh\napt-get install -y wget",
>     "LogicalID" : "logic2"
> }
> EOF
validate:ok
```

Provision - send via stdin:

```
$ build/cli instance --name instance-file provision << EOF
> {
>     "Properties" : {
>         "version" : "v0.0.1",
>         "groups" : {
>             "managers" : {
>                 "driver" : "infrakit/quorum"
>             },
>             "small" : {
>                 "driver" : "infrakit/scaler",
>                 "properties" : {
>                     "size" : 3
>                 }
>             },
>             "large" : {
>                 "driver" : "infrakit/scaler",
>                 "properties" : {
>                     "size" : 3
>                 }
>             }
>         }
>     },
>     "Tags" : {
>         "instanceType" : "small",
>         "group" : "test2"
>     },
>     "Init" : "#!/bin/sh\napt-get install -y wget",
>     "LogicalID" : "logic2"
> }
> EOF
instance-1474873473
```

List instances

```
$ build/cli instance --name instance-file describe
ID                            	LOGICAL                       	TAGS
instance-1474850397           	  -                           	group=test,instanceType=small
instance-1474850412           	  -                           	group=test2,instanceType=small
instance-1474851747           	logic2                        	group=test2,instanceType=small
instance-1474873473           	logic2                        	group=test2,instanceType=small
```
Destroy

```
$ build/cli instance --name instance-file destroy instance-1474873473
destroyed instance-1474873473
```

### Working with Group Plugin

```
$ build/cli group -h
Access group plugin

Usage:
  build/cli group [command]

Available Commands:
  describe    describe update (describe - or describe filename)
  destroy     destroy the group
  inspect     inspect the group
  stop        stop updating the group
  unwatch     unwatch the group
  update      update group (update < file or update filename)
  watch       watch the group

Flags:
      --name string   Name of plugin

Global Flags:
      --dir string   Dir path for plugin discovery (default "/run/infrakit/plugins")
      --log int      Logging level. 0 is least verbose. Max is 5 (default 4)

Use "build/cli group [command] --help" for more information about a command.
```

### Working with Flavor Plugin

```
$ build/cli flavor -h
Access flavor plugin

Usage:
  build/cli flavor [command]

Available Commands:
  healthy     checks for health
  prepare     prepare the provision data
  validate    validate input

Flags:
      --name string   Name of plugin

Global Flags:
      --dir string   Dir path for plugin discovery (default "/run/infrakit/plugins")
      --log int      Logging level. 0 is least verbose. Max is 5 (default 4)

Use "build/cli flavor [command] --help" for more information about a command.
```
