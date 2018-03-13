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

Now commit the configuration for resources:

```
infrakit local mystack/inventory commit -y ./inventory.yml
```
