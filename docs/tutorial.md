# A Quick Tour of InfraKit

To illustrate the concept of working with Group, Flavor, and Instance plugins, we use a simple setup composed of
  + The default `group` plugin - to manage a collection of instances
  + The `file` instance plugin - to provision instances by writing files to disk
  + The `vanilla` flavor plugin - to provide context/ flavor to the configuration of the instances

For more information on plugins and how they work, please see the [docs](./plugins/README.md).

## Building the binaries

First, build the plugins:
```shell
$ make binaries
```

## Starting up InfraKit

InfraKit is made up of a collection of small microservice controllers that are commonly referred to as 'plugins'.
'Plugins' implement different Service Provider Interfaces in InfraKit:

   + The 'Instance' SPI is concerned with provisioning a resource
   + The 'Metadata' SPI provides a cluster-wide 'sysfs' where readable properties about the cluster are exposed
   and accessible as paths like a filesystem.
   + The 'Event' SPI allows the client to subscribe to topics (discoverable as paths) for events that are generated
   by the resources and controllers in the cluster.  For example, you could have a topic called `aws/ec2-instance/lost`
   that you can subscribe to receive notification when ec2 instances are lost in the cluster due to crashes or
   terminations.

InfraKit can be run in different ways such as in Docker containers or as simple daemons.  Here we are
going with the simple daemons that are built from source.  For a quick start with pre-built Docker containers,
you can take a look at the [Playbook](./playbooks/README.md).

There are many different plugins that InfraKit can use to provision resources.
In this tutorial we use the very basic file plugin, which simply creates files on disk.

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

## The CLI

As a user, you typically interact with the cluster and resources you provisioned via the `infrakit` CLI.
The CLI can connect to local system as well as remote clusters (called 'remotes').

Which remote or local target to connect to is controlled by the `INFRAKIT_HOST` environment variable.
When this variable is unset or not defined, `infrakit` CLI will look at local plugins which are
discoverable on your localhost at `$INFRAKIT_HOME/plugins`.

To see, add or remove remotes, you would use the `infrakit remote` subcommand. For example:

```shell
$ build/infrakit remote ls
HOST                          	URL LIST
docker4mac                    	localhost:24864
if1                           	54.219.137.138:24864
swarm1                        	52.53.247.176:24864,54.215.167.235:24864,54.193.100.40:24864
test1                         	54.215.224.155:24864
```

Once a 'remote' has been added, you can change the target of the CLI by setting the `INFRAKIT_HOST`
environment variable.

The `infrakit` CLI dynamically configures itself based on the set of plugins it has access to.

Show the plugins:

```shell
$ build/infrakit plugin ls
INTERFACE           LISTEN                                            NAME
Flavor/0.1.0        /Users/davidchung/.infrakit/plugins/flavor-vanillaflavor-vanilla
Group/0.1.0         /Users/davidchung/.infrakit/plugins/group         group
Metadata/0.1.0      /Users/davidchung/.infrakit/plugins/group         group
Instance/0.5.0      /Users/davidchung/.infrakit/plugins/instance-file instance-file
```

Doing a simple `infrakit -h` will show all the possible commands and options:

```shell
$ build/infrakit -h


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  event          Access event exposed by infrakit plugins
  flavor-vanilla Access plugin flavor-vanilla which implements Flavor/0.1.0
  group          Access plugin group which implements Group/0.1.0,Metadata/0.1.0
  instance-file  Access plugin instance-file which implements Instance/0.5.0
  manager        Access the manager
  metadata       Access metadata exposed by infrakit plugins
  playbook       Manage playbooks
  plugin         Manage plugins
  remote         Manage remotes
  template       Render an infrakit template at given url.  If url is '-', read from stdin
  util           Utilties
  version        Print build version information
  x              Experimental features

Flags:
      --httptest.serve string   if non-empty, httptest.NewServer serves on this address and blocks
      --log int                 log level (default 4)
      --log-caller              include caller function (default true)
      --log-format string       log format: logfmt|term|json (default "term")
      --log-stack               include caller stack
      --log-stdout              log to stdout
```

Note that in this case, we have three commands that are dynamically created for accessing
the running `flavor-vanilla`, `group`, `instance-file` plugins:

```
  flavor-vanilla Access plugin flavor-vanilla which implements Flavor/0.1.0
  group          Access plugin group which implements Group/0.1.0,Metadata/0.1.0
  instance-file  Access plugin instance-file which implements Instance/0.5.0
```

For example:

