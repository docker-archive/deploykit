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
a simulator plugin simulating providers in `cloud1` and another in `cloud2`.

Now commit the configuration for resources:

```
infrakit local mystack/resource commit -y ./resources.yml
```
