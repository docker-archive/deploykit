End to End Workflow
===================

## Overview

This demo shows the following:

   + Manage groups -- covering the legacy and new specification formats.
   + Ingress -- loadbalancer routing traffic to the groups
   + Enrollment -- we want to authorize nfs access for each new node spun up.
   Remove  nfs authorization if node goes away.

There are two groups: `miners` and `cattle` and 3 loadbalancers:

  + `miners` group is comprised of 5 instances initially
  + `cattle` group is comprised of 10 instances initially
  + Loadbalancer `simulator/lb1` points traffic to `miners` group
  + Loadbalancer `simulator/lb2` points traffic to `miners` group as well, with a static route of port 8080 to 8080
  + Loadbalancer `simulator/lb3` points traffic to `cattle` group.

The configuration `miners.yml` follows the new schema, while `cattle.json` is a JSON in the legacy format.
The file `ingress.yml` follows the new schema.

Other plugins include an enrollment controller running with the name `nfs` (see `start.sh`), while another simulator
instance runs with the name `nfs-auth`. These are controller and plugin that perform simulated nfs-auth each time
and new node comes up or goes away.

### Initial

## Steps

Start everything:

```
$ ./start.sh
```

You can tail the log at

```
$ tail -f docs/e2e/infrakit.log
```

### What's Running

Because you haven't set the environment variable `INFRAKIT_HOST`, you will have a subcommand `local`.
Type this to see what's discovered of the plugins that are running:

```
infrakit local
```

If you are connecting to a remote cluster and set your `INFRAKIT_HOST` to `foo` (which must be
a valid name of a remote in `infrakit remote`), you will have a subcommand `foo` instead.


### Manage Groups

The description of the `miners` group is in the new format:

```
infrakit local mystack/groups commit -y ./miners.yml
```

Old format:

```
infrakit local mystack/groups commit-group ./cattle.json
```

### List group members
`ls` returns a list of instances in a group:

```
infrakit local mystack/miners ls
infrakit local mystack/cattle ls
```

### Set Up Ingress

Ingress brings traffic from a loadbalancer (see `simulator/lb1`, `simulator/lb2`, and `simulator/lb3`) to the
nodes in the different groups.  The ingress controller synchronizes the routes and backends as groups scale
up and down.

```
infrakit local mystack/ingress commit -y ./ingress.yml
```

Because we've set up ingress to send L4 traffic to the group members, show the routes
and backends:

Because the ingress controller associates the different groups as backends of different load balancers,
we should see changes to the backends of the various load balancers.

Verify backends of various load balancers:

```
infrakit local simulator/lb1 backends ls
infrakit local simulator/lb1 routes ls
```

```
infrakit local simulator/lb2 backends ls
infrakit local simulator/lb2 routes ls
```

```
infrakit local simulator/lb3 backends ls
infrakit local simulator/lb3 routes ls
```

### Set up Enrollment for NFS Auth

Enrollment controller can watch group membership and add/remove resources accordingly.  In this example
we are simulating adding and removing of NFS volume / host authorizations as groups of nodes (`miners` and `cattle`)
scale up and down.

```
infrakit local mystack/nfs commit -y ./enrollment.yml
```

See the nfs authorizations:

```
infrakit local nfs-auth/disk describe
```

### Scale Up / Down Groups

Get the current size of the groups:

```
infrakit local mystack/miners scale
infrakit local mystack/cattle scale
```

Change the size of the groups:

```
infrakit local mystack/miners scale 10
infrakit local mystack/cattle scale 20
```

Because the ingress controller associates the different groups as backends of different load balancers,
we should see changes to the backends of the various load balancers.

Verify backends of various load balancers:

```
infrakit local simulator/lb1 backends ls
infrakit local simulator/lb1 routes ls
```

```
infrakit local simulator/lb2 backends ls
infrakit local simulator/lb2 routes ls
```

```
infrakit local simulator/lb3 backends ls
infrakit local simulator/lb3 routes ls
```

See the NFS authorizations change (for `cattle` group only):

```
infrakit local nfs-auth/disk describe
```


### Clean Up

Destroy will terminate and remove resources, while Free frees the resources from management / monitoring.
Once freed, a commit is required for Infrakit to resume monitoring.  Destroy is a terminal and irreversible operation.

#### Destroy Instances and Groups

```
infrakit local mystack/miners destroy-instace
infrakit local mystack/miners destroy
```

Free the group from management:

```
infrakit local mystack/cattle free
```

## Design / Roadmap

