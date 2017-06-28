InfraKit Flavor Plugin - Zookeeper
==================================

A [reference](/README.md#reference-implementations) implementation of a Flavor Plugin that creates an
[Apache ZooKeeper](https://zookeeper.apache.org/) ensemble.

This is a plugin for handling Zookeeper ensemble.

## Running

Begin by building plugin [binaries](/README.md#binaries).

Start the [vagrant instance plugin](/examples/instance/vagrant):

```shell
$ build/infrakit-instance-vagrant
INFO[0000] Listening at: ~/.infrakit/plugins/instance-vagrant
```

Start the [Group plugin](/cmd/group):

```shell
$ build/infrakit-group-default
INFO[0000] Listening at: ~/.infrakit/plugins/group
```

Start Zookeeper flavor plugin:

```shell
$ build/infrakit-flavor-zookeeper
INFO[0000] Listening at: ~/.infrakit/plugins/flavor-zookeeper
```

Be sure to verify that the plugin is [discoverable](/cmd/infrakit/README.md#list-plugins).

Here's a JSON for the group we'd like to see [vagrant-zk.json](./vagrant-zk.json):

<!-- blockcheck vagrant-zk.json -->
```json
{
  "ID": "zk",
  "Properties": {
    "Allocation": {
      "LogicalIDs": ["192.168.1.200", "192.168.1.201", "192.168.1.202"]
    },
    "Instance" : {
      "Plugin": "instance-vagrant",
      "Properties": {
        "Box": "bento/ubuntu-16.04"
      }
    },
    "Flavor" : {
      "Plugin": "flavor-zookeeper",
      "Properties": {
        "Type": "member"
      }
    }
  }
}
```

Now commit the group configuration:

```shell
$ build/infrakit group commit vagrant-zk.json
watching zk
```
