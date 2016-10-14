InfraKit CLI
============

This is a CLI for working with various infrakit plugins.  In the simplest form, an InfraKit plugin
is simply a daemon that communicates over unix domain sockets.  InfraKit plugins can find each
other by the socket files in a common directory.  The CLI uses the common directory as a discovery
mechanism and offers various subcommands for working with plugins. In general, plugin methods are
exposed as verbs and configuration JSON can be read from local file or standard input.

## Building

Begin by building plugin [binaries](../../README.md#binaries).

### List Plugins

```
$ build/infrakit plugin ls
Plugins:
NAME                	LISTEN
flavor-swarm        	~/.infrakit/plugins/flavor-swarm
flavor-zookeeper    	~/.infrakit/plugins/flavor-zookeeper
group               	~/.infrakit/plugins/group
instance-file       	~/.infrakit/plugins/instance-file
```

Once you know the plugins by name, you can make calls to them.  For example, the instance plugin
`instance-file` is a simple plugin that "provisions" an instance by writing the instructions to
a file in a local directory.

You can access the following plugins and their methods via command line:

  + instance
  + group
  + flavor

### Working with Instance Plugin

Using the plugin `instance-file` as an example:

`describe` calls the `DescribeInstances` endpoint of the plugin:

```
$ build/infrakit instance --name instance-file describe
ID                            	LOGICAL                       	TAGS
instance-1474850397           	  -                           	group=test,instanceType=small
instance-1474850412           	  -                           	group=test2,instanceType=small
instance-1474851747           	logic2                        	instanceType=small,group=test2
```

Validate - send the config JSON via stdin:

```
$ build/infrakit instance --name instance-file validate << EOF
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
$ build/infrakit instance --name instance-file provision << EOF
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
$ build/infrakit instance --name instance-file describe
ID                            	LOGICAL                       	TAGS
instance-1474850397           	  -                           	group=test,instanceType=small
instance-1474850412           	  -                           	group=test2,instanceType=small
instance-1474851747           	logic2                        	group=test2,instanceType=small
instance-1474873473           	logic2                        	group=test2,instanceType=small
```
Destroy

```
$ build/infrakit instance --name instance-file destroy instance-1474873473
destroyed instance-1474873473
```
