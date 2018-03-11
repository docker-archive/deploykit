Resource Controller
===================

The Resource Controller (`resource` kind) is a controller that can monitor and provision collections
of resources. The resources are named and backed by an instance plugin.  The instance plugins provide
observations / availability information of the resources as well as methods to provision them if needed.
The controller takes the input configuration and first queries the infrastructure to determine the presence
of the resources.  If they are found, no provisioning is needed.  If resources are missing the controller
will provision the resources based on the configuration specs provided.  Dependencies are supported within
a collection of resources in the configuration yml/json.  The resource controller continuously monitor
the infrastructure and will provision new instances as necessary.  Because the controller uses a discovery-matching
strategy rather than maintaining its own view/state of the infrastructure, it's possible to manually create
resources out-of-band and have these resources be included in management, provided the manually created
resources are labeled so that the instance plugins can return them in a DescribeInstances during discovery.

There are examples in this directory.  The yml `chain.yml` demonstrates the use of dependency in the `Properties`
of the instance plugin responsible for provisioning the resource.  For example

```
  properties:
    az1-net1:
      plugin: az1/net
      labels:
        az: az1
        type: network
      ObserveInterval: 1s                               # optional - specified here to control polling interval
      KeySelector: \{\{.Tags.infrakit_resource_name\}\} # optional specified here to control key extraction
      Properties:
        cidr: 10.20.100.0/24
        gateway: 10.20.0.1
    az1-disk0:
      plugin: az1/disk
      labels:
        az: az1
        type: storage
      Properties:
        dep: "@depend('az1-net1/ID')@"
        depname: "@depend('az1-net1/Tags/infrakit_resource_name')@"
        gw: "@depend('az1-net1/Properties/gateway')@"
        fs: ext4
        size: 1TB
    az1-disk1:
      plugin: az1/disk
      labels:
        az: az1
        type: storage
      Properties:
        dep: "@depend('az1-disk0/ID')@"
        depname: "@depend('az1-disk0/Tags/infrakit_resource_name')@"
        gw: "@depend('az1-net1/Properties/gateway')@"
        fs: ext4
        size: 2TB

```
In this example, resource `az-disk0` has dependencies on `az1-net1` resource because it requires `az1-net1`'s
attributes `ID`, a tag `infrakit_resource_name` and `gateway`.  Resource `az1-disk1` similarly has dependencies
on `az1-disk0` and `az1-net1`.   When the workflow state machines in the controller determines that resources
need to be provisioned, it will launch all tasks in parallel and as data arrive (once depended on resources have
been provisioned), it unblocks the waiting tasks so that provision can proceed.  A provisioning task can block
indefinitely if the depended on resources were never successfully created and this process can be completely
asynchronous: as soon as data arrives (say the resource is now fixed and provisioned out of band, properly labeled,
and thus discoverable by the controller's associated instance plugins), the other tasks will be unblocked and
proceed.  Even if the controller process is terminated (by `kill -9`) and restarted, the controller will simply
discover the actual resources and reconstruct the necessary relationships and thus the state prior to restart.

## Walk-Through

In the walk-through we use the simulator to different resource types on different providers.
A playbook is included for you play along.

Add the playbook (assuming your working directory is here):

```
infrakit playbook add res file://$(pwd)/playbook.yml
```

Start infrakit:

```
infrakit use res start
```
This starts up the manager as `mystack` and the resource controller,
a simulator plugin simulating providers in `az` and another in `az2`.

To see events from the controller, in a separate terminal:

```
infrakit local resource tail / --view 'str://{{.Type}} - {{.ID}} - {{.Message}}'
```

This will subscribe to all events from the top `/`.

Now commit the configuration for resources:

```
infrakit local mystack/resource commit -y <(infrakit use res chain.yml)
```

or via unix pipe:

```
infrakit use res chain.yml | infrakit local mystack/resource commit -y -
```
This will create a `chain` collection in the resource controller.  As the controller tries to reconcile the
discovered state with the specification, you will see event messages that correspond to the state transition
of each represented resources (e.g. `chain/az1-disk0`).  The `chain.yml` file is essentially a linked list
where one resource depends on the successful provision of the one before it.  So you will see that as soon
as `az1-net1` is provisioned, `az1-disk0` will start, followed by `az1-disk1`, etc.

```
$ infrakit local resource tail / --view 'str://{{.Type}} - {{.ID}} - {{.Message}}'
INFO[0000] Connecting to broker url= unix://resource topic= / opts= {/Users/davidchung/.infrakit/plugins /events}
CollectionUpdate - chain/az1-disk0 - update collection
CollectionUpdate - chain/az1-disk4 - update collection
CollectionUpdate - chain/az1-disk6 - update collection
CollectionUpdate - chain/az1-disk8 - update collection
CollectionUpdate - chain/az1-disk9 - update collection
CollectionUpdate - chain/az1-disk1 - update collection
CollectionUpdate - chain/az1-disk2 - update collection
CollectionUpdate - chain/az1-disk3 - update collection
CollectionUpdate - chain/az1-disk5 - update collection
CollectionUpdate - chain/az1-disk7 - update collection
CollectionUpdate - chain/az1-net1 - update collection
Provision - chain/az1-net1 - provisioning resource
Pending - chain/az1-disk0 - resource blocked waiting on dependencies
Pending - chain/az1-disk7 - resource blocked waiting on dependencies
Pending - chain/az1-disk5 - resource blocked waiting on dependencies
Pending - chain/az1-disk3 - resource blocked waiting on dependencies
Pending - chain/az1-disk2 - resource blocked waiting on dependencies
Pending - chain/az1-disk1 - resource blocked waiting on dependencies
Pending - chain/az1-disk9 - resource blocked waiting on dependencies
Pending - chain/az1-disk8 - resource blocked waiting on dependencies
Pending - chain/az1-disk6 - resource blocked waiting on dependencies
Pending - chain/az1-disk4 - resource blocked waiting on dependencies
Ready - chain/az1-net1 - resource ready
MetadataUpdate - chain/az1-net1 - update metadata
Provision - chain/az1-disk0 - provisioning resource
Ready - chain/az1-disk0 - resource ready
MetadataUpdate - chain/az1-disk0 - update metadata
Provision - chain/az1-disk1 - provisioning resource
Ready - chain/az1-disk1 - resource ready
MetadataUpdate - chain/az1-disk1 - update metadata
Provision - chain/az1-disk2 - provisioning resource
MetadataUpdate - chain/az1-disk2 - update metadata
Ready - chain/az1-disk2 - resource ready
Provision - chain/az1-disk3 - provisioning resource
Ready - chain/az1-disk3 - resource ready
MetadataUpdate - chain/az1-disk3 - update metadata
Provision - chain/az1-disk4 - provisioning resource
Ready - chain/az1-disk4 - resource ready
MetadataUpdate - chain/az1-disk4 - update metadata
Provision - chain/az1-disk5 - provisioning resource
Ready - chain/az1-disk5 - resource ready
MetadataUpdate - chain/az1-disk5 - update metadata
Provision - chain/az1-disk6 - provisioning resource
Ready - chain/az1-disk6 - resource ready
MetadataUpdate - chain/az1-disk6 - update metadata
Provision - chain/az1-disk7 - provisioning resource
Ready - chain/az1-disk7 - resource ready
MetadataUpdate - chain/az1-disk7 - update metadata
Provision - chain/az1-disk8 - provisioning resource
Ready - chain/az1-disk8 - resource ready
MetadataUpdate - chain/az1-disk8 - update metadata
Provision - chain/az1-disk9 - provisioning resource
Ready - chain/az1-disk9 - resource ready
MetadataUpdate - chain/az1-disk9 - update metadata
```

You can also see the metadata exported by the controller:

```
$ infrakit local resource/chain keys -al
total 151:
az1-disk0/ID
az1-disk0/LogicalID
az1-disk0/Properties/dep
az1-disk0/Properties/depname
az1-disk0/Properties/fs
az1-disk0/Properties/gw
az1-disk0/Properties/size
az1-disk0/Tags/az
#....
```

And reading values:

```
$ infrakit local resource/chain cat az1-disk0/Properties
{
"dep": "1520578926171361274",
"depname": "az1-net1",
"fs": "ext4",
"gw": "10.20.0.1",
"size": "1TB"
}
```
