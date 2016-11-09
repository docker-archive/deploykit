InfraKit Manager
================

The Manager is a binary that offers a Group interface while providing the following:

  + Leadership detection - for coordinating multiple sets (replicas) of InfraKit plugins
  + State storage - persists user configuration in some backend

Both file-based and Docker Swarm (Swarm Mode) based leadership detection and state storage are
available.

## Group Interface

Currently the manager exposes the same Group plugin interface as the `infrakit-group-default`.
This means `infrakit group ...` command will work as usual.  The manager expects a group plugin
to be running prior to starting up and it functions as proxy for that group plugin:

  + When user does a `infrakit group watch` or `infrakit group update`, the manager will
  persist the input configuration in the data store it was configured at startup time.
  + If the data store is configured with a backend that is shared or replicated across multiple
  instances of InfraKit ensemble (all the collaborating plugins), high availability can be
  achieved via leader detection and global availabilit of state (the stored config).
  + Multiple replicas of the manager can do leader detection so that only one is active.  As
  soon as leadership changes, the responsibility of maintaining infrastructure state is transfered
  to the new manager that became active.

## Leadership

The manager can use either `os` or `swarm` for leadership detection:

### OS mode (via the `os` subcommand)

  1. Assumes multiple instances of managers can access a shared file (e.g. over NFS or FUSE on S3).
  2. Each manager starts up with a name (the `--name` flag).
  3. The manager instance with the name that matches the content of the shared file is the leader.

### Swarm mode (via the `swarm` subcommand)

  1. Assumes there's a manager instance per Docker Swarm manager instance
  2. Leadership depends on the status of the Swarm manager node.  If the Swarm manager node is the
  leader, then the InfraKit manager instance running on that node is the leader.
  3. When leadership changes in the Swarm, InfraKit leadership follows.

When an instance assumes leadership:

  + State is retrieved from shared storage (see below) and for each group in the config, a group
  `watch` is invoked so that the new leader can begin watching the groups
  + Since this is the frontend for the stateless group, it records any input the user provides when the
  user performs and update.  The new config is then written in the shared store and `update` is forwarded
  to the actual group plugin to do the real work.

When an instance loses leadership:

  + The manager uses previous configuration and 'deactivates' the local group plugin by calling `unwatch`
  on the downstream group plugin
  + It rejects user's attempt to `update` since it's not the leader.


## State Storage

The manager can use either `os` or `swarm` for state storage:

### OS mode (via the `os` subcommand)

  1. State is stored in a local file that is well-known and defined at startup of the manager.
  2. This file is a global config that can include multiple groups.

### Swarm mode (via the `swarm` subcommand)

  1. State is stored in the Swarm via annotations
  2. A single global state is stored in a single annotation.  The data is compressed and encoded.


## Fronted (Proxy) for Group

The manager requires a group plugin to be running so that it can forward calls to it to actually
perform the work of watching and updating:

  + When you intend to use the manager, you should start your default group plugin with a name like
  `group-stateless`
  + Then when starting the manager, set the `--proxy-for-group` flag to the name of the group plugin
  (e.g. `group-stateless`).  By default, the manager starts up with the name of `group`.  This matches
  the default name that the CLI (`infrakit group ...`) uses.


## Running

```shell
$ infrakit-manager -h
Manager

Usage:
  infrakit-manager [command]

Available Commands:
  os          os
  swarm       swarm mode for leader detection and storage
  version     print build version information

Flags:
      --log int                  Logging level. 0 is least verbose. Max is 5 (default 4)
      --name string              Name of the manager (default "group")
      --proxy-for-group string   Name of the group plugin to proxy for. (default "group-stateless")

Use "infrakit-manager [command] --help" for more information about a command.
```

### Running in OS Mode

Useful for local testing:

```shell
$ infrakit-manager os --log 5
```

### Running in Swarm Mode

First enable Swarm mode:

```shell
docker swarm init
```

On each Swarm manager node:

```shell
$ infrakit-manager swarm --log 5
```
will connect to Docker using defaulted Docker socket.
