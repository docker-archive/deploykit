# A Quick Tutorial

To illustrate the concept of working with Group, Flavor, and Instance plugins, we use a simple setup composed of
  + The default `group` plugin - to manage a collection of instances
  + The `file` instance plugin - to provision instances by writing files to disk
  + The `vanilla` flavor plugin - to provide context/ flavor to the configuration of the instances

All InfraKit plugins will by default open the unix socket located at /run/infrakit/plugins. Make sure this directory
exists on your host:

```shell
$ mkdir -p /run/infrakit/plugins/
$ chmod 777 /run/infrakit/plugins
```

Start the default Group plugin

```shell
$ build/infrakit-group-default --log 5
DEBU[0000] Opening: /run/infrakit/plugins
INFO[0000] Listening on: unix:///run/infrakit/plugins/group.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/group.sock err= <nil>
```

Start the file Instance plugin

```shell
$ mkdir -p tutorial
$ build/infrakit-instance-file --log 5 --dir ./tutorial/
INFO[0000] Listening on: unix:///run/infrakit/plugins/instance-file.sock
DEBU[0000] file instance plugin. dir= ./tutorial/
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/instance-file.sock err= <nil>
```
Note the directory `./tutorial` where the plugin will store the instances as they are provisioned.
We can look at the files here to see what's being created and how they are configured.

Start the vanilla Flavor plugin

```shell
$ build/infrakit-flavor-vanilla --log 5
INFO[0000] Listening on: unix:///run/infrakit/plugins/flavor-vanilla.sock
INFO[0000] listener protocol= unix addr= /run/infrakit/plugins/flavor-vanilla.sock err= <nil>
```

Show the plugins:

```shell
$ build/infrakit plugin ls
Plugins:
NAME                    LISTEN
flavor-vanilla          unix:///run/infrakit/plugins/flavor-vanilla.sock
group                   unix:///run/infrakit/plugins/group.sock
instance-file           unix:///run/infrakit/plugins/instance-file.sock
```

Note the names of the plugin.  We will use the names in the `--name` flag of the plugin CLI to refer to them.

Here we have a configuration JSON for the group.  In general, the JSON structures follow a pattern:

```json
{
   "SomeKey": "ValueForTheKey",
   "Properties": {
   }
}
```

The `Properties` field has a value of that is an opaque and correct JSON value.  This raw JSON value here
will configure the plugin specified. 

The plugins are free to define their own configuration schema.
In our codebase, we follow a convention.  The values of the `Properties` field are decoded using
a `Spec` Go struct.  The [`group.Spec`](/plugin/group/types/types.go) in the default Group plugin, and
[`vanilla.Spec`](/plugin/flavor/vanilla/flavor.go) are examples of this pattern.

For the File Instance Plugin, we have the spec:

```json
{
    "Note": "Instance properties version 1.0"
}
```

For the Vanilla Flavor Plugin, we have the spec:

```json
{
    "Size": 5,
    "UserData": [
        "sudo apt-get update -y",
        "sudo apt-get install -y nginx",
        "sudo service nginx start"
    ],
    "Labels": {
        "tier": "web",
        "project": "infrakit"
    }
}
```

The default Group plugin is a composition of the instance and flavor plugins and has the form:

```json
{
    "Instance": {
        "Plugin": "name of the instance plugin",
        "Properties": {
        }
    },
    "Flavor": {
        "Plugin": "name of the flavor plugin",
        "Properties": {
        }
    }
}
```

From listing the plugins earlier, we have two plugins running. `instance-file` is the name of the File Instance Plugin,
and `flavor-vanilla` is the name of the Vanilla Flavor Plugin.
So now we have the names of the plugins and their configurations.

Putting everything together, we have the configuration to give to the default Group plugin:

```json
{
    "ID": "cattle",
    "Properties": {
        "Instance": {
            "Plugin": "instance-file",
            "Properties": {
                "Note": "Instance properties version 1.0"
            }
        },
        "Flavor": {
            "Plugin": "flavor-vanilla",
            "Properties": {
                "Size": 5,
                "UserData": [
                    "sudo apt-get update -y",
                    "sudo apt-get install -y nginx",
                    "sudo service nginx start"
                ],

                "Labels": {
                    "tier": "web",
                    "project": "infrakit"
                }
            }
        }
    }
}
```

You can save the JSON above in a file (say, `group.json`)

Note that we specify the number of instances via the `Size` parameter in the `flavor-vanilla` plugin.  It's possible
that a specialized flavor plugin doesn't even accept a size for the group, but rather computes the optimal size based on
some criteria.

Checking for the instances via the CLI:

```shell
$ build/infrakit instance --name instance-file describe
ID                              LOGICAL                         TAGS

```

Let's tell the group plugin to `watch` our group by providing the group plugin with the configuration:

