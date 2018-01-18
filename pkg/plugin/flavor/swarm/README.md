InfraKit Flavor Plugin - Swarm
==============================

A [reference](/README.md#reference-implementations) implementation of a Flavor Plugin that creates a Docker
cluster in [Swarm Mode](https://docs.docker.com/engine/swarm/).

## Schema & Templates

This plugin has a schema that looks like this:
```json
{
   "InitScriptTemplateURL": "http://your.github.io/your/project/swarm/worker-init.sh",
   "SwarmJoinIP": "192.168.2.200",
   "Docker" : {
     "Host" : "tcp://192.168.2.200:4243"
   },
   "EngineLabels": {
     "storage": "ssd",
     "data": ""
   }
 }
```
Note that the Docker connection information, as well as what IP in the Swarm the managers and workers should use
to join the swarm, are now part of the plugin configuration.

This plugin makes heavy use of Golang template to enable customization of instance behavior on startup.  For example,
the `InitScriptTemplateURL` field above is a URL where a init script template is served.  The plugin will fetch this
template from the URL and processes the template to render the final init script for the instance.

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

mkdir -p /etc/docker
cat << EOF > /etc/docker/daemon.json
{
  "labels": {{ INFRAKIT_LABELS | jsonEncode }}
}
EOF

{{/* Reload the engine labels */}}
kill -s HUP $(cat /var/run/docker.pid)
sleep 5

{{ if eq INSTANCE_LOGICAL_ID SPEC.SwarmJoinIP }}

  {{/* The first node of the special allocations will initialize the swarm. */}}
  docker swarm init --advertise-addr {{ INSTANCE_LOGICAL_ID }}

  # Tell Docker to listen on port 4243 for remote API access. This is optional.
  echo DOCKER_OPTS="\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\"" >> /etc/default/docker

  # Restart Docker to let port listening take effect.
  service docker restart

{{ else }}

  {{/* The rest of the nodes will join as followers in the manager group. */}}
  docker swarm join --token {{ SWARM_JOIN_TOKENS.Manager }} {{ SPEC.SwarmJoinIP }}:2377

{{ end }}
```

There are tags such as `{{ SWARM_JOIN_TOKENS.Manager }}` or `{{ INSTANCE_LOGICAL_ID }}`: these are made available by the
plugin and they are evaluated / interpolated during the `Prepare` phase of the plugin.  The plugin will substitute
these 'placeholders' with actual values.  The templating engine also supports inclusion of other templates / files, as
seen in the `{{ include "install-docker.sh" }}` tag above.  This makes it easy to embed actual shell scripts, and other
texts, without painful and complicated escapes to meet the JSON syntax requirements. For example, the 'include' tag
above will embed the `install-docker.sh` template/file:

```
# Tested on Ubuntu/trusty

apt-get update -y
apt-get upgrade -y
wget -qO- https://get.docker.com/ | sh

# Tell Docker to listen on port 4243 for remote API access. This is optional.
echo DOCKER_OPTS=\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\" >> /etc/default/docker

# Restart Docker to let port listening take effect.
service docker restart

```

### A Word on Security

Since Swarm Mode uses [join-tokens](https://docs.docker.com/engine/swarm/join-nodes/) to authorize nodes, initializing
the Swarm requires:

a. exposing the Docker remote API for the InfraKit plugin to access join tokens
b. running InfraKit on the manager nodes to access join tokens via the Docker socket

We recommend approach (b) for anything but demonstration purposes unless the Docker daemon socket is
[secured](https://docs.docker.com/engine/security/https/).  For simplicity, **this example does not secure
Docker socket**.


### Building & Running -- An Example

There are scripts in this directory to illustrate how to start up the InfraKit plugin ensemble and examples for creating
a Docker swarm via vagrant.

Building the binaries - do this from the top level project directory:
```shell
make binaries
```

Start required plugins.  We use the `infrakit plugin start` utility and a `plugins.json` to start up all the plugins,
along with the InfraKit manager:

```shell
~/projects/src/github.com/docker/infrakit$ examples/flavor/swarm/start-plugins.sh
INFO[11-03|14:34:31] config                                   module=cli/plugin url=file:////Users/me/go/src/github.com/docker/infrakit/pkg/plugin/flavor/swarm/plugins.json fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-03|14:34:31] Launching                                module=cli/plugin kind=manager name=group fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-03|14:34:31] Launching                                module=cli/plugin kind=group name=group-stateless fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-03|14:34:31] Starting plugin                          module=core/launch executor=inproc key=manager name=group exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-03|14:34:31] Decoded input                            module=run/manager config="{Options:{Name: Group:group-stateless Metadata:vars Plugins:<nil> Leader:<nil> LeaderStore:<nil> SpecStore:<nil> MetadataStore:<nil> MetadataRefreshInterval:5000000000 LeaderCommitSpecsRetries:10 LeaderCommitSpecsRetryInterval:2000000000} Backend:file Settings:{\n\"PollInterval\": \"5s\",\n\"LeaderFile\": \"/Users/me/.infrakit/leader\",\n\"StoreDir\": \"/Users/me/.infrakit/configs\",\n\"ID\": \"manager1\"\n} Mux:0xc4204ca0a0 cleanUpFunc:<nil>}" fn=github.com/docker/infrakit/pkg/run/v0/manager.Run
INFO[11-03|14:34:31] Starting up                              module=run/manager backend=file fn=github.com/docker/infrakit/pkg/run/v0/manager.Run
INFO[11-03|14:34:31] starting up file backend                 module=run/manager options="{PollInterval:5000000000 LeaderFile:/Users/me/.infrakit/leader StoreDir:/Users/me/.infrakit/configs ID:manager1}" fn=github.com/docker/infrakit/pkg/run/v0/manager.Run
INFO[11-03|14:34:31] file backend                             module=run/manager leader="&{pollInterval:5000000000 tick:0xc4203561e0 stop:<nil> pollFunc:0x1689b90 store:<nil> lock:{state:0 sema:0} receivers:[]}" store="&{dir:/Users/me/.infrakit/configs name:global.config}" cleanup=<nil> fn=github.com/docker/infrakit/pkg/run/v0/manager.Run
INFO[11-03|14:34:31] Start manager                            module=run/manager m="&{Options:{Name:group Group:group-stateless Metadata:vars Plugins:0x16d71a0 Leader:0xc42008a190 LeaderStore:/Users/me/.infrakit/leader.loc SpecStore:0xc4204ca1e0 MetadataStore:0xc4204ca200 MetadataRefreshInterval:5000000000 LeaderCommitSpecsRetries:10 LeaderCommitSpecsRetryInterval:2000000000} Plugin:0xc42031a060 Updatable:0xc42031a0c0 isLeader:false lock:{state:0 sema:0} stop:<nil> running:<nil> Status:0xc4202ba680 doneStatusUpdates:0xc42007e240 backendOps:<nil>}" fn=github.com/docker/infrakit/pkg/run/v0/manager.Run
INFO[11-03|14:34:31] Manager starting                         module=manager fn=github.com/docker/infrakit/pkg/manager.(*manager).Start
INFO[11-03|14:34:31] Manager running                          module=run/manager fn=github.com/docker/infrakit/pkg/run/v0/manager.Run
INFO[11-03|14:34:31] Starting mux server                      module=run/manager listen=:24864 advertise=localhost:24864 fn=github.com/docker/infrakit/pkg/run/v0/manager.Run
INFO[11-03|14:34:31] written pid                              module=rpc/mux path=/Users/me/.infrakit/plugins/24864.pid fn=github.com/docker/infrakit/pkg/rpc/mux.SavePID
INFO[11-03|14:34:31] Listening                                module=rpc/mux listen=:24864 fn=github.com/docker/infrakit/pkg/rpc/mux.NewServer
INFO[11-03|14:34:31] exported objects                         module=run/manager fn=github.com/docker/infrakit/pkg/run/v0/manager.Run
INFO[11-03|14:34:31] Object is an event producer              module=rpc/server object=&{keyed:0xc42017e208} discover=/Users/me/.infrakit/plugins/group fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-03|14:34:31] Listening                                module=rpc/server discover=/Users/me/.infrakit/plugins/group fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-03|14:34:31] Waiting for startup                      module=core/launch key=manager name=group config="{\n          \"Kind\": \"manager\",\n          \"Options\": {\n            \"Name\": \"\",\n            \"Group\": \"group-stateless\"\n          },\n\t  \"Settings\": {\n\t\t  \"LeaderFile\": \"/Users/me/.infrakit/leader\",\n\t\t  \"StoreDir\": \"/Users/me/.infrakit/configs\"\n\t  },\n          \"Backend\": \"file\"\n        }" as=group fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-03|14:34:31] Starting plugin                          module=core/launch executor=inproc key=group name=group-stateless exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-03|14:34:31] Launching                                module=cli/plugin kind=flavor-swarm name=flavor-swarm fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-03|14:34:31] PID file created                         module=run path=/Users/me/.infrakit/plugins/group.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:34:31] Server started                           module=run discovery=/Users/me/.infrakit/plugins/group fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:34:31] Object is an event producer              module=rpc/server object=&{keyed:0xc42017e720} discover=/Users/me/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-03|14:34:31] Listening                                module=rpc/server discover=/Users/me/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-03|14:34:31] Waiting for startup                      module=core/launch key=group name=group-stateless config="{\n\"Kind\": \"group\",\n\"Options\": {\n\"PollInterval\": \"10s\",\n\"MaxParallelNum\": 0,\n\"PollIntervalGroupSpec\": \"10s\",\n\"PollIntervalGroupDetail\": \"10s\"\n}\n}" as=group-stateless fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-03|14:34:31] Starting plugin                          module=core/launch executor=inproc key=flavor-swarm name=flavor-swarm exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-03|14:34:31] Launching                                module=cli/plugin kind=instance-vagrant name=instance-vagrant fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-03|14:34:31] PID file created                         module=run path=/Users/me/.infrakit/plugins/group-stateless.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:34:31] Server started                           module=run discovery=/Users/me/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:34:31] Listening                                module=rpc/server discover=/Users/me/.infrakit/plugins/flavor-swarm fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-03|14:34:31] Waiting for startup                      module=core/launch key=flavor-swarm name=flavor-swarm config="{\n          \"Kind\": \"swarm\"\n        }" as=flavor-swarm fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-03|14:34:31] Starting plugin                          module=core/launch executor=inproc key=instance-vagrant name=instance-vagrant exec=inproc fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-03|14:34:31] PID file created                         module=run path=/Users/me/.infrakit/plugins/flavor-swarm.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:34:31] Server started                           module=run discovery=/Users/me/.infrakit/plugins/flavor-swarm fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:34:31] Listening                                module=rpc/server discover=/Users/me/.infrakit/plugins/instance-vagrant fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath
INFO[11-03|14:34:31] Waiting for startup                      module=core/launch key=instance-vagrant name=instance-vagrant config="{\n          \"Kind\": \"vagrant\",\n          \"Options\": {\n          }\n        }" as=instance-vagrant fn=github.com/docker/infrakit/pkg/launch.(*Monitor).Start.func1
INFO[11-03|14:34:31] Done waiting on plugin starts            module=cli/plugin fn=github.com/docker/infrakit/cmd/infrakit/plugin.Command.func2
INFO[11-03|14:34:31] PID file created                         module=run path=/Users/me/.infrakit/plugins/instance-vagrant.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:34:31] Server started                           module=run discovery=/Users/me/.infrakit/plugins/instance-vagrant fn=github.com/docker/infrakit/pkg/run.run.func1
Plugins started.
Do something like: infrakit manager commit file:///Users/me/projects/src/github.com/docker/infrakit/examples/flavor/swarm/groups-fast.json


