InfraKit Flavor Plugin - Zookeeper
==================================

This is a plugin for handling Zookeeper ensemble.

## Building

When you do `make -k all` in the top level directory, the CLI binary will be built and can be
found as `./infrakit/cli` from the project's top level directory.

## Usage

```
$ ./infrakit/zookeeper -h
Zookeeper flavor plugin

Usage:
  ./infrakit/zookeeper [flags]
  ./infrakit/zookeeper [command]

Available Commands:
  version     print build version information

Flags:
      --listen string   listen address (unix or tcp) for the control endpoint (default "unix:///run/infrakit/plugins/flavor-zookeeper.sock")
      --log int         Logging level. 0 is least verbose. Max is 5 (default 4)

Use "./infrakit/zookeeper [command] --help" for more information about a command.
```

## Test

Start the [vagrant instance plugin](/example/instance/vagrant):

```
$ infrakit/vagrant
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/instance-vagrant.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/instance-vagrant.sock err= <nil>
```

Start the [Group plugin](/cmd/group):

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

Start Zookeeper flavor plugin:

```
$ ./infrakit/zookeeper
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/flavor-zookeeper.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/flavor-zookeeper.sock err= <nil>
```

Check everything's running:

```
$ ./infrakit/cli plugin ls
Plugins:
NAME                	LISTEN
flavor-zookeeper    	unix:///run/infrakit/plugins/flavor-zookeeper.sock
group               	unix:///run/infrakit/plugins/group.sock
instance-vagrant    	unix:///run/infrakit/plugins/instance-vagrant.sock
```

Here's a JSON for the group we'd like to see [vagrant-zk-example.json](./vagrant-zk-example.json):

```
{
    "ID": "zk",
    "Properties": {
        "Instance" : {
            "Plugin": "instance-vagrant",
            "Properties": {
                "Box": "bento/ubuntu-16.04"
            }
        },
        "Flavor" : {
            "Plugin": "flavor-zookeeper",
            "Properties": {
                "type": "member",
                "IPs": ["192.168.1.200", "192.168.1.201", "192.168.1.202"]
            }
        }
    }
}
```

Now tell the group plugin to watch the zk group, create if necessary:

```
infrakit/cli group --name group watch ./vagrant-zk-example.json
watching zk
```