```shell
$ build/infrakit instance-file -h


Access plugin instance-file which implements Instance/0.5.0

Usage:
  infrakit instance-file [command]

Available Commands:
  describe    Describe all managed instances across all groups, subject to filter
  destroy     Destroy the instance
  info        print plugin info
  provision   Provisions an instance.  Read from stdin if url is '-'
  validate    Validates an flavor config.  Read from stdin if url is '-'
```

The verbs as available commands are available based on the interface the plugin object implements.

To list all the 'file' instances we have managed by the `instance-file` plugin, we simply do this:

```shell
infrakit@ Wed May 24-16:24:09 demo % infrakit instance-file describe
ID                            	LOGICAL                       	TAGS
```

At this point, we have no instances under management.  So let's provision some.

## Provision a Group of Instances

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
the `Properties` schema for each plugin.  The [`group.Spec`](../pkg/plugin/group/types/types.go) in the default Group plugin,
and [`vanilla.Spec`](../pkg/plugin/flavor/vanilla/flavor.go) are examples of this pattern.

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
        },
        "Attachments" :  [{"ID":"attachid", "Type": "attachtype"}]
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
$ build/infrakit instance-file describe
ID                              LOGICAL                         TAGS
```

Let's tell the group plugin to `commit` our group by providing the group plugin with the configuration:

```shell
$ build/infrakit group commit ./cattle.json
Committed cattle: Managing 5 instances
```

Checking with the group plugin, we should see a group called `cattle`:

```shell
$ build/infrakit group ls
ID
cattle
```

The group plugin is responsible for ensuring that the infrastructure state matches with your specifications.  Since we
started out with nothing, it will create 5 instances and maintain that state by monitoring the instances:
```shell
$ build/infrakit group describe cattle
ID                            	LOGICAL                       	TAGS
instance-4582464082013813178  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
instance-4657666275748037214  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
instance-5419344861148823408  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
instance-6391471917728203585  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
instance-7797144284686029457  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
```

The Instance Plugin can also report instances, it will report all instances across all groups (not just `cattle`).

```shell
$ build/infrakit instance-file describe
ID                            	LOGICAL                       	TAGS
instance-4582464082013813178  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
instance-4657666275748037214  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
instance-5419344861148823408  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
instance-6391471917728203585  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
instance-7797144284686029457  	  -                           	infrakit.config_sha=x4nrsdaibscmj7awpkxzx5vvpt6pilw2,infrakit.group=cattle,project=infrakit,tier=web
```

At any point you can safely `free` a group.  This is a non-destructive action, which instructs _InfraKit_ to cease
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
        },
        "Attachments" :  [{"ID":"attachid", "Type": "attachtype"}]
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
ID                            	LOGICAL                       	TAGS
instance-4857202356361893780  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-5331231286773283071  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-5527424552965675861  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-6711510839918342232  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-6870580757786415410  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-7164654173522392740  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-7416472420378252869  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-7470730388550100679  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-7585119672637883592  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-8916238683700118734  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
```

Note the instances now have a new SHA `jpfedp4sefvncwye5b6yre5qoz2odtnd` while perviously we had `x4nrsdaibscmj7awpkxzx5vvpt6pilw2`.

To see that the Group plugin can enforce the size of the group, let's simulate an instance disappearing.

For comparison, we capture the listing before we destroy instances
```shell
$ build/infrakit group describe cattle > before
```

```shell
$ rm ./tutorial/instance-4857202356361893780 ./tutorial/instance-8916238683700118734
```

After a few moments, let's capture the listing
```shell
$ build/infrakit group describe cattle > after
```

A quick diff shows that 2 instances have been replaced:

```shell
$ diff before after
2c2,3
< instance-4857202356361893780  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
---
> instance-2554519562373330601  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
> instance-3947516675797073281  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
11d11
< instance-8916238683700118734  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
```

Let's look at the instances:
```shell
$ build/infrakit group describe cattle
ID                            	LOGICAL                       	TAGS
instance-2554519562373330601  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-3947516675797073281  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-5331231286773283071  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-5527424552965675861  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-6711510839918342232  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-6870580757786415410  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-7164654173522392740  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-7416472420378252869  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-7470730388550100679  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
instance-7585119672637883592  	  -                           	infrakit.config_sha=jpfedp4sefvncwye5b6yre5qoz2odtnd,infrakit.group=cattle,project=infrakit,tier=web
```

We are back to 10 instances.

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


## Next Step

Now that you have completed the tutorial, it's time to explore the Playbooks.  Playbooks are 'scripts' that can
be shared and reused.  Playbooks can drive the `infrakit` CLI by defining new commands and flags.  A good one to
start is the [LinuxKit playbook](./docs/playbooks/linuxkit),
where we explore integration with [LinuxKit](https://github.com/linuxkit/linuxkit).