```

Now start up the cluster comprised of a manager and a worker group.  In this case, see `groups-fast.json` where we will create
a manager group of 3 nodes and a worker group of 3 nodes. The topology in this is a single ensemble of infrakit running on
your local machine that manages 6 vagrant vms running Docker in Swarm Mode.  The `groups-fast.json` is named fast because
we are using a Vagrant box (image) that already has Docker installed.  A slower version, that uses just `ubuntu/trusty64` and
a full Docker install, can be found in `groups.json`.

```shell
~/projects/src/github.com/docker/infrakit$ infrakit manager commit file:///Users/davidchung/projects/src/github.com/docker/infrakit/examples/flavor/swarm/groups-fast.json
INFO[0000] Found manager group is leader =  true        
INFO[0000] Found manager as group at /Users/davidchung/.infrakit/plugins/group
INFO[0000] Using file:///Users/davidchung/projects/src/github.com/docker/infrakit/examples/flavor/swarm/groups-fast.json for reading template

Group swarm-workers with plugin group plan: Managing 3 instances
Group swarm-managers with plugin group plan: Managing 3 instances
```

Now it will take some time for the entire cluster to come up.  During this, you may want to see the Vagrant and Swarm plugins
in action.  If you look at the `plugins.json` you will see that the plugins are started with `stdout` and `stderr` being
directed to a `{{env "INFRAKIT_HOME"}}/logs` directory.  The `$INFRAKIT_HOME` environment variable is set by the `start-plugins.sh`
script and is set to `~/.infrakit`.

So, to look at the logs, just do this in another terminal:

```
tail -f ~/.infrakit/logs/*.log
```

You will see the sequential interactions of the plugins.  Here's an example:

```shell

==> group-stateless.log <==
time="2017-01-31T16:38:22-08:00" level=debug msg="Received response HTTP/1.1 200 OK\r\nContent-Length: 1288\r\nContent-Type: text/plain; charset=utf-8\r\nDate: Wed, 01 Feb 2017 00:38:22 GMT\r\n\r\n{\"jsonrpc\":\"2.0\",\"result\":{\"Type\":\"manager\",\"Spec\":{\"Properties\":{\"Box\":\"ubuntu/trusty64\"},\"Tags\":{\"infrakit-link\":\"HmjVIS7jEMNMu3mU\",\"infrakit-link-context\":\"swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\",\"infrakit.config.hash\":\"UBS6BBMA84vUjjtMceeraQGP_eQ=\",\"infrakit.group\":\"swarm-managers\",\"infrakit.cluster.id\":\"t9vg5zqmdtw8ovhniwifszwbw\"},\"Init\":\"#!/bin/sh\\nset -o errexit\\nset -o nounset\\nset -o xtrace\\n\\n\\n\\n# Tested on Ubuntu/trusty\\n\\napt-get update -y\\napt-get upgrade -y\\nwget -qO- https://get.docker.com/ | sh\\n\\n# Tell Docker to listen on port 4243 for remote API access. This is optional.\\necho DOCKER_OPTS=\\\\\\\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\\\\\\\" \\u003e\\u003e /etc/default/docker\\n\\n# Restart Docker to let port listening take effect.\\nservice docker restart\\n\\n\\nmkdir -p /etc/docker\\ncat \\u003c\\u003c EOF \\u003e /etc/docker/daemon.json\\n{\\n  \\\"labels\\\": [\\n  \\\"infrakit-link=HmjVIS7jEMNMu3mU\\\",\\n  \\\"infrakit-link-context=swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\\\"\\n]\\n}\\nEOF\\n\\n\\nkill -s HUP $(cat /var/run/docker.pid)\\nsleep 5\\n\\n\\n\\n  \\n  \\n\\n  \\n  docker swarm join 192.168.2.200:4243 --token SWMTKN-1-69316t20viiyh4bwwg8ae2rx6p6obqojfnnxbwfo4dbn3b0npx-68n9c4khokso84f3unn53c15h\\n\\n\\n\\n\\n\",\"LogicalID\":\"192.168.2.202\",\"Attachments\":null}},\"id\":204406773822125757}\n"
time="2017-01-31T16:38:22-08:00" level=debug msg="Received response HTTP/1.1 200 OK\r\nContent-Length: 1289\r\nContent-Type: text/plain; charset=utf-8\r\nDate: Wed, 01 Feb 2017 00:38:22 GMT\r\n\r\n{\"jsonrpc\":\"2.0\",\"result\":{\"Type\":\"manager\",\"Spec\":{\"Properties\":{\"Box\":\"ubuntu/trusty64\"},\"Tags\":{\"infrakit-link\":\"gOII3YTODwN55QNZ\",\"infrakit-link-context\":\"swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\",\"infrakit.config.hash\":\"UBS6BBMA84vUjjtMceeraQGP_eQ=\",\"infrakit.group\":\"swarm-managers\",\"infrakit.cluster.id\":\"t9vg5zqmdtw8ovhniwifszwbw\"},\"Init\":\"#!/bin/sh\\nset -o errexit\\nset -o nounset\\nset -o xtrace\\n\\n\\n\\n# Tested on Ubuntu/trusty\\n\\napt-get update -y\\napt-get upgrade -y\\nwget -qO- https://get.docker.com/ | sh\\n\\n# Tell Docker to listen on port 4243 for remote API access. This is optional.\\necho DOCKER_OPTS=\\\\\\\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\\\\\\\" \\u003e\\u003e /etc/default/docker\\n\\n# Restart Docker to let port listening take effect.\\nservice docker restart\\n\\n\\nmkdir -p /etc/docker\\ncat \\u003c\\u003c EOF \\u003e /etc/docker/daemon.json\\n{\\n  \\\"labels\\\": [\\n  \\\"infrakit-link=gOII3YTODwN55QNZ\\\",\\n  \\\"infrakit-link-context=swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\\\"\\n]\\n}\\nEOF\\n\\n\\nkill -s HUP $(cat /var/run/docker.pid)\\nsleep 5\\n\\n\\n\\n  \\n  \\n\\n  \\n  docker swarm join 192.168.2.200:4243 --token SWMTKN-1-69316t20viiyh4bwwg8ae2rx6p6obqojfnnxbwfo4dbn3b0npx-68n9c4khokso84f3unn53c15h\\n\\n\\n\\n\\n\",\"LogicalID\":\"192.168.2.201\",\"Attachments\":null}},\"id\":5501896949660804411}\n"
time="2017-01-31T16:38:22-08:00" level=debug msg="Sending request POST / HTTP/1.1\r\nHost: a\r\nContent-Type: application/json\r\n\r\n{\"jsonrpc\":\"2.0\",\"method\":\"Instance.Provision\",\"params\":{\"Type\":\"\",\"Spec\":{\"Properties\":{\"Box\":\"ubuntu/trusty64\"},\"Tags\":{\"infrakit-link\":\"HmjVIS7jEMNMu3mU\",\"infrakit-link-context\":\"swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\",\"infrakit.config.hash\":\"UBS6BBMA84vUjjtMceeraQGP_eQ=\",\"infrakit.group\":\"swarm-managers\",\"infrakit.cluster.id\":\"t9vg5zqmdtw8ovhniwifszwbw\"},\"Init\":\"#!/bin/sh\\nset -o errexit\\nset -o nounset\\nset -o xtrace\\n\\n\\n\\n# Tested on Ubuntu/trusty\\n\\napt-get update -y\\napt-get upgrade -y\\nwget -qO- https://get.docker.com/ | sh\\n\\n# Tell Docker to listen on port 4243 for remote API access. This is optional.\\necho DOCKER_OPTS=\\\\\\\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\\\\\\\" \\u003e\\u003e /etc/default/docker\\n\\n# Restart Docker to let port listening take effect.\\nservice docker restart\\n\\n\\nmkdir -p /etc/docker\\ncat \\u003c\\u003c EOF \\u003e /etc/docker/daemon.json\\n{\\n  \\\"labels\\\": [\\n  \\\"infrakit-link=HmjVIS7jEMNMu3mU\\\",\\n  \\\"infrakit-link-context=swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\\\"\\n]\\n}\\nEOF\\n\\n\\nkill -s HUP $(cat /var/run/docker.pid)\\nsleep 5\\n\\n\\n\\n  \\n  \\n\\n  \\n  docker swarm join 192.168.2.200:4243 --token SWMTKN-1-69316t20viiyh4bwwg8ae2rx6p6obqojfnnxbwfo4dbn3b0npx-68n9c4khokso84f3unn53c15h\\n\\n\\n\\n\\n\",\"LogicalID\":\"192.168.2.202\",\"Attachments\":null}},\"id\":3237913983924951943}"
time="2017-01-31T16:38:22-08:00" level=debug msg="Sending request POST / HTTP/1.1\r\nHost: a\r\nContent-Type: application/json\r\n\r\n{\"jsonrpc\":\"2.0\",\"method\":\"Instance.Provision\",\"params\":{\"Type\":\"\",\"Spec\":{\"Properties\":{\"Box\":\"ubuntu/trusty64\"},\"Tags\":{\"infrakit-link\":\"gOII3YTODwN55QNZ\",\"infrakit-link-context\":\"swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\",\"infrakit.config.hash\":\"UBS6BBMA84vUjjtMceeraQGP_eQ=\",\"infrakit.group\":\"swarm-managers\",\"infrakit.cluster.id\":\"t9vg5zqmdtw8ovhniwifszwbw\"},\"Init\":\"#!/bin/sh\\nset -o errexit\\nset -o nounset\\nset -o xtrace\\n\\n\\n\\n# Tested on Ubuntu/trusty\\n\\napt-get update -y\\napt-get upgrade -y\\nwget -qO- https://get.docker.com/ | sh\\n\\n# Tell Docker to listen on port 4243 for remote API access. This is optional.\\necho DOCKER_OPTS=\\\\\\\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\\\\\\\" \\u003e\\u003e /etc/default/docker\\n\\n# Restart Docker to let port listening take effect.\\nservice docker restart\\n\\n\\nmkdir -p /etc/docker\\ncat \\u003c\\u003c EOF \\u003e /etc/docker/daemon.json\\n{\\n  \\\"labels\\\": [\\n  \\\"infrakit-link=gOII3YTODwN55QNZ\\\",\\n  \\\"infrakit-link-context=swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\\\"\\n]\\n}\\nEOF\\n\\n\\nkill -s HUP $(cat /var/run/docker.pid)\\nsleep 5\\n\\n\\n\\n  \\n  \\n\\n  \\n  docker swarm join 192.168.2.200:4243 --token SWMTKN-1-69316t20viiyh4bwwg8ae2rx6p6obqojfnnxbwfo4dbn3b0npx-68n9c4khokso84f3unn53c15h\\n\\n\\n\\n\\n\",\"LogicalID\":\"192.168.2.201\",\"Attachments\":null}},\"id\":823635831294852014}"

==> instance-vagrant.log <==
time="2017-01-31T16:38:22-08:00" level=debug msg="Received request POST / HTTP/1.1\r\nHost: a\r\nAccept-Encoding: gzip\r\nContent-Length: 1311\r\nContent-Type: application/json\r\nUser-Agent: Go-http-client/1.1\r\n\r\n{\"jsonrpc\":\"2.0\",\"method\":\"Instance.Provision\",\"params\":{\"Type\":\"\",\"Spec\":{\"Properties\":{\"Box\":\"ubuntu/trusty64\"},\"Tags\":{\"infrakit-link\":\"HmjVIS7jEMNMu3mU\",\"infrakit-link-context\":\"swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\",\"infrakit.config.hash\":\"UBS6BBMA84vUjjtMceeraQGP_eQ=\",\"infrakit.group\":\"swarm-managers\",\"infrakit.cluster.id\":\"t9vg5zqmdtw8ovhniwifszwbw\"},\"Init\":\"#!/bin/sh\\nset -o errexit\\nset -o nounset\\nset -o xtrace\\n\\n\\n\\n# Tested on Ubuntu/trusty\\n\\napt-get update -y\\napt-get upgrade -y\\nwget -qO- https://get.docker.com/ | sh\\n\\n# Tell Docker to listen on port 4243 for remote API access. This is optional.\\necho DOCKER_OPTS=\\\\\\\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\\\\\\\" \\u003e\\u003e /etc/default/docker\\n\\n# Restart Docker to let port listening take effect.\\nservice docker restart\\n\\n\\nmkdir -p /etc/docker\\ncat \\u003c\\u003c EOF \\u003e /etc/docker/daemon.json\\n{\\n  \\\"labels\\\": [\\n  \\\"infrakit-link=HmjVIS7jEMNMu3mU\\\",\\n  \\\"infrakit-link-context=swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\\\"\\n]\\n}\\nEOF\\n\\n\\nkill -s HUP $(cat /var/run/docker.pid)\\nsleep 5\\n\\n\\n\\n  \\n  \\n\\n  \\n  docker swarm join 192.168.2.200:4243 --token SWMTKN-1-69316t20viiyh4bwwg8ae2rx6p6obqojfnnxbwfo4dbn3b0npx-68n9c4khokso84f3unn53c15h\\n\\n\\n\\n\\n\",\"LogicalID\":\"192.168.2.202\",\"Attachments\":null}},\"id\":3237913983924951943}"
time="2017-01-31T16:38:22-08:00" level=debug msg="Received request POST / HTTP/1.1\r\nHost: a\r\nAccept-Encoding: gzip\r\nContent-Length: 1310\r\nContent-Type: application/json\r\nUser-Agent: Go-http-client/1.1\r\n\r\n{\"jsonrpc\":\"2.0\",\"method\":\"Instance.Provision\",\"params\":{\"Type\":\"\",\"Spec\":{\"Properties\":{\"Box\":\"ubuntu/trusty64\"},\"Tags\":{\"infrakit-link\":\"gOII3YTODwN55QNZ\",\"infrakit-link-context\":\"swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\",\"infrakit.config.hash\":\"UBS6BBMA84vUjjtMceeraQGP_eQ=\",\"infrakit.group\":\"swarm-managers\",\"infrakit.cluster.id\":\"t9vg5zqmdtw8ovhniwifszwbw\"},\"Init\":\"#!/bin/sh\\nset -o errexit\\nset -o nounset\\nset -o xtrace\\n\\n\\n\\n# Tested on Ubuntu/trusty\\n\\napt-get update -y\\napt-get upgrade -y\\nwget -qO- https://get.docker.com/ | sh\\n\\n# Tell Docker to listen on port 4243 for remote API access. This is optional.\\necho DOCKER_OPTS=\\\\\\\"-H tcp://0.0.0.0:4243 -H unix:///var/run/docker.sock\\\\\\\" \\u003e\\u003e /etc/default/docker\\n\\n# Restart Docker to let port listening take effect.\\nservice docker restart\\n\\n\\nmkdir -p /etc/docker\\ncat \\u003c\\u003c EOF \\u003e /etc/docker/daemon.json\\n{\\n  \\\"labels\\\": [\\n  \\\"infrakit-link=gOII3YTODwN55QNZ\\\",\\n  \\\"infrakit-link-context=swarm/t9vg5zqmdtw8ovhniwifszwbw/manager\\\"\\n]\\n}\\nEOF\\n\\n\\nkill -s HUP $(cat /var/run/docker.pid)\\nsleep 5\\n\\n\\n\\n  \\n  \\n\\n  \\n  docker swarm join 192.168.2.200:4243 --token SWMTKN-1-69316t20viiyh4bwwg8ae2rx6p6obqojfnnxbwfo4dbn3b0npx-68n9c4khokso84f3unn53c15h\\n\\n\\n\\n\\n\",\"LogicalID\":\"192.168.2.201\",\"Attachments\":null}},\"id\":823635831294852014}"
time="2017-01-31T16:38:25-08:00" level=info msg="Vagrant STDOUT: Bringing machine 'default' up with 'virtualbox' provider...\n"
time="2017-01-31T16:38:25-08:00" level=info msg="Vagrant STDOUT: Bringing machine 'default' up with 'virtualbox' provider...\n"
time="2017-01-31T16:38:25-08:00" level=info msg="Vagrant STDOUT: ==> default: Importing base box 'ubuntu/trusty64'...\n"
time="2017-01-31T16:38:25-08:00" level=info msg="Vagrant STDOUT: ==> default: Importing base box 'ubuntu/trusty64'...\n"
time="2017-01-31T16:38:40-08:00" level=info msg="Vagrant STDOUT: \r\x1b[KProgress: 90%\r\x1b[K==> default: Matching MAC address for NAT networking...\n"
time="2017-01-31T16:38:40-08:00" level=info msg="Vagrant STDOUT: \r\x1b[KProgress: 90%\r\x1b[K==> default: Matching MAC address for NAT networking...\n"
time="2017-01-31T16:38:40-08:00" level=info msg="Vagrant STDOUT: ==> default: Checking if box 'ubuntu/trusty64' is up to date...\n"
time="2017-01-31T16:38:40-08:00" level=info msg="Vagrant STDOUT: ==> default: Checking if box 'ubuntu/trusty64' is up to date...\n"

```

That's a lot of log entries!  You can always lower the volume by adjusting the `--log` parameters in the `plugins.json`
for the next time.

After some time, you should be able to see the Swarm cluster up and running by doing:

```shell
vagrant global-status
```

or conveniently in a separate window,

```shell
watch -d vagrant global-status  # using watch to monitor the vagrant vms
```

Now check the swarm:

```shell
~/projects/src/github.com/docker/infrakit$ docker -H tcp://192.168.2.200:4243 node ls
ID                           HOSTNAME   STATUS  AVAILABILITY  MANAGER STATUS
1ag3s3m1avdahg19933io5xjr *  infrakit   Ready   Active        Leader
3pbow8fnfwf0y5d8pvibpx7or    localhost  Ready   Active        
45x97w5jipsm9xonprgj5393y    localhost  Ready   Active        Reachable
6v91xdon6e6lommqne9eeb776    localhost  Ready   Active        
c4z4a4h3p2jz5vwg36zy2rww9    localhost  Ready   Active        
ezkfjfjqmphi90daup2ur1yas    localhost  Ready   Active        Reachable
```

Or use Infrakit `group describe` to see the instances:
```shell
~/projects/src/github.com/docker/infrakit$ infrakit group describe swarm-managers
ID                            	LOGICAL                       	TAGS
infrakit-240440289            	192.168.2.201                 	infrakit-link-context=swarm/37phvxcyelv8js1lyqi76hnau/manager,infrakit-link=nbix0txooYIoyiUQ,infrakit.config.hash=mt39WMxI1MX4mFIg03moQjy4OjA=,infrakit.group=swarm-managers,infrakit.cluster.id=37phvxcyelv8js1lyqi76hnau
infrakit-428836874            	192.168.2.202                 	infrakit-link-context=swarm/37phvxcyelv8js1lyqi76hnau/manager,infrakit-link=dmVwcQ7w49aTj6K7,infrakit.config.hash=mt39WMxI1MX4mFIg03moQjy4OjA=,infrakit.group=swarm-managers,infrakit.cluster.id=37phvxcyelv8js1lyqi76hnau
infrakit-541061527            	192.168.2.200                 	infrakit-link-context=swarm/?/manager,infrakit-link=wz5NEiBoC02DtO4q,infrakit.config.hash=mt39WMxI1MX4mFIg03moQjy4OjA=,infrakit.group=swarm-managers,infrakit.cluster.id=?
~/projects/src/github.com/docker/infrakit$ infrakit group describe swarm-workers
ID                            	LOGICAL                       	TAGS
infrakit-177710302            	  -                           	infrakit-link-context=swarm/37phvxcyelv8js1lyqi76hnau/worker,infrakit-link=Q9Pu1jBVSbwMF8mz,infrakit.config.hash=TbwYlrX9Efh6_wIrLKG6B7Zd24s=,infrakit.group=swarm-workers,infrakit.cluster.id=
infrakit-266730661            	  -                           	infrakit-link-context=swarm/37phvxcyelv8js1lyqi76hnau/worker,infrakit-link=BQe9jpmy5cx24XVd,infrakit.config.hash=TbwYlrX9Efh6_wIrLKG6B7Zd24s=,infrakit.group=swarm-workers,infrakit.cluster.id=
infrakit-782909388            	  -                           	infrakit-link-context=swarm/37phvxcyelv8js1lyqi76hnau/worker,infrakit-link=jjAgxzSlQrpEVLo3,infrakit.config.hash=TbwYlrX9Efh6_wIrLKG6B7Zd24s=,infrakit.group=swarm-workers,infrakit.cluster.id=
~/projects/src/github.com/docker/infrakit$
```

We can clean up vms after this brief demo:

```shell
~/projects/src/github.com/docker/infrakit$ infrakit group destroy swarm-workers
destroy swarm-workers initiated
~/projects/src/github.com/docker/infrakit$ infrakit group destroy swarm-managers
destroy swarm-managers initiated
```

And stop all the plugins:

```
~/projects/src/github.com/docker/infrakit$ infrakit plugin stop --all
INFO[11-03|14:41:17] Stopping                                 module=run/manager name=group-stateless pid=7201 fn=github.com/docker/infrakit/pkg/run/manager.(*Manager).Terminate
INFO[11-03|14:41:17] Process exited                           module=run/manager name=group-stateless fn=github.com/docker/infrakit/pkg/run/manager.(*Manager).Terminate
INFO[11-03|14:41:17] Stopping                                 module=run/manager name=instance-vagrant pid=7201 fn=github.com/docker/infrakit/pkg/run/manager.(*Manager).Terminate
INFO[11-03|14:41:17] Process exited                           module=run/manager name=instance-vagrant fn=github.com/docker/infrakit/pkg/run/manager.(*Manager).Terminate
INFO[11-03|14:41:17] listener stopped                         module=rpc/mux fn=github.com/docker/infrakit/pkg/rpc/mux.NewServer.func2.1
INFO[11-03|14:41:17] Stopping                                 module=run/manager name=flavor-swarm pid=7201 fn=github.com/docker/infrakit/pkg/run/manager.(*Manager).Terminate
INFO[11-03|14:41:17] Process exited                           module=run/manager name=flavor-swarm fn=github.com/docker/infrakit/pkg/run/manager.(*Manager).Terminate
INFO[11-03|14:41:17] Stopping                                 module=run/manager name=group pid=7201 fn=github.com/docker/infrakit/pkg/run/manager.(*Manager).Terminate
INFO[11-03|14:41:17] Stop checking leadership                 module=rpc/mux fn=github.com/docker/infrakit/pkg/rpc/mux.NewServer.func1
INFO[11-03|14:41:17] Stopping event relay                     module=rpc/server fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath.func1
INFO[11-03|14:41:17] Stopping event relay                     module=rpc/server fn=github.com/docker/infrakit/pkg/rpc/server.startAtPath.func1
INFO[11-03|14:41:17] Process exited                           module=run/manager name=group fn=github.com/docker/infrakit/pkg/run/manager.(*Manager).Terminate
INFO[11-03|14:41:17] Removed PID file                         module=run path=/Users/me/.infrakit/plugins/instance-vagrant.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:41:17] Removed discover file                    module=run path=/Users/me/.infrakit/plugins/instance-vagrant fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:41:17] Removed PID file                         module=run path=/Users/me/.infrakit/plugins/group-stateless.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:41:17] Removed discover file                    module=run path=/Users/me/.infrakit/plugins/group-stateless fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:41:17] Removed PID file                         module=run path=/Users/me/.infrakit/plugins/group.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:41:17] Removed PID file                         module=run path=/Users/me/.infrakit/plugins/flavor-swarm.pid fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:41:17] Removed discover file                    module=run path=/Users/me/.infrakit/plugins/flavor-swarm fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:41:17] Snapshot updater stopped                 module=run/group fn=github.com/docker/infrakit/pkg/run/v0/group.Run.func3
INFO[11-03|14:41:17] Snapshot updater stopped                 module=flavor/swarm fn=github.com/docker/infrakit/pkg/plugin/flavor/swarm.(*baseFlavor).runMetadataSnapshot.func1
INFO[11-03|14:41:17] Removed discover file                    module=run path=/Users/me/.infrakit/plugins/group fn=github.com/docker/infrakit/pkg/run.run.func1
INFO[11-03|14:41:17] Snapshot updater stopped                 module=flavor/swarm fn=github.com/docker/infrakit/pkg/plugin/flavor/swarm.(*baseFlavor).runMetadataSnapshot.func1
```
