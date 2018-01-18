# A Quick Tour of InfraKit


## Overview

InfraKit is made up of a collection of small microservice controllers that are commonly referred to as 'plugins'.
'Plugins' implement different Service Provider Interfaces (SPI) in InfraKit:

   + The 'Instance' SPI is concerned with provisioning a resource.  Examples include the `aws` plugin which can
   provision resources such as `ec2-instance` or `ec2-spot-instance`, or `ec2-subnet`.
   + The 'Flavor' SPI is concerned with injecting application-specific configurations as well as implementing
   application-specific health checks. Examples of a flavor includes the Swarm and Kubernetes plugins, where they
   can inject cluster join tokens or perform application-specific operations like node drains.
   + The 'Metadata' SPI provides a cluster-wide 'sysfs' where readable properties about the cluster are exposed
   and accessible as paths like a filesystem.  There are dedicated plugins that implement this interface, as
   well as, instance plugins that expose this interface in addition to the instance provisioning SPI (see above).
   + The 'Event' SPI allows the client to subscribe to topics (discoverable as paths) for events that are generated
   by the resources and controllers in the cluster.  For example, you can have a topic called `aws/ec2-instance/lost`
   that you can subscribe to receive notification when ec2 instances are lost in the cluster due to crashes or
   terminations.

In addition to plugins that implement these interfaces, there are controllers.  Controllers in general care about
state, and they are the entities that actively monitor the infrastructure and drives convergence towards the user's
specification.  Examples of controllers include:

   + Group controller -- this controller manages groups of instances.  It allows the scale-up/down of the groups,
   as well as managing rolling updates when group configuration changes.
   + Ingress controller -- this controller manages connecting traffic via routes to backends.  This controller
   synchronizes the routes and backend nodes for a loadbalancer (which is also a plugin that implements the
   L4 SPI). For example, this controller ensures as a Group scales up, new node members are added to the backend
   pool of the L4 loadbalancer it manages.  When the routes change, this controller also ensures they are reflected
   in the L4 loadbalancer as well.
   + Enrollment controller -- this controller watches the Group controller for group membership changes and makes
   sure that the group membership is reflected 1 to 1 in an instance plugin.   For example, the enrollment controller
   can watch a Group of hosts and for each node added/ removed from the group, an Instance is added / removed in the
   downstream Instance plugin that manages NFS volume authorization.  So when the group scales up, new nodes are
   also authorized for NFS volume access.

Because these SPI are generic and reflect common patterns, complex use cases can be supported via composition and
delegation.  For example, the Instance SPI is a generic interface for creating and removing resources.  By implementing
the same interface, there are 'selectors' that instead of creating / destroying resources themselves, delegate
calls to other Instance plugins.  So it's possible to compose new instance plugins to support complex use cases:

   + A cross-zone / cross-cloud Instance plugin that can provision compute across one or more zones, regions, or clouds:
       - Weighted - where the instances are distributed across the Instance plugins according to a set of weights.
       - Spread - where the instances are distributed evenly across all the delegate Instance plugins.
   + Tiered - an instance plugin that tries to provision instances according to priority.  For example, a Vsphere
   instance plugin may be first to provision a new vm instances, and if there are no nodes available, uses a second
   public cloud, on-demand instance plugin to provision the instance.


InfraKit can be run in different ways such as in Docker containers or as simple daemons.  Here we are
going with the simple daemons that are built from source.  For a quick start with pre-built Docker containers,
you can take a look at the [Playbook](../playbooks/README.md).

## Tutorial

To illustrate the concept of working with Group, Flavor, and Instance plugins, we use a simple setup composed of
  + The default `group` plugin - to manage a collection of instances
  + The `simulator` instance plugin - to simulate provisioning of various resource types.
  + The `vanilla` flavor plugin - to provide context/ flavor to the configuration of the instances

For more information on plugins and how they work, please see the [docs](../plugins/README.md).

### Building infrakit

For the tutorial, we only need the `infrakit` binary.  Build it:
```shell
$ make build/infrakit
$ cp build/infrakit /usr/local/bin/
```

### Starting up InfraKit

There are some basic environment set up required for infrakit.  Infrakit uses
the environment variable `INFRAKIT_HOME` to locate the directory where it stores
various files and create socket files for the various plugins that the daemon
starts.  The `INFRAKIT_HOME` directory is typically `~/.infrakit`.

