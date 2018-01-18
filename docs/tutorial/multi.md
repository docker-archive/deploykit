# Multi-Zone and Tiered Deployment

Infrakit defines several interfaces or SPIs (Service Provider Interfaces) and there
are many implementations of these interfaces that perform the CRUD operations on
infrastructure -- from AWS to VSphere.  However, there are several "instance" plugins
that, by implementing the Instance SPI, enable interesting compositions of other
Instance plugins to create new Instance plugins that can support complex provisioning
scenarios.  These plugins are known as 'selectors', and they all implement the same
instance SPI while delegating actual provisioning operations to other Instance plugins.
These selectors can therefore be included in a group YML just like a normal Instance
plugin, but they enable complex use cases:

  + Weighted -- this selector is a composition of N instance plugins, each carrying
  a weight.  When provisioning a new instance, the downstream instance plugins are
  called based on the weighted probability.  For example, you can have a 'us-east-1a'
  and a 'us-east1b' instance plugin that have the weights of 80 and 20 respectively.
  When provisioning a new instance, the weighted plugin will have a 0.8 probability
  of calling the 'us-east-1a' plugin vs a 0.2 probability for 'us-east-1b'. So a large
  cluster will have a 4 to 1 distribution of instances across these two zones.
  + Spread -- instead of relying on weighting, this selector places the instances
  so that the instances are spread evenly across all zones.  In the example above,
  both 'us-east-1a' and 'us-east-1b' will have roughly equal number of instances.
  + Tiered -- a tiered selector is an Instance plugin that is composed of a prioritized
  list of instance plugins.  During a Provision call, the list of plugins is tried
  in sequence and continue until success or exhaustion of all choices.  This makes it
  possible to first try to provision a bare-metal instance (e.g. via `maas` or `oneview`),
  followed by a public cloud spot instance (`aws/ec2-spot-instance`) to finally an
  on-demand instance in the cloud (`aws/ec2-instance`).

When used with a Group controller, selectors enable complex provisioning scenarios in a
familiar autoscaling group API / UX of simply scaling up or down the group.

## Tutorial

Here we use the simulator and other plugins to demonstrate how to construct these
complex scaling groups.  We will use multiple instances of the `simulator` to mimic
different zones.

```shell
$ INFRAKIT_SELECTOR_SPREAD_PLUGINS='us-east-1a/compute;us-east-1b/compute' \
infrakit plugin start manager group vanilla \
simulator:us-east-1a simulator:us-east-1b selector/spread
```
Note that the `simulator` is used twice: one for instances in `us-east-1a` and the
other in `us-east-1b`.  These become the socket names of different running instances of
the `simulator` plugin.  We also start up the `selector/spread` plugin.

The environment `INFRAKIT_SELECTOR_SPREAD_PLUGINS` is necessary to tell the spread
selector a list of plugins to choose from.

Checking on the CLI, we see that there are new objects / commands available:

```shell
$ infrakit -h

# ....

Available Commands:
  group              Access object group which implements Metadata/0.1.0,Updatable/0.1.0,Controller/0.1.0,Group/0.1.0,Manager/0.1.0
  group-stateless    Access object group-stateless which implements Group/0.1.0,Metadata/0.1.0
  manager            Access the manager
  playbook           Manage playbooks
  plugin             Manage plugins
  remote             Manage remotes
  selector/spread    Access object selector/spread which implements Instance/0.6.0
  template           Render an infrakit template at given url.  If url is '-', read from stdin
  up                 Up everything
  us-east-1a/compute Access object us-east-1a/compute which implements Instance/0.6.0
  us-east-1a/disk    Access object us-east-1a/disk which implements Instance/0.6.0
  us-east-1a/lb1     Access object us-east-1a/lb1 which implements L4/0.6.0
  us-east-1a/lb2     Access object us-east-1a/lb2 which implements L4/0.6.0
  us-east-1a/lb3     Access object us-east-1a/lb3 which implements L4/0.6.0
  us-east-1a/net     Access object us-east-1a/net which implements Instance/0.6.0
  us-east-1b/compute Access object us-east-1b/compute which implements Instance/0.6.0
  us-east-1b/disk    Access object us-east-1b/disk which implements Instance/0.6.0
  us-east-1b/lb1     Access object us-east-1b/lb1 which implements L4/0.6.0
  us-east-1b/lb2     Access object us-east-1b/lb2 which implements L4/0.6.0
  us-east-1b/lb3     Access object us-east-1b/lb3 which implements L4/0.6.0
  us-east-1b/net     Access object us-east-1b/net which implements Instance/0.6.0
```

We see that in the simulation, we have access to resources in `us-east-1a` and `us-east-1b`.
We also see that `simulator/spread` is available. These all become valid plugin names
we will reference in our config YAMLs.

