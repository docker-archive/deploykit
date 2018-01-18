init
====

The `init` command takes a spec (`groups.json`) and starts
the flavor plugin and generates the init script of the boot
node as though it's provisioned by the group and instance plugins.
This is used primarily as a way too bootstrap the first node of a
cluster.

The `init` command also implicitly starts a `vars` plugin for storing
your varaibles so that through successive evaluations of templates, your
configuration information is always accessible.

This example here uses the `vars.json`, `groups.json`, `common.ikt`, and `init.sh`
in this directory.

### Operation

To render the init script from the groups specification is complex and involves
multiple steps.  This is because bootstrapping from a single node requires a mix
of user-provided data, as well as values that are available *after* some additional
future step (e.g. getting a cluster master to provide a join token -- which is not
available until the cluster is bootstrapping itself).  So while the configuration specs
and scripts look like a set of point-in-time configurations, multiple evaluations
of these as templates are performed as more data become available.  The sequence
of `init` works as follows:

  1. The user provides the spec's URL (the `groups.json` URL)
  2. The CLI fetches this and evaluates it as a template.
    + The engine uses the `INFRAKIT_VARS_TEMPLATE` env to determine a variables JSON to
    initialize some variables.  These can be overridden by appropriately named `--var`
    and `--metadata` flags in the command line.
    + The engine uses the `--var` to set parameters in memory for the scope of *this*
    evaluation.  These are ephemeral and can override the defaults set in above.
    + The engine uses the `--metadata` to set parameters that will persist, as some
    kind of cluster-state.  These values are persistent and are written to the backend
    based on your configuration (eg. swarm, etcd, or file).
  3. The CLI parses the spec, and locates the section for the group as specified by
  the `group-id` flag.
  4. The CLI now starts the plugins specified in the spec.  For example, the spec
  here references the `swarm/managers` plugin, so the CLI starts up the `swarm` plugin.
  5. The CLI invokes the flavor plugin's `Prepare` method.
  6. The Flavor plugin performs the necessary work such as generating tokens, etc. and
  renders a template.  Depending on the plugin implementation, values that are not known
  at this time may be deferred (as multipass = true -- see `swarm/flavor.go#31).
  7. The CLI renders the text blob from the `Init` field of the instance spec returned
  by the `Prepare` method.  This will apply the same set of `--var` as variables.
  8. The CLI prints out the rendered init script
  9. If `--persist` is set, the CLI will commit the current state of the vars plugin
  (which was started in the beginning of this process).

** This is pretty complicated but the details here are presented for documentation.
The end user only knows there are certain variables to set to bootstrap a cluster,
and the values are set via the `--var` and `--metadata` flags (e.g. the size of the cluster).

To run

```shell
$ INFRAKIT_VARS_TEMPLATE=file://$(pwd)/vars.json infrakit util init --group-id managers groups.json --var vars/config/root=file://$(pwd)
#!/bin/bash

echo "This is the init script"
echo "The config root is file:///Users/davidchung/project5/src/github.com/docker/infrakit/docs/cmd/infrakit/util/init"
echo "The cluster size is 5"
```

The output will be the shell script that the first node of the group `managers` should
execute.  You can pipe this to `sh` and the node that runs this script will become
the first node of the cluster.  Note that there are other requirements on the first node
in terms of its configuration and labels (must have a label `infrakit.config.hash=bootstrap`)
such that it's not possible to run this on your mac and expect it to be the first node of
a cluster in AWS.  This is meant to be for bootstrapping in the cloud environment where the
cluster will be.

Note that we have the `init.sh` like so:

```
#!/bin/bash

echo "This is the init script"
echo "The config root is {{ var `vars/config/root` }}"
echo "The cluster size is {{ var `vars/cluster/size` }}"
```

### Persist values via `--metadata`

We don't always want every parameter used for templates to be stored as cluster state,
but there are times, some parameters need to persist from node to node and through time.
For those parameters, use `--metadata` followed by a `--persist` to ensure data is persisted
into the the backend that's configured.

```shell
$ INFRAKIT_VARS_TEMPLATE=file://$(pwd)/vars.json infrakit util init --group-id managers groups.json --metadata vars/config/root=file://$(pwd) --persist
```

This yields the same:

```
#!/bin/bash

echo "This is the init script"
echo "The config root is file:///Users/davidchung/project5/src/github.com/docker/infrakit/docs/cmd/infrakit/util/init"
echo "The cluster size is 5"
```

However, the data has been snapshoted and persisted -- in this case as a file:

```shell
$ export INFRAKIT_HOME=~/.infrakit
$ cat $INFRAKIT_HOME/configs/vars.vars
{
    "cluster": {
      "name": "test",
      "size": 5,
      "swarm": {
        "joinIP": "10.20.100.101",
        "managerIPs": [
          "10.20.100.101",
          "10.20.100.102",
          "10.20.100.103"
        ]
      },
      "user": {
        "name": "user"
      }
    },
    "config": {
      "root": "file:///Users/davidchung/project5/src/github.com/docker/infrakit/docs/cmd/infrakit/util/init"
    },
    "shell": "/bin/bash",
    "zones": {
      "east": {
        "cidr": "10.20.100.100/24"
      },
      "west": {
        "cidr": "10.20.100.200/24"
      }
    }
  }
```

Next time if `manager` is started it will automatically load this state.  In case
of failover, the manager on another node will load this as well as the cluster spec
as part of failing over.  So we have the cluster specs as well as user-provided
data fully available.

### `--var` or `--metadata`?

This is a matter of taste and requirements, as they serve different purposes.
If a parameter is only meant to be temporary, `--var` will avoid cluttering up
the data store.  However,
`--metadata` is easier to reason about across time and location because that data
is highly available.  For simplicity, `--metadata` may be the way to start off developing
your custom templates.
