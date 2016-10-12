InfraKit Flavor Plugin - Zookeeper
==================================

This is a plugin for handling Zookeeper ensemble.

## Running

Begin by building plugin [binaries](../../../README.md#binaries).

Start the [vagrant instance plugin](/example/instance/vagrant):

```
$ build/infrakit-instance-vagrant
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/instance-vagrant.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/instance-vagrant.sock err= <nil>
```

Start the [Group plugin](/cmd/group):

```
$ build/infrakit-group-default --log=5
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
$ build/infrakit-flavor-zookeeper
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/flavor-zookeeper.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/flavor-zookeeper.sock err= <nil>
```

Be sure to verify that the plugin is [discoverable](../../../cmd/cli/README.md#list-plugins).

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
        }
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
$ build/infrakit group --name group watch ./vagrant-zk-example.json
watching zk
```