### A Group.yml across two zones

Here is an example ([`multi.yml`](./multi.yml))

```yaml
kind: group
metadata:
    name: workers
properties:
    Allocation:
      Size: 10
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
      # here we reference the spread  selector at the socket location, selector.
      Plugin: selector/spread
      Properties:
        # The configuration follows the format of a map of pluginName : properties
        us-east-1a/compute:
          apiProperty1 : value1
          apiProperty2 : us-east-1a
        # The configuration follows the format of a map of pluginName : properties
        us-east-1b/compute:
          apiProperty1 : value1
          apiProperty2 : us-east-1b

```

Note that in `multi.yml`, the group specification looks similar to the original
`goup.yml` in terms of the flavor plugin set up and size/allocation of the group.
However, the Instance plugin section has been replaced with a composition of the
`us-east-1a/compute` and `us-east-1b/compute` instance plugins inside a `selector/spread`
plugin.

So now when we commit this YAML to the group, we expect to see instances spread
across these two 'zones'.

### Commit the Specification

Do this to commit this spec:

```shell
$ infrakit group controller commit -y ./multi.yml
```

Now we have a new group `group/workers`:

```shell
$ infrakit group/workers describe
ID                            	LOGICAL                       	TAGS
1506528402034960841           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402036560109           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402038196749           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402039391770           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402041415151           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402042777184           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402044321191           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402045768137           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402049931166           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402051590289           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
```

And we can see that they are from different zones:

```shell
$ infrakit us-east-1a/compute describe
ID                            	LOGICAL                       	TAGS
1506528402034960841           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402038196749           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402039391770           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402044321191           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402045768137           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
```

and in `us-east-1b`:

```shell
$ infrakit us-east-1b/compute describe
ID                            	LOGICAL                       	TAGS
1506528402036560109           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402041415151           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402042777184           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402049931166           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402051590289           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
```

Scaling down the group will remove the instances evenly:

```shell
$ infrakit group/workers scale 4
Group workers at 10 instances, scale to 4
$ infrakit us-east-1b/compute describe
ID                            	LOGICAL                       	TAGS
1506528402051590289           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402049931166           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
$ infrakit us-east-1a/compute describe
ID                            	LOGICAL                       	TAGS
1506528402044321191           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
1506528402045768137           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=6ukklp23vulgg343jobm4as6ndyo4a5r,infrakit.group=workers
```

Destroying the group will remove all instances across the two zones:

```shell
$ infrakit group/workers destroy
Destroy workers initiated
$ infrakit us-east-1a/compute describe
ID                            	LOGICAL                       	TAGS
$ infrakit us-east-1b/compute describe
ID                            	LOGICAL                       	TAGS
```

### Tiered Provisioning

In this tutorial, we are going to build a scaling group can provision
instances by attempting in the following order:

  1. First provision a cheap spot instance.  If the bid price is too
  low, and we are unable to provision, then
  2. Provision an on-demand instance.

We are going to use the `simulator` to mimic the different types of compute
instances:

```shell
$ export INFRAKIT_SELECTOR_TIERED_PLUGINS='spot/compute;ondemand/compute'
$ infrakit plugin start manager group vanilla simulator:ondemand simulator:spot selector/tiered
```

Note here we are starting up the `selector/tiered` plugin, along with a `simulator` running
as `ondemand` and another as `spot`.

The CLI now reflects these objects that are running:

```shell
$ infrakit -h


infrakit command line interface

Usage:
  infrakit [command]

Available Commands:
  group            Access object group which implements Group/0.1.0,Manager/0.1.0,Metadata/0.1.0,Updatable/0.1.0,Controller/0.1.0
  group-stateless  Access object group-stateless which implements Group/0.1.0,Metadata/0.1.0
  manager          Access the manager
  ondemand/compute Access object ondemand/compute which implements Instance/0.6.0
  ondemand/disk    Access object ondemand/disk which implements Instance/0.6.0
  ondemand/lb1     Access object ondemand/lb1 which implements L4/0.6.0
  ondemand/lb2     Access object ondemand/lb2 which implements L4/0.6.0
  ondemand/lb3     Access object ondemand/lb3 which implements L4/0.6.0
  ondemand/net     Access object ondemand/net which implements Instance/0.6.0
  playbook         Manage playbooks
  plugin           Manage plugins
  remote           Manage remotes
  selector/tiered  Access object selector/tiered which implements Instance/0.6.0
  spot/compute     Access object spot/compute which implements Instance/0.6.0
  spot/disk        Access object spot/disk which implements Instance/0.6.0
  spot/lb1         Access object spot/lb1 which implements L4/0.6.0
  spot/lb2         Access object spot/lb2 which implements L4/0.6.0
  spot/lb3         Access object spot/lb3 which implements L4/0.6.0
  spot/net         Access object spot/net which implements Instance/0.6.0
  template         Render an infrakit template at given url.  If url is '-', read from stdin
  up               Up everything
  util             Utilities
  vanilla          Access object vanilla which implements Flavor/0.1.0
  version          Print build version information
  x                Experimental features
```

