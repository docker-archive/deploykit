GC Controller
=============

The GC Controller (`gc` kind) is a controller that periodically synchronizes two lists of resources that
are conceptually one.  An example of this is Docker Swarm nodes.  A Docker Swarm node is really made up
of two sides:

  + The `Instance` corresponding to an instance of the physical infrastructure like a host, and
  + The `Node` corresponding to the Docker engine running on the `Instance`.

Because there exists ways to manipulate these resources independently -- you can delete the engine running
on a host, or terminating an instance that has a running engine -- these two sides can go out of sync.
For example, for instances that have been terminated manually, Docker Swarm can show them as `Down` nodes,
and we would like to remove these entries from the cluster.  Conversely, instances can have poorly configured
Docker engine that never join a swarm... in this case we'd like to terminate the instance and retry.

The two sides (`Node` and `Instance`) are joined by a special link label.  We assume that this "join key"
exists as labels in the `Node` and `Instance` descriptions such that a template "selector" can extract
the join key from the `Node` and `Instance` `instance.Description` struct that's returned by the instance
plugins the controller is configured to reconcile.  In the case of Docker Swarm nodes, we would

  + Provision a vm instance (`Instance`) and tag the resource `link=foo`
  + On this vm instance, we install a Docker engine and set its label (via `daemon.json`) to have `link=foo`.

Periodically the GC Controller will query the `Instance` and `Node` instance plugins (specified in the yml)
and each will return `[]instance.Description` slices.  The controller will build an index in memory based
on the join key (e.g. `foo`) and will track the state changes of these two sides (including when objects
disappear due to termination).  Based on the state changes and rules, the GC controller will call `Destroy`
on the appropriate side to make sure that orphaned instances are removed.

## Walk-Through

In the walk-through we use the simulator to simulate Docker and vm instances.  A playbook is included
for you play along.

Add the playbook (assuming your working directory is here):

```
infrakit playbook add gc file://$(pwd)/playbook.yml
```

Start infrakit:

```
infrakit use gc start
```
This starts up the manager as `mystack` and the gc controller, a simulator plugin simulating Docker instances (Nodes),
a simulator plugin simulating vm instances (Instances).

Now commit the configuration for garbage collection:

```
infrakit local mystack/gc commit -y ./gc.yml
```

This will start the garbage collector (gc) controller in the infrakit process started earlier.  Now we can
simulate provisioning of vm instances (via the `vm/compute` instance plugin) and installation of Docker engines
(via the `docker/compute` instance plugin).  We can then remove the instances to simulate removing the Docker engine
or terminating the vm instance and watch the garbage collector go to work.

Provision and Install Docker

```
$ infrakit use gc provision-vm --join-key link1
1518572955590056477
```
Because a link key is important, the playbook command will ask for a join key to tag the instance.
Enter any value.

Now let's pretend we are installing Docker on the instance...  The key here is to use the same join key:

```
$ infrakit use gc install-docker --join-key link1
1518572966936665649
```

Verify the instances are created, and we can select the join key from the instance's properties using `{{.Tags.link}}`:

```
$ infrakit local vm/compute describe -p --properties-view={{.Tags.link}}
ID                            	LOGICAL                       	TAGS                          	PROPERTIES
1518573178297386167           	  -                           	created=2018-02-13,link=link1,type=vm,user=davidchung	link1
```

```
$ infrakit local docker/compute describe -p --properties-view={{.Tags.link}}
ID                            	LOGICAL                       	TAGS                          	PROPERTIES
1518573183314395293           	  -                           	created=2018-02-13,link=link1,type=docker-engine,user=davidchung	link1
```
So this represents some state of the cluster where we have an instance running a node that are associated together
via the link label.  This allows us correlate physical resources with entities in a cluster.

At this point, you may want to watch the instance plugins. In separate windows:

```
watch -d infrakit local vm/compute describe
```
This will list all the vm instances.

```
watch -d infrakit local docker/compute describe -p --properties-view {{.Properties.status.state}}
```
The `properties-view` expression will select the simulated Docker node state (e.g. `ready`, `down`)


