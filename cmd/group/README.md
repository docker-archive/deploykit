InfraKit Group Plugin
=====================

This is the default implementation of the Group Plugin that can manage collections of resources.
This plugin works in conjunction with the Instance and Flavor plugins, which separately define
the properties of the physical resource (Instance plugin) and semantics or nature  of the node
(Flavor plugin).


## Running

Begin by building plugin [binaries](../../README.md#binaries).

The plugin may be started without any arguments and will default to using unix socket in
`~/.infrakit/plugins` for communications with the CLI and other plugins:

```shell
$ b$ build/infrakit plugin start group
INFO[11-06|16:58:10] config                                   module=cli/plugin url= fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] Launching                                module=cli/plugin kind=group name=group-stateless fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] Starting plugin                          module=core/launch executor=inproc key=group name=group-stateless exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:58:10] Object is an event producer              module=rpc/server object=&{keyed:0xc4201862f8} discover=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:58:10] Listening                                module=rpc/server discover=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-06|16:58:10] Waiting for startup                      module=core/launch key=group name=group-stateless config="{\n\"Kind\": \"group\",\n\"Options\": {\n\"PollInterval\": \"10s\",\n\"MaxParallelNum\": 0,\n\"PollIntervalGroupSpec\": \"10s\",\n\"PollIntervalGroupDetail\": \"10s\"\n}\n}" as=group-stateless fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-06|16:58:10] Done waiting on plugin starts            module=cli/plugin fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-06|16:58:10] PID file created                         module=run path=/Users/infrakit/.infrakit/plugins/group-stateless.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-06|16:58:10] Server started                           module=run discovery=/Users/infrakit/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/run.run.func1
```