```shell
$ # Do this for the first time... Infrakit supports HA out of box and this sets
$ # the identity of the leader
$ mkdir -p ~/.infrakit/
$ export INFRAKIT_HOME=~/.infrakit
$ echo manager1 > $INFRAKIT_HOME/leader
```

The statements above sets the directory and specifies the identity of the leader
manager (`manager1`) using a file as a way to store the leader's identity (other
backends are available, but we are using the file backend for now).

In one terminal session, start infrakit with the group, vanilla, and simulator plugins:

Starting up the daemon:

```shell
$ infrakit plugin start manager group vanilla simulator
```

Leave this running, and use another terminal session for the CLI.

### The CLI

As a user, you typically interact with the cluster and resources you provisioned via the `infrakit` CLI.
The `infrakit` CLI is used to start up servers as well as functions as a client.  A client can be used
to manage multiple infrakit installations, where each environment is referred to as a 'remote'.
In this case, the `INFRAKIT_HOST` environment variable is not set, so the `infrakit` CLI is pointing
to the local infrakit instance you just started.

First, let's see that the plugins are running:

```shell
$ infrakit plugin ls
INTERFACE           NAME                          LISTEN
INTERFACE           NAME                          LISTEN
Group/0.1.0         group-stateless               /Users/davidchung/.infrakit/plugins/group-stateless
Metadata/0.1.0      group-stateless               /Users/davidchung/.infrakit/plugins/group-stateless
Controller/0.1.0    group                         /Users/davidchung/.infrakit/plugins/group
Group/0.1.0         group                         /Users/davidchung/.infrakit/plugins/group
Manager/0.1.0       group                         /Users/davidchung/.infrakit/plugins/group
Metadata/0.1.0      group                         /Users/davidchung/.infrakit/plugins/group
Updatable/0.1.0     group                         /Users/davidchung/.infrakit/plugins/group
Instance/0.6.0      simulator/compute             /Users/davidchung/.infrakit/plugins/simulator
Instance/0.6.0      simulator/disk                /Users/davidchung/.infrakit/plugins/simulator
L4/0.6.0            simulator/lb1                 /Users/davidchung/.infrakit/plugins/simulator
L4/0.6.0            simulator/lb2                 /Users/davidchung/.infrakit/plugins/simulator
L4/0.6.0            simulator/lb3                 /Users/davidchung/.infrakit/plugins/simulator
Instance/0.6.0      simulator/net                 /Users/davidchung/.infrakit/plugins/simulator
Flavor/0.1.0        vanilla                       /Users/davidchung/.infrakit/plugins/vanilla
```

Here we can see that the plugins are listening at various sockets (e.g. at `/Users/me/.infrakit/plugins/group`)
and the plugin objects can expose different SPI (e.g. `Instance/0.6.0` and `Flavor/0.1.0`).

As a client, `infrakit` is able to discover a set of services in a running infrakit instance and dynamically
configures its command hierarchy based on the objects that it discovers:


```shell
$ infrakit

infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  group             Access object group which implements Controller/0.1.0,Group/0.1.0,Manager/0.1.0,Metadata/0.1.0,Updatable/0.1.0
  group-stateless   Access object group-stateless which implements Group/0.1.0,Metadata/0.1.0
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

Note that because we have the `group`, `vanilla`, and `simulator` plugins running, the CLI
has commands for interacting with them:

```
  group             Access object group-stateless which implements Group/0.1.0,Metadata/0.1.0
  simulator/compute Access object simulator/compute which implements Instance/0.6.0
  simulator/disk    Access object simulator/disk which implements Instance/0.6.0
  simulator/lb1     Access object simulator/lb1 which implements L4/0.6.0
  simulator/lb2     Access object simulator/lb2 which implements L4/0.6.0
  simulator/lb3     Access object simulator/lb3 which implements L4/0.6.0
  simulator/net     Access object simulator/net which implements Instance/0.6.0
  vanilla           Access object vanilla which implements Flavor/0.1.0
```

For example, trying the `simulator/compute` subcommand:

```shell
$ infrakit simulator/compute -h


Access object simulator/compute which implements Instance/0.6.0

Usage:
  infrakit simulator/compute [command]

Available Commands:
  describe    Describe all managed instances across all groups, subject to filter
  destroy     Destroy the instance
  info        print plugin info
  provision   Provisions an instance.  Read from stdin if url is '-'
  validate    Validates an flavor config.  Read from stdin if url is '-'
