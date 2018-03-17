Inventory Controller
===================

The Inventory Controller (`resource` kind) is a controller that can monitors collections of resources.
You can specify how to observe and take inventory of your resources by first tagging them (using the
infrastructure tools your platform vendor provides and then specify the tags in the config file.
In the config file, you can specify targets, which are plugin references with a set of label/tag filters.
The inventory controller exposes all the discovered resources via the Metadata interface so you can
use `infrakit local inventory keys -al` and `cat` to list and see their properties.  There's also an
event interface you can subscribe to and watch as resources are found and gone.

## Walk-Through

In the walk-through we use the simulator to different resource types on different providers.  We also
will use the `resource` controller to create resources which are discovered and tracked by the inventory
controller.

A playbook is included for you play along.

Add the playbook (assuming your working directory is here):

```
infrakit playbook add inventory file://$(pwd)/playbook.yml
```

Start infrakit:

```
infrakit use inventory start
```
This starts up the manager as `mystack` and the resource controller, the inventory controller, and
simulator plugins simulating providers in `az` and another in `az2`.

To see events from the controller, in a separate terminal:

```
infrakit local inventory tail / --view 'str://{{.Type}} - {{.ID}} - {{.Message}}'
```

This will subscribe to all events from the top `/`.

Now commit the configuration `inventory.yml` to start monitoring the resources.  This example
watches the resources provisioned by the `resource` plugin via  the `az1/net`, `az2/net`, and `az1/disk`
and `az2/disk` plugins:

```
infrakit local mystack/inventory commit -y <(infrakit use inventory inventory.yml)
```

Now we can provision in az1:
```
infrakit local mystack/resource commit -y <(infrakit use inventory az1.yml)
```

in az2
```
infrakit local mystack/resource commit -y <(infrakit use inventory az2.yml)
```

The inventory controller will report events as resource show up and if you destroy resources
you will see the 'gone' events.  The resource controller will detect that the resources
have disappeared and reprovision them, at which point you will see events as well.

The inventory controller also exposes all the known resources via the metatadata api. So to list:

```
infrakit local inventory/mystack-inventory  keys -al
```

And to see the values:

```
$ infrakit local inventory/mystack-inventory cat az2-resources/az2-net1/Properties/cidr
10.20.200.0/24
```
