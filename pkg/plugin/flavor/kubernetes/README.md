InfraKit Flavor Plugin - Kubernetes
==============================

A [reference](/README.md#reference-implementations) implementation of a Flavor Plugin that creates a [Kubernetes](https://kubernetes.io/) cluster.

## Schema & Templates

This plugin has a schema that looks like this:

For manager
```json
{
    "InitScriptTemplateURL": "file:///home/ubuntu/go/src/github.com/docker/infrakit/examples/flavor/kubernetes/manager-init.sh",
    "KubeJoinIP": "192.168.2.200",
    "KubeBindPort": 6443,
    "KubeAddOns": [
        {
            "Name" : "flannel",
            "Type" : "network",
            "Path" : ""
        }
    ]

}
```
For workers
```json
{
    "InitScriptTemplateURL": "file:///home/ubuntu/go/src/github.com/docker/infrakit/examples/flavor/kubernetes/worker-init.sh",
    "KubeJoinIP": "192.168.2.200",
    "KubeBindPort": 6443,
}
 
```

Note `KubeJoinIP`, `KubeBindPort` that the Kubernetes connection information, as well as what IP in the Kubernetes managers and workers should use
to advertise and join.

`KubeAddOns` is list of [kubernetes addons](https://kubernetes.io/docs/concepts/cluster-administration/addons/). 
You can set Type as network or visualise.
`network` Type addon should be set as your cluster will not be Ready status until network addon is applyed.

This plugin makes heavy use of Golang template to enable customization of instance behavior on startup.  For example,
the `InitScriptTemplateURL` field above is a URL where a init script template is served.  The plugin will fetch this
template from the URL and processe the template to render the final init script for the instance.

The plugin exposes a set of template functions that can be used, along with primitives already in [Golang template]
(https://golang.org/pkg/text/template/) and functions from [Sprig](https://github.com/Masterminds/sprig#functions).
This makes it possible to have complex templates for generating the user data / init script of the instances.

For example, this is a template for the init script of a manager node:

```
#!/bin/sh
set -o errexit
set -o nounset
set -o xtrace

{{/* Install Docker */}}
{{ include "install-docker.sh" }}

{{/* Install Kubeadm */}}
{{ include "install_kubeadam.sh" }}
kubeadm init --token {{ KUBEADM_JOIN_TOKEN }}
export KUBECONFIG=/etc/kubernetes/admin.conf
{{ if NETWORK_ADDON }}
    kubectl apply -f {{ NETWORK_ADDON }}
{{ else }}
{{ end }}
```

There are tags such as `{{ KUBEADM_JOIN_TOKEN }}` or `{{ INSTANCE_LOGICAL_ID }}`: these are made available by the
plugin and they are evaluated / interpolated during the `Prepare` phase of the plugin.  The plugin will substitute
these 'placeholders' with actual values.  The templating engine also supports inclusion of other templates / files, as
seen in the `{{ include "install-docker.sh" }}` tag above.  This makes it easy to embed actual shell scripts, and other
texts, without painful and complicated escapes to meet the JSON syntax requirements. For example, the 'include' tag
above will embed the `install-docker.sh` template/file:

```
# Tested on Ubuntu/trusty

apt-get update -y
wget -qO- https://get.docker.com/ | sh

```

### A Word on Security

Since Kubeadm use Token to authorize nodes, initializing
Kubernetes requires:

Docker engine exposes its remote API, but it is protected by TLS. Infrakit intends to make access to kubernetes manager from the side, but we can not send commands such as `get nodes` yet.
For installation, we use [kubeadm](https://kubernetes.io/docs/admin/kubeadm/) and build a secure cluster.


### Building & Running -- An Example

There are scripts in this directory to illustrate how to start up the InfraKit plugin ensemble and examples for creating a kubernetes via vagrant.

Building the binaries - do this from the top level project directory:
```shell
make binaries
```

Start required plugins.
We can use the plugin utility to start up all the plugins along with the InfraKit manager:

```shell
export INFRAKIT_LEADER_FILE=$HOME/.infrakit/leader
echo "manager1" > $INFRAKIT_LEADER_FILE
export PATH=$PWD/build:$PATH
infrakit plugin start manager group vagrant kubernetes &
```

Now start up the cluster comprised of a manager and a worker group.  In this case, see `groups-master.json` where we will create a manager group of one node and in `group-worker.json` create a worker group of 3 nodes. The topology in this is a single ensemble of infrakit running on your local machine that manages 4 vagrant vms running Kubernetes.  
At Kubernetes flavor, you should run manager group first.
Worker group will try to connect to manager before start.
And as this flavor is based on kubeadm, it currently supports only one manager node.

```shell
infrakit group commit groups-manager.json
```
Wait for manager to come up.
As it needs to install docker and kubeadm, it takes a little time...

```shell
infrakit group commit groups-worker.json
```

Now cluster will come up.
Now check the kubernetes:
You should log in to manager node.
Then

```shell
export KUBECONFIG=/etc/kubernetes/admin.conf
kubectl get nodes
NAME               STATUS    AGE       VERSION
ip-192.168.2.200   Ready     4m        v1.6.3
ip-192.168.2.2     Ready     2m        v1.6.3
ip-192.168.2.3     Ready     2m        v1.6.3
ip-192.168.2.4     Ready     2m        v1.6.3
```