```

In the context of the `simulator/compute` plugin, which represents simulated `compute`
resource, there are now verbs such as `provision` (create), `destroy` (terminate), and
`describe`.

To list all the `simulator/compute` instance, we simply do:

```shell
infrakit@ Wed May 24-16:24:09 demo % infrakit simulator/compute describe
ID                            	LOGICAL                       	TAGS
```
At this point, we have no instances under management.  So let's provision some.

### Provision a Single Instance

You can use the `provision` verb on a instance plugin to provision an instance
of the resource it represents. To create a single instance of `simulator/compute`
we create a simple YAML (or JSON) that specifies the property of the single instance:

```yaml
# When provisioning a single instance, you can specify
# tags that are associated with the instance.
Tags:
  custom.tag1 : tutorial
  custom.tag2 : single-instance

# Each instance has a notion of initialization. Often, on
# public clouds this would correspond to some compute note
# bootscript or cloud-init.
Init: |
  #!/bin/bash
  sudo apt-get update -y
  sudo apt-get install wget curl
  wget -qO- https://get.docker.com | sh

# Properties contain properties that are important for the downstream
# systems that interfaces with this plugin plugin.  Often these
# map to the specific API structures that are used to provision an instance.
Properties:
  apiProperty1 : value1
  apiProperty2 : value2
```

The specification of a single instance consists of three sections:

  + Tags - for associating some user-defined metadata with the instance.  This
  section has the structure of key: value (a map).
  + Init - for compute instances, this is typically the scripts used on boot.
  This section is a string and you can use the `|` operator in YAML to include
  complex scripts.
  + Properties - these are properties often required by the backend systems to
  provision a physical instance.  This section follows the format required by
  the backend and is generally treated as a blob.

It's also useful to note that this specification YAML itself can be a Go-style
template.  So you can embed functions and variables and even include other templates,
and Infrakit's templating engine can perform string interpolation and render the
template before sending it to the plugin.  Templating functions supported include:

  + Sprig [functions](https://github.com/Masterminds/sprig)
  + Functions for including, sourceing of other templates and setting global scoped
  variables, as well as utility functions for decoding/ encoding of JSON, YAML, and
  HCL content.

Having created a specification YAML, let's create an instance:

```shell
$ infrakit simulator/compute provision -y ./single.yml
1506515055431980898
```

The argument `./single.yml` is the path of local file, or a URL.  This makes it
possible to reuse templated specs from Github or other sources.  The `-y` options
tells Infrakit to interpet the input as YAML (because it accepts JSON by default).

Now list to see the instance:

```shell
$ infrakit simulator/compute describe
ID                            	LOGICAL                       	TAGS
1506515055431980898           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance
```

At this point, a single instance has been created.  Doing a `simulator/compute describe`
lists the instances known by this plugin.

You can also get detailed information about this instance:

```shell
$ infrakit simulator/compute describe -pry
- ID: "1506515055431980898"
  LogicalID: null
  Properties:
    Attachments: null
    Init: |
      #!/bin/bash
      sudo apt-get update -y
      sudo apt-get install wget curl
      wget -qO- https://get.docker.com | sh
    LogicalID: null
    Properties:
      apiProperty1: value1
      apiProperty2: value2
    Tags:
      custom.tag1: tutorial
      custom.tag2: single-instance
  Tags:
    custom.tag1: tutorial
    custom.tag2: single-instance