In the group YAML config, we now replace the `Instance` section with this
(see ([`tiered.yml`](./tiered.yml)):

```yaml
    Instance:

      # here we reference the tiered selector at the socket location, selector.
      Plugin: selector/tiered
      Properties:

        spot/compute:
          # This is a special property that the simulator understands to limit
          # the number of instances it can provision.  So for cluster size > 3
          # we expect to see the ondemand/compute instances getting added.
          Cap: 3
          instanceType: small
          bid: 0.02
          labels:
            project: infrakit
            billing: spot

        # Properties for the ondemand/compute instances.  The section after the
        # name of the plugin (ondemand/compute) is the properties blob to be fed
        # to the plugin.
        ondemand/compute:
          instanceType: small
          labels:
            project: infrakit
            billing: ondemand
```

Note that we now use the `selector/tiered` selector while the `Properties` section
within that now have the configurations of the different plugins.

In order to simulate limited capacity, the `simulator` plugin has a "magic" property
called `Cap`. If set, the simulator will provision at most that many instances.  So,
in this case, we have a total of 5 instances in this group (see tiered.yml), and only
3 will be found under `spot/compute`, while any instances beyond that will be provisioned
with `ondemand/compute` instances.

Commit this YAML:

```shell
$ infrakit group controller commit -y ./tiered.yml
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
    Plugin: selector/tiered
    Properties:
      ondemand/compute:
        instanceType: small
        labels:
          billing: ondemand
          project: infrakit
      spot/compute:
        Cap: 3
        bid: 0.02
        instanceType: small
        labels:
          billing: spot
          project: infrakit
version: ""
```

Now checkt the group:

```shell
$ infrakit group/workers describe
ID                            	LOGICAL                       	TAGS
1506529768374681221           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768376553466           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768377518027           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768383265310           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768385038942           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
```

And from the different instance plugins:

```shell
$ infrakit spot/compute describe
ID                            	LOGICAL                       	TAGS
1506529768374681221           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768376553466           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768377518027           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
```

and ondemand instances

```shell
$ infrakit ondemand/compute describe
ID                            	LOGICAL                       	TAGS
1506529768383265310           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768385038942           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
```

So we see that the first 3 instances were provisioned with spot instances which will
reach the simulated max capacity and the remaining instances were provisioned as
on-demand instances.

Now scaling up the cluster:

```shell
$ infrakit group/workers scale 10
Group workers at 5 instances, scale to 10
```

We see that 3 spot instances remain:

```shell
$ infrakit spot/compute describe
ID                            	LOGICAL                       	TAGS
1506529768374681221           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768376553466           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768377518027           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
```

while more on-demand instances are found:

```shell
$ infrakit ondemand/compute describe
ID                            	LOGICAL                       	TAGS
1506529768385038942           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529988381197479           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529988381729041           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529988382941302           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529988384036924           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529988384666030           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529768383265310           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
```

Now scaling down:

```shell
$ infrakit group/workers scale 4
Group workers at 10 instances, scale to 4
$ infrakit ondemand/compute describe
ID                            	LOGICAL                       	TAGS
1506529988381729041           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529988382941302           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529988384036924           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
1506529988384666030           	  -                           	custom.tag1=tutorial,custom.tag2=single-instance,custom.tag3=by-group,infrakit.config.hash=kyexlihytvcnhjgod4ebi4mf3prsuebu,infrakit.group=workers
$ infrakit spot/compute describe
ID                            	LOGICAL                       	TAGS
```

In this case, the scale down follows the same ordering so the spot instances are removed.
In this current implementation, a single priority ordering is used for both provisioning
and termination of instances.  Let us know if this is a reasonable assumption by opening
an issue.

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

## Summary

In this tutorial, we explored how infrakit can support complex provisioning use cases
by creating scaling groups via composition of multiple instance plugins into _selectors_.
The use cases that are possible are:

  + Multiple availability zones in a single cloud
  + Multiple clouds (by mixing and matching different plugins e.g. `aws/ec2-instance` and
  `gcp/compute`.
  + Tiered provisioning where we try to first provision `spot` instances and use
  the more expensive `ondemand` instances only if we can't secure enough capacity.

These use cases also illustrate the flexibility of composition and layering in Infrakit.
It's possible to build complex scaling groups / autoscalers for your cluster that follow
complex provisioning business logic this way.
