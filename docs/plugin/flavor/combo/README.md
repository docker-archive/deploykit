InfraKit Flavor Plugin - Combo
==============================

A [reference](/README.md#reference-implementations) implementation of a Flavor Plugin that supports composition
of other Flavors.

The Combo plugin allows you to use Flavors as mixins, combining their Instance properties:
  * `Tags`: combined, with any colliding values determined by the last Plugin to set them
  * `Init`: concatenated in the order of the configuration, separated by a newline
  * `Attachments`: combined in the order of the configuration

## Schema

Here's a skeleton of this Plugin's schema:
```json
[
  {
    "Plugin": "",
    "Properties": {
  },
  {
    "Plugin": "",
    "Properties": {
  },...
]
```


## Example

To demonstrate how the Combo Flavor plugin works, we will compose two uses of the Vanilla plugin together.

First, start up the plugins we will use:

```shell
$ build/infrakit plugin start group:group combo vanilla simulator
```
This will start up all the plugins running in a single process.

Now in another terminal session:

Your `infrakit` command shows:

```shell
$ infrakit


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  combo             Access object combo which implements Flavor/0.1.0
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
  vanilla           Access object vanilla which implements Flavor/0.1.0
  version           Print build version information
  x                 Experimental features
```

Check that there are no instances provisioned -- that we are in a clean state:

```shell
$ build/infrakit simulator/compute describe
ID                            	LOGICAL                       	TAGS
$ build/infrakit group ls
ID
```

Using the [example](example.yml) configuration, commit a group:
```shell
$ build/infrakit group commit -y docs/plugins/flavor/combo/example.yml
Committed combo: Managing 2 instances
```

Checking on the group:

```shell
$ build/infrakit group ls combo
ID
combo
$ build/infrakit group describe combo
ID                            	LOGICAL                       	TAGS
1505887656884092558           	  -                           	infrakit.config.hash=k4kacxuwykbyba6ydi36w6tjwj2c3plw,infrakit.group=combo,v1=tag one,v2=tag two
1505887656884528218           	  -                           	infrakit.config.hash=k4kacxuwykbyba6ydi36w6tjwj2c3plw,infrakit.group=combo,v1=tag one,v2=tag two
```

Note that now two instances are created and each instance has the tags from
the two chained flavors: `v1: tag one` and `v2: tag two`.

Get the full details of each instance:

```shell
$ build/infrakit simulator/compute  describe -pry
- ID: "1505887656884092558"
  LogicalID: null
  Properties:
    Attachments: []
    Init: |-
      vanilla one
      vanilla two
    LogicalID: null
    Properties:
      Note: custom field
    Tags:
      infrakit.config.hash: k4kacxuwykbyba6ydi36w6tjwj2c3plw
      infrakit.group: combo
      v1: tag one
      v2: tag two
  Tags:
    infrakit.config.hash: k4kacxuwykbyba6ydi36w6tjwj2c3plw
    infrakit.group: combo
    v1: tag one
    v2: tag two
- ID: "1505887656884528218"
  LogicalID: null
  Properties:
    Attachments: []
    Init: |-
      vanilla one
      vanilla two
    LogicalID: null
    Properties:
      Note: custom field
    Tags:
      infrakit.config.hash: k4kacxuwykbyba6ydi36w6tjwj2c3plw
      infrakit.group: combo
      v1: tag one
      v2: tag two
  Tags:
    infrakit.config.hash: k4kacxuwykbyba6ydi36w6tjwj2c3plw
    infrakit.group: combo
    v1: tag one
    v2: tag two
```

Note that the `Init` are also chained together in sequence:

```
- ID: "1505887656884528218"
  LogicalID: null
  Properties:
    Attachments: []
    Init: |-
      vanilla one   # From the first vanilla
      vanilla two   # From the second vanilla
    LogicalID: null
    Properties:
      Note: custom field
    Tags:
      infrakit.config.hash: k4kacxuwykbyba6ydi36w6tjwj2c3plw
      infrakit.group: combo
      v1: tag one
      v2: tag two
  Tags:
    infrakit.config.hash: k4kacxuwykbyba6ydi36w6tjwj2c3plw
    infrakit.group: combo
    v1: tag one
    v2: tag two
```