```

The `-p` option shows the properties of the instances, while the `r` and `y` options
mean showing the properties in `r`aw format as `y`AML.  

You can also apply a template to select the property of interest to the output:

```shell
$ infrakit simulator/compute describe -p --properties-view 'str://{{.Properties.apiProperty1}}'
ID                            	LOGICAL                       	TAGS                          	PROPERTIES
1506515055431980898           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance	value1
```
You have successfully created your first instance using the Instance plugin.

Now let's terminate this instance.  First we `describe` the instances to get a listing
of the instances known.  Then we take one of the instance ID's and use that as input
to the `destroy` verb:

```shell
$ infrakit simulator/compute describe
ID                            	LOGICAL                       	TAGS
1506515055431980898           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance
$ infrakit simulator/compute destroy 1506515055431980898
destroyed 1506515055431980898
$ infrakit simulator/compute describe
ID                            	LOGICAL                       	TAGS
```

The instance that we provisioned early has been terminated and removed.

Layering and composition are key design tenets of Infrakit.  Now that we can
provision and destroy single instances via the Instance plugins, let's layer on this
with managing groups or collections of instances via the Group controller.

### Provision a Group of Instances

While instance plugins provide methods for creating, terminating and describing
resources such as compute (e.g. `simulator/compute`), the Group controller and other
types of controllers form layers on top of the instance plugins to provide
higher-level abstractions such as groups and enrollments.

As a layer above the simple Instance plugins, the Group controller provides a
collection or scaling group-like abstraction on top of instance plugins which
handle single, disparate instances of resources.  With the Group abstraction,
you can manipulate the collection of instances such as scaling up (add more instaces)
or down (removing instances), or perform a rolling update.  In addition, the
Group controller will ensure the state of the infrastructure matches your
specification.  When a node crashes, for example, the Group controller will
spin up a new replacement.

In addition to the Instance plugin which performs the actual infrastructure CRUD
operations, the Group controller also depends on a 'Flavor` plugin.
Conceptually, you can think of the Flavor plugin as a mix-in or decorator of
configurations that are used for Instances.  Flavor plugins (e.g. one for
Docker Swarm mode) embody application specific logic to:

  + decorate or inject additional init script -- for example, injecting
  `docker swarm join` commands with join tokens,
  + performing health checks -- for example, making sure that the node is in fact a
  member of the Swarm cluster,
  + perform 'drains' or pre-termination processes prior to an instance's termination
  on the `Destroy` command.

Because of its layering / dependence on the Instance and Flavor plugins, the Group
controller specification is a composition of an Instance plugin specification (which
we used in provisioning the `simulator/compute` instance earlier), and a Flavor
plugin specification.

For example, let's create this specification:

```yaml
#
#  A group of workers
#
#  Start up -- plugin start should include manager, vanilla, simulator, and group
#  Then commit
#
#  infrakit group controller commit -y docs/tutorial/group.yml
#
kind: group
metadata:
    name: workers
properties:
    Allocation:
      Size: 5
    Flavor:
      Plugin: vanilla
      Properties:
        Init:
          - sudo apt-get update -y
          - sudo apt-get install wget curl
          - wget -qO- https://get.docker.com | sh
        Tags:
          custom.tag1 : tutorial
          custom.tag2 : single-instance
          custom.tag3 : by-group

    Instance:
      Plugin: simulator/compute
      # This section here for the Instance plugin is the same as the example
      # for creating a single instance.  The Tags and Init sections are now
      # handled by the Flavor plugin
      Properties:
        apiProperty1 : value1
        apiProperty2 : value2

```

In this spec, we see that there is a group called `workers`.  It uses the `vanilla`
Flavor plugin as well as the `simulator/compute` Instance plugin.  Note this spec
is in essence a composition of two plugins with some additional properties like
the size of the group (in the `Allocation` section, property `Size`).

To create this group, do

```shell
$ infrakit group controller commit -y ./group.yml
kind: group
metadata:
  name: workers
  tags: null
properties:
  Allocation:
    Size: 5
  Flavor:
    Plugin: vanilla
    Properties:
      Init:
      - sudo apt-get update -y
      - sudo apt-get install wget curl
      - wget -qO- https://get.docker.com | sh
      Tags:
        custom.tag1: tutorial
        custom.tag2: single-instance
        custom.tag3: by-group
  Instance:
    Plugin: simulator/compute
    Properties:
      apiProperty1: value1
      apiProperty2: value2
version: ""
```

Now let's look at the CLI:

```shell
$ infrakit -h


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  group             Access object group which implements Controller/0.1.0,Group/0.1.0,Manager/0.1.0,Metadata/0.1.0,Updatable/0.1.0
  group-stateless   Access object group-stateless which implements Group/0.1.0,Metadata/0.1.0
  group/workers     Access object group/workers which implements Controller/0.1.0,Group/0.1.0
  manager           Access the manager
```

Note that a new command now appears (`group/workers`) in the CLI.  This is the
subcommand for you to interact with the group that was created:

```shell
$ infrakit group/workers -h


Access object group/workers which implements Controller/0.1.0,Group/0.1.0

Usage:
  infrakit group/workers [command]

Available Commands:
  commit            Commit a group configuration. Read from stdin if url is '-'
  controller        Commands to access the Controller SPI
  describe          Describe a group. Returns a list of members
  destroy           Destroy a group by terminating and removing all members from infrastructure
  destroy-instances Destroy a group's instances
  free              Free a group nonedestructively from active monitoring
  info              print plugin info
  inspect           Inspect a group. Returns the raw configuration associated with a group
  ls                List groups
  scale             Returns size of the group if no args provided. Otherwise set the target size.
```

