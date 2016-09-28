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

When you do `make -k all` in the top level directory, the CLI binary will be built and can be
found as `./infrakit/cli` from the project's top level directory.

## Usage

```
$ infrakit/vanilla -h
Vanilla flavor plugin

Usage:
  infrakit/vanilla [flags]
  infrakit/vanilla [command]

Available Commands:
  version     print build version information

Flags:
      --listen string   listen address (unix or tcp) for the control endpoint (default "unix:///run/infrakit/plugins/flavor-vanilla.sock")
      --log int         Logging level. 0 is least verbose. Max is 5 (default 4)

Use "infrakit/vanilla [command] --help" for more information about a command.
```

## Example with the [File Instance Plugin](/example/instance/file)

Using the File instance plugin, we can very quickly work with Groups -- the instances that
are 'provisioned' are just files on disk and we can examine the result to see how the Group
plugin and Flavor plugins work together to provision instances through the Instance plugin.

```
{
    "ID": "cattle",
    "Properties": {
        "InstancePlugin": "instance-file",
        "InstancePluginProperties": {
            "Note": "Here is a property that only the instance plugin cares about"
        },
        "FlavorPlugin": "flavor-vanilla",
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

Start the plugins.

```
$ infrakit/file --log 5 --dir ./vanilla/
INFO[0000] Starting plugin
INFO[0000] Listening on: unix:///run/infrakit/plugins/instance-file.sock
DEBU[0000] file instance plugin. dir= ./vanilla/
INFO[0000] listener protocol= unix addr= /run/infrakit/
```

Look at our plugins:

```
$ infrakit/cli plugin ls
Plugins:
NAME                	LISTEN
flavor-vanilla        	unix:///run/infrakit/plugins/flavor-vanilla.sock
group               	unix:///run/infrakit/plugins/group.sock
instance-file       	unix:///run/infrakit/plugins/instance-file.sock
```

See what instances are there:

```
$ infrakit/cli instance --name instance-file describe
ID                            	LOGICAL                       	TAGS
```

Watch this group....

```
$ infrakit/cli group --name group watch << EOF
> {
>     "ID": "cattle",
>     "Properties": {
>         "InstancePlugin": "instance-file",
>         "InstancePluginProperties": {
>             "Note": "Here is a property that only the instance plugin cares about"
>         },
>         "FlavorPlugin": "flavor-vanilla",
>         "FlavorPluginProperties": {
>             "Size" : 5,
>
>             "UserData" : [
>                 "sudo apt-get update -y",
>                 "sudo apt-get install -y nginx",
>                 "sudo service nginx start"
>             ],
>
>             "Labels" : {
>                 "tier" : "web",
>                 "project" : "infrakit"
>             }
>         }
>     }
> }
> EOF
watching cattle

```

Now the Group plugin starts watching this group of `cattle` and will make sure there are
five instances:

```
$ ls -al vanilla/
total 40
drwxr-xr-x   7 davidchung  staff   238 Sep 27 22:08 .
drwxr-xr-x  38 davidchung  staff  1292 Sep 27 22:03 ..
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:07 instance-1475039263
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:07 instance-1475039273
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:08 instance-1475039283
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:08 instance-1475039293
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:08 instance-1475039303

```

Let's remove a couple of these:

```
$ rm vanilla/instance-1475039303 vanilla/instance-1475039293
```

and a short time, the Group plugin will create new instances to match the desired 5 instances:

```
ls -al vanilla
total 40
drwxr-xr-x   7 davidchung  staff   238 Sep 27 22:18 .
drwxr-xr-x  38 davidchung  staff  1292 Sep 27 22:12 ..
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:07 instance-1475039263
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:07 instance-1475039273
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:08 instance-1475039283
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:18 instance-1475039923 <----- NEW
-rw-r--r--   1 davidchung  staff   681 Sep 27 22:18 instance-1475039933 <----- NEW
```

Now, let's change the configuration...

```
$ infrakit/cli group --name group update << EOF
> {
>     "ID": "cattle",
>     "Properties": {
>         "InstancePlugin": "instance-file",
>         "InstancePluginProperties": {
>             "Note": "This is a new property"
>         },
>         "FlavorPlugin": "flavor-vanilla",
>         "FlavorPluginProperties": {
>             "Size" : 3,
>
>             "UserData" : [
>                 "sudo apt-get update -y",
>                 "sudo apt-get install -y nginx curl wget zookeeper",
>                 "sudo service nginx start"
>             ],
>
>             "Labels" : {
>                 "tier" : "web-new",
>                 "project" : "infrakit"
>             }
>         }
>     }
> }
> EOF
```

And now we have

```
$ ls -al vanilla
total 24
drwxr-xr-x   5 davidchung  staff   170 Sep 27 22:27 .
drwxr-xr-x  38 davidchung  staff  1292 Sep 27 22:12 ..
-rw-r--r--   1 davidchung  staff   643 Sep 27 22:26 instance-1475040413
-rw-r--r--   1 davidchung  staff   643 Sep 27 22:27 instance-1475040423
-rw-r--r--   1 davidchung  staff   643 Sep 27 22:27 instance-1475040433
```
