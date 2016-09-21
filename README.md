InfraKit
========

[![Circle CI](https://circleci.com/gh/docker/libmachete.png?style=shield&circle-token=50d2063f283f98b7d94746416c979af3102275b5)](https://circleci.com/gh/docker/libmachete)
[![codecov.io](https://codecov.io/github/docker/libmachete/coverage.svg?branch=master&token=z08ZKeIJfA)](https://codecov.io/github/docker/libmachete?branch=master)

_InfraKit_ is a developer toolkit for creating and managing declarative infrastructure in any environment.  It is
designed to break infrastructure automation down into simple pluggable components.  _InfraKit_ can be used to create
an active (always-on) or passive (on-demand) cluster management system, or even self-healing infrastructure.


### Overview
When automating cluster infrastructure, it is common to manage logical _groups_ of instances rather than individual
machines (instances). Instances in a Group are configured identically or nearly-identically, and often operate as
replicas of an infrastructure service.  For example, members of an etcd cluster match this definition.

_InfraKit_ provides primitives to manage Groups, which are composed of many Instances.  Operators can focus on Groups
while _InfraKit_ handles Instances.

Since _InfraKit_ defines declarative infrastructure, you will not find operations to move machines or Groups from one
state to another.  Instead, you will _declare_ your desired state of the infrastructure.  _InfraKit_ is responsible
for converging towards the desired state.


### Plugins
The real power of _InfraKit_ lies in plugins and the ability to compose them for different needs.  Strictly-speaking,
a plugin is a local HTTP server exposed through a Unix socket.  While there is currently scaffolding and precedent for
plugins authored in Go, any HTTP server can be a plugin.

Currently, there are three types of plugins, described below.

#### Instance
An [instance plugin](spi/instance/spi.go) manages physical compute resources.  It knows only about individual instances
and nothing about Groups.  Instance is technically defined by the plugin, and need not be a physical machine at all.

| plugin| description                  |
|:------|:-----------------------------|
| [libmachete.aws](https://github.com/docker/libmachete.aws) | creates instances in Amazon AWS |
| vagrant | TODO: implement |


#### Flavor
A [flavor plugin](plugin/group/types/types.go) defines what runs on an Instance.  It is responsible for dictating
commands to run services, and check the health of those services.

| plugin| description                  |
|:------|:-----------------------------|
| etcd | TODO: implement |
| [swarm](https://github.com/docker/libmachete/plugin/group/swarm) | configures instances with Docker in Swarm mode |
| zookeeper | TODO: implement |


#### Group
A [group plugin](spi/group/spi.go) manages Groups of Instances.  This is actually the only plugin type that _InfraKit_
directly interfaces with.

Group plugins authored in Go may use our [scaffolding](plugin/groupserver/run.go) to expose plugin operations via HTTP.

##### Default Group plugin
_InfraKit_ provides a default Group plugin implementation, intended to suit common use cases.  The default Group plugin
manages Instances of a specific Flavor.  Instance and Flavor plugins can be composed to manage different types of
services on different infrastructure providers.

While it's generally simplest to use the default Group plugin, custom implementations may be valuable to adapt another
infrastructure management system.  This would allow you to use _InfraKit_ tooling to perform basic operations on widely
different infrastructure using the same interface.

| plugin| description                  |
|:------|:-----------------------------|
| [libmachete](https://github.com/docker/libmachete) | supports Instance and Flavor plugins, rolling updates |


### Building
#### Binaries
```shell
$ make binaries
```


#### Running tests
```shell
$ make ci
```

### Usage
#### Configuration
_InfraKit_ uses JSON for configuration.  As an example, if you wanted to manage a Group of NGINX servers, you could
write a custom Group plugin for ultimate customizability.  The most concise configuration looks something like this:

```json
{
  "id": "nginx",
  "plugin": "my-nginx-group-plugin",
  "properties": {
    "port": 8080
  }
}
````

However, you would likey prefer to use the default Group plugin and implement a Flavor plugin to focus on
application-specific behavior.  This gives you immediate support for any infrastructure that has an Instance plugin.
Your resulting configuration might look something like this:

```json
{
  "id": "nginx",
  "plugin": "group",
  "properties": {
    "size": 10,
    "instance": {
      "plugin": "aws",
      "properties": {
        "region": "us-west-2",
        "ami": "ami-123456"
      }
    },
    "flavor": {
      "plugin": "nginx",
      "properties": {
        "port": 8080
      }
    }
  }
}
```

#### Create, update, and destroy a Group
For development, it's typically easiest to use the Vagrant Instance plugin.  We will start with this configuration:

```shell
$ cat zk.conf
{
  "id": "zk",
  “plugin”: “group”,
  “properties”: {
    “ips”: [“192.168.0.4”, “192.168.0.5”, “192.168.0.6”],
    “instance”: {
      “plugin”: “vagrant”
    },
    “flavor”: {
      “plugin”: “zookeeper”
    }
  }
}
```

```shell
$ infrakit watch zk.conf
```

To perform a rolling update to the Group, we use the `update` command.  First, it's a good idea to describe the proposed
update to ensure the expected operations will be performed:

```shell
$ infrakit update zk.conf --describe
Noop
```

Since we have not edited `zk.conf`, there are no changes to be made.  First, let's edit the configuration:

```shell
$ cat zk.conf
{
  "id": "zk",
  “plugin”: “group”,
  “properties”: {
    “ips”: [“192.168.0.4”, “192.168.0.5”, “192.168.0.6”],
    “instance”: {
      “plugin”: “vagrant”,
      "properties": {
        "cpu": 2
      }
    },
    “flavor”: {
      “plugin”: “zookeeper”
    }
  }
}
```

```shell
$ infrakit update zk.conf --describe
Performs a rolling update of 3 instances

$ infrakit update zk.conf
```

For high-traffic clusters, ZooKeeper supports Observer nodes.  We can add another Group to include Observers:

```shell
$ cat zk-observer.conf
{
  "id": "zk-observers",
  “plugin”: “group”,
  “properties”: {
    "size": 3,
    “instance”: {
      “plugin”: “vagrant”
    },
    “flavor”: {
      “plugin”: “zookeeper”,
      "properties": {
        "mode": "observer"
      }
    }
  }
}

$ infrakit watch zk-observer.conf
```

Finally, we can terminate the instances when finished with them:

```shell
$ infrakit destroy zk
$ infrakit destroy zk-observers

```
