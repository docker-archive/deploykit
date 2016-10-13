InfraKit Flavor Plugin - Vanilla
================================

This is a plain vanilla flavor plugin that doesn't do anything special. However, it

  + Lets the user specify the [`AllocationMethod`](/spi/flavor/spi.go) which determines the size
  of the group by either specifying a size or by a list of logical ids.
  + Lets the user specify the labels to tag the instances with
  + Lets the user specify the `UserData` which determine the instance's `Init` (init script).

While we can specify a list of logical ID's (for example, IP addresses), the UserData and Labels
are all statically defined in the config JSON.  This means all the members of the group are
considered identical.

You can name your cattle but they are still cattle.  Pets, however, would imply strong identity
*as well as* special handling.  This is done via the behavior provided by the `Prepare` method of
the plugin.  This plugin simply applies the static configuration.


## Schema

Wherever the config JSON blob is (usually the value of a `Properties` field), the schema for the
configuration of this plugin looks like:

```
{
    "Size" : 5,

    "UserData" : [
        "sudo apt-get update -y",
        "sudo apt-get install -y nginx",
        "sudo service nginx start"
    ],

    "Labels" : {
        "tier" : "web",
        "project" : "infrakit"
    }
}
```

So in a larger config for the group -- the default [infrakit/group](/cmd/group) plugin -- the config
may look like:

```
{
    "ID": "cattle",
    "Properties": {
        "InstancePlugin": "instance-vagrant",
        "InstancePluginProperties": {
            "Box": "bento/ubuntu-16.04"
        },
        "FlavorPlugin": "flavor-plain",
        "FlavorPluginProperties": {
            "Size" : 5,

            "UserData" : [
                "sudo apt-get update -y",
                "sudo apt-get install -y nginx",
                "sudo service nginx start"
            ],

            "Labels" : {
                "tier" : "web",
                "project" : "infrakit"
            }
        }

    }
}
```

Or with assigned ID's:

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
            "LogicalIDs" : [
                "192.168.0.1",
                "192.168.0.2",
                "192.168.0.3",
                "192.168.0.4",
                "192.168.0.5"
            ],

            "UserData" : [
                "sudo apt-get update -y",
                "sudo apt-get install -y nginx",
                "sudo service nginx start"
            ],

            "Labels" : {
                "tier" : "web",
                "project" : "infrakit"
            }
        }

    }
}
```



## Building

Begin by building plugin [binaries](../../../README.md#binaries).

## Usage

```
$ build/infrakit-flavor-vanilla -h
Vanilla flavor plugin

Usage:
  build/infrakit-flavor-vanilla [flags]
  build/infrakit-flavor-vanilla [command]

Available Commands:
  version     print build version information

Flags:
      --listen string   listen address (unix or tcp) for the control endpoint (default "unix:///run/infrakit/plugins/flavor-vanilla.sock")
      --log int         Logging level. 0 is least verbose. Max is 5 (default 4)

Use "build/infrakit-flavor-vanilla [command] --help" for more information about a command.
```

## Example

This plugin will be called whenever you use a Flavor plugin and reference the plugin by name
in your config JSON.  For instance, you may start up this plugin as `french-vanilla`:

```shell
$ build/infrakit-flavor-vanilla --listen=unix:///run/infrakit/plugins/french-vanilla.sock
INFO[0000] Starting plugin                              
INFO[0000] Listening on: unix:///run/infrakit/plugins/french-vanilla.sock 
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/french-vanilla.sock err= <nil> 
```

Then in your JSON config for the default group plugin, you would reference it by name:

```json
{
    "ID": "cattle",
    "Properties": {
        "Instance" : {
            "Plugin": "instance-file",
            "Properties": {
                "Note": "Here is a property that only the instance plugin cares about"
            }
        },
        "Flavor": {
            "Plugin" : "french-vanilla",
            "Properties": {
                "Size" : 5,

                "UserData" : [
                    "sudo apt-get update -y",
                    "sudo apt-get install -y nginx",
                    "sudo service nginx start"
                ],

                "Labels" : {
                    "tier" : "web",
                    "project" : "infrakit"
                }
            }
        }
    }
}
```
Then when you watch a group with the config above (`cattle`), the cattle will be `french-vanilla` flavored.

Watch this group:
```
$ build/infrakit group --name group watch << EOF
> {
>     "ID": "cattle",
>     "Properties": {
>         "Instance" : {
>             "Plugin": "instance-file",
>             "Properties": {
>                 "Note": "Here is a property that only the instance plugin cares about"
>             }
>         },
>         "Flavor": {
>             "Plugin" : "french-vanilla",
>             "Properties": {
>                 "Size" : 5,
> 
>                 "UserData" : [
>                     "sudo apt-get update -y",
>                     "sudo apt-get install -y nginx",
>                     "sudo service nginx start"
>                 ],
> 
>                 "Labels" : {
>                     "tier" : "web",
>                     "project" : "infrakit"
>                 }
>             }
>         }
>     }
> }
> EOF
watching cattle

```
