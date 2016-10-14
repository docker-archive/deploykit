InfraKit Flavor Plugin - Zookeeper
==================================

A [reference](../../../README.md#reference-implementations) implementation of a Flavor Plugin that creates an
[Apache ZooKeeper](https://zookeeper.apache.org/) ensemble.

This is a plugin for handling Zookeeper ensemble.

## Running

Begin by building plugin [binaries](../../../README.md#binaries).

Start the [vagrant instance plugin](/example/instance/vagrant):

```
$ build/infrakit-instance-vagrant
INFO[0000] Listening at: ~/.infrakit/plugins/instance-vagrant
```

Start the [Group plugin](/cmd/group):

```
$ build/infrakit-group-default
INFO[0000] Listening at: ~/.infrakit/plugins/group
```

Start Zookeeper flavor plugin:

```
$ build/infrakit-flavor-zookeeper
INFO[0000] Listening at: ~/.infrakit/plugins/flavor-zookeeper
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
$ build/infrakit group watch ./vagrant-zk-example.json
watching zk
```
