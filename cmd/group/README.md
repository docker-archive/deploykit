InfraKit Group Plugin
=====================

This is the default implementation of the Group Plugin that can manage collections of resources.
This plugin works in conjunction with the Instance and Flavor plugins, which separately define
the properties of the physical resource (Instance plugin) and semantics or nature  of the node
(Flavor plugin).


## Running

Begin by building plugin [binaries](../../README.md#binaries).

The plugin may be started without any arguments and will default to using unix socket in
`/run/infrakit/plugins` for communications with the CLI and other plugins:

```
$ build/infrakit-group-default --log=5
DEBU[0000] Opening: /run/infrakit/plugins
DEBU[0000] Discovered plugin at unix:///run/infrakit/plugins/flavor-swarm.sock
DEBU[0000] Discovered plugin at unix:///run/infrakit/plugins/flavor-zookeeper.sock
DEBU[0000] Discovered plugin at unix:///run/infrakit/plugins/instance-file.sock
INFO[0000] Listening on: unix:///run/infrakit/plugins/group.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/group.sock err= <nil>
```