With the general Controller interface, there are overlaps with the Group controller SPI.  We should take
advantage of interface embedding / composition in Go to generalize the Group controller SPI as an interface
that embeds the Controller interface.  The Controller interface contains methods for declarative specification
and convergence but does not have any model-specific methods such as getting size of a group.
The Group interface on the other hand is derived from (or composed of) the Controller interface because the
Group SPI supports declarative configuration and convergence.  In addition, the Group interface contains
model-specific methods such as setting and getting the size of the group.  So it can be seen that
the Group interface is a hybrid of the declarative interface that Controller offers and an imperative
one that offers methods for manipulating group-specific concepts such as scaling up/down and destroying instances.

Controller:

```
// Controller is the interface that all controllers implement.  Controllers are managed by pkg/manager/Manager
type Controller interface {

	// Plan is a commit without actually making the changes.  The controller returns a proposed object state
	// after commit, with a Plan, or error.
	Plan(Operation, types.Spec) (types.Object, Plan, error)

	// Commit commits the spec to the controller for management or destruction.  The controller's job is to ensure reality
	// matches the operation and the specification.  The spec can be composed and references other controllers or plugins.
	// When a spec is committed to a controller, the controller returns the object state corresponding to
	// the spec.  When operation is Destroy, only Metadata portion of the spec is needed to identify
	// the object to be destroyed.
	Commit(Operation, types.Spec) (types.Object, error)

	// Describe returns a list of objects matching the metadata provided. A list of objects are possible because
	// metadata can be a tags search.  An object has state, and its original spec can be accessed as well.
	// A nil Metadata will instruct the controller to return all objects under management.
	Describe(*types.Metadata) ([]types.Object, error)

	// Free tells the controller to pause management of objects matching.  To resume, commit again.
	Free(*types.Metadata) ([]types.Object, error)
}
```

Group:

```
// Plugin defines the functions for a Group plugin.
type Plugin interface {
	CommitGroup(grp Spec, pretend bool) (string, error)  // ==> controller.Commit

	FreeGroup(ID) error                                  // ==> controller.Free

	DescribeGroup(ID) (Description, error)               // ==> controller.Describe

	DestroyGroup(ID) error                               // ==> ADD controller.Destroy

	InspectGroups() ([]Spec, error)                      // ==> ADD controller.Inspect

	// DestroyInstances deletes instances from this group. Error is returned either on
	// failure or if any instances don't belong to the group. This function
	// should wait until group size is updated.
	DestroyInstances(ID, []instance.ID) error

	// Size returns the current size of the group.
	Size(ID) (int, error)

	// SetSize sets the size.
	// This function should block until completion.
	SetSize(ID, int) error
}
```

Controller's will evolve to look like:

```
type Controller interface {

	// Plan is a commit without actually making the changes.  The controller returns a proposed object state
	// after commit, with a Plan, or error.
	Plan(Operation, types.Spec) (types.Object, Plan, error)

	// Commit commits the spec to the controller for management or destruction.  The controller's job is to ensure reality
	// matches the operation and the specification.  The spec can be composed and references other controllers or plugins.
	// When a spec is committed to a controller, the controller returns the object state corresponding to
	// the spec.  When operation is Destroy, only Metadata portion of the spec is needed to identify
	// the object to be destroyed.
	Commit(Operation, types.Spec) (types.Object, error)

	// Describe returns a list of objects matching the metadata provided. A list of objects are possible because
	// metadata can be a tags search.  An object has state, and its original spec can be accessed as well.
	// A nil Metadata will instruct the controller to return all objects under management.
	Describe(*types.Metadata) ([]types.Object, error)

	// Free tells the controller to pause management of objects matching.  To resume, commit again.
	Free(*types.Metadata) ([]types.Object, error)

	// NEW
	// Destroy a managed
	Destroy(*types.Metadata) error

	// Spec returns the spec
	Spec() (types.Spec, error)
}
```

Group will become a composition of Controller plus other model-specific methods:

```
// Plugin defines the functions for a Group plugin.
type Plugin interface {

        // Controller embeds the controller interface
        Controller

	// DestroyInstances deletes instances from this group. Error is returned either on
	// failure or if any instances don't belong to the group. This function
	// should wait until group size is updated.
	DestroyInstances(ID, []instance.ID) error

	// Size returns the current size of the group.
	Size(ID) (int, error)

	// SetSize sets the size.
	// This function should block until completion.
	SetSize(ID, int) error
}
```

These method names are then reflected in the CLI.

### Decision / Discussion:

The `Plugin` interface may evolve to embody a 'specific' group, rather than a 'manager' of groups:
  + `Size(ID)` should really become `Size()`
  + `SetSize(ID, int)` should really become `SetSize(int)`


## Summary

The changes to Group reflects a common design common pattern:

  + A given controller's SPI should embed the `Controller` interface
  + The controller's SPI can add additional, model-specific imperative methods (e.g. get/set Size, etc.)
  + Impact on UX / CLI:
    - A remote object will always have methods from Controller: commit, destroy, etc.
    - A remote object will optionally have additional verbs / commands: e.g. scale, destroy-instances, etc.
