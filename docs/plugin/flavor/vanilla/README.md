InfraKit Flavor Plugin - Vanilla
================================

A [reference](/README.md#reference-implementations) implementation of a Flavor Plugin that supports direct
injection of Instance fields.

While we can specify a list of logical ID's (for example, IP addresses), `Init` and `Tags`
are all statically defined in the config JSON.  This means all the members of the group are
considered identical.

You can name your cattle but they are still cattle.  Pets, however, would imply strong identity
*as well as* special handling.  This is done via the behavior provided by the `Prepare` method of
the plugin.  This plugin applies the static configuration.


## Schema

Here's a skeleton of this Plugin's schema:
```json
{
  "Init": [],
  "Tags": {},
  "InitScriptTemplateURL": "http://your.github.io/your/project/script.sh"
}
```

The supported fields are:
* `Init`: an array of shell code lines to use for the Instance's Init script
* `Tags`: a string-string mapping of keys and values to add as Instance Tags
* `InitScriptTemplateURL`: string URL where a init script template is served.  The plugin will fetch this
template from the URL and process the template to render the final init script for the instance.

Here's an example Group configuration using the default [infrakit/group](/cmd/group) Plugin and the Vanilla Plugin:
```json
{
  "ID": "cattle",
  "Properties": {
    "Allocation": {
      "Size": 5
    },
    "Instance": {
      "Plugin": "instance-vagrant",
      "Properties": {
        "Box": "bento/ubuntu-16.04"
      }
    },
    "Flavor": {
      "Plugin": "flavor-vanilla",
      "Properties": {
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

Or with assigned IDs:
```json
{
  "ID": "named-cattle",
  "Properties": {
    "Allocation": {
      "LogicalIDs": [
        "192.168.0.1",
        "192.168.0.2",
        "192.168.0.3",
        "192.168.0.4",
        "192.168.0.5"
      ]
    },
    "Instance": {
      "Plugin": "instance-vagrant",
      "Properties": {
        "Box": "bento/ubuntu-16.04"
      }
    },
    "Flavor": {
      "Plugin": "flavor-vanilla",
      "Properties": {
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


## Example

Begin by building plugin [binaries](../../../README.md#binaries).

This plugin will be called whenever you use a Flavor plugin and reference the plugin by name
in your config JSON.  For instance, you may start up this plugin as `french-vanilla`:

```shell
$ build/infrakit plugin start vanilla:french-vanilla simulator group:group
INFO[0000] Listening at: ~/.infrakit/plugins/french_vanilla
```

Now -- in another terminal session:

Your CLI should discover the new objects:

```shell
$ infrakit


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  french_vanilla    Access object french_vanilla which implements Flavor/0.1.0
  group             Access object group which implements Group/0.1.0,Metadata/0.1.0
  manager           Access the manager
  playbook          Manage playbooks
  plugin            Manage plugins
  remote            Manage remotes
  simulator/compute Access object simulator/compute which implements Instance/0.6.0
  simulator/disk    Access object simulator/disk which implements Instance/0.6.0
  simulator/lb1     Access object simulator/lb1 which implements L4/0.6.0
  simulator/lb2     Access object simulator/lb2 which implements L4/0.6.0
  simulator/lb3     Access object simulator/lb3 which implements L4/0.6.0
  simulator/net     Access object simulator/net which implements Instance/0.6.0
  template          Render an infrakit template at given url.  If url is '-', read from stdin
  up                Up everything
  util              Utilities
  version           Print build version information
  x                 Experimental features
```

Note `french_vanilla` and other objects are now accessible.

Commit a group using the `example.yml`

```shell
$ build/infrakit group commit -y docs/plugins/flavor/vanilla/example.yml
Committed vanilla: Managing 1 instances
```

Now we see one instance provisioned:

```shell
$ build/infrakit group ls
ID
vanilla
$ build/infrakit group describe vanilla
ID                            	LOGICAL                       	TAGS
1505889013394060520           	  -                           	infrakit.config.hash=rvhmljoz72va6rrmbypwsxahwkb6g6sq,infrakit.group=vanilla,project=infrakit,tier=web
```

Checking on the actual instance:

```shell
$ build/infrakit simulator/compute describe -pry
- ID: "1505889013394060520"
  LogicalID: null
  Properties:
    Attachments: null
    Init: |-
      sudo apt-get update -y
      sudo apt-get install -y nginx
      sudo service nginx start
    LogicalID: null
    Properties:
      Note: custom field
    Tags:
      infrakit.config.hash: rvhmljoz72va6rrmbypwsxahwkb6g6sq
      infrakit.group: vanilla
      project: infrakit
      tier: web
  Tags:
    infrakit.config.hash: rvhmljoz72va6rrmbypwsxahwkb6g6sq
    infrakit.group: vanilla
    project: infrakit
    tier: web
```

Note that the vanilla flavor (`french_vanilla`) has injected the init and
tags into the configuration of this instance.
