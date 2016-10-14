InfraKit Flavor Plugin - Vanilla
================================

A [reference](../../../README.md#reference-implementations) implementation of a Flavor Plugin that supports direct
injection of Instance fields.

It supports:
  + the [`AllocationMethod`](/spi/flavor/spi.go) to define the size or logical IDs of the Group
  + Instance `Tags` and `Init`

While we can specify a list of logical ID's (for example, IP addresses), `Init` and `Tags`
are all statically defined in the config JSON.  This means all the members of the group are
considered identical.

You can name your cattle but they are still cattle.  Pets, however, would imply strong identity
*as well as* special handling.  This is done via the behavior provided by the `Prepare` method of
the plugin.  This plugin simply applies the static configuration.


## Schema

Here's a skeleton of this Plugin's schema:
```
{
    "Size" : 0,
    "LogicalIDs": [],
    "Init" : [
    ],
    "Tags" : {
    }
}
```

The supported fields are:
* `Size`: the number of Instances in the Group, if this is a scaling Group
* `LogicalIDs`: the fixed logical identifiers for Group members, if this is a quorum Group
* `UserData`: an array of shell code lines to use for the Instance's Init script
* `Labels`: a string-string mapping of keys and values to add as Instance Tags

Here's an example Group configuration using the default [infrakit/group](/cmd/group) Plugin and the Vanilla Plugin:
```
{
    "ID": "cattle",
    "Properties": {
        "InstancePlugin": "instance-vagrant",
        "InstancePluginProperties": {
            "Box": "bento/ubuntu-16.04"
        },
        "FlavorPlugin": "flavor-vanilla",
        "FlavorPluginProperties": {
            "Size": 5,
            "Init": [
                "sudo apt-get update -y",
                "sudo apt-get install -y nginx",
                "sudo service nginx start"
            ],
            "Tags": {
                "tier": "web",
                "project": "infrakit"
            }
        }

    }
}
```

Or with assigned IDs:
```
{
    "ID": "named-cattle",
    "Properties": {
        "InstancePlugin": "instance-vagrant",
        "InstancePluginProperties": {
            "Box": "bento/ubuntu-16.04"
        },
        "FlavorPlugin": "flavor-vanilla",
        "FlavorPluginProperties": {
            "LogicalIDs": [
                "192.168.0.1",
                "192.168.0.2",
                "192.168.0.3",
                "192.168.0.4",
                "192.168.0.5"
            ],
            "Init": [
                "sudo apt-get update -y",
                "sudo apt-get install -y nginx",
                "sudo service nginx start"
            ],
            "Tags": {
                "tier": "web",
                "project": "infrakit"
            }
        }

    }
}
```


## Example

Begin by building plugin [binaries](../../../README.md#binaries).

This plugin will be called whenever you use a Flavor plugin and reference the plugin by name
in your config JSON.  For instance, you may start up this plugin as `french-vanilla`:

```shell
$ build/infrakit-flavor-vanilla --listen=unix:///run/infrakit/plugins/french-vanilla.sock
INFO[0000] Listening on: unix:///run/infrakit/plugins/french-vanilla.sock 
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/french-vanilla.sock err= <nil> 
```

Then in your JSON config for the default group plugin, you would reference it by name:

```json
{
    "ID": "cattle",
    "Properties": {
        "Instance": {
            "Plugin": "instance-file",
            "Properties": {
                "Note": "Here is a property that only the instance plugin cares about"
            }
        },
        "Flavor": {
            "Plugin": "french-vanilla",
            "Properties": {
                "Size": 5,
                "Init": [
                    "sudo apt-get update -y",
                    "sudo apt-get install -y nginx",
                    "sudo service nginx start"
                ],
                "Tags": {
                    "tier": "web",
                    "project": "infrakit"
                }
            }
        }
    }
}
```
Then when you watch a group with the configuration above (`cattle`), the cattle will be `french-vanilla` flavored.