### Working with Groups

To show the instances in this group:

```shell
$ infrakit group/workers describe
ID                            	LOGICAL                       	TAGS
1506521294112164848           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521294112934083           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521294113965200           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521294108257066           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521294108791344           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
```

Note that we now have 5 instances in this group as specified.  Doing a
`infrakit group/workers describe -ry` will return a big raw YAML dump of all the
instances with their properties.

Note that the instances all have additional tags such as `infrakit.config` and
`infrakit.group` in addition to the tags specified in our YAML file.  This is
how the Group controller can keep track of instances in a group -- by injecting
additional metadata via tags.

We can look at the 'scale' or size of the group:

```shell
$ infrakit group/workers scale
Group workers target size= 5
```

Scaling up (adding more nodes):

```shell
$ infrakit group/workers scale 8
Group workers at 5 instances, scale to 8
```

After a short while, let's check:

```shell
$ infrakit group/workers scale
Group workers at 8 instances
$ infrakit group/workers describe
ID                            	LOGICAL                       	TAGS
1506521644119689998           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644120385464           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644121107015           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644121475146           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644119306272           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644118947079           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644120035107           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644120738724           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
```

We can use the Instance plugin to remove some instances:

```shell
$ infrakit simulator/compute destroy 1506521644118947079 1506521644120035107
destroyed 1506521644118947079
destroyed 1506521644120035107
```

and see that the group controller will maintain the original specification:

```shell
$ infrakit group/workers scale
Group workers at 8 instances
$ infrakit group/workers describe
ID                            	LOGICAL                       	TAGS
1506521644119306272           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644119689998           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644120385464           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644121107015           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644121475146           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521644120738724           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521824108334555           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521824108717234           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
```

We can scale down the group:

```shell
$ infrakit group/workers scale 2
Group workers at 8 instances, scale to 2
$ infrakit group/workers describe
ID                            	LOGICAL                       	TAGS
1506521824108334555           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers
1506521824108717234           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=oskdoyfaewtrrmwlychnja3ejwz574ue,infrakit.group=workers

```

We can also change the configuration of the group, which will trigger a rolling
update.  In this example, let's change the properties of the instance plugins:

```yaml
#
#  A group of workers
#
#  Start up -- plugin start should include manager, vanilla, simulator, and group
#  Then commit
#
#  infrakit group controller commit -y docs/tutorial/group.yml
#
kind: group
metadata:
    name: workers
properties:
    Allocation:
      Size: 5
    Flavor:
      Plugin: vanilla
      Properties:
        Init:
          - sudo apt-get update -y
          - sudo apt-get install wget curl
          - wget -qO- https://get.docker.com | sh
        Tags:
          custom.tag1 : tutorial
          custom.tag2 : single-instance
          custom.tag3 : by-group

    Instance:
      Plugin: simulator/compute
      # This section here for the Instance plugin is the same as the example
      # for creating a single instance.  The Tags and Init sections are now
      # handled by the Flavor plugin
      Properties:
        apiProperty1 : value1
        apiProperty2 : new value # <---- change this value

```

Using `group2.yml` as input, we commit the changes again:


```shell
$ infrakit group/workers controller commit -y ./group2.yml
kind: group
metadata:
  name: workers
  tags: null
properties:
  Allocation:
    Size: 5
  Flavor:
    Plugin: vanilla
    Properties:
      Init:
      - sudo apt-get update -y
      - sudo apt-get install wget curl
      - wget -qO- https://get.docker.com | sh
      Tags:
        custom.tag1: tutorial
        custom.tag2: single-instance
        custom.tag3: by-group
  Instance:
    Plugin: simulator/compute
    Properties:
      apiProperty1: value1
      apiProperty2: new value
version: ""
```

Let's check:

```shell
$ infrakit group/workers describe
ID                            	LOGICAL                       	TAGS
1506522342940736322           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=d2m6ncxw5kprf7j35tpl32pcygskuegh,infrakit.group=workers
1506522362939812225           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=d2m6ncxw5kprf7j35tpl32pcygskuegh,infrakit.group=workers
1506522362940587401           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=d2m6ncxw5kprf7j35tpl32pcygskuegh,infrakit.group=workers
1506522352940180493           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=d2m6ncxw5kprf7j35tpl32pcygskuegh,infrakit.group=workers
1506522362940219804           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=d2m6ncxw5kprf7j35tpl32pcygskuegh,infrakit.group=workers
```

