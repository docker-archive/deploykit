# A Quick Tutorial

To illustrate the concept of working with Group, Flavor, and Instance plugins, we use a simple setup composed of
  + The default `group` plugin - to manage a collection of instances
  + The `file` instance plugin - to provision instances by writing files to disk
  + The `vanilla` flavor plugin - to provide context/ flavor to the configuration of the instances

It may be helpful to familiarize yourself with [plugin discovery](../README.md#plugin-discovery) if you have not already
done so.

First, build the plugins:
```shell
$ make binaries
```

Start the default Group plugin

```shell
$ build/infrakit-group-default
INFO[0000] Listening at: ~/.infrakit/plugins/group
```

Start the file Instance plugin

```shell
$ mkdir -p tutorial
$ build/infrakit-instance-file --dir ./tutorial
INFO[0000] Listening at: ~/.infrakit/plugins/instance-file
```
Note the directory `./tutorial` where the plugin will store the instances as they are provisioned.
We can look at the files here to see what's being created and how they are configured.

Start the vanilla Flavor plugin

```shell
$ build/infrakit-flavor-vanilla
INFO[0000] Listening at: ~/.infrakit/plugins/flavor-vanilla
```

Show the plugins:

```shell
$ build/infrakit plugin ls
Plugins:
NAME                    LISTEN
flavor-vanilla          ~/.infrakit/plugins/flavor-vanilla
group                   ~/.infrakit/plugins/group
instance-file           ~/.infrakit/plugins/instance-file
```

Note the names of the plugin.  We will use the names in the `--name` flag of the plugin CLI to refer to them.

Now we must create the JSON for a group.  You will find that the JSON structures follow a pattern:

```json
{
   "Plugin": "PluginName",
   "Properties": {
   }
}
```

This defines the name of the `Plugin` to use and the `Properties` to configure it with.  The plugins are free to define
their own configuration schema.  Plugins in this repository follow a convention of using a `Spec` Go struct to define
the `Properties` schema for each plugin.  The [`group.Spec`](/plugin/group/types/types.go) in the default Group plugin,
and [`vanilla.Spec`](/plugin/flavor/vanilla/flavor.go) are examples of this pattern.

From listing the plugins earlier, we have two plugins running. `instance-file` is the name of the File Instance Plugin,
and `flavor-vanilla` is the name of the Vanilla Flavor Plugin.
So now we have the names of the plugins and their configurations.

Putting everything together, we have the configuration to give to the default Group plugin:

<!-- blockcheck cattle.json -->
```json
{
  "ID": "cattle",
  "Properties": {
    "Allocation": {
      "Size": 5
    },
    "Instance": {
      "Plugin": "instance-file",
      "Properties": {
        "Note": "Instance properties version 1.0"
      }
    },
    "Flavor": {
      "Plugin": "flavor-vanilla",
      "Properties": {
        "Init": [
          "docker pull nginx:alpine",
          "docker run -d -p 80:80 nginx-alpine"
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
*Save this as `cattle.json`*

Note that we specify the number of instances via the `Size` parameter in the `flavor-vanilla` plugin.  It's possible
that a specialized Flavor plugin doesn't even accept a size for the group, but rather computes the optimal size based on
some criteria.

Checking for the instances via the CLI:

```shell
$ build/infrakit instance --name instance-file describe
ID                              LOGICAL                         TAGS
```

Let's tell the group plugin to `commit` our group by providing the group plugin with the configuration:

```shell
$ build/infrakit group commit cattle.json
Committed cattle: Managing 5 instances
```

The group plugin is responsible for ensuring that the infrastructure state matches with your specifications.  Since we
started out with nothing, it will create 5 instances and maintain that state by monitoring the instances:
```shell
$ build/infrakit group describe cattle
ID                             	LOGICAL                        	TAGS
instance-5993795900014843850   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
instance-6529053068646043018   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
instance-7203714904652099824   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
instance-8430289623921829870   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
instance-9014687032220994836   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
```

The Instance Plugin can also report instances, it will report all instances across all groups (not just `cattle`).

```shell
$ build/infrakit instance --name instance-file describe
ID                             	LOGICAL                        	TAGS
instance-5993795900014843850   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
instance-6529053068646043018   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
instance-7203714904652099824   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
instance-8430289623921829870   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
instance-9014687032220994836   	  -                            	infrakit.config_sha=006438mMXW8gXeYtUxgf9Zbg94Y=,infrakit.group=cattle,project=infrakit,tier=web
```

At any point you can safely `release` a group.  This is a non-destructive action, which instructs _InfraKit_ to cease
active monitoring.  No instances are affected, but _InfraKit_ will no longer manage them.
```shell
$ build/infrakit group free cattle
Freed cattle
```

You can `commit` the group to start monitoring it again:
```shell
$ build/infrakit group commit cattle.json
Committed cattle: Managing 5 instances
```

Check which groups are being managed:
```shell
$ build/infrakit group ls
ID
cattle
```

Now let's update the configuration by changing the size of the group and a property of the instance.  Save this file as
`cattle2.json`:

<!-- blockcheck cattle2.json -->
```json
{
  "ID": "cattle",
  "Properties": {
    "Allocation": {
      "Size": 10
    },
    "Instance": {
      "Plugin": "instance-file",
      "Properties": {
        "Note": "Instance properties version 2.0"
      }
    },
    "Flavor": {
      "Plugin": "flavor-vanilla",
      "Properties": {
        "Init": [
          "docker pull nginx:alpine",
          "docker run -d -p 80:80 nginx-alpine"
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
*Save this as `cattle2.json`*

```shell
$ diff cattle.json cattle2.json 
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
```shell
$ build/infrakit group commit cattle2.json --pretend 
Committing cattle would involve: Performing a rolling update on 5 instances, then adding 5 instances to increase the group size to 10
```

So here 5 instances will be updated via rolling update, while 5 new instances at the new configuration will
be created.

Let's apply the new config:

```shell
$ build/infrakit group commit cattle2.json 
Committed cattle: Performing a rolling update on 5 instances, then adding 5 instances to increase the group size to 10
```

If we poll the group, we can see state will converging until all instances have been updated:
```shell
$ build/infrakit group describe cattle
ID                             	LOGICAL                        	TAGS
instance-1422140834255860063   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-1478871890164117825   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-1507972539885141336   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-1665488406863611296   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-2340140454359833670   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-2796731287627125229   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-285480170677988698    	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-4084455402433225349   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-5591036640758692177   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-6810420924276316298   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
```

Note the instances now have a new SHA `eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=` (vs `006438mMXW8gXeYtUxgf9Zbg94Y_M60vIq7CufFmQWk=` previously)

To see that the Group plugin can enforce the size of the group, let's simulate an instance disappearing.

```shell
$ rm tutorial/instance-1422140834255860063 tutorial/instance-1478871890164117825 tutorial/instance-1507972539885141336
```

After a few moments, the missing instances will be replaced (we've highlighted new instances with `-->`):
```shell
$ build/infrakit group describe cattle
ID                             	LOGICAL                        	TAGS
--> instance-1265288729718091217   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-1665488406863611296   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
--> instance-1952247477026188949   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-2340140454359833670   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-2796731287627125229   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-285480170677988698    	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-4084455402433225349   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
--> instance-4161733946225446641   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-5591036640758692177   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
instance-6810420924276316298   	  -                            	infrakit.config_sha=eB2JuP0c5Sf41X5e2vc2gJ4ZTVg=,infrakit.group=cattle,project=infrakit,tier=web
```

We see that 3 new instance have been created to replace the three removed, to match our
original specification of 10 instances.

Finally, let's clean up:

```shell
$ build/infrakit group destroy cattle
```

This concludes our quick tutorial.  In this tutorial we:
  + Started the plugins and learned to access them
  + Created a configuration for a group we wanted to manage
  + Verified the instances created matched the specifications
  + Updated the configurations of the group and scaled up the group
  + Reviewed the proposed changes
  + Applied the update across the group
  + Removed some instances and observed that the group self-healed
  + Destroyed the group
