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
in terms of its configuration and labels (must have a label `infrakit.config_sha=bootstrap`)
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
