# Plugins

Much of the behavior in _InfraKit_ is defined by Plugins.  Technically, a Plugin is an HTTP server with a well-defined
API, listening on a unix socket.

## Plugin Discovery

Multiple _InfraKit_ plugins are typically used together to support a declared configuration.  These plugins discover
each other by looking for socket files in a common plugin directory, and communicate via HTTP.

InfraKit stores a number of files (for configurations, plugin socket files, etc.) in a directory tree.  Typically
the environment variable `INFRAKIT_HOME` designates where the directory is.  It is typically `~/.infrakit`.
For plugin discovery, the default directory is `~/.infrakit/plugins`, and can be overridden with the environment variable
`INFRAKIT_PLUGINS_DIR`.  This is the directory where the unix sockets are found.  The name of a socket file corresponds
to the name the plugin is referenced throughout the system.  For example, a

Note that multiple instances of a plugin may run, provided they have different names for discovery.  This may be useful,
for example, if a plugin can be configured to behave differently. For example:

The CLI shows which plugins are [discoverable](../../cmd/infrakit/README.md#list-plugins).

## Plugin types
### Group
When managing infrastructure like computing clusters, Groups make good abstraction, and working with groups is easier
than managing individual instances. For example, a group can be made up of a collection
of machines as individual instances. The machines in a group can have identical configurations (replicas, or cattle).
They can also have slightly different properties like identity and ordering (as members of a quorum or pets).

_InfraKit_ provides primitives to manage Groups: a group has a given size and can shrink or grow based on some
specification, whether it's human generated or machine computed.
Group members can also be updated in a rolling fashion so that the configuration of the instance members reflect a new
desired state.  Operators can focus on Groups while _InfraKit_ handles the necessary coordination of Instances.

Since _InfraKit_ emphasizes on declarative infrastructure, there are no operations to move machines or Groups from one
state to another.  Instead, you _declare_ your desired state of the infrastructure.  _InfraKit_ is responsible
for converging towards, and maintaining, that desired state.

Therefore, a [group plugin](../../pkg/spi/group/spi.go) manages Groups of Instances and exposes the operations that are of
interest to a user:

  + commit a group configuration, to start managing a group
  + inspect a group
  + destroy a group

#### Default Group plugin
_InfraKit_ provides a default Group plugin implementation, intended to suit common use cases.  The default Group plugin
manages Instances of a specific Flavor.  Instance and Flavor plugins can be composed to manage different types of
services on different infrastructure providers.

While it's generally simplest to use the default Group plugin, custom implementations may be valuable to adapt another
infrastructure management system.  This would allow you to use _InfraKit_ tooling to perform basic operations on widely
different infrastructure using the same interface.

### Instance
Instances are members of a group. An [instance plugin](../../pkg/spi/instance/spi.go) manages some physical resource instances.
It knows only about individual instances and nothing about Groups.  Instance is technically defined by the plugin, and
need not be a physical machine at all.

For compute, for example, instances can be VM instances of identical spec. Instances
support the notions of attachment to auxiliary resources.  Instances may be tagged, and tags are assumed to be
persistent which allows the state of the cluster to be inferred and computed.

In some cases, instances can be identical, while in other cases the members of a group require stronger identities and
persistent, stable state. These properties are captured via the _flavors_ of the instances.

### Flavor
Flavors help distinguish members of one group from another by describing how these members should be treated.
A [flavor plugin](../../pkg/spi/flavor/spi.go) can be thought of as defining what runs on an Instance.
It is responsible for dictating commands to run services, and check the health of those services.

Flavors allow a group of instances to have different characteristics.  In a group of cattle,
all members are treated identically and individual members do not have strong identity.  In a group of pets,
however, the members may require special handling and demand stronger notions of identity and state.

### Creating a plugin
A plugin must be an HTTP server that implements one of the plugin [APIs](#apis), listening on a Unix socket.  While
a plugin can be written in any programming language, [utilities](../../pkg/rpc) are available as libraries to simplify Plugin
development in Go.  Our [reference implementations](../../examples/instance) should provide a good starting point
for building a new plugin using these utilities.

#### APIs
_InfraKit_ plugins are exposed via HTTP, using [JSON-RPC 2.0](http://www.jsonrpc.org/specification).

API requests can be made manually with `curl`.  For example, the following command will list all groups:
```console
$ curl -X POST --unix-socket ~/.infrakit/plugins/group http://rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"Group.InspectGroups","params":{},"id":1}'
{"jsonrpc":"2.0","result":{"Groups":null},"id":1}
```

API errors are surfaced with the `error` response field:
```console
$ curl -X POST --unix-socket ~/.infrakit/plugins/group http://rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"Group.CommitGroup","params":{},"id":1}'
{"jsonrpc":"2.0","error":{"code":-32000,"message":"Group ID must not be blank","data":null},"id":1}
```

Per the JSON-RPC format, each API call has a `method` and `params`.  The following pages define the available methods
for each plugin type:
- [Flavor](flavor.md)
- [Group](group.md)
- [Instance](instance.md)

See also: documentation on common API [types](types.md).

Additionally, all plugins will log each API HTTP request and response when run with the `--log 5` command line argument.

##### API identification
Plugins are required to identify the name and version of plugin APIs they implement.  This is done with a request
like the following:

```console
$ curl -X POST --unix-socket ~/.infrakit/plugins/group http://rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"Plugin.Implements","params":{},"id":1}'
{"jsonrpc":"2.0","result":{"Interfaces":[{"Name":"Group","Version":"0.1.0"}]},"id":1}
```