We see that we have come back to 5 instances in this group (as specified in `group2.yml`).
In addition, the instances are actually different now, as indicated by a different
SHA (`d2m6ncxw5kprf7j35tpl32pcygskuegh` compared to `oskdoyfaewtrrmwlychnja3ejwz574ue`).
Also, the instances all have new IDs now.


Finally, let's destroy the group:

```shell
$ infrakit group/workers destroy workers
Destroy workers initiated
```

The CLI will update itself to remove the `group/workers` subcommand:

```shell
$ infrakit -h


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  group             Access object group which implements Controller/0.1.0,Group/0.1.0,Manager/0.1.0,Metadata/0.1.0,Updatable/0.1.0
  group-stateless   Access object group-stateless which implements Group/0.1.0,Metadata/0.1.0
  manager           Access the manager
  playbook          Manage playbooks
  plugin            Manage plugins
```

And the group `workers` is no longer known to Infrakit:

```shell
$ infrakit group/workers describe
CRIT[09-27|07:20:25] error executing                          module=main cmd=infrakit err="Group 'workers' is not being watched" fn=main.main
Group 'workers' is not being watched
```

### Use Real Plugins

In this tutorial, we used the `simulator` in `infrakit plugin start` to simulate
a number of different instance plugins (e.g. `ondemand` and `us-east-1a`).  Applying
the patterns here to real plugins involve the following steps:

  1. Start up the real plugins you intend to use in `infrakit plugin start`.
  2. Modify the configuration YAML to include the actual `Properties` required
  by the real plugin.

For example, for AWS, you can run multiple instances of the Instance plugin listening
at different socket names just like how we ran the `simulator`.
An instance plugin for `us-east-1a` availability zone and another in `us-east-1b` can be
started as

```shell
infrakit plugin start manager group swarm aws:us-east-1a aws:us-east-1b
```

This will start up a `us-east-1a/ec2-instance` instance plugin and another,
`us-east-1b/ec2-instance`, plugin.   Then in your configuration, the plugins will
be referred to using the names `us-east-1b/ec2-instance` and `us-east-1b/ec2-instance`.
Note that all these plugins are running in a single process, even though they are
listening on different sockets identified by the names after the `:` in the `plugin start`
arguments.

If you are using templates, be careful with the `include` template function
when embedding YAML content.  This is because YAML uses indentations to describe
the level of nesting for objects.  The template `include` function simply includes
the text inline and will confuse the parser since the nested structure of the
properties are now lost.  If you are going to use templates, consider using JSON
configuration instead because JSON is better at preserving structure of nesting when
texts are inserted without regards to indentation.

## Conclusion

This concludes our quick tutorial.  In this tutorial we:
  + Set up basic infrakit environment
  + Start up infrakit with a set of plugins in a single daemon process
  + Explored the dynamic CLI of infrakit
  + Provisioned an instance using the instance plugin.
  + Layering on top of the instance plugin to create a group controller to
  manage a group of instances.
  + Created a group of instances and visualize their properties
  + Scaled up and down the group.
  + Terminated some instances with the Instance plugin and watched the Group controller
  enforce the original specification of the group by creating new instances to match
  the user's specified size.
  + Performed a rolling update of the group after a configuration change.
  + Destroyed / terminated the group.


## Next Step

Now that you have completed the tutorial, there are additional topics you can explore:

  + [Multi-zone and tiered deployments](./multi.md).  Expanding on the composition of plugins,
  you can create a group controller that spans across multiple 'zones' or clouds,
  across different instance types.  In the tiered deployment example, you can see how
  the same composition approach is used to create a Group controller that can
  provision resources following a proritized list of plugins so that some plugins
  are tried to provision resources while others are used as fallbacks when earlier
  provisioning requests cannot be fulfilled.
  + Playbooks: Playbooks are 'scripts' that can be shared and reused.
  Playbooks can drive the `infrakit` CLI by defining new commands and flags.
  A good one to start is the [LinuxKit playbook](../playbooks/linuxkit),
  where we explore integration with [LinuxKit](https://github.com/linuxkit/linuxkit).
  + Other controllers such as the Ingress controller for routing of traffic from
  the internet to the cluster, and the Enrollment controller that performs associations
  of instances in a group to instances of other resource types.
