InfraKit Flavor Plugin - Swarm
==============================

A [reference](../../../README.md#reference-implementations) implementation of a Flavor Plugin that creates a Docker
cluster in [Swarm Mode](https://docs.docker.com/engine/swarm/).


## Schema

Here's a skeleton of this Plugin's schema:
```json
{
  "Type": ""
}
```

The supported fields are:
* `Type`: The Swarm mode node type, `manager` or `worker`


## Example

Begin by building plugin [binaries](../../../README.md#binaries).

### Security

Since Swarm Mode uses [join-tokens](https://docs.docker.com/engine/swarm/join-nodes/) to authorize nodes, initializing
the Swarm requires:

a. exposing the Docker remote API for the InfraKit plugin to access join tokens
b. running InfraKit on the manager nodes to access join tokens via the Docker socket
 
We recommend approach (b) for anything but demonstration purposes unless the Docker daemon socket is
appropriately [secured(https://docs.docker.com/engine/security/https/).  For simplicity, **this example does not secure
Docker socket**


### Running

Start required plugins:

```shell
$ build/infrakit-group-default
```

```shell
$ build/infrakit-flavor-vanilla
```

```shell
$ build/infrakit-instance-vagrant
```

```shell
$ build/infrakit-flavor-swarm --host tcp://192.168.2.200:4243
```

Note that the Swarm Plugin is configured with a Docker host.  This is used to determine where the join tokens are
fetched from.  In this case, we are pointing at the yet-to-be-created Swarm manager node.


Next, create the [manager node](swarm-vagrant-manager.json) and initialize the cluster:

```shell
$ build/infrakit group watch example/flavor/swarm/swarm-vagrant-manager.json
```

Once the first node has been successfully created, confirm that the Swarm was initialized:
```shell
$ docker -H tcp://192.168.2.200:4243 node ls
ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
exid5ftbv15pgqkfzastnpw9n *  infrakit  Ready   Active        Leader
```
 
Now the [worker group](swarm-vagrant-workers.json) may be created:
```shell
$ build/infrakit group watch example/flavor/swarm/swarm-vagrant-workers.json
```

Once completed, the cluster contains two nodes:
```shell
$ docker -H tcp://192.168.2.200:4243 node ls
ID                           HOSTNAME   STATUS  AVAILABILITY  MANAGER STATUS
39hrnf71gzjve3slg51i6mjs4    localhost  Ready   Active
exid5ftbv15pgqkfzastnpw9n *  infrakit   Ready   Active        Leader
```

Finally, clean up the resources:
```shell
$ build/infrakit group destroy swarm-workers

$ build/infrakit group destroy swarm-managers
```