```shell
$ build/infrakit group --name group watch <<EOF
{
    "ID": "cattle",
    "Properties": {
        "Instance": {
            "Plugin": "instance-file",
            "Properties": {
                "Note": "Instance properties version 1.0"
            }
        },
        "Flavor": {
            "Plugin": "flavor-vanilla",
            "Properties": {
                "Size": 5,
                "UserData": [
                    "sudo apt-get update -y",
                    "sudo apt-get install -y nginx",
                    "sudo service nginx start"
                ],
                "Labels": {
                    "tier": "web",
                    "project": "infrakit"
                }
            }
        }
    }
}
EOF
watching cattle
```

**_NOTE:_** You can also specify a file name to load from instead of using stdin, like this:

```
$ build/infrakit group --name group watch group.json
```

The group plugin is responsible for ensuring that the infrastructure state matches with your specifications.  Since we
started out with nothing, it will create 5 instances and maintain that state by monitoring the instances:


```shell
$ build/infrakit group --name group inspect cattle
ID                              LOGICAL         TAGS
instance-1475104926           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475104936           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475104946           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475104956           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475104966           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
```

The Instance Plugin can also report instances, it will report all instances across all groups (not just `cattle`).

```shell
$ build/infrakit instance --name instance-file describe
ID                              LOGICAL         TAGS
instance-1475104926           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475104936           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475104946           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475104956           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475104966           	  -             infrakit.config_sha=Y23cKqyRpkQ_M60vIq7CufFmQWk=,infrakit.group=cattle,project=infrakit,tier=web
```

Now let's update the configuration by changing the size of the group and a property of the instance:

```json
{
    "ID": "cattle",
    "Properties": {
        "Instance": {
            "Plugin": "instance-file",
            "Properties": {
                "Note": "Instance properties version 2.0"
            }
        },
        "Flavor": {
            "Plugin": "flavor-vanilla",
            "Properties": {
                "Size": 10,
                "UserData": [
                    "sudo apt-get update -y",
                    "sudo apt-get install -y nginx",
                    "sudo service nginx start"
                ],
                "Labels": {
                    "tier": "web",
                    "project": "infrakit"
                }
            }
        }
    }
}
```

(You can also save the edits in a new file, `group2.json`).

```shell
$ diff group.json group2.json 
7c7
<                 "Note": "Instance properties version 1.0"
---
>                 "Note": "Instance properties version 2.0"
13c13
<                 "Size": 5,
---
>                 "Size": 10,
```
Before we do an update, we can see what the proposed changes are:

```
$ build/infrakit group --name group describe group2.json 
cattle : Performs a rolling update on 5 instances, then adds 5 instances to increase the group size to 10
```

So here 5 instances will be updated via rolling update, while 5 new instances at the new configuration will
be created.

Let's apply the new config:

```shell
$ build/infrakit group --name group update group2.json 

# ..... wait a bit...
update cattle completed
```
Now we can check:

```shell
$ build/infrakit group --name group inspect cattle
ID                              LOGICAL         TAGS
instance-1475105646           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105656           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105666           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105676           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105686           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105696           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105706           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105716           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105726           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
instance-1475105736           	  -             infrakit.config_sha=BXedrwY0GdZlHhgHmPAzxTN4oHM=,infrakit.group=cattle,project=infrakit,tier=web
```

Note the instances now have a new SHA `BXedrwY0GdZlHhgHmPAzxTN4oHM=` (vs `Y23cKqyRpkQ_M60vIq7CufFmQWk=` previously)

To see that the Group plugin can enforce the size of the group, let's simulate an instance disappearing.

```shell
$ rm tutorial/instance-1475105646 tutorial/instance-1475105686 tutorial/instance-1475105726

# ... now check

$ ls -al tutorial
total 104
drwxr-xr-x  15 davidchung  staff   510 Sep 28 16:40 .
drwxr-xr-x  36 davidchung  staff  1224 Sep 28 16:39 ..
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:34 instance-1475105656
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:34 instance-1475105666
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:34 instance-1475105676
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:34 instance-1475105696
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:35 instance-1475105706
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:35 instance-1475105716
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:35 instance-1475105736
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:40 instance-1475106016 <-- new instance
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:40 instance-1475106026 <-- new instance
-rw-r--r--   1 davidchung  staff   654 Sep 28 16:40 instance-1475106036 <-- new instance
```

We see that 3 new instance has been created to replace the three removed, to match our
original specification of 10 instances.

Finally, let's clean up:

```
$ build/infrakit group --name group destroy cattle
```

This concludes our quick tutorial.  In this tutorial we have
  + Started the plugins and learned to access them
  + Created a configuration for a group we want to watch
  + See the instances created to match the specifications
  + Updated the configurations of the group and scale up the group
  + Reviewed the proposed changes
  + Apply the update across the group
  + Removed some instances and see that the group self-healed
  + Destroyed the group
